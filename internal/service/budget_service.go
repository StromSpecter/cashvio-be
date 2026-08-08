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
)

type BudgetService interface {
	CreateBudget(ctx context.Context, userID uuid.UUID, req *model.CreateBudgetRequest) (*model.Budget, error)
	GetBudgetByID(ctx context.Context, id, userID uuid.UUID) (*model.Budget, error)
	GetAllBudgets(ctx context.Context, q *model.BudgetQuery, userID uuid.UUID) ([]*model.Budget, int, error)
	UpdateBudget(ctx context.Context, id, userID uuid.UUID, req *model.UpdateBudgetRequest) (*model.Budget, error)
	DeleteBudget(ctx context.Context, id, userID uuid.UUID) error
	GetOverview(ctx context.Context, userID uuid.UUID) (*model.BudgetOverview, error)
}

type budgetService struct {
	repo    repository.BudgetRepository
	txnRepo repository.TransactionRepository
	catRepo repository.CategoryBudgetRepository
}

func NewBudgetService(repo repository.BudgetRepository, txnRepo repository.TransactionRepository, catRepo repository.CategoryBudgetRepository) BudgetService {
	return &budgetService{repo: repo, txnRepo: txnRepo, catRepo: catRepo}
}

func ParseBudgetQuery(c_query *model.BudgetQuery, limit, offset, search, sortBy, order string) *model.BudgetQuery {
	c_query.Search = search
	c_query.SortBy = sortBy
	c_query.Order = order

	if l, err := strconv.Atoi(limit); err == nil {
		if l <= 0 {
			l = 10
		}
		if l > 100 {
			l = 100
		}
		c_query.Limit = l
	}

	if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
		c_query.Offset = o
	}

	if c_query.SortBy == "" {
		c_query.SortBy = "created_at"
	}
	if c_query.Order == "" {
		c_query.Order = "desc"
	}

	return c_query
}

func (s *budgetService) CreateBudget(ctx context.Context, userID uuid.UUID, req *model.CreateBudgetRequest) (*model.Budget, error) {
	now := time.Now()
	budget := &model.Budget{
		ID:        uuid.New(),
		UserID:    userID,
		Amount:    req.Amount,
		Note:      strings.TrimSpace(req.Note),
		Month:     strings.TrimSpace(req.Month),
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.Create(ctx, budget); err != nil {
		return nil, errors.New("failed to create budget")
	}

	return budget, nil
}

func (s *budgetService) GetBudgetByID(ctx context.Context, id, userID uuid.UUID) (*model.Budget, error) {
	budget, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("budget not found")
		}
		return nil, errors.New("failed to get budget")
	}
	return budget, nil
}

func (s *budgetService) GetAllBudgets(ctx context.Context, q *model.BudgetQuery, userID uuid.UUID) ([]*model.Budget, int, error) {
	budgets, err := s.repo.GetByUserID(ctx, q, userID)
	if err != nil {
		return nil, 0, errors.New("failed to retrieve budgets")
	}
	total, err := s.repo.CountByUserID(ctx, q, userID)
	if err != nil {
		return nil, 0, errors.New("failed to count budgets")
	}
	return budgets, total, nil
}

func (s *budgetService) UpdateBudget(ctx context.Context, id, userID uuid.UUID, req *model.UpdateBudgetRequest) (*model.Budget, error) {
	budget, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("budget not found")
		}
		return nil, errors.New("failed to get budget")
	}

	if req.Amount > 0 {
		budget.Amount = req.Amount
	}
	if req.Note != "" {
		budget.Note = strings.TrimSpace(req.Note)
	}
	if req.Month != "" {
		budget.Month = strings.TrimSpace(req.Month)
	}

	if err := s.repo.Update(ctx, budget); err != nil {
		return nil, errors.New("failed to update budget")
	}

	return budget, nil
}

func (s *budgetService) DeleteBudget(ctx context.Context, id, userID uuid.UUID) error {
	if _, err := s.repo.GetByID(ctx, id, userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("budget not found")
		}
		return errors.New("failed to get budget")
	}

	if err := s.repo.Delete(ctx, id, userID); err != nil {
		return errors.New("failed to delete budget")
	}
	return nil
}

func (s *budgetService) GetOverview(ctx context.Context, userID uuid.UUID) (*model.BudgetOverview, error) {
	bq := model.NewBudgetQuery()
	bq.Limit = 1
	budgets, err := s.repo.GetByUserID(ctx, bq, userID)
	if err != nil {
		return nil, errors.New("failed to retrieve budgets")
	}

	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 1, 0)
	spent, err := s.txnRepo.SumExpenseByPeriod(ctx, userID, start, end)
	if err != nil {
		return nil, errors.New("failed to compute spending")
	}

	cq := model.NewCategoryBudgetQuery()
	cq.Limit = 100
	cats, err := s.catRepo.GetByUserID(ctx, cq, userID)
	if err != nil {
		return nil, errors.New("failed to retrieve budget categories")
	}

	ov := &model.BudgetOverview{
		Categories: cats,
		Spent:      absAmount(spent),
	}
	if len(budgets) > 0 {
		ov.Budget = budgets[0]
		ov.Income = budgets[0].Amount
	}

	var total float64
	for _, c := range cats {
		if c.Type == "percent" && c.Percent != nil {
			total += ov.Income * (*c.Percent) / 100
		} else if c.Type == "amount" && c.Amount != nil {
			total += *c.Amount
		}
	}
	ov.TotalAllocated = total
	ov.Unallocated = ov.Income - total
	ov.Remaining = ov.Income - ov.Spent
	if ov.Income > 0 {
		ov.AllocatedPct = total / ov.Income * 100
	}

	return ov, nil
}
