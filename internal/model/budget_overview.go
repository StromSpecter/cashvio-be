package model

type BudgetOverview struct {
	Income         float64           `json:"income"`
	Spent          float64           `json:"spent"`
	Remaining      float64           `json:"remaining"`
	Unallocated    float64           `json:"unallocated"`
	TotalAllocated float64           `json:"total_allocated"`
	AllocatedPct   float64           `json:"allocated_pct"`
	Categories     []*CategoryBudget `json:"categories"`
}
