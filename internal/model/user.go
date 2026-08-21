package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	RoleFree    = "free"
	RolePremium = "premium"
)

type User struct {
	ID               uuid.UUID  `json:"id" db:"id"`
	Name             string     `json:"name" db:"name"`
	Email            string     `json:"email" db:"email"`
	Password         string     `json:"-" db:"password"`
	Role             string     `json:"role" db:"role"`
	PremiumExpiresAt *time.Time `json:"premium_expires_at,omitempty" db:"premium_expires_at"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at" db:"updated_at"`
}

func (u *User) IsPremium() bool {
	if u.Role != RolePremium {
		return false
	}
	if u.PremiumExpiresAt == nil {
		return true
	}
	return u.PremiumExpiresAt.After(time.Now())
}

func (u *User) SetPremium(durationDays int) {
	base := time.Now()
	if u.PremiumExpiresAt != nil && u.PremiumExpiresAt.After(base) {
		base = *u.PremiumExpiresAt
	}
	expires := base.AddDate(0, 0, durationDays)
	u.Role = RolePremium
	u.PremiumExpiresAt = &expires
}

type CreateUserRequest struct {
	Name     string `json:"name" binding:"required,min=2,max=100"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6,max=100"`
}

type UpdateUserRequest struct {
	Name     string `json:"name" binding:"omitempty,min=2,max=100"`
	Email    string `json:"email" binding:"omitempty,email"`
	Password string `json:"password" binding:"omitempty,min=6,max=100"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"`
}
