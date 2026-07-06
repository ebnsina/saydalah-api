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

-- name: ListSaleItems :many
SELECT * FROM sale_items WHERE sale_id = $1 ORDER BY id;

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
