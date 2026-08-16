package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/cashvio/cashvio-be/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type StockPriceRepository interface {
	UpsertMany(ctx context.Context, prices []*model.StockPrice) error
	// GetLatestBySymbols returns the most recent row per symbol.
	GetLatestBySymbols(ctx context.Context, symbols []string) ([]*model.StockPrice, error)
	// ExistsToday returns true when a row exists for the symbol fetched today
	// or dated today.
	ExistsToday(ctx context.Context, symbol string, today string) (bool, error)
}

type stockPriceRepository struct {
	db *pgxpool.Pool
}

func NewStockPriceRepository(db *pgxpool.Pool) StockPriceRepository {
	return &stockPriceRepository{db: db}
}

func (r *stockPriceRepository) UpsertMany(ctx context.Context, prices []*model.StockPrice) error {
	if len(prices) == 0 {
		return nil
	}

	var sb strings.Builder
	sb.WriteString(`INSERT INTO stock_prices (id, symbol, date, open, high, low, close, volume, change, change_pct, fetched_at) VALUES `)

	args := make([]interface{}, 0, len(prices)*11)
	pos := 1
	for _, p := range prices {
		if pos > 1 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)", pos, pos+1, pos+2, pos+3, pos+4, pos+5, pos+6, pos+7, pos+8, pos+9, pos+10))
		pos += 11
		args = append(args,
			p.ID, p.Symbol, p.Date, p.Open, p.High, p.Low, p.Close,
			p.Volume, p.Change, p.ChangePct, p.FetchedAt,
		)
	}

	sb.WriteString(` ON CONFLICT (symbol, date) DO UPDATE SET
		open = EXCLUDED.open,
		high = EXCLUDED.high,
		low = EXCLUDED.low,
		close = EXCLUDED.close,
		volume = EXCLUDED.volume,
		change = EXCLUDED.change,
		change_pct = EXCLUDED.change_pct,
		fetched_at = EXCLUDED.fetched_at`)

	_, err := r.db.Exec(ctx, sb.String(), args...)
	return err
}

func (r *stockPriceRepository) GetLatestBySymbols(ctx context.Context, symbols []string) ([]*model.StockPrice, error) {
	if len(symbols) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(symbols))
	args := make([]interface{}, len(symbols))
	for i, s := range symbols {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = s
	}

	query := fmt.Sprintf(`
		SELECT id, symbol, date, open, high, low, close, volume, change, change_pct, fetched_at
		FROM stock_prices
		WHERE symbol IN (%s)
		  AND date = (SELECT MAX(date) FROM stock_prices sp WHERE sp.symbol = stock_prices.symbol)
	`, strings.Join(placeholders, ","))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	prices := make([]*model.StockPrice, 0)
	for rows.Next() {
		p := &model.StockPrice{}
		if err := rows.Scan(
			&p.ID, &p.Symbol, &p.Date, &p.Open, &p.High, &p.Low, &p.Close,
			&p.Volume, &p.Change, &p.ChangePct, &p.FetchedAt,
		); err != nil {
			return nil, err
		}
		prices = append(prices, p)
	}
	return prices, nil
}

func (r *stockPriceRepository) ExistsToday(ctx context.Context, symbol string, today string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM stock_prices
			WHERE symbol = $1
			AND (to_char(fetched_at, 'YYYY-MM-DD') = $2 OR to_char(date, 'YYYY-MM-DD') = $2)
		)`,
		symbol, today,
	).Scan(&exists)
	return exists, err
}
