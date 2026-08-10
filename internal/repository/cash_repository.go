package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cashvio/cashvio-be/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CashRepository interface {
	Create(ctx context.Context, cash *model.Cash) error
	GetByUserID(ctx context.Context, userID uuid.UUID) (*model.Cash, error)
	AdjustBalanceTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID, delta float64) error
	AdjustSourceBalanceTx(ctx context.Context, tx pgx.Tx, accountType string, accountID, userID uuid.UUID, delta float64) error

	CreateWithdrawalTx(ctx context.Context, tx pgx.Tx, withdrawal *model.CashWithdrawal) error
	GetWithdrawalByID(ctx context.Context, id, userID uuid.UUID) (*model.CashWithdrawal, error)
	GetWithdrawalsByUserID(ctx context.Context, q *model.CashWithdrawalQuery, userID uuid.UUID) ([]*model.CashWithdrawal, error)
	CountWithdrawalsByUserID(ctx context.Context, q *model.CashWithdrawalQuery, userID uuid.UUID) (int, error)
	DeleteWithdrawalTx(ctx context.Context, tx pgx.Tx, id, userID uuid.UUID) error
}

type cashRepository struct {
	db *pgxpool.Pool
}

func NewCashRepository(db *pgxpool.Pool) CashRepository {
	return &cashRepository{db: db}
}

func (r *cashRepository) Create(ctx context.Context, cash *model.Cash) error {
	query := `
		INSERT INTO cashes (id, user_id, balance_idr, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.Exec(ctx, query,
		cash.ID, cash.UserID, cash.BalanceIDR, cash.CreatedAt, cash.UpdatedAt,
	)
	return err
}

func (r *cashRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*model.Cash, error) {
	query := `
		SELECT id, user_id, balance_idr, created_at, updated_at
		FROM cashes WHERE user_id = $1
	`
	cash := &model.Cash{}
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&cash.ID, &cash.UserID, &cash.BalanceIDR, &cash.CreatedAt, &cash.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return cash, nil
}

func (r *cashRepository) AdjustBalanceTx(ctx context.Context, tx pgx.Tx, userID uuid.UUID, delta float64) error {
	query := `
		UPDATE cashes SET balance_idr = balance_idr + $1, updated_at = $2
		WHERE user_id = $3
	`
	_, err := tx.Exec(ctx, query, delta, time.Now(), userID)
	return err
}

func (r *cashRepository) AdjustSourceBalanceTx(ctx context.Context, tx pgx.Tx, accountType string, accountID, userID uuid.UUID, delta float64) error {
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

func (r *cashRepository) CreateWithdrawalTx(ctx context.Context, tx pgx.Tx, withdrawal *model.CashWithdrawal) error {
	query := `
		INSERT INTO cash_withdrawals (id, user_id, from_type, from_id, amount, fee, note, date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := tx.Exec(ctx, query,
		withdrawal.ID, withdrawal.UserID, withdrawal.FromType, withdrawal.FromID,
		withdrawal.Amount, withdrawal.Fee, withdrawal.Note,
		withdrawal.Date, withdrawal.CreatedAt, withdrawal.UpdatedAt,
	)
	return err
}

func (r *cashRepository) GetWithdrawalByID(ctx context.Context, id, userID uuid.UUID) (*model.CashWithdrawal, error) {
	query := `
		SELECT id, user_id, from_type, from_id, amount, fee, note, date, created_at, updated_at
		FROM cash_withdrawals WHERE id = $1 AND user_id = $2
	`
	withdrawal := &model.CashWithdrawal{}
	err := r.db.QueryRow(ctx, query, id, userID).Scan(
		&withdrawal.ID, &withdrawal.UserID, &withdrawal.FromType, &withdrawal.FromID,
		&withdrawal.Amount, &withdrawal.Fee, &withdrawal.Note,
		&withdrawal.Date, &withdrawal.CreatedAt, &withdrawal.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return withdrawal, nil
}

func (r *cashRepository) GetWithdrawalsByUserID(ctx context.Context, q *model.CashWithdrawalQuery, userID uuid.UUID) ([]*model.CashWithdrawal, error) {
	conditions, args, argPos := r.buildWithdrawalConditions(q, userID)

	sortCol := q.SortBy
	if !q.ValidSortFields[sortCol] {
		sortCol = "date"
	}
	order := strings.ToUpper(q.Order)
	if order != "ASC" {
		order = "DESC"
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, from_type, from_id, amount, fee, note, date, created_at, updated_at
		FROM cash_withdrawals WHERE %s
		ORDER BY %s %s LIMIT $%d OFFSET $%d
	`, strings.Join(conditions, " AND "), sortCol, order, argPos, argPos+1)
	args = append(args, q.Limit, q.Offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	withdrawals := make([]*model.CashWithdrawal, 0)
	for rows.Next() {
		withdrawal := &model.CashWithdrawal{}
		err := rows.Scan(
			&withdrawal.ID, &withdrawal.UserID, &withdrawal.FromType, &withdrawal.FromID,
			&withdrawal.Amount, &withdrawal.Fee, &withdrawal.Note,
			&withdrawal.Date, &withdrawal.CreatedAt, &withdrawal.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		withdrawals = append(withdrawals, withdrawal)
	}
	return withdrawals, nil
}

func (r *cashRepository) CountWithdrawalsByUserID(ctx context.Context, q *model.CashWithdrawalQuery, userID uuid.UUID) (int, error) {
	conditions, args, _ := r.buildWithdrawalConditions(q, userID)

	query := fmt.Sprintf(`
		SELECT COUNT(*) FROM cash_withdrawals WHERE %s
	`, strings.Join(conditions, " AND "))

	var total int
	err := r.db.QueryRow(ctx, query, args...).Scan(&total)
	return total, err
}

func (r *cashRepository) buildWithdrawalConditions(q *model.CashWithdrawalQuery, userID uuid.UUID) ([]string, []interface{}, int) {
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

func (r *cashRepository) DeleteWithdrawalTx(ctx context.Context, tx pgx.Tx, id, userID uuid.UUID) error {
	query := `DELETE FROM cash_withdrawals WHERE id = $1 AND user_id = $2`
	_, err := tx.Exec(ctx, query, id, userID)
	return err
}
