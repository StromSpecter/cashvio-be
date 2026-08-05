package service

import (
	"context"
	"errors"
	"time"

	"github.com/cashvio/cashvio-be/internal/config"
	"github.com/cashvio/cashvio-be/internal/model"
	"github.com/cashvio/cashvio-be/internal/repository"
	"github.com/cashvio/cashvio-be/internal/util"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type UserService interface {
	CreateUser(ctx context.Context, req *model.CreateUserRequest) (*model.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	GetAllUsers(ctx context.Context, limit, offset int) ([]*model.User, error)
	UpdateUser(ctx context.Context, id uuid.UUID, req *model.UpdateUserRequest) (*model.User, error)
	DeleteUser(ctx context.Context, id uuid.UUID) error
	Login(ctx context.Context, req *model.LoginRequest) (*model.LoginResponse, error)
}

type userService struct {
	repo   repository.UserRepository
	config *config.Config
}

func NewUserService(repo repository.UserRepository, cfg *config.Config) UserService {
	return &userService{repo: repo, config: cfg}
}

func (s *userService) CreateUser(ctx context.Context, req *model.CreateUserRequest) (*model.User, error) {
	existing, _ := s.repo.GetByEmail(ctx, req.Email)
	if existing != nil {
		return nil, errors.New("email already registered")
	}

	hashedPassword, err := util.HashPassword(req.Password)
	if err != nil {
		return nil, errors.New("failed to hash password")
	}

	now := time.Now()
	user := &model.User{
		ID:        uuid.New(),
		Name:      req.Name,
		Email:     req.Email,
		Password:  hashedPassword,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, errors.New("failed to create user")
	}

	return user, nil
}

func (s *userService) GetUserByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		return nil, errors.New("failed to get user")
	}
	return user, nil
}

func (s *userService) GetAllUsers(ctx context.Context, limit, offset int) ([]*model.User, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	users, err := s.repo.GetAll(ctx, limit, offset)
	if err != nil {
		return nil, errors.New("failed to retrieve users")
	}
	return users, nil
}

func (s *userService) UpdateUser(ctx context.Context, id uuid.UUID, req *model.UpdateUserRequest) (*model.User, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("user not found")
		}
		return nil, errors.New("failed to get user")
	}

	if req.Name != "" {
		user.Name = req.Name
	}
	if req.Email != "" {
		existing, _ := s.repo.GetByEmail(ctx, req.Email)
		if existing != nil && existing.ID != user.ID {
			return nil, errors.New("email already in use")
		}
		user.Email = req.Email
	}
	if req.Password != "" {
		hashedPassword, err := util.HashPassword(req.Password)
		if err != nil {
			return nil, errors.New("failed to hash password")
		}
		user.Password = hashedPassword
	}

	if err := s.repo.Update(ctx, user); err != nil {
		return nil, errors.New("failed to update user")
	}

	return user, nil
}

func (s *userService) DeleteUser(ctx context.Context, id uuid.UUID) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("user not found")
		}
		return errors.New("failed to get user")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return errors.New("failed to delete user")
	}
	return nil
}

func (s *userService) Login(ctx context.Context, req *model.LoginRequest) (*model.LoginResponse, error) {
	user, err := s.repo.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("invalid email or password")
		}
		return nil, errors.New("failed to get user")
	}

	if err := util.ComparePassword(user.Password, req.Password); err != nil {
		return nil, errors.New("invalid email or password")
	}

	token, err := util.GenerateJWT(user.ID.String(), user.Email, s.config.JWT.Secret, s.config.JWT.ExpiresIn)
	if err != nil {
		return nil, errors.New("failed to generate token")
	}

	return &model.LoginResponse{
		Token:     token,
		ExpiresIn: s.config.JWT.ExpiresIn * 3600,
	}, nil
}
