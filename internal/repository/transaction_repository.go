package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cashvio/cashvio-be/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type execer interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
}

type TransactionRepository interface {
	Create(ctx context.Context, txn *model.Transaction) error
	CreateTx(ctx context.Context, tx pgx.Tx, txn *model.Transaction) error
	GetByID(ctx context.Context, id, userID uuid.UUID) (*model.Transaction, error)
	GetByUserID(ctx context.Context, q *model.TransactionQuery, userID uuid.UUID) ([]*model.Transaction, error)
	GetAllByUserID(ctx context.Context, q *model.TransactionQuery, userID uuid.UUID) ([]*model.Transaction, error)
	CountByUserID(ctx context.Context, q *model.TransactionQuery, userID uuid.UUID) (int, error)
	Update(ctx context.Context, txn *model.Transaction) error
	UpdateTx(ctx context.Context, tx pgx.Tx, txn *model.Transaction) error
	Delete(ctx context.Context, id, userID uuid.UUID) error
	DeleteTx(ctx context.Context, tx pgx.Tx, id, userID uuid.UUID) error
	AdjustBalanceTx(ctx context.Context, tx pgx.Tx, accountType string, accountID, userID uuid.UUID, delta float64) error
	SumExpenseByPeriod(ctx context.Context, userID uuid.UUID, start, end time.Time) (float64, error)
	SumIncomeByPeriod(ctx context.Context, userID uuid.UUID, start, end time.Time) (float64, error)
	FlowSeries(ctx context.Context, userID uuid.UUID, start, end time.Time, trunc string) ([]*model.FlowRow, error)
	ExpenseByCategory(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]*model.CategorySpending, error)
	MaxExpenseByPeriod(ctx context.Context, userID uuid.UUID, start, end time.Time) (*model.Transaction, error)
}

type transactionRepository struct {
	db *pgxpool.Pool
}

func NewTransactionRepository(db *pgxpool.Pool) TransactionRepository {
	return &transactionRepository{db: db}
}

func (r *transactionRepository) Create(ctx context.Context, txn *model.Transaction) error {
	return r.create(ctx, r.db, txn)
}

func (r *transactionRepository) CreateTx(ctx context.Context, tx pgx.Tx, txn *model.Transaction) error {
	return r.create(ctx, tx, txn)
}

func (r *transactionRepository) create(ctx context.Context, e execer, txn *model.Transaction) error {
	query := `
		INSERT INTO transactions (id, user_id, name, amount, type, category, status, account_type, account_id, date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err := e.Exec(ctx, query,
		txn.ID, txn.UserID, txn.Name, txn.Amount, txn.Type, txn.Category,
		txn.Status, txn.AccountType, txn.AccountID, txn.Date,
		txn.CreatedAt, txn.UpdatedAt,
	)
	return err
}

func (r *transactionRepository) GetByID(ctx context.Context, id, userID uuid.UUID) (*model.Transaction, error) {
	query := `
		SELECT id, user_id, name, amount, type, category, status, account_type, account_id, date, created_at, updated_at
		FROM transactions WHERE id = $1 AND user_id = $2
	`
	txn := &model.Transaction{}
	err := r.db.QueryRow(ctx, query, id, userID).Scan(
		&txn.ID, &txn.UserID, &txn.Name, &txn.Amount, &txn.Type, &txn.Category,
		&txn.Status, &txn.AccountType, &txn.AccountID, &txn.Date,
		&txn.CreatedAt, &txn.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return txn, nil
}

func (r *transactionRepository) GetByUserID(ctx context.Context, q *model.TransactionQuery, userID uuid.UUID) ([]*model.Transaction, error) {
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
		SELECT id, user_id, name, amount, type, category, status, account_type, account_id, date, created_at, updated_at
		FROM transactions WHERE %s
		ORDER BY %s %s LIMIT $%d OFFSET $%d
	`, strings.Join(conditions, " AND "), sortCol, order, argPos, argPos+1)
	args = append(args, q.Limit, q.Offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	transactions := make([]*model.Transaction, 0)
	for rows.Next() {
		txn := &model.Transaction{}
		err := rows.Scan(
			&txn.ID, &txn.UserID, &txn.Name, &txn.Amount, &txn.Type, &txn.Category,
			&txn.Status, &txn.AccountType, &txn.AccountID, &txn.Date,
			&txn.CreatedAt, &txn.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, txn)
	}
	return transactions, nil
}

