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

type BudgetRepository interface {
	Create(ctx context.Context, budget *model.Budget) error
	GetByID(ctx context.Context, id, userID uuid.UUID) (*model.Budget, error)
	GetByUserID(ctx context.Context, q *model.BudgetQuery, userID uuid.UUID) ([]*model.Budget, error)
	CountByUserID(ctx context.Context, q *model.BudgetQuery, userID uuid.UUID) (int, error)
	Update(ctx context.Context, budget *model.Budget) error
	Delete(ctx context.Context, id, userID uuid.UUID) error
}

type budgetRepository struct {
	db *pgxpool.Pool
}

func NewBudgetRepository(db *pgxpool.Pool) BudgetRepository {
	return &budgetRepository{db: db}
}

func (r *budgetRepository) Create(ctx context.Context, budget *model.Budget) error {
	query := `
		INSERT INTO budgets (id, user_id, amount, note, month, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.Exec(ctx, query,
		budget.ID, budget.UserID, budget.Amount, budget.Note, budget.Month,
		budget.CreatedAt, budget.UpdatedAt,
	)
	return err
}

func (r *budgetRepository) GetByID(ctx context.Context, id, userID uuid.UUID) (*model.Budget, error) {
	query := `
		SELECT id, user_id, amount, note, month, created_at, updated_at
		FROM budgets WHERE id = $1 AND user_id = $2
	`
	budget := &model.Budget{}
	err := r.db.QueryRow(ctx, query, id, userID).Scan(
		&budget.ID, &budget.UserID, &budget.Amount, &budget.Note, &budget.Month,
		&budget.CreatedAt, &budget.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return budget, nil
}

func (r *budgetRepository) GetByUserID(ctx context.Context, q *model.BudgetQuery, userID uuid.UUID) ([]*model.Budget, error) {
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
		SELECT id, user_id, amount, note, month, created_at, updated_at
		FROM budgets WHERE %s
		ORDER BY %s %s LIMIT $%d OFFSET $%d
	`, strings.Join(conditions, " AND "), sortCol, order, argPos, argPos+1)
	args = append(args, q.Limit, q.Offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	budgets := make([]*model.Budget, 0)
	for rows.Next() {
		budget := &model.Budget{}
		err := rows.Scan(
			&budget.ID, &budget.UserID, &budget.Amount, &budget.Note, &budget.Month,
			&budget.CreatedAt, &budget.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		budgets = append(budgets, budget)
	}
	return budgets, nil
}

func (r *budgetRepository) CountByUserID(ctx context.Context, q *model.BudgetQuery, userID uuid.UUID) (int, error) {
	conditions, args, _ := r.buildConditions(q, userID)

	query := fmt.Sprintf(`
		SELECT COUNT(*) FROM budgets WHERE %s
	`, strings.Join(conditions, " AND "))

	var total int
	err := r.db.QueryRow(ctx, query, args...).Scan(&total)
	return total, err
}

func (r *budgetRepository) buildConditions(q *model.BudgetQuery, userID uuid.UUID) ([]string, []interface{}, int) {
	var conditions []string
	var args []interface{}
	argPos := 1

	conditions = append(conditions, fmt.Sprintf("user_id = $%d", argPos))
	args = append(args, userID)
	argPos++

	if q.Search != "" {
		conditions = append(conditions, fmt.Sprintf("note ILIKE $%d", argPos))
		args = append(args, "%"+q.Search+"%")
		argPos++
	}

	return conditions, args, argPos
}

func (r *budgetRepository) Update(ctx context.Context, budget *model.Budget) error {
	query := `
		UPDATE budgets SET amount = $1, note = $2, month = $3, updated_at = $4
		WHERE id = $5 AND user_id = $6
	`
	_, err := r.db.Exec(ctx, query,
		budget.Amount, budget.Note, budget.Month, time.Now(),
		budget.ID, budget.UserID,
	)
	return err
}

func (r *budgetRepository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	query := `DELETE FROM budgets WHERE id = $1 AND user_id = $2`
	_, err := r.db.Exec(ctx, query, id, userID)
	return err
}
