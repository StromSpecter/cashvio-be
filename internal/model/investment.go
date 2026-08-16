package model

import (
	"time"

	"github.com/google/uuid"
)

type Investment struct {
	ID            uuid.UUID `json:"id" db:"id"`
	UserID        uuid.UUID `json:"user_id" db:"user_id"`
	Type          string    `json:"type" db:"type"`
	Name          string    `json:"name" db:"name"`
	Ticker        string    `json:"ticker" db:"ticker"`
	App           string    `json:"app" db:"app"`
	AccountType   string      `json:"account_type" db:"account_type"`
	AccountID     *uuid.UUID  `json:"account_id" db:"account_id"`
	Units         float64     `json:"units" db:"units"`
	BuyPrice      float64     `json:"buy_price" db:"buy_price"`
	Date          time.Time   `json:"date" db:"date"`
	TransactionID *uuid.UUID  `json:"transaction_id" db:"transaction_id"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

type CreateInvestmentRequest struct {
	Type        string     `json:"type" binding:"required,oneof=stock mutual_fund bond gold crypto forex"`
	Name        string     `json:"name" binding:"required,min=2,max=100"`
	Ticker      string     `json:"ticker" binding:"omitempty,max=20"`
	App         string     `json:"app" binding:"omitempty,max=50"`
	AccountType string     `json:"account_type" binding:"omitempty,oneof=wallet card cash"`
	AccountID   *uuid.UUID `json:"account_id" binding:"omitempty"`
	Units       float64    `json:"units" binding:"required,gt=0"`
	BuyPrice    float64    `json:"buy_price" binding:"required,gt=0"`
	Date        string     `json:"date" binding:"omitempty"`
}

type UpdateInvestmentRequest struct {
	Type        string     `json:"type" binding:"omitempty,oneof=stock mutual_fund bond gold crypto forex"`
	Name        string     `json:"name" binding:"omitempty,min=2,max=100"`
	Ticker      string     `json:"ticker" binding:"omitempty,max=20"`
	App         string     `json:"app" binding:"omitempty,max=50"`
	AccountType *string    `json:"account_type" binding:"omitempty,oneof=wallet card cash"`
	AccountID   *uuid.UUID `json:"account_id" binding:"omitempty"`
	Units       float64    `json:"units" binding:"omitempty,gt=0"`
	BuyPrice    float64    `json:"buy_price" binding:"omitempty,gt=0"`
	Date        string     `json:"date" binding:"omitempty"`
}

type InvestmentQuery struct {
	Limit           int
	Offset          int
	Search          string
	SortBy          string
	Order           string
	Type            string
	ValidSortFields map[string]bool
}

func NewInvestmentQuery() *InvestmentQuery {
	return &InvestmentQuery{
		Limit:  10,
		Offset: 0,
		SortBy: "date",
		Order:  "desc",
		ValidSortFields: map[string]bool{
			"date":       true,
			"name":       true,
			"type":       true,
			"units":      true,
			"buy_price":  true,
			"created_at": true,
			"updated_at": true,
		},
	}
}

type StockPrice struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Symbol    string    `json:"symbol" db:"symbol"`
	Date      time.Time `json:"date" db:"date"`
	Open      float64   `json:"open" db:"open"`
	High      float64   `json:"high" db:"high"`
	Low       float64   `json:"low" db:"low"`
	Close     float64   `json:"close" db:"close"`
	Volume    int64     `json:"volume" db:"volume"`
	Change    float64   `json:"change" db:"change"`
	ChangePct float64   `json:"change_pct" db:"change_pct"`
	FetchedAt time.Time `json:"fetched_at" db:"fetched_at"`
}

func (s *StockPrice) DateString() string {
	return s.Date.Format("2006-01-02")
}

type InvestmentPrice struct {
	Symbol    string  `json:"symbol"`
	Name      string  `json:"name"`
	Date      string  `json:"date"`
	Price     float64 `json:"price"`
	Change    float64 `json:"change"`
	ChangePct float64 `json:"change_pct"`
	Stale     bool    `json:"stale"`
}
