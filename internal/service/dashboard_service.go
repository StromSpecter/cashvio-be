package service

import (
	"context"
	"errors"
	"time"

	"github.com/cashvio/cashvio-be/internal/model"
	"github.com/cashvio/cashvio-be/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type DashboardService interface {
	GetOverview(ctx context.Context, userID uuid.UUID) (*model.DashboardOverview, error)
}

type dashboardService struct {
	txnRepo    repository.TransactionRepository
	walletRepo repository.WalletRepository
	cardRepo   repository.CardRepository
	cashRepo   repository.CashRepository
}

func NewDashboardService(txnRepo repository.TransactionRepository, walletRepo repository.WalletRepository, cardRepo repository.CardRepository, cashRepo repository.CashRepository) DashboardService {
	return &dashboardService{txnRepo: txnRepo, walletRepo: walletRepo, cardRepo: cardRepo, cashRepo: cashRepo}
}

var categoryLabels = map[string]string{
	"income":         "Income",
	"transfer":       "Transfer",
	"salary":         "Salary",
	"freelance":      "Freelance",
	"gift":           "Gift",
	"bonus":          "Bonus",
	"food":           "Food & Drinks",
	"transportation": "Transportation",
	"housing":        "Housing",
	"shopping":       "Shopping",
	"entertainment":  "Entertainment",
	"health":         "Health",
	"education":      "Education",
	"groceries":      "Groceries",
	"subscription":   "Subscription",
	"travel":         "Travel",
	"investment":     "Investment",
}

func (s *dashboardService) GetOverview(ctx context.Context, userID uuid.UUID) (*model.DashboardOverview, error) {
	now := time.Now()
	loc := now.Location()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	monthEnd := monthStart.AddDate(0, 1, 0)
	lastMonthStart := monthStart.AddDate(0, -1, 0)
	lastMonthEnd := monthStart
	dayEnd := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, 1)

	income, err := s.txnRepo.SumIncomeByPeriod(ctx, userID, monthStart, monthEnd)
	if err != nil {
		return nil, errors.New("failed to compute income")
	}
	expense, err := s.txnRepo.SumExpenseByPeriod(ctx, userID, monthStart, monthEnd)
	if err != nil {
		return nil, errors.New("failed to compute expense")
	}
	incomePrev, err := s.txnRepo.SumIncomeByPeriod(ctx, userID, lastMonthStart, lastMonthEnd)
	if err != nil {
		return nil, errors.New("failed to compute previous income")
	}
	expensePrev, err := s.txnRepo.SumExpenseByPeriod(ctx, userID, lastMonthStart, lastMonthEnd)
	if err != nil {
		return nil, errors.New("failed to compute previous expense")
	}

	wallets, err := s.walletRepo.GetByUserID(ctx, s.walletQuery(), userID)
	if err != nil {
		return nil, errors.New("failed to retrieve wallets")
	}
	cards, err := s.cardRepo.GetByUserID(ctx, s.cardQuery(), userID)
	if err != nil {
		return nil, errors.New("failed to retrieve cards")
	}

	cash := &model.Cash{}
	if c, err := s.cashRepo.GetByUserID(ctx, userID); err == nil {
		cash = c
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("failed to retrieve cash balance")
	}

	var totalBalance float64
	for _, w := range wallets {
		totalBalance += w.BalanceIDR
	}
	for _, c := range cards {
		totalBalance += c.BalanceIDR
	}
	totalBalance += cash.BalanceIDR

	expense = absAmount(expense)
	expensePrev = absAmount(expensePrev)
	savings := income - expense
	savingsPrev := incomePrev - expensePrev
	savingsRate := 0.0
	if income > 0 {
		savingsRate = savings / income * 100
	}

	balanceOverview := map[string][]*model.FlowPoint{
		"7d":  s.flowSeries(ctx, userID, dayEnd.AddDate(0, 0, -7), dayEnd, "day"),
		"30d": s.flowSeries(ctx, userID, dayEnd.AddDate(0, 0, -30), dayEnd, "day"),
		"90d": s.flowSeries(ctx, userID, monthStart.AddDate(0, -3, 0), dayEnd, "month"),
	}

	spending := []*model.CategorySpending{}
	catRows, err := s.txnRepo.ExpenseByCategory(ctx, userID, monthStart, monthEnd)
	if err != nil {
		return nil, errors.New("failed to compute spending by category")
	}
	var spendingTotal float64
	for _, row := range catRows {
		row.Amount = absAmount(row.Amount)
		spendingTotal += row.Amount
	}
	for _, row := range catRows {
		if spendingTotal > 0 {
			row.Percentage = row.Amount / spendingTotal * 100
		}
		if row.Label == "" {
			row.Label = categoryLabels[row.Category]
		}
		if row.Label == "" {
			row.Label = row.Category
		}
		spending = append(spending, row)
	}

	cashFlow := s.flowSeries(ctx, userID, monthStart.AddDate(0, -5, 0), monthEnd, "month")

	recent := []*model.Transaction{}
	{
		q := model.NewTransactionQuery()
		q.Limit = 5
		q.SortBy = "date"
		q.Order = "desc"
		if rows, err := s.txnRepo.GetByUserID(ctx, q, userID); err == nil {
			recent = rows
		}
	}

	largest, _ := s.txnRepo.MaxExpenseByPeriod(ctx, userID, monthStart, monthEnd)

	return &model.DashboardOverview{
		TotalBalance: totalBalance,
		Income:       income,
		Expense:      expense,
		Savings:      savings,
		SavingsRate:  savingsRate,
		Changes: model.DashboardChanges{
			Income:  pctChange(income, incomePrev),
			Expense: pctChange(expense, expensePrev),
			Savings: pctChange(savings, savingsPrev),
		},
		Balance:  balanceOverview,
		Spending: spending,
		CashFlow: cashFlow,
		Accounts: model.DashboardAccounts{
			Wallets: wallets,
			Cards:   cards,
			Cash:    cash,
		},
		Recent:  recent,
		Largest: largest,
	}, nil
}

