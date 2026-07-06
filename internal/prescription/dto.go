// Package prescription records customer prescriptions and dispenses them,
// reusing the sales FEFO checkout to turn prescribed items into a sale.
package prescription

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// CreateRequest records a prescription and its prescribed items. BranchID is
// optional for branch staff and required for admins.
type CreateRequest struct {
	BranchID   *uuid.UUID  `json:"branch_id"`
	CustomerID uuid.UUID   `json:"customer_id" validate:"required"`
	DoctorName string      `json:"doctor_name" validate:"max=120"`
	Notes      string      `json:"notes"       validate:"max=500"`
	Items      []ItemInput `json:"items"       validate:"required,min=1,dive"`
}

// ItemInput is a prescribed product line.
type ItemInput struct {
	ProductID uuid.UUID `json:"product_id" validate:"required"`
	Qty       int32     `json:"qty"        validate:"required,gt=0"`
	Dosage    string    `json:"dosage"     validate:"max=120"`
}

// DispenseRequest carries the payment details for filling a prescription. The
// products and quantities come from the prescription itself.
type DispenseRequest struct {
	PaymentMethod store.PaymentMethod `json:"payment_method" validate:"required,oneof=cash card mobile"`
	Discount      decimal.Decimal     `json:"discount"`
	Paid          decimal.Decimal     `json:"paid"`
}

// Response is the client-facing prescription with its items.
type Response struct {
	ID          uuid.UUID      `json:"id"`
	CustomerID  uuid.UUID      `json:"customer_id"`
	BranchID    uuid.UUID      `json:"branch_id"`
	DoctorName  string         `json:"doctor_name"`
	Notes       string         `json:"notes"`
	DispensedAt *time.Time     `json:"dispensed_at"`
	CreatedAt   time.Time      `json:"created_at"`
	Items       []ItemResponse `json:"items"`
}

// ItemResponse is a prescribed line in a response.
type ItemResponse struct {
	ProductID uuid.UUID `json:"product_id"`
	Qty       int32     `json:"qty"`
	Dosage    string    `json:"dosage"`
}

func toResponse(p store.Prescription, items []store.PrescriptionItem) Response {
	out := Response{
		ID:          p.ID,
		CustomerID:  p.CustomerID,
		BranchID:    p.BranchID,
		DoctorName:  p.DoctorName,
		Notes:       p.Notes,
		DispensedAt: p.DispensedAt,
		CreatedAt:   p.CreatedAt,
		Items:       make([]ItemResponse, len(items)),
	}
	for i, it := range items {
		out.Items[i] = ItemResponse{ProductID: it.ProductID, Qty: it.Qty, Dosage: it.Dosage}
	}
	return out
}