func (r *transactionRepository) GetAllByUserID(ctx context.Context, q *model.TransactionQuery, userID uuid.UUID) ([]*model.Transaction, error) {
	conditions, args, _ := r.buildConditions(q, userID)

	sortCol := q.SortBy
	if !q.ValidSortFields[sortCol] {
		sortCol = "date"
	}
	order := strings.ToUpper(q.Order)
	if order != "ASC" {
		order = "DESC"
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, name, amount, type, category, status, account_type, account_id, date, created_at, updated_at
		FROM transactions WHERE %s
		ORDER BY %s %s
	`, strings.Join(conditions, " AND "), sortCol, order)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	transactions := make([]*model.Transaction, 0)
	for rows.Next() {
		txn := &model.Transaction{}
		err := rows.Scan(
			&txn.ID, &txn.UserID, &txn.Name, &txn.Amount, &txn.Type, &txn.Category,
			&txn.Status, &txn.AccountType, &txn.AccountID, &txn.Date,
			&txn.CreatedAt, &txn.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, txn)
	}
	return transactions, nil
}

func (r *transactionRepository) CountByUserID(ctx context.Context, q *model.TransactionQuery, userID uuid.UUID) (int, error) {
	conditions, args, _ := r.buildConditions(q, userID)

	query := fmt.Sprintf(`
		SELECT COUNT(*) FROM transactions WHERE %s
	`, strings.Join(conditions, " AND "))

	var total int
	err := r.db.QueryRow(ctx, query, args...).Scan(&total)
	return total, err
}

func (r *transactionRepository) buildConditions(q *model.TransactionQuery, userID uuid.UUID) ([]string, []interface{}, int) {
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

	if q.Category != "" {
		conditions = append(conditions, fmt.Sprintf("category = $%d", argPos))
		args = append(args, q.Category)
		argPos++
	}

	if q.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argPos))
		args = append(args, q.Status)
		argPos++
	}

	return conditions, args, argPos
}

func (r *transactionRepository) Update(ctx context.Context, txn *model.Transaction) error {
	return r.update(ctx, r.db, txn)
}

func (r *transactionRepository) UpdateTx(ctx context.Context, tx pgx.Tx, txn *model.Transaction) error {
	return r.update(ctx, tx, txn)
}

func (r *transactionRepository) update(ctx context.Context, e execer, txn *model.Transaction) error {
	query := `
		UPDATE transactions SET name = $1, amount = $2, type = $3, category = $4, status = $5,
		account_type = $6, account_id = $7, date = $8, updated_at = $9
		WHERE id = $10 AND user_id = $11
	`
	_, err := e.Exec(ctx, query,
		txn.Name, txn.Amount, txn.Type, txn.Category, txn.Status,
		txn.AccountType, txn.AccountID, txn.Date, time.Now(),
		txn.ID, txn.UserID,
	)
	return err
}

func (r *transactionRepository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	return r.delete(ctx, r.db, id, userID)
}

func (r *transactionRepository) DeleteTx(ctx context.Context, tx pgx.Tx, id, userID uuid.UUID) error {
	return r.delete(ctx, tx, id, userID)
}

func (r *transactionRepository) delete(ctx context.Context, e execer, id, userID uuid.UUID) error {
	query := `DELETE FROM transactions WHERE id = $1 AND user_id = $2`
	_, err := e.Exec(ctx, query, id, userID)
	return err
}

func (r *transactionRepository) AdjustBalanceTx(ctx context.Context, tx pgx.Tx, accountType string, accountID, userID uuid.UUID, delta float64) error {
	var query string
	var args []interface{}
	switch accountType {
	case "wallet":
		query = `UPDATE wallets SET balance_idr = balance_idr + $1, updated_at = NOW() WHERE id = $2 AND user_id = $3`
		args = []interface{}{delta, accountID, userID}
	case "card":
		query = `UPDATE cards SET balance_idr = balance_idr + $1, updated_at = NOW() WHERE id = $2 AND user_id = $3`
		args = []interface{}{delta, accountID, userID}
	case "cash":
		query = `UPDATE cashes SET balance_idr = balance_idr + $1, updated_at = NOW() WHERE user_id = $2`
		args = []interface{}{delta, userID}
	default:
		return fmt.Errorf("invalid account type: %s", accountType)
	}

	_, err := tx.Exec(ctx, query, args...)
	return err
}

func (r *transactionRepository) SumExpenseByPeriod(ctx context.Context, userID uuid.UUID, start, end time.Time) (float64, error) {
	query := `
		SELECT COALESCE(SUM(amount), 0)
		FROM transactions
		WHERE user_id = $1 AND type = 'expense' AND date >= $2 AND date < $3
	`
	var total float64
	err := r.db.QueryRow(ctx, query, userID, start, end).Scan(&total)
	return total, err
}

func (r *transactionRepository) SumIncomeByPeriod(ctx context.Context, userID uuid.UUID, start, end time.Time) (float64, error) {
	query := `
		SELECT COALESCE(SUM(amount), 0)
		FROM transactions
		WHERE user_id = $1 AND type = 'income' AND status = 'completed' AND date >= $2 AND date < $3
	`
	var total float64
	err := r.db.QueryRow(ctx, query, userID, start, end).Scan(&total)
	return total, err
}

func (r *transactionRepository) FlowSeries(ctx context.Context, userID uuid.UUID, start, end time.Time, trunc string) ([]*model.FlowRow, error) {
	query := fmt.Sprintf(`
		SELECT date_trunc('%s', date) AS bucket,
			COALESCE(SUM(amount) FILTER (WHERE type = 'income' AND status = 'completed'), 0) AS income,
			COALESCE(SUM(amount) FILTER (WHERE type = 'expense'), 0) AS expense
		FROM transactions
		WHERE user_id = $1 AND date >= $2 AND date < $3
		GROUP BY bucket
		ORDER BY bucket
	`, trunc)

	rows, err := r.db.Query(ctx, query, userID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*model.FlowRow, 0)
	for rows.Next() {
		row := &model.FlowRow{}
		if err := rows.Scan(&row.Bucket, &row.Income, &row.Expense); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, nil
}

func (r *transactionRepository) ExpenseByCategory(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]*model.CategorySpending, error) {
	query := `
		SELECT category, COALESCE(SUM(amount), 0) AS total
		FROM transactions
		WHERE user_id = $1 AND type = 'expense' AND date >= $2 AND date < $3
		GROUP BY category
		ORDER BY total DESC
	`

	rows, err := r.db.Query(ctx, query, userID, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]*model.CategorySpending, 0)
	for rows.Next() {
		row := &model.CategorySpending{}
		if err := rows.Scan(&row.Category, &row.Amount); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, nil
}

func (r *transactionRepository) MaxExpenseByPeriod(ctx context.Context, userID uuid.UUID, start, end time.Time) (*model.Transaction, error) {
	query := `
		SELECT id, user_id, name, amount, type, category, status, account_type, account_id, date, created_at, updated_at
		FROM transactions
		WHERE user_id = $1 AND type = 'expense' AND date >= $2 AND date < $3
		ORDER BY amount ASC
		LIMIT 1
	`
	txn := &model.Transaction{}
	err := r.db.QueryRow(ctx, query, userID, start, end).Scan(
		&txn.ID, &txn.UserID, &txn.Name, &txn.Amount, &txn.Type, &txn.Category,
		&txn.Status, &txn.AccountType, &txn.AccountID, &txn.Date,
		&txn.CreatedAt, &txn.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return txn, nil
}
