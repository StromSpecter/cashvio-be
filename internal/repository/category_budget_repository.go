package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cashvio/cashvio-be/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CategoryBudgetRepository interface {
	Create(ctx context.Context, cat *model.CategoryBudget) error
	GetByID(ctx context.Context, id, userID uuid.UUID) (*model.CategoryBudget, error)
	GetByUserID(ctx context.Context, q *model.CategoryBudgetQuery, userID uuid.UUID) ([]*model.CategoryBudget, error)
	CountByUserID(ctx context.Context, q *model.CategoryBudgetQuery, userID uuid.UUID) (int, error)
	Update(ctx context.Context, cat *model.CategoryBudget) error
	Delete(ctx context.Context, id, userID uuid.UUID) error
}

type categoryBudgetRepository struct {
	db *pgxpool.Pool
}

func NewCategoryBudgetRepository(db *pgxpool.Pool) CategoryBudgetRepository {
	return &categoryBudgetRepository{db: db}
}

func (r *categoryBudgetRepository) Create(ctx context.Context, cat *model.CategoryBudget) error {
	query := `
		INSERT INTO category_budgets (id, user_id, name, type, percent, amount, color, icon, "desc", created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := r.db.Exec(ctx, query,
		cat.ID, cat.UserID, cat.Name, cat.Type,
		cat.Percent, cat.Amount, cat.Color, cat.Icon, cat.Desc,
		cat.CreatedAt, cat.UpdatedAt,
	)
	return err
}

func (r *categoryBudgetRepository) GetByID(ctx context.Context, id, userID uuid.UUID) (*model.CategoryBudget, error) {
	query := `
		SELECT id, user_id, name, type, percent, amount, color, icon, "desc", created_at, updated_at
		FROM category_budgets WHERE id = $1 AND user_id = $2
	`
	cat := &model.CategoryBudget{}
	err := r.db.QueryRow(ctx, query, id, userID).Scan(
		&cat.ID, &cat.UserID, &cat.Name, &cat.Type,
		&cat.Percent, &cat.Amount, &cat.Color, &cat.Icon, &cat.Desc,
		&cat.CreatedAt, &cat.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return cat, nil
}

func (r *categoryBudgetRepository) GetByUserID(ctx context.Context, q *model.CategoryBudgetQuery, userID uuid.UUID) ([]*model.CategoryBudget, error) {
	conditions, args, argPos := r.buildConditions(q, userID)

	sortCol := q.SortBy
	if !q.ValidSortFields[sortCol] {
		sortCol = "created_at"
	}
	order := strings.ToUpper(q.Order)
	if order != "ASC" {
		order = "DESC"
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, name, type, percent, amount, color, icon, "desc", created_at, updated_at
		FROM category_budgets WHERE %s
		ORDER BY %s %s LIMIT $%d OFFSET $%d
	`, strings.Join(conditions, " AND "), sortCol, order, argPos, argPos+1)
	args = append(args, q.Limit, q.Offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cats := make([]*model.CategoryBudget, 0)
	for rows.Next() {
		cat := &model.CategoryBudget{}
		err := rows.Scan(
			&cat.ID, &cat.UserID, &cat.Name, &cat.Type,
			&cat.Percent, &cat.Amount, &cat.Color, &cat.Icon, &cat.Desc,
			&cat.CreatedAt, &cat.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		cats = append(cats, cat)
	}
	return cats, nil
}

func (r *categoryBudgetRepository) CountByUserID(ctx context.Context, q *model.CategoryBudgetQuery, userID uuid.UUID) (int, error) {
	conditions, args, _ := r.buildConditions(q, userID)

	query := fmt.Sprintf(`
		SELECT COUNT(*) FROM category_budgets WHERE %s
	`, strings.Join(conditions, " AND "))

	var total int
	err := r.db.QueryRow(ctx, query, args...).Scan(&total)
	return total, err
}

func (r *categoryBudgetRepository) buildConditions(q *model.CategoryBudgetQuery, userID uuid.UUID) ([]string, []interface{}, int) {
	var conditions []string
	var args []interface{}
	argPos := 1

	conditions = append(conditions, fmt.Sprintf("user_id = $%d", argPos))
	args = append(args, userID)
	argPos++

	if q.Search != "" {
		conditions = append(conditions, fmt.Sprintf("name ILIKE $%d", argPos))
		args = append(args, "%"+q.Search+"%")
		argPos++
	}

	if q.Type != "" {
		conditions = append(conditions, fmt.Sprintf("type = $%d", argPos))
		args = append(args, q.Type)
		argPos++
	}

	return conditions, args, argPos
}

func (r *categoryBudgetRepository) Update(ctx context.Context, cat *model.CategoryBudget) error {
	query := `
		UPDATE category_budgets SET name = $1, type = $2, percent = $3, amount = $4,
		color = $5, icon = $6, "desc" = $7, updated_at = $8
		WHERE id = $9 AND user_id = $10
	`
	_, err := r.db.Exec(ctx, query,
		cat.Name, cat.Type, cat.Percent, cat.Amount,
		cat.Color, cat.Icon, cat.Desc, time.Now(),
		cat.ID, cat.UserID,
	)
	return err
}

func (r *categoryBudgetRepository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	query := `DELETE FROM category_budgets WHERE id = $1 AND user_id = $2`
	_, err := r.db.Exec(ctx, query, id, userID)
	return err
}
