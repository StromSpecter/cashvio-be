package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/cashvio/cashvio-be/internal/model"
	"github.com/cashvio/cashvio-be/internal/repository"
	"github.com/cashvio/cashvio-be/internal/util"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type CardService interface {
	CreateCard(ctx context.Context, userID uuid.UUID, req *model.CreateCardRequest) (*model.Card, error)
	GetCardByID(ctx context.Context, id, userID uuid.UUID) (*model.Card, error)
	GetAllCards(ctx context.Context, q *model.CardQuery, userID uuid.UUID) ([]*model.Card, int, error)
	UpdateCard(ctx context.Context, id, userID uuid.UUID, req *model.UpdateCardRequest) (*model.Card, error)
	DeleteCard(ctx context.Context, id, userID uuid.UUID) error
}

type cardService struct {
	repo repository.CardRepository
}

func NewCardService(repo repository.CardRepository) CardService {
	return &cardService{repo: repo}
}

func ParseCardQuery(c_query *model.CardQuery, limit, offset, search, sortBy, order string) *model.CardQuery {
	c_query.Search = search
	c_query.SortBy = sortBy
	c_query.Order = order

	if l, err := strconv.Atoi(limit); err == nil {
		if l <= 0 {
			l = 10
		}
		if l > 100 {
			l = 100
		}
		c_query.Limit = l
	}

	if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
		c_query.Offset = o
	}

	if c_query.SortBy == "" {
		c_query.SortBy = "created_at"
	}
	if c_query.Order == "" {
		c_query.Order = "desc"
	}

	return c_query
}

func (s *cardService) CreateCard(ctx context.Context, userID uuid.UUID, req *model.CreateCardRequest) (*model.Card, error) {
	now := time.Now()
	card := &model.Card{
		ID:         uuid.New(),
		UserID:     userID,
		Bank:       req.Bank,
		Number:     strings.Join(strings.Fields(req.Number), ""),
		BalanceIDR: req.BalanceIDR,
		Gradient:   req.Gradient,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	card.Masked = util.MaskCardNumber(card.Number)

	if err := s.repo.Create(ctx, card); err != nil {
		return nil, errors.New("failed to create card")
	}

	return card, nil
}

func (s *cardService) GetCardByID(ctx context.Context, id, userID uuid.UUID) (*model.Card, error) {
	card, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("card not found")
		}
		return nil, errors.New("failed to get card")
	}
	return card, nil
}

func (s *cardService) GetAllCards(ctx context.Context, q *model.CardQuery, userID uuid.UUID) ([]*model.Card, int, error) {
	cards, err := s.repo.GetByUserID(ctx, q, userID)
	if err != nil {
		return nil, 0, errors.New("failed to retrieve cards")
	}
	total, err := s.repo.CountByUserID(ctx, q, userID)
	if err != nil {
		return nil, 0, errors.New("failed to count cards")
	}
	return cards, total, nil
}

func (s *cardService) UpdateCard(ctx context.Context, id, userID uuid.UUID, req *model.UpdateCardRequest) (*model.Card, error) {
	card, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("card not found")
		}
		return nil, errors.New("failed to get card")
	}

	if req.Bank != "" {
		card.Bank = req.Bank
	}
	if req.Number != "" {
		card.Number = strings.Join(strings.Fields(req.Number), "")
		card.Masked = util.MaskCardNumber(card.Number)
	}
	if req.BalanceIDR >= 0 {
		card.BalanceIDR = req.BalanceIDR
	}
	if req.Gradient != "" {
		card.Gradient = req.Gradient
	}

	if err := s.repo.Update(ctx, card); err != nil {
		return nil, errors.New("failed to update card")
	}

	return card, nil
}

func (s *cardService) DeleteCard(ctx context.Context, id, userID uuid.UUID) error {
	if _, err := s.repo.GetByID(ctx, id, userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("card not found")
		}
		return errors.New("failed to get card")
	}

	if err := s.repo.Delete(ctx, id, userID); err != nil {
		return errors.New("failed to delete card")
	}
	return nil
}
