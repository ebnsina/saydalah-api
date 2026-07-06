-- name: CreateBranch :one
INSERT INTO branches (name, address, phone)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetBranch :one
SELECT * FROM branches WHERE id = $1;

-- name: ListBranches :many
SELECT * FROM branches
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountBranches :one
SELECT count(*) FROM branches;

-- name: UpdateBranch :one
UPDATE branches
SET name = $2, address = $3, phone = $4, active = $5, updated_at = now()
WHERE id = $1
RETURNING *;
