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

type TransactionService interface {
	CreateTransaction(ctx context.Context, userID uuid.UUID, req *model.CreateTransactionRequest) (*model.Transaction, error)
	GetTransactionByID(ctx context.Context, id, userID uuid.UUID) (*model.Transaction, error)
	GetAllTransactions(ctx context.Context, q *model.TransactionQuery, userID uuid.UUID) ([]*model.Transaction, int, error)
	ExportTransactions(ctx context.Context, q *model.TransactionQuery, userID uuid.UUID) ([]*model.Transaction, error)
	UpdateTransaction(ctx context.Context, id, userID uuid.UUID, req *model.UpdateTransactionRequest) (*model.Transaction, error)
	DeleteTransaction(ctx context.Context, id, userID uuid.UUID) error
}

type transactionService struct {
	repo       repository.TransactionRepository
	db         *pgxpool.Pool
	walletRepo repository.WalletRepository
	cardRepo   repository.CardRepository
	cashRepo   repository.CashRepository
}

func NewTransactionService(repo repository.TransactionRepository, db *pgxpool.Pool, walletRepo repository.WalletRepository, cardRepo repository.CardRepository, cashRepo repository.CashRepository) TransactionService {
	return &transactionService{repo: repo, db: db, walletRepo: walletRepo, cardRepo: cardRepo, cashRepo: cashRepo}
}

func ParseTransactionQuery(q *model.TransactionQuery, limit, offset, search, sortBy, order, t, category, status string) *model.TransactionQuery {
	q.Search = search
	q.SortBy = sortBy
	q.Order = order
	q.Type = t
	q.Category = category
	q.Status = status

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

func (s *transactionService) CreateTransaction(ctx context.Context, userID uuid.UUID, req *model.CreateTransactionRequest) (*model.Transaction, error) {
	if err := s.validateAccount(ctx, req.AccountType, req.AccountID, userID); err != nil {
		return nil, err
	}

	date, err := parseTransactionDate(req.Date)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	txn := &model.Transaction{
		ID:          uuid.New(),
		UserID:      userID,
		Name:        strings.TrimSpace(req.Name),
		Amount:      signedAmount(req.Type, req.Amount),
		Type:        req.Type,
		Category:    req.Category,
		Status:      defaultStatus(req.Status),
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

	if err := s.repo.CreateTx(ctx, tx, txn); err != nil {
		return nil, errors.New("failed to create transaction")
	}

	if err := s.repo.AdjustBalanceTx(ctx, tx, txn.AccountType, txn.AccountID, userID, txn.Amount); err != nil {
		return nil, errors.New("failed to update account balance")
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, errors.New("failed to commit transaction")
	}

	return txn, nil
}

func (s *transactionService) GetTransactionByID(ctx context.Context, id, userID uuid.UUID) (*model.Transaction, error) {
	txn, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("transaction not found")
		}
		return nil, errors.New("failed to get transaction")
	}
	return txn, nil
}

func (s *transactionService) GetAllTransactions(ctx context.Context, q *model.TransactionQuery, userID uuid.UUID) ([]*model.Transaction, int, error) {
	transactions, err := s.repo.GetByUserID(ctx, q, userID)
	if err != nil {
		return nil, 0, errors.New("failed to retrieve transactions")
	}
	total, err := s.repo.CountByUserID(ctx, q, userID)
	if err != nil {
		return nil, 0, errors.New("failed to count transactions")
	}
	return transactions, total, nil
}

func (s *transactionService) ExportTransactions(ctx context.Context, q *model.TransactionQuery, userID uuid.UUID) ([]*model.Transaction, error) {
	transactions, err := s.repo.GetAllByUserID(ctx, q, userID)
	if err != nil {
		return nil, errors.New("failed to retrieve transactions")
	}
	return transactions, nil
}

func (s *transactionService) UpdateTransaction(ctx context.Context, id, userID uuid.UUID, req *model.UpdateTransactionRequest) (*model.Transaction, error) {
	old, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("transaction not found")
		}
		return nil, errors.New("failed to get transaction")
	}

	updated := *old
	updated.UpdatedAt = time.Now()

	if req.Name != "" {
		updated.Name = strings.TrimSpace(req.Name)
	}
	if req.Amount > 0 {
		updated.Amount = signedAmount(updated.Type, req.Amount)
	}
	if req.Type != "" {
		updated.Type = req.Type
		updated.Amount = signedAmount(updated.Type, absAmount(updated.Amount))
	}
	if req.Category != "" {
		updated.Category = req.Category
	}
	if req.Status != "" {
		updated.Status = req.Status
	}
	if req.AccountType != "" {
		updated.AccountType = req.AccountType
	}
	if req.AccountID != nil {
		updated.AccountID = *req.AccountID
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

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, errors.New("failed to begin transaction")
	}
	defer tx.Rollback(ctx)

	if err := s.repo.UpdateTx(ctx, tx, &updated); err != nil {
		return nil, errors.New("failed to update transaction")
	}

	if accountChanged {
		if err := s.repo.AdjustBalanceTx(ctx, tx, old.AccountType, old.AccountID, userID, -old.Amount); err != nil {
			return nil, errors.New("failed to update account balance")
		}
		if err := s.repo.AdjustBalanceTx(ctx, tx, updated.AccountType, updated.AccountID, userID, updated.Amount); err != nil {
			return nil, errors.New("failed to update account balance")
		}
	} else if updated.Amount != old.Amount {
		delta := updated.Amount - old.Amount
		if err := s.repo.AdjustBalanceTx(ctx, tx, updated.AccountType, updated.AccountID, userID, delta); err != nil {
			return nil, errors.New("failed to update account balance")
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, errors.New("failed to commit transaction")
	}

	return &updated, nil
}

func (s *transactionService) DeleteTransaction(ctx context.Context, id, userID uuid.UUID) error {
	old, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("transaction not found")
		}
		return errors.New("failed to get transaction")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return errors.New("failed to begin transaction")
	}
	defer tx.Rollback(ctx)

	if err := s.repo.DeleteTx(ctx, tx, id, userID); err != nil {
		return errors.New("failed to delete transaction")
	}

	if err := s.repo.AdjustBalanceTx(ctx, tx, old.AccountType, old.AccountID, userID, -old.Amount); err != nil {
		return errors.New("failed to update account balance")
	}

	if err := tx.Commit(ctx); err != nil {
		return errors.New("failed to commit transaction")
	}

	return nil
}

func (s *transactionService) validateAccount(ctx context.Context, accountType string, accountID, userID uuid.UUID) error {
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

func parseTransactionDate(value string) (time.Time, error) {
	if value == "" {
		return time.Now(), nil
	}
	formats := []string{time.RFC3339, "2006-01-02"}
	for _, f := range formats {
		if t, err := time.Parse(f, value); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("invalid date format, expected RFC3339 or YYYY-MM-DD")
}

func signedAmount(t string, amount float64) float64 {
	if amount < 0 {
		amount = -amount
	}
	if t == "expense" {
		return -amount
	}
	return amount
}

func absAmount(amount float64) float64 {
	if amount < 0 {
		return -amount
	}
	return amount
}

func defaultStatus(status string) string {
	if status == "" {
		return "completed"
	}
	return status
}
