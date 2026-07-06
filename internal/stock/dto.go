// Package stock handles manual inventory movements — quantity adjustments
// (corrections, damage, loss) and customer returns — plus a read-only view of
// the movement ledger. Every write updates a batch and appends a movement row
// in one transaction so stock and its audit trail stay in lockstep.
package stock

import (
	"time"

	"github.com/google/uuid"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// AdjustRequest applies a signed correction to a batch's quantity. Delta may be
// negative (damage, loss, count-down) or positive (found stock, count-up); the
// resulting quantity may not go below zero.
type AdjustRequest struct {
	BatchID uuid.UUID `json:"batch_id" validate:"required"`
	Delta   int32     `json:"delta"    validate:"required"`
	Note    string    `json:"note"     validate:"max=255"`
}

// ReturnRequest returns quantity to a batch (e.g. a customer refund). Qty is the
// positive number of units coming back into stock.
type ReturnRequest struct {
	BatchID uuid.UUID  `json:"batch_id" validate:"required"`
	Qty     int32      `json:"qty"      validate:"required,gt=0"`
	SaleID  *uuid.UUID `json:"sale_id"`
	Note    string     `json:"note"     validate:"max=255"`
}

// BatchResponse is the batch state after a write.
type BatchResponse struct {
	ID         uuid.UUID `json:"id"`
	ProductID  uuid.UUID `json:"product_id"`
	BranchID   uuid.UUID `json:"branch_id"`
	BatchNo    string    `json:"batch_no"`
	Quantity   int32     `json:"quantity"`
	ExpiryDate time.Time `json:"expiry_date"`
}

// MovementResponse is one row of the movement ledger.
type MovementResponse struct {
	ID          uuid.UUID          `json:"id"`
	ProductID   uuid.UUID          `json:"product_id"`
	ProductName string             `json:"product_name"`
	BranchID    uuid.UUID          `json:"branch_id"`
	BatchID     *uuid.UUID         `json:"batch_id"`
	Type        store.MovementType `json:"type"`
	Qty         int32              `json:"qty"`
	RefType     string             `json:"ref_type"`
	RefID       *uuid.UUID         `json:"ref_id"`
	Note        string             `json:"note"`
	CreatedAt   time.Time          `json:"created_at"`
}

func batchResponse(b store.StockBatch) BatchResponse {
	return BatchResponse{
		ID:         b.ID,
		ProductID:  b.ProductID,
		BranchID:   b.BranchID,
		BatchNo:    b.BatchNo,
		Quantity:   b.Quantity,
		ExpiryDate: b.ExpiryDate,
	}
}

func movementResponse(r store.ListStockMovementsRow) MovementResponse {
	return MovementResponse{
		ID:          r.ID,
		ProductID:   r.ProductID,
		ProductName: r.ProductName,
		BranchID:    r.BranchID,
		BatchID:     r.BatchID,
		Type:        r.Type,
		Qty:         r.Qty,
		RefType:     r.RefType,
		RefID:       r.RefID,
		Note:        r.Note,
		CreatedAt:   r.CreatedAt,
	}
}
