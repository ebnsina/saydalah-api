package auth

import (
	"time"

	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// LoginRequest is the credentials payload.
type LoginRequest struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse returns the access token (and its expiry), a rotating refresh
// token, and the signed-in user.
type LoginResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	User         UserInfo  `json:"user"`
}

// RefreshRequest carries a refresh token, used by both refresh and logout.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// UserInfo is the safe subset of a user returned to clients.
type UserInfo struct {
	ID       uuid.UUID      `json:"id"`
	Email    string         `json:"email"`
	FullName string         `json:"full_name"`
	Role     store.UserRole `json:"role"`
	BranchID *uuid.UUID     `json:"branch_id"`
}

func userInfo(u store.User) UserInfo {
	return UserInfo{
		ID:       u.ID,
		Email:    u.Email,
		FullName: u.FullName,
		Role:     u.Role,
		BranchID: u.BranchID,
	}
}
