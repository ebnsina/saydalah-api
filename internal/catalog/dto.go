// Package catalog manages the shared product (drug) master catalog used across
// all branches. It follows the reference layering from the branch module.
package catalog

import (
	"time"

	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// CreateRequest is the payload to add a product to the catalog. Barcode is
// optional; when present it must be unique across the catalog.
type CreateRequest struct {
	Name         string  `json:"name"          validate:"required,min=2,max=160"`
	GenericName  string  `json:"generic_name"  validate:"max=160"`
	Form         string  `json:"form"          validate:"max=60"`
	Strength     string  `json:"strength"      validate:"max=60"`
	Barcode      *string `json:"barcode"       validate:"omitempty,max=64"`
	Category     string  `json:"category"      validate:"max=80"`
	Unit         string  `json:"unit"          validate:"max=32"`
	ReorderLevel int32   `json:"reorder_level" validate:"gte=0"`
}

// UpdateRequest replaces a product's mutable fields.
type UpdateRequest struct {
	Name         string  `json:"name"          validate:"required,min=2,max=160"`
	GenericName  string  `json:"generic_name"  validate:"max=160"`
	Form         string  `json:"form"          validate:"max=60"`
	Strength     string  `json:"strength"      validate:"max=60"`
	Barcode      *string `json:"barcode"       validate:"omitempty,max=64"`
	Category     string  `json:"category"      validate:"max=80"`
	Unit         string  `json:"unit"          validate:"max=32"`
	ReorderLevel int32   `json:"reorder_level" validate:"gte=0"`
	Active       bool    `json:"active"`
}

// Response is the client-facing product representation.
type Response struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	GenericName  string    `json:"generic_name"`
	Form         string    `json:"form"`
	Strength     string    `json:"strength"`
	Barcode      *string   `json:"barcode"`
	Category     string    `json:"category"`
	Unit         string    `json:"unit"`
	ReorderLevel int32     `json:"reorder_level"`
	Active       bool      `json:"active"`
	OnHand       int64     `json:"on_hand"` // stock at the queried branch (0 if none)
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// listRowToResponse maps a ListProducts row (which carries on_hand) to Response.
func listRowToResponse(p store.ListProductsRow) Response {
	return Response{
		ID:           p.ID,
		Name:         p.Name,
		GenericName:  p.GenericName,
		Form:         p.Form,
		Strength:     p.Strength,
		Barcode:      p.Barcode,
		Category:     p.Category,
		Unit:         p.Unit,
		ReorderLevel: p.ReorderLevel,
		Active:       p.Active,
		OnHand:       p.OnHand,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
}

func toResponse(p store.Product) Response {
	return Response{
		ID:           p.ID,
		Name:         p.Name,
		GenericName:  p.GenericName,
		Form:         p.Form,
		Strength:     p.Strength,
		Barcode:      p.Barcode,
		Category:     p.Category,
		Unit:         p.Unit,
		ReorderLevel: p.ReorderLevel,
		Active:       p.Active,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
}