func (s *dashboardService) flowSeries(ctx context.Context, userID uuid.UUID, start, end time.Time, trunc string) []*model.FlowPoint {
	rows, err := s.txnRepo.FlowSeries(ctx, userID, start, end, trunc)
	if err != nil {
		return []*model.FlowPoint{}
	}

	byBucket := make(map[string]*model.FlowPoint)
	for _, r := range rows {
		key := r.Bucket.Format(time.RFC3339)
		byBucket[key] = &model.FlowPoint{
			Label:   bucketLabel(r.Bucket, trunc),
			Income:  r.Income,
			Expense: absAmount(r.Expense),
		}
	}

	points := make([]*model.FlowPoint, 0)
	for cur := start; cur.Before(end); cur = nextBucket(cur, trunc) {
		key := cur.Format(time.RFC3339)
		if p, ok := byBucket[key]; ok {
			points = append(points, p)
			continue
		}
		points = append(points, &model.FlowPoint{
			Label:   bucketLabel(cur, trunc),
			Income:  0,
			Expense: 0,
		})
	}
	return points
}

func nextBucket(t time.Time, trunc string) time.Time {
	if trunc == "month" {
		return time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())
	}
	return t.AddDate(0, 0, 1)
}

func bucketLabel(t time.Time, trunc string) string {
	if trunc == "month" {
		return t.Format("Jan")
	}
	return t.Format("Jan 2")
}

func pctChange(current, previous float64) float64 {
	if previous == 0 {
		return 0
	}
	return (current - previous) / previous * 100
}

func (s *dashboardService) walletQuery() *model.WalletQuery {
	q := model.NewWalletQuery()
	q.Limit = 100
	q.SortBy = "created_at"
	q.Order = "asc"
	return q
}

func (s *dashboardService) cardQuery() *model.CardQuery {
	q := model.NewCardQuery()
	q.Limit = 100
	q.SortBy = "created_at"
	q.Order = "asc"
	return q
}
