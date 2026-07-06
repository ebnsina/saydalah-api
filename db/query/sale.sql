-- name: CreateSale :one
INSERT INTO sales (
    branch_id, cashier_id, customer_id, prescription_id,
    subtotal, discount, total, paid, payment_method
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: AddSaleItem :one
INSERT INTO sale_items (sale_id, batch_id, product_id, qty, unit_price)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetSale :one
SELECT * FROM sales WHERE id = $1;

-- Ordered by the dispensed batch's expiry (FEFO / dispensing order), with id as
-- a stable tie-breaker, so receipt line items appear deterministically.
-- name: ListSaleItems :many
SELECT si.* FROM sale_items si
JOIN stock_batches sb ON sb.id = si.batch_id
WHERE si.sale_id = $1
ORDER BY sb.expiry_date, si.id;

-- name: ListSales :many
SELECT * FROM sales
WHERE branch_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountSales :one
SELECT count(*) FROM sales WHERE branch_id = $1;

-- Total units already returned to a given batch against a given sale. Used to
-- cap sale-linked returns at the quantity actually dispensed.
-- name: SumReturnedForSaleBatch :one
SELECT COALESCE(SUM(qty), 0)::bigint AS returned
FROM stock_movements
WHERE type = 'return' AND ref_type = 'sale' AND ref_id = $1 AND batch_id = $2;

-- Mark a sale voided. The guard makes a second void a no-op (0 rows), so a
-- sale is never reversed twice.
-- name: MarkSaleVoided :one
UPDATE sales
SET voided_at = now(), voided_by = $2
WHERE id = $1 AND voided_at IS NULL
RETURNING *;
