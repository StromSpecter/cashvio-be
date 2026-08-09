package model

import (
	"time"

	"github.com/google/uuid"
)

type Transfer struct {
	ID        uuid.UUID `json:"id" db:"id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	FromType  string    `json:"from_type" db:"from_type"`
	FromID    uuid.UUID `json:"from_id" db:"from_id"`
	ToType    string    `json:"to_type" db:"to_type"`
	ToID      uuid.UUID `json:"to_id" db:"to_id"`
	Amount    float64   `json:"amount" db:"amount"`
	Fee       float64   `json:"fee" db:"fee"`
	Note      string    `json:"note,omitempty" db:"note"`
	Date      time.Time `json:"date" db:"date"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type CreateTransferRequest struct {
	FromType string    `json:"from_type" binding:"required,oneof=wallet card"`
	FromID   uuid.UUID `json:"from_id" binding:"required"`
	ToType   string    `json:"to_type" binding:"required,oneof=wallet card"`
	ToID     uuid.UUID `json:"to_id" binding:"required"`
	Amount   float64   `json:"amount" binding:"required,gt=0"`
	Fee      float64   `json:"fee" binding:"omitempty,min=0"`
	Note     string    `json:"note" binding:"omitempty,max=255"`
	Date     string    `json:"date" binding:"omitempty"`
}

type TransferQuery struct {
	Limit           int
	Offset          int
	Search          string
	SortBy          string
	Order           string
	ValidSortFields map[string]bool
}

func NewTransferQuery() *TransferQuery {
	return &TransferQuery{
		Limit:  10,
		Offset: 0,
		SortBy: "date",
		Order:  "desc",
		ValidSortFields: map[string]bool{
			"date":       true,
			"amount":     true,
			"note":       true,
			"created_at": true,
			"updated_at": true,
		},
	}
}
