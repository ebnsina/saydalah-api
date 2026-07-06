// Package sales is the point of sale. Checkout is transactional and dispenses
// stock FEFO (First-Expiry-First-Out): it consumes the earliest-expiring
// non-expired batches first, decrements them, writes the invoice and line
// items, and appends sale movements — all atomically.
package sales

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// CreateRequest is a checkout: one or more product lines plus payment details.
// BranchID is optional for branch staff (their branch is used) and required for
// admins. Quantities are dispensed FEFO across available batches.
type CreateRequest struct {
	BranchID       *uuid.UUID          `json:"branch_id"`
	CustomerID     *uuid.UUID          `json:"customer_id"`
	PrescriptionID *uuid.UUID          `json:"prescription_id"`
	PaymentMethod  store.PaymentMethod `json:"payment_method" validate:"required,oneof=cash card mobile"`
	Discount       decimal.Decimal     `json:"discount"`
	Paid           decimal.Decimal     `json:"paid"`
	Lines          []LineInput         `json:"lines"          validate:"required,min=1,dive"`
}

// LineInput is a requested product and quantity to dispense.
type LineInput struct {
	ProductID uuid.UUID `json:"product_id" validate:"required"`
	Qty       int32     `json:"qty"        validate:"required,gt=0"`
}

// Response is the completed sale (invoice) with its line items.
type Response struct {
	ID             uuid.UUID           `json:"id"`
	BranchID       uuid.UUID           `json:"branch_id"`
	CashierID      uuid.UUID           `json:"cashier_id"`
	CustomerID     *uuid.UUID          `json:"customer_id"`
	PrescriptionID *uuid.UUID          `json:"prescription_id"`
	Subtotal       decimal.Decimal     `json:"subtotal"`
	Discount       decimal.Decimal     `json:"discount"`
	Total          decimal.Decimal     `json:"total"`
	Paid           decimal.Decimal     `json:"paid"`
	PaymentMethod  store.PaymentMethod `json:"payment_method"`
	CreatedAt      time.Time           `json:"created_at"`
	Items          []ItemResponse      `json:"items"`
}

// ItemResponse is one dispensed line (a batch allocation).
type ItemResponse struct {
	ProductID uuid.UUID       `json:"product_id"`
	BatchID   uuid.UUID       `json:"batch_id"`
	Qty       int32           `json:"qty"`
	UnitPrice decimal.Decimal `json:"unit_price"`
}

func toResponse(s store.Sale, items []store.SaleItem) Response {
	out := Response{
		ID:             s.ID,
		BranchID:       s.BranchID,
		CashierID:      s.CashierID,
		CustomerID:     s.CustomerID,
		PrescriptionID: s.PrescriptionID,
		Subtotal:       s.Subtotal,
		Discount:       s.Discount,
		Total:          s.Total,
		Paid:           s.Paid,
		PaymentMethod:  s.PaymentMethod,
		CreatedAt:      s.CreatedAt,
		Items:          make([]ItemResponse, len(items)),
	}
	for i, it := range items {
		out.Items[i] = ItemResponse{
			ProductID: it.ProductID,
			BatchID:   it.BatchID,
			Qty:       it.Qty,
			UnitPrice: it.UnitPrice,
		}
	}
	return out
}
