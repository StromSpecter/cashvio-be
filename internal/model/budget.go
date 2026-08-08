package model

import (
	"time"

	"github.com/google/uuid"
)

type Budget struct {
	ID        uuid.UUID `json:"id" db:"id"`
	UserID    uuid.UUID `json:"user_id" db:"user_id"`
	Amount    float64   `json:"amount" db:"amount"`
	Note      string    `json:"note,omitempty" db:"note"`
	Month     string    `json:"month,omitempty" db:"month"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type CreateBudgetRequest struct {
	Amount float64 `json:"amount" binding:"required,gt=0"`
	Note   string  `json:"note" binding:"omitempty,max=255"`
	Month  string  `json:"month" binding:"omitempty,max=20"`
}

type UpdateBudgetRequest struct {
	Amount float64 `json:"amount" binding:"omitempty,gt=0"`
	Note   string  `json:"note" binding:"omitempty,max=255"`
	Month  string  `json:"month" binding:"omitempty,max=20"`
}

type BudgetQuery struct {
	Limit           int
	Offset          int
	Search          string
	SortBy          string
	Order           string
	ValidSortFields map[string]bool
}

type BudgetOverview struct {
	Budget         *Budget           `json:"budget,omitempty"`
	Income         float64           `json:"income"`
	Spent          float64           `json:"spent"`
	Remaining      float64           `json:"remaining"`
	Unallocated    float64           `json:"unallocated"`
	TotalAllocated float64           `json:"total_allocated"`
	AllocatedPct   float64           `json:"allocated_pct"`
	Categories     []*CategoryBudget `json:"categories"`
}

func NewBudgetQuery() *BudgetQuery {
	return &BudgetQuery{
		Limit:  10,
		Offset: 0,
		SortBy: "created_at",
		Order:  "desc",
		ValidSortFields: map[string]bool{
			"created_at": true,
			"updated_at": true,
			"amount":     true,
			"note":       true,
			"month":      true,
		},
	}
}
