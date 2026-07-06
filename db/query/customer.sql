-- name: CreateCustomer :one
INSERT INTO customers (name, phone, address)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetCustomer :one
SELECT * FROM customers WHERE id = $1;

-- name: ListCustomers :many
SELECT * FROM customers
WHERE (
    sqlc.narg('search')::text IS NULL
    OR name ILIKE '%' || sqlc.narg('search') || '%'
    OR phone ILIKE '%' || sqlc.narg('search') || '%'
)
ORDER BY name
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountCustomers :one
SELECT count(*) FROM customers
WHERE (
    sqlc.narg('search')::text IS NULL
    OR name ILIKE '%' || sqlc.narg('search') || '%'
    OR phone ILIKE '%' || sqlc.narg('search') || '%'
);

-- name: UpdateCustomer :one
UPDATE customers
SET name = $2, phone = $3, address = $4, updated_at = now()
WHERE id = $1
RETURNING *;
