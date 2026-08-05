package model

import (
	"time"

	"github.com/google/uuid"
)

type Card struct {
	ID        uuid.UUID `json:"id" db:"id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	Bank      string    `json:"bank" db:"bank"`
	Number    string    `json:"number" db:"number"`
	Masked    string    `json:"masked" db:"masked"`
	BalanceIDR float64   `json:"balance_idr" db:"balance_idr"`
	Gradient  string    `json:"gradient" db:"gradient"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type CreateCardRequest struct {
	Bank      string  `json:"bank" binding:"required,min=2,max=100"`
	Number    string  `json:"number" binding:"required,min=1,max=50"`
	BalanceIDR float64 `json:"balance_idr" binding:"min=0"`
	Gradient  string  `json:"gradient" binding:"omitempty,max=100"`
}

type UpdateCardRequest struct {
	Bank       string  `json:"bank" binding:"omitempty,min=2,max=100"`
	Number     string  `json:"number" binding:"omitempty,min=1,max=50"`
	BalanceIDR float64 `json:"balance_idr" binding:"omitempty,min=0"`
	Gradient   string  `json:"gradient" binding:"omitempty,max=100"`
}

type CardQuery struct {
	Limit   int
	Offset  int
	Search  string
	SortBy  string
	Order   string
	ValidSortFields map[string]bool
}

func NewCardQuery() *CardQuery {
	return &CardQuery{
		Limit: 10,
		Offset: 0,
		SortBy: "created_at",
		Order:  "desc",
		ValidSortFields: map[string]bool{
			"created_at":   true,
			"updated_at":   true,
			"bank":         true,
			"balance_idr":  true,
			"number":       true,
		},
	}
}
