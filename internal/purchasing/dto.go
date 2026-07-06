// Package purchasing manages purchase orders and goods receipt. Receiving an
// order is transactional: it creates per-batch stock and appends movement-ledger
// rows atomically, so inventory and the audit trail never drift apart.
package purchasing

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// CreateRequest places a purchase order with one or more line items. BranchID is
// optional for branch-scoped staff (their own branch is used) and required for
// admins.
type CreateRequest struct {
	BranchID   *uuid.UUID  `json:"branch_id"`
	SupplierID uuid.UUID   `json:"supplier_id" validate:"required"`
	Reference  string      `json:"reference"   validate:"max=80"`
	Items      []ItemInput `json:"items"       validate:"required,min=1,dive"`
}

// ItemInput is a single ordered product line.
type ItemInput struct {
	ProductID uuid.UUID       `json:"product_id" validate:"required"`
	Qty       int32           `json:"qty"        validate:"required,gt=0"`
	UnitCost  decimal.Decimal `json:"unit_cost"`
}

// ReceiveRequest records goods received against an order. Each line becomes a
// stock batch (with its own batch number and expiry) and a movement entry.
type ReceiveRequest struct {
	Lines []ReceiveLine `json:"lines" validate:"required,min=1,dive"`
}

// ReceiveLine is a received batch of a product. ExpiryDate is an RFC3339
// timestamp (date component is what matters).
type ReceiveLine struct {
	ProductID  uuid.UUID       `json:"product_id"  validate:"required"`
	BatchNo    string          `json:"batch_no"    validate:"max=80"`
	Quantity   int32           `json:"quantity"    validate:"required,gt=0"`
	CostPrice  decimal.Decimal `json:"cost_price"`
	SalePrice  decimal.Decimal `json:"sale_price"`
	ExpiryDate time.Time       `json:"expiry_date" validate:"required"`
}

// OrderResponse is the client-facing purchase order with its line items.
type OrderResponse struct {
	ID         uuid.UUID      `json:"id"`
	BranchID   uuid.UUID      `json:"branch_id"`
	SupplierID uuid.UUID      `json:"supplier_id"`
	Status     store.PoStatus `json:"status"`
	Reference  string         `json:"reference"`
	OrderedAt  *time.Time     `json:"ordered_at"`
	ReceivedAt *time.Time     `json:"received_at"`
	CreatedAt  time.Time      `json:"created_at"`
	Items      []ItemResponse `json:"items"`
}

// ItemResponse is a purchase order line in a response.
type ItemResponse struct {
	ProductID uuid.UUID       `json:"product_id"`
	Qty       int32           `json:"qty"`
	UnitCost  decimal.Decimal `json:"unit_cost"`
}

func toResponse(po store.PurchaseOrder, items []store.PurchaseOrderItem) OrderResponse {
	out := OrderResponse{
		ID:         po.ID,
		BranchID:   po.BranchID,
		SupplierID: po.SupplierID,
		Status:     po.Status,
		Reference:  po.Reference,
		OrderedAt:  po.OrderedAt,
		ReceivedAt: po.ReceivedAt,
		CreatedAt:  po.CreatedAt,
		Items:      make([]ItemResponse, len(items)),
	}
	for i, it := range items {
		out.Items[i] = ItemResponse{ProductID: it.ProductID, Qty: it.Qty, UnitCost: it.UnitCost}
	}
	return out
}
