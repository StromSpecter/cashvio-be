package service

import (
	"context"
	"errors"
	"time"

	"github.com/cashvio/cashvio-be/internal/model"
	"github.com/cashvio/cashvio-be/internal/repository"
	"github.com/google/uuid"
)

type BudgetOverviewService interface {
	GetOverview(ctx context.Context, userID uuid.UUID) (*model.BudgetOverview, error)
}

type budgetOverviewService struct {
	txnRepo repository.TransactionRepository
	catRepo repository.CategoryBudgetRepository
}

func NewBudgetOverviewService(txnRepo repository.TransactionRepository, catRepo repository.CategoryBudgetRepository) BudgetOverviewService {
	return &budgetOverviewService{txnRepo: txnRepo, catRepo: catRepo}
}

func (s *budgetOverviewService) GetOverview(ctx context.Context, userID uuid.UUID) (*model.BudgetOverview, error) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 1, 0)

	spent, err := s.txnRepo.SumExpenseByPeriod(ctx, userID, start, end)
	if err != nil {
		return nil, errors.New("failed to compute spending")
	}
	income, err := s.txnRepo.SumIncomeByPeriod(ctx, userID, start, end)
	if err != nil {
		return nil, errors.New("failed to compute income")
	}

	cq := model.NewCategoryBudgetQuery()
	cq.Limit = 100
	cats, err := s.catRepo.GetByUserID(ctx, cq, userID)
	if err != nil {
		return nil, errors.New("failed to retrieve budget categories")
	}

	ov := &model.BudgetOverview{
		Categories: cats,
		Income:     income,
		Spent:      absAmount(spent),
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
