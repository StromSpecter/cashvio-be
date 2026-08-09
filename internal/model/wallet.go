package model

import (
	"time"

	"github.com/google/uuid"
)

type Wallet struct {
	ID         uuid.UUID `json:"id" db:"id"`
	UserID     uuid.UUID `json:"user_id" db:"user_id"`
	Name       string    `json:"name" db:"name"`
	Number     string    `json:"number,omitempty" db:"number"`
	Masked     string    `json:"masked,omitempty" db:"masked"`
	BalanceIDR float64   `json:"balance_idr" db:"balance_idr"`
	Tone       string    `json:"tone,omitempty" db:"tone"`
	Status     string    `json:"status" db:"status"`
	Primary    bool      `json:"primary" db:"primary"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

type CreateWalletRequest struct {
	Name       string  `json:"name" binding:"required,min=2,max=100"`
	Number     string  `json:"number,omitempty" binding:"omitempty,max=50"`
	BalanceIDR float64 `json:"balance_idr" binding:"min=0"`
	Tone       string  `json:"tone,omitempty" binding:"omitempty,max=200"`
	Primary    bool    `json:"primary"`
}

type UpdateWalletRequest struct {
	Name       string  `json:"name,omitempty" binding:"omitempty,min=2,max=100"`
	Number     string  `json:"number,omitempty" binding:"omitempty,max=50"`
	BalanceIDR float64 `json:"balance_idr,omitempty" binding:"omitempty,min=0"`
	Tone       string  `json:"tone,omitempty" binding:"omitempty,max=200"`
	Status     string  `json:"status,omitempty" binding:"omitempty,oneof=active inactive"`
	Primary    *bool   `json:"primary,omitempty"`
}

type WalletQuery struct {
	Limit           int
	Offset          int
	Search          string
	SortBy          string
	Order           string
	ValidSortFields map[string]bool
}

func NewWalletQuery() *WalletQuery {
	return &WalletQuery{
		Limit:  10,
		Offset: 0,
		SortBy: "created_at",
		Order:  "desc",
		ValidSortFields: map[string]bool{
			"created_at":  true,
			"updated_at":  true,
			"name":        true,
			"balance_idr": true,
			"number":      true,
			"status":      true,
		},
	}
}
