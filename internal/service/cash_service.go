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

type CashService interface {
	GetCash(ctx context.Context, userID uuid.UUID) (*model.Cash, error)
	CreateWithdrawal(ctx context.Context, userID uuid.UUID, req *model.CreateCashWithdrawalRequest) (*model.CashWithdrawal, error)
	GetWithdrawalByID(ctx context.Context, id, userID uuid.UUID) (*model.CashWithdrawal, error)
	GetAllWithdrawals(ctx context.Context, q *model.CashWithdrawalQuery, userID uuid.UUID) ([]*model.CashWithdrawal, int, error)
	DeleteWithdrawal(ctx context.Context, id, userID uuid.UUID) error
}

type cashService struct {
	repo       repository.CashRepository
	db         *pgxpool.Pool
	walletRepo repository.WalletRepository
	cardRepo   repository.CardRepository
}

func NewCashService(repo repository.CashRepository, db *pgxpool.Pool, walletRepo repository.WalletRepository, cardRepo repository.CardRepository) CashService {
	return &cashService{repo: repo, db: db, walletRepo: walletRepo, cardRepo: cardRepo}
}

func ParseCashWithdrawalQuery(q *model.CashWithdrawalQuery, limit, offset, search, sortBy, order string) *model.CashWithdrawalQuery {
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

func (s *cashService) GetCash(ctx context.Context, userID uuid.UUID) (*model.Cash, error) {
	cash, err := s.repo.GetByUserID(ctx, userID)
	if err == nil {
		return cash, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("failed to get cash balance")
	}

	now := time.Now()
	cash = &model.Cash{
		ID:         uuid.New(),
		UserID:     userID,
		BalanceIDR: 0,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.repo.Create(ctx, cash); err != nil {
		return nil, errors.New("failed to create cash balance")
	}
	return cash, nil
}

func (s *cashService) CreateWithdrawal(ctx context.Context, userID uuid.UUID, req *model.CreateCashWithdrawalRequest) (*model.CashWithdrawal, error) {
	source, err := s.getAccount(ctx, req.FromType, req.FromID, userID)
	if err != nil {
		return nil, err
	}

	if req.Fee < 0 {
		return nil, errors.New("fee cannot be negative")
	}

	if req.Fee >= req.Amount {
		return nil, errors.New("fee must be less than withdrawal amount")
	}

	if source.BalanceIDR < req.Amount+req.Fee {
		return nil, errors.New("insufficient balance in source account")
	}

	date, err := parseTransactionDate(req.Date)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	withdrawal := &model.CashWithdrawal{
		ID:        uuid.New(),
		UserID:    userID,
		FromType:  req.FromType,
		FromID:    req.FromID,
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

	if _, err := s.repo.GetByUserID(ctx, userID); errors.Is(err, pgx.ErrNoRows) {
		cash := &model.Cash{
			ID:         uuid.New(),
			UserID:     userID,
			BalanceIDR: 0,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if err := s.repo.Create(ctx, cash); err != nil {
			return nil, errors.New("failed to create cash balance")
		}
	} else if err != nil {
		return nil, errors.New("failed to get cash balance")
	}

	if err := s.repo.CreateWithdrawalTx(ctx, tx, withdrawal); err != nil {
		return nil, errors.New("failed to create withdrawal")
	}

	if err := s.repo.AdjustSourceBalanceTx(ctx, tx, withdrawal.FromType, withdrawal.FromID, userID, -(withdrawal.Amount + withdrawal.Fee)); err != nil {
		return nil, errors.New("failed to update source account balance")
	}

	if err := s.repo.AdjustBalanceTx(ctx, tx, userID, withdrawal.Amount); err != nil {
		return nil, errors.New("failed to update cash balance")
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, errors.New("failed to commit withdrawal")
	}

	return withdrawal, nil
}

func (s *cashService) GetWithdrawalByID(ctx context.Context, id, userID uuid.UUID) (*model.CashWithdrawal, error) {
	withdrawal, err := s.repo.GetWithdrawalByID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("withdrawal not found")
		}
		return nil, errors.New("failed to get withdrawal")
	}
	return withdrawal, nil
}

func (s *cashService) GetAllWithdrawals(ctx context.Context, q *model.CashWithdrawalQuery, userID uuid.UUID) ([]*model.CashWithdrawal, int, error) {
	withdrawals, err := s.repo.GetWithdrawalsByUserID(ctx, q, userID)
	if err != nil {
		return nil, 0, errors.New("failed to retrieve withdrawals")
	}
	total, err := s.repo.CountWithdrawalsByUserID(ctx, q, userID)
	if err != nil {
		return nil, 0, errors.New("failed to count withdrawals")
	}
	return withdrawals, total, nil
}

func (s *cashService) DeleteWithdrawal(ctx context.Context, id, userID uuid.UUID) error {
	old, err := s.repo.GetWithdrawalByID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("withdrawal not found")
		}
		return errors.New("failed to get withdrawal")
	}

	cash, err := s.repo.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("cash balance not found")
		}
		return errors.New("failed to get cash balance")
	}
	if cash.BalanceIDR < old.Amount {
		return errors.New("insufficient cash balance")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return errors.New("failed to begin transaction")
	}
	defer tx.Rollback(ctx)

	if err := s.repo.DeleteWithdrawalTx(ctx, tx, id, userID); err != nil {
		return errors.New("failed to delete withdrawal")
	}

	if err := s.repo.AdjustSourceBalanceTx(ctx, tx, old.FromType, old.FromID, userID, old.Amount+old.Fee); err != nil {
		return errors.New("failed to restore source account balance")
	}

	if err := s.repo.AdjustBalanceTx(ctx, tx, userID, -old.Amount); err != nil {
		return errors.New("failed to update cash balance")
	}

	if err := tx.Commit(ctx); err != nil {
		return errors.New("failed to commit deletion")
	}

	return nil
}

type cashAccountBalance struct {
	BalanceIDR float64
}

func (s *cashService) getAccount(ctx context.Context, accountType string, accountID, userID uuid.UUID) (*cashAccountBalance, error) {
	switch accountType {
	case "wallet":
		wallet, err := s.walletRepo.GetByID(ctx, accountID, userID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, errors.New("wallet not found")
			}
			return nil, errors.New("failed to validate wallet")
		}
		return &cashAccountBalance{BalanceIDR: wallet.BalanceIDR}, nil
	case "card":
		card, err := s.cardRepo.GetByID(ctx, accountID, userID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, errors.New("card not found")
			}
			return nil, errors.New("failed to validate card")
		}
		return &cashAccountBalance{BalanceIDR: card.BalanceIDR}, nil
	default:
		return nil, errors.New("invalid account type")
	}
}
