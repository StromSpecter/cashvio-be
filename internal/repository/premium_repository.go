package repository

import (
	"context"
	"time"

	"github.com/cashvio/cashvio-be/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PremiumRepository interface {
	Create(ctx context.Context, order *model.PremiumOrder) error
	GetByID(ctx context.Context, id, userID uuid.UUID) (*model.PremiumOrder, error)
	GetByExternalID(ctx context.Context, externalID string) (*model.PremiumOrder, error)
	ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]*model.PremiumOrder, error)
	MarkPaid(ctx context.Context, order *model.PremiumOrder) error
}

type premiumRepository struct {
	db *pgxpool.Pool
}

func NewPremiumRepository(db *pgxpool.Pool) PremiumRepository {
	return &premiumRepository{db: db}
}

func (r *premiumRepository) Create(ctx context.Context, order *model.PremiumOrder) error {
	query := `
		INSERT INTO premium_orders (
			id, user_id, plan, amount, currency, duration_days, external_id,
			qris_string, qris_image_url, status, expires_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err := r.db.Exec(ctx, query,
		order.ID, order.UserID, order.Plan, order.Amount, order.Currency,
		order.DurationDays, order.ExternalID, order.QRISString, order.QRISImageURL,
		order.Status, order.ExpiresAt, order.CreatedAt, order.UpdatedAt,
	)
	return err
}

func (r *premiumRepository) GetByID(ctx context.Context, id, userID uuid.UUID) (*model.PremiumOrder, error) {
	query := `
		SELECT id, user_id, plan, amount, currency, duration_days, external_id,
			qris_string, qris_image_url, status, paid_at, premium_expires_at, expires_at, created_at, updated_at
		FROM premium_orders WHERE id = $1 AND user_id = $2
	`
	order := &model.PremiumOrder{}
	err := r.db.QueryRow(ctx, query, id, userID).Scan(
		&order.ID, &order.UserID, &order.Plan, &order.Amount, &order.Currency,
		&order.DurationDays, &order.ExternalID, &order.QRISString, &order.QRISImageURL,
		&order.Status, &order.PaidAt, &order.PremiumExpiresAt, &order.ExpiresAt,
		&order.CreatedAt, &order.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return order, nil
}

func (r *premiumRepository) GetByExternalID(ctx context.Context, externalID string) (*model.PremiumOrder, error) {
	query := `
		SELECT id, user_id, plan, amount, currency, duration_days, external_id,
			qris_string, qris_image_url, status, paid_at, premium_expires_at, expires_at, created_at, updated_at
		FROM premium_orders WHERE external_id = $1
	`
	order := &model.PremiumOrder{}
	err := r.db.QueryRow(ctx, query, externalID).Scan(
		&order.ID, &order.UserID, &order.Plan, &order.Amount, &order.Currency,
		&order.DurationDays, &order.ExternalID, &order.QRISString, &order.QRISImageURL,
		&order.Status, &order.PaidAt, &order.PremiumExpiresAt, &order.ExpiresAt,
		&order.CreatedAt, &order.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return order, nil
}

func (r *premiumRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]*model.PremiumOrder, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	query := `
		SELECT id, user_id, plan, amount, currency, duration_days, external_id,
			qris_string, qris_image_url, status, paid_at, premium_expires_at, expires_at, created_at, updated_at
		FROM premium_orders WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2
	`
	rows, err := r.db.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]*model.PremiumOrder, 0)
	for rows.Next() {
		order := &model.PremiumOrder{}
		if err := rows.Scan(
			&order.ID, &order.UserID, &order.Plan, &order.Amount, &order.Currency,
			&order.DurationDays, &order.ExternalID, &order.QRISString, &order.QRISImageURL,
			&order.Status, &order.PaidAt, &order.PremiumExpiresAt, &order.ExpiresAt,
			&order.CreatedAt, &order.UpdatedAt,
		); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, nil
}

func (r *premiumRepository) MarkPaid(ctx context.Context, order *model.PremiumOrder) error {
	query := `
		UPDATE premium_orders SET status = $1, paid_at = $2, premium_expires_at = $3, updated_at = $4
		WHERE id = $5
	`
	_, err := r.db.Exec(ctx, query, order.Status, order.PaidAt, order.PremiumExpiresAt, time.Now(), order.ID)
	return err
}
