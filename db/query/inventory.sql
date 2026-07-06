-- Inventory write queries. Batches and the movement ledger are written by the
-- purchasing (goods-received) and sales (dispensing) flows inside a transaction.

-- name: CreateStockBatch :one
INSERT INTO stock_batches (
    product_id, branch_id, batch_no, quantity, cost_price, sale_price, expiry_date
) VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: RecordStockMovement :one
INSERT INTO stock_movements (
    product_id, branch_id, batch_id, type, qty, ref_type, ref_id, note, created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- Decrement a batch's quantity, but only if enough remains. Returns the updated
-- row; zero rows means insufficient stock (used by FEFO dispensing).
-- name: DecrementBatchQuantity :one
UPDATE stock_batches
SET quantity = quantity - sqlc.arg('qty')
WHERE id = sqlc.arg('id') AND quantity >= sqlc.arg('qty')
RETURNING *;

-- FEFO: batches with stock for a product at a branch, earliest expiry first,
-- skipping already-expired stock. Locked FOR UPDATE so concurrent sales cannot
-- oversell the same batch.
-- name: ListDispensableBatches :many
SELECT * FROM stock_batches
WHERE branch_id = $1 AND product_id = $2
  AND quantity > 0 AND expiry_date >= CURRENT_DATE
ORDER BY expiry_date ASC
FOR UPDATE;

-- name: StockOnHand :one
SELECT COALESCE(SUM(quantity), 0)::bigint AS on_hand
FROM stock_batches
WHERE branch_id = $1 AND product_id = $2;

-- Inventory read queries (per-branch stock, expiry & reorder alerts) ----------

-- name: ListBranchBatches :many
SELECT sb.*, p.name AS product_name, p.form AS product_form
FROM stock_batches sb
JOIN products p ON p.id = sb.product_id
WHERE sb.branch_id = $1 AND sb.quantity > 0
ORDER BY sb.expiry_date ASC
LIMIT $2 OFFSET $3;

-- name: CountBranchBatches :one
SELECT count(*) FROM stock_batches
WHERE branch_id = $1 AND quantity > 0;

-- name: ListNearExpiryBatches :many
SELECT sb.*, p.name AS product_name, p.form AS product_form
FROM stock_batches sb
JOIN products p ON p.id = sb.product_id
WHERE sb.branch_id = $1
  AND sb.quantity > 0
  AND sb.expiry_date <= CURRENT_DATE + (sqlc.arg('within_days')::int)
ORDER BY sb.expiry_date ASC;

-- name: ListLowStock :many
SELECT p.id AS product_id, p.name AS product_name, p.form AS product_form, p.reorder_level,
       COALESCE(SUM(sb.quantity), 0)::bigint AS on_hand
FROM products p
LEFT JOIN stock_batches sb
    ON sb.product_id = p.id AND sb.branch_id = $1
WHERE p.active
GROUP BY p.id
HAVING COALESCE(SUM(sb.quantity), 0) <= p.reorder_level
ORDER BY on_hand ASC;

-- Stock adjustment / return writes and the movement-ledger view ---------------

-- name: GetStockBatch :one
SELECT * FROM stock_batches WHERE id = $1;

-- Apply a signed delta to a batch, refusing to drive quantity negative.
-- Zero rows returned means the adjustment would go below zero.
-- name: AdjustBatchQuantity :one
UPDATE stock_batches
SET quantity = quantity + sqlc.arg('delta')
WHERE id = sqlc.arg('id') AND quantity + sqlc.arg('delta') >= 0
RETURNING *;

-- name: ListStockMovements :many
SELECT sm.*, p.name AS product_name, u.full_name AS created_by_name
FROM stock_movements sm
JOIN products p ON p.id = sm.product_id
LEFT JOIN users u ON u.id = sm.created_by
WHERE sm.branch_id = sqlc.arg('branch_id')
  AND (sqlc.narg('product_id')::uuid IS NULL OR sm.product_id = sqlc.narg('product_id'))
ORDER BY sm.created_at DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountStockMovements :one
SELECT count(*) FROM stock_movements
WHERE branch_id = sqlc.arg('branch_id')
  AND (sqlc.narg('product_id')::uuid IS NULL OR product_id = sqlc.narg('product_id'));

-- Set a batch to an absolute counted quantity (physical stock-take), locking the
-- row and returning both the new row and the previous quantity so the caller can
-- record the delta as an adjustment movement.
-- name: SetBatchQuantity :one
WITH locked AS (
    SELECT sb.id, sb.quantity AS prev
    FROM stock_batches sb
    WHERE sb.id = sqlc.arg('id')
    FOR UPDATE
)
UPDATE stock_batches sb
SET quantity = sqlc.arg('quantity')
FROM locked
WHERE sb.id = locked.id
RETURNING sb.*, locked.prev AS previous_quantity;
