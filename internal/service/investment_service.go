package service

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cashvio/cashvio-be/internal/config"
	"github.com/cashvio/cashvio-be/internal/model"
	"github.com/cashvio/cashvio-be/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type InvestmentService interface {
	CreateInvestment(ctx context.Context, userID uuid.UUID, req *model.CreateInvestmentRequest) (*model.Investment, error)
	GetInvestmentByID(ctx context.Context, id, userID uuid.UUID) (*model.Investment, error)
	GetAllInvestments(ctx context.Context, q *model.InvestmentQuery, userID uuid.UUID) ([]*model.Investment, int, error)
	GetPrices(ctx context.Context, userID uuid.UUID) ([]*model.InvestmentPrice, error)
	UpdateInvestment(ctx context.Context, id, userID uuid.UUID, req *model.UpdateInvestmentRequest) (*model.Investment, error)
	DeleteInvestment(ctx context.Context, id, userID uuid.UUID) error
}

type investmentService struct {
	repo       repository.InvestmentRepository
	txnRepo    repository.TransactionRepository
	db         *pgxpool.Pool
	walletRepo repository.WalletRepository
	cardRepo   repository.CardRepository
	cashRepo   repository.CashRepository
	stockRepo  repository.StockPriceRepository
	cfg        *config.Config
	httpClient *http.Client
}

