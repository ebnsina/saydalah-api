// Package reporting exposes read-only aggregate views: sales summaries and
// trends, inventory valuation, and best-selling products. All reports are
// branch-scoped via the caller's identity.
package reporting

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/ebnsina/saydalah-api/internal/store"
)

// DateRange is the [from, to) window a report covers.
type DateRange struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// SalesSummaryResponse totals sales over the window.
type SalesSummaryResponse struct {
	DateRange
	SaleCount     int64           `json:"sale_count"`
	Revenue       decimal.Decimal `json:"revenue"`
	DiscountTotal decimal.Decimal `json:"discount_total"`
}

// DailySales is one day's totals.
type DailySales struct {
	Day       time.Time       `json:"day"`
	SaleCount int64           `json:"sale_count"`
	Revenue   decimal.Decimal `json:"revenue"`
}

// PaymentBreakdown is one payment method's totals over a range.
type PaymentBreakdown struct {
	PaymentMethod string          `json:"payment_method"`
	SaleCount     int64           `json:"sale_count"`
	Revenue       decimal.Decimal `json:"revenue"`
}

// InventoryValuationResponse values current stock at cost and retail.
type InventoryValuationResponse struct {
	TotalUnits  int64           `json:"total_units"`
	CostValue   decimal.Decimal `json:"cost_value"`
	RetailValue decimal.Decimal `json:"retail_value"`
}

// TopProduct is one best-seller row.
type TopProduct struct {
	ProductID   uuid.UUID       `json:"product_id"`
	ProductName string          `json:"product_name"`
	UnitsSold   int64           `json:"units_sold"`
	Revenue     decimal.Decimal `json:"revenue"`
}

func dailyFromRow(r store.SalesDailyRow) DailySales {
	return DailySales{Day: r.Day, SaleCount: r.SaleCount, Revenue: r.Revenue}
}

func topFromRow(r store.TopSellingProductsRow) TopProduct {
	return TopProduct{
		ProductID:   r.ProductID,
		ProductName: r.ProductName,
		UnitsSold:   r.UnitsSold,
		Revenue:     r.Revenue,
	}
}
