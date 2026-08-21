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

type InvestmentRepository interface {
	CreateTx(ctx context.Context, tx pgx.Tx, inv *model.Investment) error
	GetByID(ctx context.Context, id, userID uuid.UUID) (*model.Investment, error)
	GetByUserID(ctx context.Context, q *model.InvestmentQuery, userID uuid.UUID) ([]*model.Investment, error)
	GetGroupedByUserID(ctx context.Context, q *model.InvestmentQuery, userID uuid.UUID) ([]*model.InvestmentGroup, error)
	CountByUserID(ctx context.Context, q *model.InvestmentQuery, userID uuid.UUID) (int, error)
	UpdateTx(ctx context.Context, tx pgx.Tx, inv *model.Investment) error
	Delete(ctx context.Context, id, userID uuid.UUID) error
	DeleteTx(ctx context.Context, tx pgx.Tx, id, userID uuid.UUID) error
}

type investmentRepository struct {
	db *pgxpool.Pool
}

func NewInvestmentRepository(db *pgxpool.Pool) InvestmentRepository {
	return &investmentRepository{db: db}
}

func (r *investmentRepository) CreateTx(ctx context.Context, tx pgx.Tx, inv *model.Investment) error {
	query := `
		INSERT INTO investments (id, user_id, type, name, ticker, app, account_type, account_id, units, buy_price, date, transaction_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`
	_, err := tx.Exec(ctx, query,
		inv.ID, inv.UserID, inv.Type, inv.Name, inv.Ticker, inv.App,
		inv.AccountType, inv.AccountID, inv.Units, inv.BuyPrice,
		inv.Date, inv.TransactionID, inv.CreatedAt, inv.UpdatedAt,
	)
	return err
}

