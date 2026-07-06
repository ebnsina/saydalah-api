-- name: CreateSupplier :one
INSERT INTO suppliers (name, contact, phone, email)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetSupplier :one
SELECT * FROM suppliers WHERE id = $1;

-- name: ListSuppliers :many
SELECT * FROM suppliers
ORDER BY name
LIMIT $1 OFFSET $2;

-- name: CountSuppliers :one
SELECT count(*) FROM suppliers;

-- name: UpdateSupplier :one
UPDATE suppliers
SET name = $2, contact = $3, phone = $4, email = $5, active = $6, updated_at = now()
WHERE id = $1
RETURNING *;
