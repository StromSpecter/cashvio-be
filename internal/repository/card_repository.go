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

type CardRepository interface {
	Create(ctx context.Context, card *model.Card) error
	GetByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.Card, error)
	GetByUserID(ctx context.Context, q *model.CardQuery, userID uuid.UUID) ([]*model.Card, error)
	CountByUserID(ctx context.Context, q *model.CardQuery, userID uuid.UUID) (int, error)
	Update(ctx context.Context, card *model.Card) error
	Delete(ctx context.Context, id, userID uuid.UUID) error
}

type cardRepository struct {
	db *pgxpool.Pool
}

func NewCardRepository(db *pgxpool.Pool) CardRepository {
	return &cardRepository{db: db}
}

func (r *cardRepository) Create(ctx context.Context, card *model.Card) error {
	query := `
		INSERT INTO cards (id, user_id, bank, number, masked, balance_idr, gradient, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.Exec(ctx, query,
		card.ID, card.UserID, card.Bank, card.Number, card.Masked,
		card.BalanceIDR, card.Gradient, card.CreatedAt, card.UpdatedAt,
	)
	return err
}

func (r *cardRepository) GetByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.Card, error) {
	query := `
		SELECT id, user_id, bank, number, masked, balance_idr, gradient, created_at, updated_at
		FROM cards WHERE id = $1 AND user_id = $2
	`
	card := &model.Card{}
	err := r.db.QueryRow(ctx, query, id, userID).Scan(
		&card.ID, &card.UserID, &card.Bank, &card.Number, &card.Masked,
		&card.BalanceIDR, &card.Gradient, &card.CreatedAt, &card.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return card, nil
}

func (r *cardRepository) GetByUserID(ctx context.Context, q *model.CardQuery, userID uuid.UUID) ([]*model.Card, error) {
	var conditions []string
	var args []interface{}
	argPos := 1

	conditions = append(conditions, fmt.Sprintf("user_id = $%d", argPos))
	args = append(args, userID)
	argPos++

	if q.Search != "" {
		conditions = append(conditions, fmt.Sprintf("bank ILIKE $%d", argPos))
		args = append(args, "%"+q.Search+"%")
		argPos++
	}

	sortCol := q.SortBy
	if !q.ValidSortFields[sortCol] {
		sortCol = "created_at"
	}
	order := strings.ToUpper(q.Order)
	if order != "ASC" {
		order = "DESC"
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, bank, number, masked, balance_idr, gradient, created_at, updated_at
		FROM cards WHERE %s
		ORDER BY %s %s LIMIT $%d OFFSET $%d
	`, strings.Join(conditions, " AND "), sortCol, order, argPos, argPos+1)
	args = append(args, q.Limit, q.Offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cards := make([]*model.Card, 0)
	for rows.Next() {
		card := &model.Card{}
		err := rows.Scan(
			&card.ID, &card.UserID, &card.Bank, &card.Number, &card.Masked,
			&card.BalanceIDR, &card.Gradient, &card.CreatedAt, &card.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		cards = append(cards, card)
	}
	return cards, nil
}

func (r *cardRepository) CountByUserID(ctx context.Context, q *model.CardQuery, userID uuid.UUID) (int, error) {
	var conditions []string
	var args []interface{}
	argPos := 1

	conditions = append(conditions, fmt.Sprintf("user_id = $%d", argPos))
	args = append(args, userID)
	argPos++

	if q.Search != "" {
		conditions = append(conditions, fmt.Sprintf("bank ILIKE $%d", argPos))
		args = append(args, "%"+q.Search+"%")
	}

	query := fmt.Sprintf(`
		SELECT COUNT(*) FROM cards WHERE %s
	`, strings.Join(conditions, " AND "))

	var total int
	err := r.db.QueryRow(ctx, query, args...).Scan(&total)
	return total, err
}

func (r *cardRepository) Update(ctx context.Context, card *model.Card) error {
	query := `
		UPDATE cards SET bank = $1, number = $2, masked = $3, balance_idr = $4,
		gradient = $5, updated_at = $6
		WHERE id = $7 AND user_id = $8
	`
	_, err := r.db.Exec(ctx, query,
		card.Bank, card.Number, card.Masked, card.BalanceIDR,
		card.Gradient, time.Now(), card.ID, card.UserID,
	)
	return err
}

func (r *cardRepository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	query := `DELETE FROM cards WHERE id = $1 AND user_id = $2`
	_, err := r.db.Exec(ctx, query, id, userID)
	return err
}
