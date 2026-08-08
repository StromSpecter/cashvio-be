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

type CategoryBudgetService interface {
	CreateCategoryBudget(ctx context.Context, userID uuid.UUID, req *model.CreateCategoryBudgetRequest) (*model.CategoryBudget, error)
	GetCategoryBudgetByID(ctx context.Context, id, userID uuid.UUID) (*model.CategoryBudget, error)
	GetAllCategoryBudgets(ctx context.Context, q *model.CategoryBudgetQuery, userID uuid.UUID) ([]*model.CategoryBudget, int, error)
	UpdateCategoryBudget(ctx context.Context, id, userID uuid.UUID, req *model.UpdateCategoryBudgetRequest) (*model.CategoryBudget, error)
	DeleteCategoryBudget(ctx context.Context, id, userID uuid.UUID) error
}

type categoryBudgetService struct {
	repo       repository.CategoryBudgetRepository
	budgetRepo repository.BudgetRepository
}

func NewCategoryBudgetService(repo repository.CategoryBudgetRepository, budgetRepo repository.BudgetRepository) CategoryBudgetService {
	return &categoryBudgetService{repo: repo, budgetRepo: budgetRepo}
}

func ParseCategoryBudgetQuery(c_query *model.CategoryBudgetQuery, limit, offset, search, sortBy, order, budgetID, t string) *model.CategoryBudgetQuery {
	c_query.Search = search
	c_query.SortBy = sortBy
	c_query.Order = order
	c_query.Type = t

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

	if id, err := uuid.Parse(budgetID); err == nil {
		c_query.BudgetID = &id
	}

	if c_query.SortBy == "" {
		c_query.SortBy = "created_at"
	}
	if c_query.Order == "" {
		c_query.Order = "desc"
	}

	return c_query
}

func (s *categoryBudgetService) CreateCategoryBudget(ctx context.Context, userID uuid.UUID, req *model.CreateCategoryBudgetRequest) (*model.CategoryBudget, error) {
	if req.BudgetID != nil {
		if err := s.validateBudget(ctx, *req.BudgetID, userID); err != nil {
			return nil, err
		}
	}

	now := time.Now()
	cat := &model.CategoryBudget{
		ID:        uuid.New(),
		UserID:    userID,
		BudgetID:  req.BudgetID,
		Name:      strings.TrimSpace(req.Name),
		Type:      req.Type,
		Color:     defaultCategoryColor(req.Color),
		Icon:      defaultCategoryIcon(req.Icon),
		Desc:      strings.TrimSpace(req.Desc),
		CreatedAt: now,
		UpdatedAt: now,
	}
	cat.Percent, cat.Amount = normalizeCategoryAllocation(cat.Type, req.Percent, req.Amount)

	if err := s.repo.Create(ctx, cat); err != nil {
		return nil, errors.New("failed to create budget category")
	}

	return cat, nil
}

func (s *categoryBudgetService) GetCategoryBudgetByID(ctx context.Context, id, userID uuid.UUID) (*model.CategoryBudget, error) {
	cat, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("budget category not found")
		}
		return nil, errors.New("failed to get budget category")
	}
	return cat, nil
}

func (s *categoryBudgetService) GetAllCategoryBudgets(ctx context.Context, q *model.CategoryBudgetQuery, userID uuid.UUID) ([]*model.CategoryBudget, int, error) {
	cats, err := s.repo.GetByUserID(ctx, q, userID)
	if err != nil {
		return nil, 0, errors.New("failed to retrieve budget categories")
	}
	total, err := s.repo.CountByUserID(ctx, q, userID)
	if err != nil {
		return nil, 0, errors.New("failed to count budget categories")
	}
	return cats, total, nil
}

func (s *categoryBudgetService) UpdateCategoryBudget(ctx context.Context, id, userID uuid.UUID, req *model.UpdateCategoryBudgetRequest) (*model.CategoryBudget, error) {
	cat, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("budget category not found")
		}
		return nil, errors.New("failed to get budget category")
	}

	if req.BudgetID != nil {
		if err := s.validateBudget(ctx, *req.BudgetID, userID); err != nil {
			return nil, err
		}
		cat.BudgetID = req.BudgetID
	}
	if req.Name != "" {
		cat.Name = strings.TrimSpace(req.Name)
	}
	if req.Type != "" {
		cat.Type = req.Type
		cat.Percent, cat.Amount = normalizeCategoryAllocation(cat.Type, req.Percent, req.Amount)
	} else {
		if req.Percent != nil {
			cat.Percent = req.Percent
			if cat.Type == "amount" {
				cat.Amount = nil
			}
		}
		if req.Amount != nil {
			cat.Amount = req.Amount
			if cat.Type == "percent" {
				cat.Percent = nil
			}
		}
	}
	if req.Color != nil {
		cat.Color = *req.Color
	}
	if req.Icon != "" {
		cat.Icon = strings.TrimSpace(req.Icon)
	}
	if req.Desc != "" {
		cat.Desc = strings.TrimSpace(req.Desc)
	}

	if err := s.repo.Update(ctx, cat); err != nil {
		return nil, errors.New("failed to update budget category")
	}

	return cat, nil
}

func (s *categoryBudgetService) DeleteCategoryBudget(ctx context.Context, id, userID uuid.UUID) error {
	if _, err := s.repo.GetByID(ctx, id, userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("budget category not found")
		}
		return errors.New("failed to get budget category")
	}

	if err := s.repo.Delete(ctx, id, userID); err != nil {
		return errors.New("failed to delete budget category")
	}
	return nil
}

func (s *categoryBudgetService) validateBudget(ctx context.Context, budgetID, userID uuid.UUID) error {
	if _, err := s.budgetRepo.GetByID(ctx, budgetID, userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("budget not found")
		}
		return errors.New("failed to validate budget")
	}
	return nil
}

func normalizeCategoryAllocation(t string, percent, amount *float64) (*float64, *float64) {
	if t == "percent" {
		if percent == nil {
			return nil, nil
		}
		return percent, nil
	}
	if amount == nil {
		return nil, nil
	}
	return nil, amount
}

func defaultCategoryColor(c *int) int {
	if c != nil && *c >= 1 && *c <= 4 {
		return *c
	}
	return 1
}

func defaultCategoryIcon(icon string) string {
	if strings.TrimSpace(icon) == "" {
		return "home"
	}
	return strings.TrimSpace(icon)
}
