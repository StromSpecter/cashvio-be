package service

import (
	"context"
	"errors"
	"time"

	"github.com/cashvio/cashvio-be/internal/model"
	"github.com/cashvio/cashvio-be/internal/payment"
	"github.com/cashvio/cashvio-be/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type PremiumService interface {
	GetPlans() []model.PremiumPlan
	CreateOrder(ctx context.Context, userID uuid.UUID, planCode string) (*model.PremiumOrder, error)
	GetOrder(ctx context.Context, userID, orderID uuid.UUID) (*model.PremiumOrder, error)
	ListOrders(ctx context.Context, userID uuid.UUID, limit int) ([]*model.PremiumOrder, error)
	MarkPaidByExternalID(ctx context.Context, externalID string) (*model.PremiumOrder, error)
	SimulatePaid(ctx context.Context, userID, orderID uuid.UUID) (*model.PremiumOrder, error)
}

type premiumService struct {
	repo     repository.PremiumRepository
	userRepo repository.UserRepository
	provider payment.Provider
	orderTTL time.Duration
}

func NewPremiumService(repo repository.PremiumRepository, userRepo repository.UserRepository, provider payment.Provider) PremiumService {
	return &premiumService{
		repo:     repo,
		userRepo: userRepo,
		provider: provider,
		orderTTL: time.Hour,
	}
}

func (s *premiumService) GetPlans() []model.PremiumPlan {
	return model.PremiumPlans
}

func (s *premiumService) CreateOrder(ctx context.Context, userID uuid.UUID, planCode string) (*model.PremiumOrder, error) {
	plan, ok := model.FindPremiumPlan(planCode)
	if !ok {
		return nil, errors.New("invalid plan")
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, errors.New("failed to get user")
	}

	now := time.Now()
	expiresAt := now.Add(s.orderTTL)
	externalID := fmtExternalID()
	order := &model.PremiumOrder{
		ID:           uuid.New(),
		UserID:       userID,
		Plan:         plan.Code,
		Amount:       plan.PriceIDR,
		Currency:     "IDR",
		DurationDays: plan.DurationDays,
		ExternalID:   externalID,
		Status:       model.OrderPending,
		ExpiresAt:    &expiresAt,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	result, err := s.provider.CreatePayment(ctx, payment.CreatePaymentRequest{
		ExternalID:    externalID,
		Amount:        plan.PriceIDR,
		Currency:      "IDR",
		CustomerName:  user.Name,
		CustomerEmail: user.Email,
	})
	if err != nil {
		return nil, errors.New("failed to create payment")
	}

	order.QRISString = result.QRISString
	order.QRISImageURL = result.QRISImageURL
	order.IsMock = s.provider.Name() == "mock"

	if err := s.repo.Create(ctx, order); err != nil {
		return nil, errors.New("failed to save order")
	}

	return order, nil
}

func (s *premiumService) GetOrder(ctx context.Context, userID, orderID uuid.UUID) (*model.PremiumOrder, error) {
	order, err := s.repo.GetByID(ctx, orderID, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("order not found")
		}
		return nil, errors.New("failed to get order")
	}
	order.IsMock = s.provider.Name() == "mock"
	return order, nil
}

func (s *premiumService) ListOrders(ctx context.Context, userID uuid.UUID, limit int) ([]*model.PremiumOrder, error) {
	orders, err := s.repo.ListByUser(ctx, userID, limit)
	if err != nil {
		return nil, errors.New("failed to list orders")
	}
	for _, o := range orders {
		o.IsMock = s.provider.Name() == "mock"
	}
	return orders, nil
}

func (s *premiumService) MarkPaidByExternalID(ctx context.Context, externalID string) (*model.PremiumOrder, error) {
	order, err := s.repo.GetByExternalID(ctx, externalID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("order not found")
		}
		return nil, errors.New("failed to get order")
	}

	return s.grantPremium(ctx, order)
}

func (s *premiumService) SimulatePaid(ctx context.Context, userID, orderID uuid.UUID) (*model.PremiumOrder, error) {
	if s.provider.Name() != "mock" {
		return nil, errors.New("simulation only available with mock provider")
	}

	order, err := s.repo.GetByID(ctx, orderID, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("order not found")
		}
		return nil, errors.New("failed to get order")
	}

	return s.grantPremium(ctx, order)
}

func (s *premiumService) grantPremium(ctx context.Context, order *model.PremiumOrder) (*model.PremiumOrder, error) {
	if order.Status == model.OrderPaid {
		return order, nil
	}

	user, err := s.userRepo.GetByID(ctx, order.UserID)
	if err != nil {
		return nil, errors.New("failed to get user")
	}

	user.SetPremium(order.DurationDays)
	if err := s.userRepo.UpdatePremium(ctx, user.ID, user.Role, user.PremiumExpiresAt); err != nil {
		return nil, errors.New("failed to activate premium")
	}

	now := time.Now()
	order.Status = model.OrderPaid
	order.PaidAt = &now
	order.PremiumExpiresAt = user.PremiumExpiresAt
	if err := s.repo.MarkPaid(ctx, order); err != nil {
		return nil, errors.New("failed to update order")
	}

	order.IsMock = s.provider.Name() == "mock"
	return order, nil
}

func fmtExternalID() string {
	return "PREMIUM-" + uuid.New().String()
}
