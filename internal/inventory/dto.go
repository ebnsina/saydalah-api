// Package inventory exposes read-only views of per-branch stock: current
// batches, near-expiry alerts, low-stock (reorder) alerts, and on-hand counts.
// Stock is written by the purchasing and sales flows, not here.
package inventory

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// BatchResponse is a stock batch with its product name.
type BatchResponse struct {
	ID          uuid.UUID       `json:"id"`
	ProductID   uuid.UUID       `json:"product_id"`
	ProductName string          `json:"product_name"`
	ProductForm string          `json:"product_form"`
	BatchNo     string          `json:"batch_no"`
	Quantity    int32           `json:"quantity"`
	SalePrice   decimal.Decimal `json:"sale_price"`
	ExpiryDate  time.Time       `json:"expiry_date"`
}

// LowStockResponse is a product at or below its reorder level in a branch.
type LowStockResponse struct {
	ProductID    uuid.UUID `json:"product_id"`
	ProductName  string    `json:"product_name"`
	ProductForm  string    `json:"product_form"`
	ReorderLevel int32     `json:"reorder_level"`
	OnHand       int64     `json:"on_hand"`
}

// BranchStockResponse is a product's on-hand quantity in one branch.
type BranchStockResponse struct {
	BranchID   uuid.UUID `json:"branch_id"`
	BranchName string    `json:"branch_name"`
	OnHand     int64     `json:"on_hand"`
}

// OnHandResponse is the total on-hand quantity of a product in a branch.
type OnHandResponse struct {
	ProductID uuid.UUID `json:"product_id"`
	BranchID  uuid.UUID `json:"branch_id"`
	OnHand    int64     `json:"on_hand"`
}

func batchFromBranchRow(r store.ListBranchBatchesRow) BatchResponse {
	return BatchResponse{
		ID:          r.ID,
		ProductID:   r.ProductID,
		ProductName: r.ProductName,
		ProductForm: r.ProductForm,
		BatchNo:     r.BatchNo,
		Quantity:    r.Quantity,
		SalePrice:   r.SalePrice,
		ExpiryDate:  r.ExpiryDate,
	}
}

func batchFromExpiryRow(r store.ListNearExpiryBatchesRow) BatchResponse {
	return BatchResponse{
		ID:          r.ID,
		ProductID:   r.ProductID,
		ProductName: r.ProductName,
		ProductForm: r.ProductForm,
		BatchNo:     r.BatchNo,
		Quantity:    r.Quantity,
		SalePrice:   r.SalePrice,
		ExpiryDate:  r.ExpiryDate,
	}
}

func lowStockFromRow(r store.ListLowStockRow) LowStockResponse {
	return LowStockResponse{
		ProductID:    r.ProductID,
		ProductName:  r.ProductName,
		ProductForm:  r.ProductForm,
		ReorderLevel: r.ReorderLevel,
		OnHand:       r.OnHand,
	}
}
