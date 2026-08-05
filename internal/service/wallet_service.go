package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/cashvio/cashvio-be/internal/model"
	"github.com/cashvio/cashvio-be/internal/repository"
	"github.com/cashvio/cashvio-be/internal/util"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type WalletService interface {
	CreateWallet(ctx context.Context, userID uuid.UUID, req *model.CreateWalletRequest) (*model.Wallet, error)
	GetWalletByID(ctx context.Context, id, userID uuid.UUID) (*model.Wallet, error)
	GetAllWallets(ctx context.Context, q *model.WalletQuery, userID uuid.UUID) ([]*model.Wallet, error)
	UpdateWallet(ctx context.Context, id, userID uuid.UUID, req *model.UpdateWalletRequest) (*model.Wallet, error)
	DeleteWallet(ctx context.Context, id, userID uuid.UUID) error
}

type walletService struct {
	repo repository.WalletRepository
}

func NewWalletService(repo repository.WalletRepository) WalletService {
	return &walletService{repo: repo}
}

func ParseWalletQuery(q *model.WalletQuery, limit, offset, search, sortBy, order string) *model.WalletQuery {
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
		q.SortBy = "created_at"
	}
	if q.Order == "" {
		q.Order = "desc"
	}

	return q
}

func (s *walletService) CreateWallet(ctx context.Context, userID uuid.UUID, req *model.CreateWalletRequest) (*model.Wallet, error) {
	now := time.Now()
	wallet := &model.Wallet{
		ID:         uuid.New(),
		UserID:     userID,
		Name:       req.Name,
		Number:     strings.Join(strings.Fields(req.Number), ""),
		BalanceIDR: req.BalanceIDR,
		Tone:       req.Tone,
		Status:     "active",
		Primary:    req.Primary,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	wallet.Masked = util.MaskWalletNumber(wallet.Number)

	if err := s.repo.Create(ctx, wallet); err != nil {
		return nil, errors.New("failed to create wallet")
	}

	return wallet, nil
}

func (s *walletService) GetWalletByID(ctx context.Context, id, userID uuid.UUID) (*model.Wallet, error) {
	wallet, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("wallet not found")
		}
		return nil, errors.New("failed to get wallet")
	}
	return wallet, nil
}

func (s *walletService) GetAllWallets(ctx context.Context, q *model.WalletQuery, userID uuid.UUID) ([]*model.Wallet, error) {
	wallets, err := s.repo.GetByUserID(ctx, q, userID)
	if err != nil {
		return nil, errors.New("failed to retrieve wallets")
	}
	return wallets, nil
}

func (s *walletService) UpdateWallet(ctx context.Context, id, userID uuid.UUID, req *model.UpdateWalletRequest) (*model.Wallet, error) {
	wallet, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("wallet not found")
		}
		return nil, errors.New("failed to get wallet")
	}

	if req.Name != "" {
		wallet.Name = req.Name
	}
	if req.Number != "" {
		wallet.Number = strings.Join(strings.Fields(req.Number), "")
		wallet.Masked = util.MaskWalletNumber(wallet.Number)
	}
	if req.BalanceIDR >= 0 {
		wallet.BalanceIDR = req.BalanceIDR
	}
	if req.Tone != "" {
		wallet.Tone = req.Tone
	}
	if req.Status != "" {
		wallet.Status = req.Status
	}
	if req.Primary != nil {
		wallet.Primary = *req.Primary
	}

	if err := s.repo.Update(ctx, wallet); err != nil {
		return nil, errors.New("failed to update wallet")
	}

	return wallet, nil
}

func (s *walletService) DeleteWallet(ctx context.Context, id, userID uuid.UUID) error {
	if _, err := s.repo.GetByID(ctx, id, userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("wallet not found")
		}
		return errors.New("failed to get wallet")
	}

	if err := s.repo.Delete(ctx, id, userID); err != nil {
		return errors.New("failed to delete wallet")
	}
	return nil
}
