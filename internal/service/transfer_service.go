package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/cashvio/cashvio-be/internal/model"
	"github.com/cashvio/cashvio-be/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransferService interface {
	CreateTransfer(ctx context.Context, userID uuid.UUID, req *model.CreateTransferRequest) (*model.Transfer, error)
	GetTransferByID(ctx context.Context, id, userID uuid.UUID) (*model.Transfer, error)
	GetAllTransfers(ctx context.Context, q *model.TransferQuery, userID uuid.UUID) ([]*model.Transfer, int, error)
	DeleteTransfer(ctx context.Context, id, userID uuid.UUID) error
}

type transferService struct {
	repo       repository.TransferRepository
	db         *pgxpool.Pool
	walletRepo repository.WalletRepository
	cardRepo   repository.CardRepository
}

func NewTransferService(repo repository.TransferRepository, db *pgxpool.Pool, walletRepo repository.WalletRepository, cardRepo repository.CardRepository) TransferService {
	return &transferService{repo: repo, db: db, walletRepo: walletRepo, cardRepo: cardRepo}
}

func ParseTransferQuery(q *model.TransferQuery, limit, offset, search, sortBy, order string) *model.TransferQuery {
	q.Search = search
	q.SortBy = sortBy
	q.Order = order

	if l, err := strconv.Atoi(limit); err == nil {
		if l <= 0 {
			l = 10
		}
		if l > 100 {
			l = 100
		}
		q.Limit = l
	}

	if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
		q.Offset = o
	}

	if q.SortBy == "" {
		q.SortBy = "date"
	}
	if q.Order == "" {
		q.Order = "desc"
	}

	return q
}

func (s *transferService) CreateTransfer(ctx context.Context, userID uuid.UUID, req *model.CreateTransferRequest) (*model.Transfer, error) {
	source, err := s.getAccount(ctx, req.FromType, req.FromID, userID)
	if err != nil {
		return nil, err
	}

	if _, err := s.getAccount(ctx, req.ToType, req.ToID, userID); err != nil {
		return nil, err
	}

	if req.FromType == req.ToType && req.FromID == req.ToID {
		return nil, errors.New("source and destination must be different accounts")
	}

	if req.Fee < 0 {
		return nil, errors.New("fee cannot be negative")
	}

	if req.Fee >= req.Amount {
		return nil, errors.New("fee must be less than transfer amount")
	}

	if source.BalanceIDR < req.Amount+req.Fee {
		return nil, errors.New("insufficient balance in source account")
	}

	date, err := parseTransactionDate(req.Date)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	transfer := &model.Transfer{
		ID:        uuid.New(),
		UserID:    userID,
		FromType:  req.FromType,
		FromID:    req.FromID,
		ToType:    req.ToType,
		ToID:      req.ToID,
		Amount:    req.Amount,
		Fee:       req.Fee,
		Note:      strings.TrimSpace(req.Note),
		Date:      date,
		CreatedAt: now,
		UpdatedAt: now,
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, errors.New("failed to begin transaction")
	}
	defer tx.Rollback(ctx)

	if err := s.repo.CreateTx(ctx, tx, transfer); err != nil {
		return nil, errors.New("failed to create transfer")
	}

	if err := s.repo.AdjustBalanceTx(ctx, tx, transfer.FromType, transfer.FromID, userID, -(transfer.Amount + transfer.Fee)); err != nil {
		return nil, errors.New("failed to update source account balance")
	}

	if err := s.repo.AdjustBalanceTx(ctx, tx, transfer.ToType, transfer.ToID, userID, transfer.Amount); err != nil {
		return nil, errors.New("failed to update destination account balance")
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, errors.New("failed to commit transfer")
	}

	return transfer, nil
}

func (s *transferService) GetTransferByID(ctx context.Context, id, userID uuid.UUID) (*model.Transfer, error) {
	transfer, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("transfer not found")
		}
		return nil, errors.New("failed to get transfer")
	}
	return transfer, nil
}

func (s *transferService) GetAllTransfers(ctx context.Context, q *model.TransferQuery, userID uuid.UUID) ([]*model.Transfer, int, error) {
	transfers, err := s.repo.GetByUserID(ctx, q, userID)
	if err != nil {
		return nil, 0, errors.New("failed to retrieve transfers")
	}
	total, err := s.repo.CountByUserID(ctx, q, userID)
	if err != nil {
		return nil, 0, errors.New("failed to count transfers")
	}
	return transfers, total, nil
}

func (s *transferService) DeleteTransfer(ctx context.Context, id, userID uuid.UUID) error {
	old, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("transfer not found")
		}
		return errors.New("failed to get transfer")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return errors.New("failed to begin transaction")
	}
	defer tx.Rollback(ctx)

	if err := s.repo.DeleteTx(ctx, tx, id, userID); err != nil {
		return errors.New("failed to delete transfer")
	}

	if err := s.repo.AdjustBalanceTx(ctx, tx, old.FromType, old.FromID, userID, old.Amount+old.Fee); err != nil {
		return errors.New("failed to restore source account balance")
	}

	if err := s.repo.AdjustBalanceTx(ctx, tx, old.ToType, old.ToID, userID, -old.Amount); err != nil {
		return errors.New("failed to restore destination account balance")
	}

	if err := tx.Commit(ctx); err != nil {
		return errors.New("failed to commit deletion")
	}

	return nil
}

type accountBalance struct {
	BalanceIDR float64
}

func (s *transferService) getAccount(ctx context.Context, accountType string, accountID, userID uuid.UUID) (*accountBalance, error) {
	switch accountType {
	case "wallet":
		wallet, err := s.walletRepo.GetByID(ctx, accountID, userID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, errors.New("wallet not found")
			}
			return nil, errors.New("failed to validate wallet")
		}
		return &accountBalance{BalanceIDR: wallet.BalanceIDR}, nil
	case "card":
		card, err := s.cardRepo.GetByID(ctx, accountID, userID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, errors.New("card not found")
			}
			return nil, errors.New("failed to validate card")
		}
		return &accountBalance{BalanceIDR: card.BalanceIDR}, nil
	default:
		return nil, errors.New("invalid account type")
	}
}
