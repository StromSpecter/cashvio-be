package model

import (
	"time"

	"github.com/google/uuid"
)

type CategoryBudget struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	UserID    uuid.UUID  `json:"user_id" db:"user_id"`
	BudgetID  *uuid.UUID `json:"budget_id,omitempty" db:"budget_id"`
	Name      string     `json:"name" db:"name"`
	Type      string     `json:"type" db:"type"`
	Percent   *float64   `json:"percent,omitempty" db:"percent"`
	Amount    *float64   `json:"amount,omitempty" db:"amount"`
	Color     int        `json:"color" db:"color"`
	Icon      string     `json:"icon" db:"icon"`
	Desc      string     `json:"desc,omitempty" db:"desc"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" db:"updated_at"`
}

type CreateCategoryBudgetRequest struct {
	BudgetID *uuid.UUID `json:"budget_id" binding:"omitempty"`
	Name     string     `json:"name" binding:"required,min=1,max=100"`
	Type     string     `json:"type" binding:"required,oneof=percent amount"`
	Percent  *float64   `json:"percent" binding:"omitempty,min=0,max=100"`
	Amount   *float64   `json:"amount" binding:"omitempty,min=0"`
	Color    *int       `json:"color" binding:"omitempty,min=1,max=4"`
	Icon     string     `json:"icon" binding:"omitempty,max=50"`
	Desc     string     `json:"desc" binding:"omitempty,max=255"`
}

type UpdateCategoryBudgetRequest struct {
	BudgetID *uuid.UUID `json:"budget_id" binding:"omitempty"`
	Name     string     `json:"name" binding:"omitempty,min=1,max=100"`
	Type     string     `json:"type" binding:"omitempty,oneof=percent amount"`
	Percent  *float64   `json:"percent" binding:"omitempty,min=0,max=100"`
	Amount   *float64   `json:"amount" binding:"omitempty,min=0"`
	Color    *int       `json:"color" binding:"omitempty,min=1,max=4"`
	Icon     string     `json:"icon" binding:"omitempty,max=50"`
	Desc     string     `json:"desc" binding:"omitempty,max=255"`
}

type CategoryBudgetQuery struct {
	Limit           int
	Offset          int
	Search          string
	SortBy          string
	Order           string
	BudgetID        *uuid.UUID
	Type            string
	ValidSortFields map[string]bool
}

func NewCategoryBudgetQuery() *CategoryBudgetQuery {
	return &CategoryBudgetQuery{
		Limit:  10,
		Offset: 0,
		SortBy: "created_at",
		Order:  "desc",
		ValidSortFields: map[string]bool{
			"created_at": true,
			"updated_at": true,
			"name":       true,
			"type":       true,
			"percent":    true,
			"amount":     true,
		},
	}
}
