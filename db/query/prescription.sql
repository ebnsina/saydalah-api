-- name: CreatePrescription :one
INSERT INTO prescriptions (customer_id, branch_id, doctor_name, notes)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: AddPrescriptionItem :one
INSERT INTO prescription_items (prescription_id, product_id, qty, dosage)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetPrescription :one
SELECT * FROM prescriptions WHERE id = $1;

-- name: ListPrescriptionItems :many
SELECT * FROM prescription_items WHERE prescription_id = $1 ORDER BY id;

-- name: ListPrescriptions :many
SELECT * FROM prescriptions
WHERE branch_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountPrescriptions :one
SELECT count(*) FROM prescriptions WHERE branch_id = $1;

-- Mark a prescription dispensed once. The guard makes re-dispensing a no-op
-- (0 rows), preventing a prescription from being filled twice.
-- name: MarkPrescriptionDispensed :one
UPDATE prescriptions
SET dispensed_at = now()
WHERE id = $1 AND dispensed_at IS NULL
RETURNING *;
