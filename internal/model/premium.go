package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	PlanMonthly = "monthly"
	PlanYearly  = "yearly"
)

const (
	OrderPending = "pending"
	OrderPaid    = "paid"
	OrderExpired = "expired"
	OrderFailed  = "failed"
)

type PremiumPlan struct {
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	PriceIDR     float64 `json:"price_idr"`
	DurationDays int     `json:"duration_days"`
	Badge        string  `json:"badge,omitempty"`
}

var PremiumPlans = []PremiumPlan{
	{
		Code:         PlanMonthly,
		Name:         "Monthly",
		Description:  "Akses investment group penuh selama 30 hari.",
		PriceIDR:     49000,
		DurationDays: 30,
	},
	{
		Code:         PlanYearly,
		Name:         "Yearly",
		Description:  "Akses investment group penuh selama 365 hari. Hemat 2 bulan.",
		PriceIDR:     469000,
		DurationDays: 365,
		Badge:        "Best Value",
	},
}

func FindPremiumPlan(code string) (*PremiumPlan, bool) {
	for i := range PremiumPlans {
		if PremiumPlans[i].Code == code {
			return &PremiumPlans[i], true
		}
	}
	return nil, false
}

type CreatePremiumOrderRequest struct {
	Plan string `json:"plan" binding:"required"`
}

type PremiumOrder struct {
	ID               uuid.UUID  `json:"id" db:"id"`
	UserID           uuid.UUID  `json:"user_id" db:"user_id"`
	Plan             string     `json:"plan" db:"plan"`
	Amount           float64    `json:"amount" db:"amount"`
	Currency         string     `json:"currency" db:"currency"`
	DurationDays     int        `json:"duration_days" db:"duration_days"`
	ExternalID       string     `json:"external_id" db:"external_id"`
	QRISString       string     `json:"qris_string" db:"qris_string"`
	QRISImageURL     string     `json:"qris_image_url" db:"qris_image_url"`
	Status           string     `json:"status" db:"status"`
	IsMock           bool       `json:"is_mock" db:"-"`
	PaidAt           *time.Time `json:"paid_at,omitempty" db:"paid_at"`
	PremiumExpiresAt *time.Time `json:"premium_expires_at,omitempty" db:"premium_expires_at"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty" db:"expires_at"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
}
