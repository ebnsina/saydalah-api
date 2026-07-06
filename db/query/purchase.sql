-- name: CreatePurchaseOrder :one
INSERT INTO purchase_orders (branch_id, supplier_id, reference, status, ordered_at)
VALUES ($1, $2, $3, 'ordered', now())
RETURNING *;

-- name: AddPurchaseOrderItem :one
INSERT INTO purchase_order_items (po_id, product_id, qty, unit_cost)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetPurchaseOrder :one
SELECT * FROM purchase_orders WHERE id = $1;

-- name: ListPurchaseOrderItems :many
SELECT * FROM purchase_order_items WHERE po_id = $1 ORDER BY id;

-- name: ListPurchaseOrderItemsForOrders :many
-- Batch item-load for a page of orders (avoids an N+1 in the list endpoint).
SELECT * FROM purchase_order_items
WHERE po_id = ANY(@po_ids::uuid[])
ORDER BY po_id, id;

-- name: ListPurchaseOrders :many
SELECT * FROM purchase_orders
WHERE branch_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountPurchaseOrders :one
SELECT count(*) FROM purchase_orders WHERE branch_id = $1;

-- Mark a PO received. The status guard makes double-receipt a no-op (0 rows),
-- so stock is never added twice for the same order.
-- name: MarkPurchaseOrderReceived :one
UPDATE purchase_orders
SET status = 'received', received_at = now(), updated_at = now()
WHERE id = $1 AND status <> 'received'
RETURNING *;