func (r *investmentRepository) GetByID(ctx context.Context, id, userID uuid.UUID) (*model.Investment, error) {
	query := `
		SELECT id, user_id, type, name, ticker, app, account_type, account_id, units, buy_price, date, transaction_id, created_at, updated_at
		FROM investments WHERE id = $1 AND user_id = $2
	`
	inv := &model.Investment{}
	err := r.db.QueryRow(ctx, query, id, userID).Scan(
		&inv.ID, &inv.UserID, &inv.Type, &inv.Name, &inv.Ticker, &inv.App,
		&inv.AccountType, &inv.AccountID, &inv.Units, &inv.BuyPrice,
		&inv.Date, &inv.TransactionID, &inv.CreatedAt, &inv.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return inv, nil
}

func (r *investmentRepository) GetByUserID(ctx context.Context, q *model.InvestmentQuery, userID uuid.UUID) ([]*model.Investment, error) {
	groups, err := r.GetGroupedByUserID(ctx, q, userID)
	if err != nil {
		return nil, err
	}

	investments := make([]*model.Investment, 0)
	for _, g := range groups {
		investments = append(investments, g.Purchases...)
	}
	return investments, nil
}

func (r *investmentRepository) GetGroupedByUserID(ctx context.Context, q *model.InvestmentQuery, userID uuid.UUID) ([]*model.InvestmentGroup, error) {
	conditions, args, argPos := r.buildConditions(q, userID)
	where := strings.Join(conditions, " AND ")

	sortCol := q.SortBy
	if !q.ValidSortFields[sortCol] {
		sortCol = "date"
	}
	order := strings.ToUpper(q.Order)
	if order != "ASC" {
		order = "DESC"
	}

	// Limit/offset applies to asset groups (type, name, ticker, app), not raw
	// purchase rows, so every purchase of a listed asset is returned together.
	groupSortExpr := map[string]string{
		"date":       "MAX(date)",
		"name":       "MIN(name)",
		"type":       "MIN(type)",
		"units":      "SUM(units)",
		"buy_price":  "MAX(buy_price)",
		"created_at": "MIN(created_at)",
		"updated_at": "MAX(updated_at)",
	}[sortCol]

	groupQuery := fmt.Sprintf(`
		SELECT type, name, ticker, app
		FROM investments WHERE %s
		GROUP BY type, name, ticker, app
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, where, groupSortExpr, order, argPos, argPos+1)
	groupArgs := append(append([]interface{}{}, args...), q.Limit, q.Offset)

	rows, err := r.db.Query(ctx, groupQuery, groupArgs...)
	if err != nil {
		return nil, err
	}

	type groupKey struct {
		Type   string
		Name   string
		Ticker string
		App    string
	}
	var keys []groupKey
	for rows.Next() {
		var k groupKey
		if err := rows.Scan(&k.Type, &k.Name, &k.Ticker, &k.App); err != nil {
			rows.Close()
			return nil, err
		}
		keys = append(keys, k)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return make([]*model.InvestmentGroup, 0), nil
	}

	placeholders := make([]string, len(keys))
	purchaseArgs := append([]interface{}{}, args...)
	pos := argPos
	for i, k := range keys {
		placeholders[i] = fmt.Sprintf("($%d,$%d,$%d,$%d)", pos, pos+1, pos+2, pos+3)
		purchaseArgs = append(purchaseArgs, k.Type, k.Name, k.Ticker, k.App)
		pos += 4
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, type, name, ticker, app, account_type, account_id, units, buy_price, date, transaction_id, created_at, updated_at
		FROM investments WHERE %s AND (type, name, ticker, app) IN (%s)
		ORDER BY %s %s
	`, where, strings.Join(placeholders, ", "), sortCol, order)

	rows, err = r.db.Query(ctx, query, purchaseArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groups := make([]*model.InvestmentGroup, len(keys))
	index := make(map[groupKey]int, len(keys))
	for i, k := range keys {
		groups[i] = &model.InvestmentGroup{Type: k.Type, Name: k.Name, Ticker: k.Ticker, App: k.App}
		index[k] = i
	}
	for rows.Next() {
		inv := &model.Investment{}
		err := rows.Scan(
			&inv.ID, &inv.UserID, &inv.Type, &inv.Name, &inv.Ticker, &inv.App,
			&inv.AccountType, &inv.AccountID, &inv.Units, &inv.BuyPrice,
			&inv.Date, &inv.TransactionID, &inv.CreatedAt, &inv.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		k := groupKey{Type: inv.Type, Name: inv.Name, Ticker: inv.Ticker, App: inv.App}
		if gi, ok := index[k]; ok {
			groups[gi].Purchases = append(groups[gi].Purchases, inv)
		}
	}
	return groups, nil
}

func (r *investmentRepository) CountByUserID(ctx context.Context, q *model.InvestmentQuery, userID uuid.UUID) (int, error) {
	conditions, args, _ := r.buildConditions(q, userID)

	query := fmt.Sprintf(
		`SELECT COUNT(*) FROM (SELECT DISTINCT type, name, ticker, app FROM investments WHERE %s) t`,
		strings.Join(conditions, " AND "),
	)

	var total int
	err := r.db.QueryRow(ctx, query, args...).Scan(&total)
	return total, err
}

func (r *investmentRepository) buildConditions(q *model.InvestmentQuery, userID uuid.UUID) ([]string, []interface{}, int) {
	var conditions []string
	var args []interface{}
	argPos := 1

	conditions = append(conditions, fmt.Sprintf("user_id = $%d", argPos))
	args = append(args, userID)
	argPos++

	if q.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(name ILIKE $%d OR ticker ILIKE $%d OR app ILIKE $%d)", argPos, argPos+1, argPos+2))
		args = append(args, "%"+q.Search+"%", "%"+q.Search+"%", "%"+q.Search+"%")
		argPos += 3
	}

	if q.Type != "" {
		conditions = append(conditions, fmt.Sprintf("type = $%d", argPos))
		args = append(args, q.Type)
		argPos++
	}

	return conditions, args, argPos
}

func (r *investmentRepository) UpdateTx(ctx context.Context, tx pgx.Tx, inv *model.Investment) error {
	query := `
		UPDATE investments SET type = $1, name = $2, ticker = $3, app = $4, account_type = $5, account_id = $6,
		units = $7, buy_price = $8, date = $9, transaction_id = $10, updated_at = $11
		WHERE id = $12 AND user_id = $13
	`
	_, err := tx.Exec(ctx, query,
		inv.Type, inv.Name, inv.Ticker, inv.App, inv.AccountType, inv.AccountID,
		inv.Units, inv.BuyPrice, inv.Date, inv.TransactionID,
		inv.UpdatedAt, inv.ID, inv.UserID,
	)
	return err
}

func (r *investmentRepository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	query := `DELETE FROM investments WHERE id = $1 AND user_id = $2`
	_, err := r.db.Exec(ctx, query, id, userID)
	return err
}

func (r *investmentRepository) DeleteTx(ctx context.Context, tx pgx.Tx, id, userID uuid.UUID) error {
	query := `DELETE FROM investments WHERE id = $1 AND user_id = $2`
	_, err := tx.Exec(ctx, query, id, userID)
	return err
}
