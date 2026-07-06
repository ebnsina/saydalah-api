// Package supplier manages the suppliers that branches purchase stock from. It
// follows the reference layering from the branch module.
package supplier

import (
	"time"

	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// CreateRequest is the payload to add a supplier.
type CreateRequest struct {
	Name    string `json:"name"    validate:"required,min=2,max=160"`
	Contact string `json:"contact" validate:"max=120"`
	Phone   string `json:"phone"   validate:"max=40"`
	Email   string `json:"email"   validate:"omitempty,email,max=160"`
}

// UpdateRequest replaces a supplier's mutable fields.
type UpdateRequest struct {
	Name    string `json:"name"    validate:"required,min=2,max=160"`
	Contact string `json:"contact" validate:"max=120"`
	Phone   string `json:"phone"   validate:"max=40"`
	Email   string `json:"email"   validate:"omitempty,email,max=160"`
	Active  bool   `json:"active"`
}

// Response is the client-facing supplier representation.
type Response struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Contact   string    `json:"contact"`
	Phone     string    `json:"phone"`
	Email     string    `json:"email"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toResponse(s store.Supplier) Response {
	return Response{
		ID:        s.ID,
		Name:      s.Name,
		Contact:   s.Contact,
		Phone:     s.Phone,
		Email:     s.Email,
		Active:    s.Active,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}
