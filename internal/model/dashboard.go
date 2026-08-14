package model

import "time"

type DashboardOverview struct {
	TotalBalance float64                 `json:"total_balance"`
	Income       float64                 `json:"income"`
	Expense      float64                 `json:"expense"`
	Savings      float64                 `json:"savings"`
	SavingsRate  float64                 `json:"savings_rate"`
	Changes      DashboardChanges        `json:"changes"`
	Balance      map[string][]*FlowPoint `json:"balance_overview"`
	Spending     []*CategorySpending     `json:"spending"`
	CashFlow     []*FlowPoint            `json:"cash_flow"`
	Accounts     DashboardAccounts       `json:"accounts"`
	Recent       []*Transaction          `json:"recent_transactions"`
	Largest      *Transaction            `json:"largest_expense"`
}

type DashboardChanges struct {
	Income  float64 `json:"income"`
	Expense float64 `json:"expense"`
	Savings float64 `json:"savings"`
}

type FlowPoint struct {
	Label   string  `json:"label"`
	Income  float64 `json:"income"`
	Expense float64 `json:"expense"`
}

type FlowRow struct {
	Bucket  time.Time
	Income  float64
	Expense float64
}

type CategorySpending struct {
	Category   string  `json:"category"`
	Label      string  `json:"label"`
	Amount     float64 `json:"amount"`
	Percentage float64 `json:"percentage"`
}

type DashboardAccounts struct {
	Wallets []*Wallet `json:"wallets"`
	Cards   []*Card   `json:"cards"`
	Cash    *Cash     `json:"cash"`
}
