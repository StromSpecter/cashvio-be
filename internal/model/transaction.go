package model

import (
	"time"

	"github.com/google/uuid"
)

type Transaction struct {
	ID          uuid.UUID `json:"id" db:"id"`
	UserID      uuid.UUID `json:"user_id" db:"user_id"`
	Name        string    `json:"name" db:"name"`
	Amount      float64   `json:"amount" db:"amount"`
	Type        string    `json:"type" db:"type"`
	Category    string    `json:"category" db:"category"`
	Status      string    `json:"status" db:"status"`
	AccountType string    `json:"account_type" db:"account_type"`
	AccountID   uuid.UUID `json:"account_id" db:"account_id"`
	Date        time.Time `json:"date" db:"date"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type CreateTransactionRequest struct {
	Name        string    `json:"name" binding:"required,min=2,max=100"`
	Amount      float64   `json:"amount" binding:"required,gt=0"`
	Type        string    `json:"type" binding:"required,oneof=income expense"`
	Category    string    `json:"category" binding:"required,oneof=income salary shopping groceries subscription travel transfer"`
	Status      string    `json:"status" binding:"omitempty,oneof=completed pending failed"`
	AccountType string    `json:"account_type" binding:"required,oneof=wallet card"`
	AccountID   uuid.UUID `json:"account_id" binding:"required"`
	Date        string    `json:"date" binding:"omitempty"`
}

type UpdateTransactionRequest struct {
	Name        string    `json:"name" binding:"omitempty,min=2,max=100"`
	Amount      float64   `json:"amount" binding:"omitempty,gt=0"`
	Type        string    `json:"type" binding:"omitempty,oneof=income expense"`
	Category    string    `json:"category" binding:"omitempty,oneof=income salary shopping groceries subscription travel transfer"`
	Status      string    `json:"status" binding:"omitempty,oneof=completed pending failed"`
	AccountType string     `json:"account_type" binding:"omitempty,oneof=wallet card"`
	AccountID   *uuid.UUID `json:"account_id" binding:"omitempty"`
	Date        string     `json:"date" binding:"omitempty"`
}

type TransactionQuery struct {
	Limit           int
	Offset          int
	Search          string
	SortBy          string
	Order           string
	Type            string
	Category        string
	Status          string
	ValidSortFields map[string]bool
}

func NewTransactionQuery() *TransactionQuery {
	return &TransactionQuery{
		Limit:  10,
		Offset: 0,
		SortBy: "date",
		Order:  "desc",
		ValidSortFields: map[string]bool{
			"date":        true,
			"amount":      true,
			"name":        true,
			"category":    true,
			"status":      true,
			"type":        true,
			"created_at":  true,
			"updated_at":  true,
		},
	}
}