func NewInvestmentService(repo repository.InvestmentRepository, txnRepo repository.TransactionRepository, db *pgxpool.Pool, cfg *config.Config, stockRepo repository.StockPriceRepository, walletRepo repository.WalletRepository, cardRepo repository.CardRepository, cashRepo repository.CashRepository) InvestmentService {
	return &investmentService{
		repo: repo, txnRepo: txnRepo, db: db,
		walletRepo: walletRepo, cardRepo: cardRepo, cashRepo: cashRepo,
		stockRepo: stockRepo, cfg: cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func ParseInvestmentQuery(q *model.InvestmentQuery, limit, offset, search, sortBy, order, t string) *model.InvestmentQuery {
	q.Search = search
	q.SortBy = sortBy
	q.Order = order
	q.Type = t

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

func (s *investmentService) CreateInvestment(ctx context.Context, userID uuid.UUID, req *model.CreateInvestmentRequest) (*model.Investment, error) {
	if err := s.validateAccount(ctx, req.AccountType, req.AccountID, userID); err != nil {
		return nil, err
	}

	date, err := parseTransactionDate(req.Date)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	amountInvested := req.Units * req.BuyPrice
	transactionID := uuid.New()
	inv := &model.Investment{
		ID:            uuid.New(),
		UserID:        userID,
		Type:          req.Type,
		Name:          strings.TrimSpace(req.Name),
		Ticker:        strings.ToUpper(strings.TrimSpace(req.Ticker)),
		App:           strings.TrimSpace(req.App),
		AccountType:   req.AccountType,
		AccountID:     req.AccountID,
		Units:         req.Units,
		BuyPrice:      req.BuyPrice,
		Date:          date,
		TransactionID: transactionID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	txn := &model.Transaction{
		ID:          transactionID,
		UserID:      userID,
		Name:        inv.Name,
		Amount:      -amountInvested,
		Type:        "expense",
		Category:    "investment",
		Status:      "completed",
		AccountType: req.AccountType,
		AccountID:   req.AccountID,
		Date:        date,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, errors.New("failed to begin transaction")
	}
	defer tx.Rollback(ctx)

	if err := s.repo.CreateTx(ctx, tx, inv); err != nil {
		return nil, errors.New("failed to create investment")
	}

	if err := s.txnRepo.CreateTx(ctx, tx, txn); err != nil {
		return nil, errors.New("failed to create linked transaction")
	}

	if err := s.txnRepo.AdjustBalanceTx(ctx, tx, txn.AccountType, txn.AccountID, userID, txn.Amount); err != nil {
		return nil, errors.New("failed to update account balance")
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, errors.New("failed to commit investment")
	}

	return inv, nil
}

func (s *investmentService) GetInvestmentByID(ctx context.Context, id, userID uuid.UUID) (*model.Investment, error) {
	inv, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("investment not found")
		}
		return nil, errors.New("failed to get investment")
	}
	return inv, nil
}

func (s *investmentService) GetAllInvestments(ctx context.Context, q *model.InvestmentQuery, userID uuid.UUID) ([]*model.Investment, int, error) {
	investments, err := s.repo.GetByUserID(ctx, q, userID)
	if err != nil {
		return nil, 0, errors.New("failed to retrieve investments")
	}
	total, err := s.repo.CountByUserID(ctx, q, userID)
	if err != nil {
		return nil, 0, errors.New("failed to count investments")
	}
	return investments, total, nil
}

func (s *investmentService) UpdateInvestment(ctx context.Context, id, userID uuid.UUID, req *model.UpdateInvestmentRequest) (*model.Investment, error) {
	old, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("investment not found")
		}
		return nil, errors.New("failed to get investment")
	}

	updated := *old
	updated.UpdatedAt = time.Now()

	if req.Type != "" {
		updated.Type = req.Type
	}
	if req.Name != "" {
		updated.Name = strings.TrimSpace(req.Name)
	}
	if req.Ticker != "" {
		updated.Ticker = strings.ToUpper(strings.TrimSpace(req.Ticker))
	}
	if req.App != "" {
		updated.App = strings.TrimSpace(req.App)
	}
	if req.AccountType != "" {
		updated.AccountType = req.AccountType
	}
	if req.AccountID != nil {
		updated.AccountID = *req.AccountID
	}
	if req.Units > 0 {
		updated.Units = req.Units
	}
	if req.BuyPrice > 0 {
		updated.BuyPrice = req.BuyPrice
	}
	if req.Date != "" {
		date, err := parseTransactionDate(req.Date)
		if err != nil {
			return nil, err
		}
		updated.Date = date
	}

	accountChanged := updated.AccountType != old.AccountType || updated.AccountID != old.AccountID
	if accountChanged {
		if err := s.validateAccount(ctx, updated.AccountType, updated.AccountID, userID); err != nil {
			return nil, err
		}
	}

	oldAmount := -old.Units * old.BuyPrice
	newAmount := -updated.Units * updated.BuyPrice

	txn := &model.Transaction{
		ID:          old.TransactionID,
		UserID:      userID,
		Name:        updated.Name,
		Amount:      newAmount,
		Type:        "expense",
		Category:    "investment",
		Status:      "completed",
		AccountType: updated.AccountType,
		AccountID:   updated.AccountID,
		Date:        updated.Date,
		UpdatedAt:   updated.UpdatedAt,
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, errors.New("failed to begin transaction")
	}
	defer tx.Rollback(ctx)

	if err := s.repo.UpdateTx(ctx, tx, &updated); err != nil {
		return nil, errors.New("failed to update investment")
	}

	if err := s.txnRepo.UpdateTx(ctx, tx, txn); err != nil {
		return nil, errors.New("failed to update linked transaction")
	}

	if accountChanged {
		if err := s.txnRepo.AdjustBalanceTx(ctx, tx, old.AccountType, old.AccountID, userID, -oldAmount); err != nil {
			return nil, errors.New("failed to update account balance")
		}
		if err := s.txnRepo.AdjustBalanceTx(ctx, tx, updated.AccountType, updated.AccountID, userID, newAmount); err != nil {
			return nil, errors.New("failed to update account balance")
		}
	} else if newAmount != oldAmount {
		if err := s.txnRepo.AdjustBalanceTx(ctx, tx, updated.AccountType, updated.AccountID, userID, newAmount-oldAmount); err != nil {
			return nil, errors.New("failed to update account balance")
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, errors.New("failed to commit investment")
	}

	return &updated, nil
}

func (s *investmentService) DeleteInvestment(ctx context.Context, id, userID uuid.UUID) error {
	old, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("investment not found")
		}
		return errors.New("failed to get investment")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return errors.New("failed to begin transaction")
	}
	defer tx.Rollback(ctx)

	if err := s.repo.DeleteTx(ctx, tx, id, userID); err != nil {
		return errors.New("failed to delete investment")
	}

	if err := s.txnRepo.DeleteTx(ctx, tx, old.TransactionID, userID); err != nil {
		return errors.New("failed to delete linked transaction")
	}

	if err := s.txnRepo.AdjustBalanceTx(ctx, tx, old.AccountType, old.AccountID, userID, old.Units*old.BuyPrice); err != nil {
		return errors.New("failed to update account balance")
	}

	if err := tx.Commit(ctx); err != nil {
		return errors.New("failed to commit investment deletion")
	}

	return nil
}

func (s *investmentService) validateAccount(ctx context.Context, accountType string, accountID, userID uuid.UUID) error {
	switch accountType {
	case "wallet":
		if _, err := s.walletRepo.GetByID(ctx, accountID, userID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errors.New("wallet not found")
			}
			return errors.New("failed to validate wallet")
		}
	case "card":
		if _, err := s.cardRepo.GetByID(ctx, accountID, userID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errors.New("card not found")
			}
			return errors.New("failed to validate card")
		}
	case "cash":
		cash, err := s.cashRepo.GetByUserID(ctx, userID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errors.New("cash balance not found")
			}
			return errors.New("failed to validate cash balance")
		}
		if cash.ID != accountID {
			return errors.New("cash account not found")
		}
	default:
		return errors.New("invalid account type")
	}
	return nil
}
