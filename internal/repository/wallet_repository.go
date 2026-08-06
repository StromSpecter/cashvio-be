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

type WalletRepository interface {
	Create(ctx context.Context, wallet *model.Wallet) error
	GetByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.Wallet, error)
	GetByUserID(ctx context.Context, q *model.WalletQuery, userID uuid.UUID) ([]*model.Wallet, error)
	CountByUserID(ctx context.Context, q *model.WalletQuery, userID uuid.UUID) (int, error)
	Update(ctx context.Context, wallet *model.Wallet) error
	Delete(ctx context.Context, id, userID uuid.UUID) error
}

type walletRepository struct {
	db *pgxpool.Pool
}

func NewWalletRepository(db *pgxpool.Pool) WalletRepository {
	return &walletRepository{db: db}
}

func (r *walletRepository) Create(ctx context.Context, wallet *model.Wallet) error {
	query := `
		INSERT INTO wallets (id, user_id, name, number, masked, balance_idr, tone, status, "primary", created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := r.db.Exec(ctx, query,
		wallet.ID, wallet.UserID, wallet.Name, wallet.Number, wallet.Masked,
		wallet.BalanceIDR, wallet.Tone, wallet.Status, wallet.Primary,
		wallet.CreatedAt, wallet.UpdatedAt,
	)
	return err
}

func (r *walletRepository) GetByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.Wallet, error) {
	query := `
		SELECT id, user_id, name, number, masked, balance_idr, tone, status, "primary", created_at, updated_at
		FROM wallets WHERE id = $1 AND user_id = $2
	`
	wallet := &model.Wallet{}
	err := r.db.QueryRow(ctx, query, id, userID).Scan(
		&wallet.ID, &wallet.UserID, &wallet.Name, &wallet.Number, &wallet.Masked,
		&wallet.BalanceIDR, &wallet.Tone, &wallet.Status, &wallet.Primary,
		&wallet.CreatedAt, &wallet.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return wallet, nil
}

func (r *walletRepository) GetByUserID(ctx context.Context, q *model.WalletQuery, userID uuid.UUID) ([]*model.Wallet, error) {
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

	sortCol := q.SortBy
	if !q.ValidSortFields[sortCol] {
		sortCol = "created_at"
	}
	order := strings.ToUpper(q.Order)
	if order != "ASC" {
		order = "DESC"
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, name, number, masked, balance_idr, tone, status, "primary", created_at, updated_at
		FROM wallets WHERE %s
		ORDER BY %s %s LIMIT $%d OFFSET $%d
	`, strings.Join(conditions, " AND "), sortCol, order, argPos, argPos+1)
	args = append(args, q.Limit, q.Offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	wallets := make([]*model.Wallet, 0)
	for rows.Next() {
		wallet := &model.Wallet{}
		err := rows.Scan(
			&wallet.ID, &wallet.UserID, &wallet.Name, &wallet.Number, &wallet.Masked,
			&wallet.BalanceIDR, &wallet.Tone, &wallet.Status, &wallet.Primary,
			&wallet.CreatedAt, &wallet.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		wallets = append(wallets, wallet)
	}
	return wallets, nil
}

func (r *walletRepository) CountByUserID(ctx context.Context, q *model.WalletQuery, userID uuid.UUID) (int, error) {
	var conditions []string
	var args []interface{}
	argPos := 1

	conditions = append(conditions, fmt.Sprintf("user_id = $%d", argPos))
	args = append(args, userID)
	argPos++

	if q.Search != "" {
		conditions = append(conditions, fmt.Sprintf("name ILIKE $%d", argPos))
		args = append(args, "%"+q.Search+"%")
	}

	query := fmt.Sprintf(`
		SELECT COUNT(*) FROM wallets WHERE %s
	`, strings.Join(conditions, " AND "))

	var total int
	err := r.db.QueryRow(ctx, query, args...).Scan(&total)
	return total, err
}

func (r *walletRepository) Update(ctx context.Context, wallet *model.Wallet) error {
	query := `
		UPDATE wallets SET name = $1, number = $2, masked = $3, balance_idr = $4,
		tone = $5, status = $6, "primary" = $7, updated_at = $8
		WHERE id = $9 AND user_id = $10
	`
	_, err := r.db.Exec(ctx, query,
		wallet.Name, wallet.Number, wallet.Masked, wallet.BalanceIDR,
		wallet.Tone, wallet.Status, wallet.Primary, time.Now(),
		wallet.ID, wallet.UserID,
	)
	return err
}

func (r *walletRepository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	query := `DELETE FROM wallets WHERE id = $1 AND user_id = $2`
	_, err := r.db.Exec(ctx, query, id, userID)
	return err
}
