-- name: CreateProduct :one
INSERT INTO products (
    name, generic_name, form, strength, barcode, category, unit, reorder_level
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetProduct :one
SELECT * FROM products WHERE id = $1;

-- name: ListProducts :many
SELECT * FROM products
WHERE (
    sqlc.narg('search')::text IS NULL
    OR name ILIKE '%' || sqlc.narg('search') || '%'
    OR generic_name ILIKE '%' || sqlc.narg('search') || '%'
    OR barcode = sqlc.narg('search')
)
ORDER BY name
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountProducts :one
SELECT count(*) FROM products
WHERE (
    sqlc.narg('search')::text IS NULL
    OR name ILIKE '%' || sqlc.narg('search') || '%'
    OR generic_name ILIKE '%' || sqlc.narg('search') || '%'
    OR barcode = sqlc.narg('search')
);

-- name: UpdateProduct :one
UPDATE products
SET name = $2, generic_name = $3, form = $4, strength = $5,
    barcode = $6, category = $7, unit = $8, reorder_level = $9,
    active = $10, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: GetProductByBarcode :one
SELECT * FROM products WHERE barcode = $1;
