// Package branch manages pharmacy branches (stores) in the chain. It is the
// reference module: every other domain follows this same handler → service →
// repository → store layering and file layout.
package branch

import (
	"time"

	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// CreateRequest is the payload to create a branch.
type CreateRequest struct {
	Name    string `json:"name"    validate:"required,min=2,max=120"`
	Address string `json:"address" validate:"max=255"`
	Phone   string `json:"phone"   validate:"max=40"`
}

// UpdateRequest is the payload to update a branch. All fields are replaced.
type UpdateRequest struct {
	Name    string `json:"name"    validate:"required,min=2,max=120"`
	Address string `json:"address" validate:"max=255"`
	Phone   string `json:"phone"   validate:"max=40"`
	Active  bool   `json:"active"`
}

// Response is the client-facing branch representation.
type Response struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	Phone     string    `json:"phone"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// toResponse maps a store row to the API representation, keeping storage types
// out of the transport layer.
func toResponse(b store.Branch) Response {
	return Response{
		ID:        b.ID,
		Name:      b.Name,
		Address:   b.Address,
		Phone:     b.Phone,
		Active:    b.Active,
		CreatedAt: b.CreatedAt,
		UpdatedAt: b.UpdatedAt,
	}
}
