package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/cashvio/cashvio-be/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransferRepository interface {
	Create(ctx context.Context, transfer *model.Transfer) error
	CreateTx(ctx context.Context, tx pgx.Tx, transfer *model.Transfer) error
	GetByID(ctx context.Context, id, userID uuid.UUID) (*model.Transfer, error)
	GetByUserID(ctx context.Context, q *model.TransferQuery, userID uuid.UUID) ([]*model.Transfer, error)
	CountByUserID(ctx context.Context, q *model.TransferQuery, userID uuid.UUID) (int, error)
	Delete(ctx context.Context, id, userID uuid.UUID) error
	DeleteTx(ctx context.Context, tx pgx.Tx, id, userID uuid.UUID) error
	AdjustBalanceTx(ctx context.Context, tx pgx.Tx, accountType string, accountID, userID uuid.UUID, delta float64) error
}

type transferRepository struct {
	db *pgxpool.Pool
}

func NewTransferRepository(db *pgxpool.Pool) TransferRepository {
	return &transferRepository{db: db}
}

func (r *transferRepository) Create(ctx context.Context, transfer *model.Transfer) error {
	return r.create(ctx, r.db, transfer)
}

func (r *transferRepository) CreateTx(ctx context.Context, tx pgx.Tx, transfer *model.Transfer) error {
	return r.create(ctx, tx, transfer)
}

func (r *transferRepository) create(ctx context.Context, e execer, transfer *model.Transfer) error {
	query := `
		INSERT INTO transfers (id, user_id, from_type, from_id, to_type, to_id, amount, fee, note, date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err := e.Exec(ctx, query,
		transfer.ID, transfer.UserID, transfer.FromType, transfer.FromID,
		transfer.ToType, transfer.ToID, transfer.Amount, transfer.Fee, transfer.Note,
		transfer.Date, transfer.CreatedAt, transfer.UpdatedAt,
	)
	return err
}

func (r *transferRepository) GetByID(ctx context.Context, id, userID uuid.UUID) (*model.Transfer, error) {
	query := `
		SELECT id, user_id, from_type, from_id, to_type, to_id, amount, fee, note, date, created_at, updated_at
		FROM transfers WHERE id = $1 AND user_id = $2
	`
	transfer := &model.Transfer{}
	err := r.db.QueryRow(ctx, query, id, userID).Scan(
		&transfer.ID, &transfer.UserID, &transfer.FromType, &transfer.FromID,
		&transfer.ToType, &transfer.ToID, &transfer.Amount, &transfer.Fee, &transfer.Note,
		&transfer.Date, &transfer.CreatedAt, &transfer.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return transfer, nil
}

func (r *transferRepository) GetByUserID(ctx context.Context, q *model.TransferQuery, userID uuid.UUID) ([]*model.Transfer, error) {
	conditions, args, argPos := r.buildConditions(q, userID)

	sortCol := q.SortBy
	if !q.ValidSortFields[sortCol] {
		sortCol = "date"
	}
	order := strings.ToUpper(q.Order)
	if order != "ASC" {
		order = "DESC"
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, from_type, from_id, to_type, to_id, amount, fee, note, date, created_at, updated_at
		FROM transfers WHERE %s
		ORDER BY %s %s LIMIT $%d OFFSET $%d
	`, strings.Join(conditions, " AND "), sortCol, order, argPos, argPos+1)
	args = append(args, q.Limit, q.Offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	transfers := make([]*model.Transfer, 0)
	for rows.Next() {
		transfer := &model.Transfer{}
		err := rows.Scan(
			&transfer.ID, &transfer.UserID, &transfer.FromType, &transfer.FromID,
			&transfer.ToType, &transfer.ToID, &transfer.Amount, &transfer.Fee, &transfer.Note,
			&transfer.Date, &transfer.CreatedAt, &transfer.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		transfers = append(transfers, transfer)
	}
	return transfers, nil
}

func (r *transferRepository) CountByUserID(ctx context.Context, q *model.TransferQuery, userID uuid.UUID) (int, error) {
	conditions, args, _ := r.buildConditions(q, userID)

	query := fmt.Sprintf(`
		SELECT COUNT(*) FROM transfers WHERE %s
	`, strings.Join(conditions, " AND "))

	var total int
	err := r.db.QueryRow(ctx, query, args...).Scan(&total)
	return total, err
}

func (r *transferRepository) buildConditions(q *model.TransferQuery, userID uuid.UUID) ([]string, []interface{}, int) {
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

func (r *transferRepository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	return r.delete(ctx, r.db, id, userID)
}

func (r *transferRepository) DeleteTx(ctx context.Context, tx pgx.Tx, id, userID uuid.UUID) error {
	return r.delete(ctx, tx, id, userID)
}

func (r *transferRepository) delete(ctx context.Context, e execer, id, userID uuid.UUID) error {
	query := `DELETE FROM transfers WHERE id = $1 AND user_id = $2`
	_, err := e.Exec(ctx, query, id, userID)
	return err
}

func (r *transferRepository) AdjustBalanceTx(ctx context.Context, tx pgx.Tx, accountType string, accountID, userID uuid.UUID, delta float64) error {
	var query string
	switch accountType {
	case "wallet":
		query = `UPDATE wallets SET balance_idr = balance_idr + $1, updated_at = NOW() WHERE id = $2 AND user_id = $3`
	case "card":
		query = `UPDATE cards SET balance_idr = balance_idr + $1, updated_at = NOW() WHERE id = $2 AND user_id = $3`
	default:
		return fmt.Errorf("invalid account type: %s", accountType)
	}

	_, err := tx.Exec(ctx, query, delta, accountID, userID)
	return err
}
