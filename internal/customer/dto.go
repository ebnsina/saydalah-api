// Package customer manages customer records used by prescriptions and sales. It
// follows the reference layering from the branch module.
package customer

import (
	"time"

	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// CreateRequest is the payload to add a customer.
type CreateRequest struct {
	Name    string `json:"name"    validate:"required,min=2,max=120"`
	Phone   string `json:"phone"   validate:"max=40"`
	Address string `json:"address" validate:"max=255"`
}

// UpdateRequest replaces a customer's mutable fields.
type UpdateRequest struct {
	Name    string `json:"name"    validate:"required,min=2,max=120"`
	Phone   string `json:"phone"   validate:"max=40"`
	Address string `json:"address" validate:"max=255"`
}

// Response is the client-facing customer representation.
type Response struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Phone     string    `json:"phone"`
	Address   string    `json:"address"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toResponse(c store.Customer) Response {
	return Response{
		ID:        c.ID,
		Name:      c.Name,
		Phone:     c.Phone,
		Address:   c.Address,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}
