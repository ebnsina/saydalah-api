// Package user manages staff accounts (admins, managers, pharmacists,
// cashiers) and their branch assignment. It follows the reference layering
// established by the branch module.
package user

import (
	"time"

	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// CreateRequest is the payload to create a staff user. BranchID is required for
// branch-scoped roles and should be omitted for chain-wide admins.
type CreateRequest struct {
	Email    string         `json:"email"     validate:"required,email,max=160"`
	Password string         `json:"password"  validate:"required,min=8,max=72"`
	FullName string         `json:"full_name" validate:"max=120"`
	Role     store.UserRole `json:"role"      validate:"required,oneof=admin manager pharmacist cashier"`
	BranchID *uuid.UUID     `json:"branch_id"`
}

// UpdateRequest replaces a user's mutable fields (not email or password).
type UpdateRequest struct {
	FullName string         `json:"full_name" validate:"max=120"`
	Role     store.UserRole `json:"role"      validate:"required,oneof=admin manager pharmacist cashier"`
	BranchID *uuid.UUID     `json:"branch_id"`
	Active   bool           `json:"active"`
}

// Response is the client-facing user representation; it never includes the
// password hash.
type Response struct {
	ID        uuid.UUID      `json:"id"`
	Email     string         `json:"email"`
	FullName  string         `json:"full_name"`
	Role      store.UserRole `json:"role"`
	BranchID  *uuid.UUID     `json:"branch_id"`
	Active    bool           `json:"active"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

func toResponse(u store.User) Response {
	return Response{
		ID:        u.ID,
		Email:     u.Email,
		FullName:  u.FullName,
		Role:      u.Role,
		BranchID:  u.BranchID,
		Active:    u.Active,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}
