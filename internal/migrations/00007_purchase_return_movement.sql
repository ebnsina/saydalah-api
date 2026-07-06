-- +goose NO TRANSACTION
-- ALTER TYPE ... ADD VALUE cannot run inside a transaction block, so this
-- migration is marked NO TRANSACTION. IF NOT EXISTS makes re-runs safe.

-- +goose Up
-- +goose StatementBegin
ALTER TYPE movement_type ADD VALUE IF NOT EXISTS 'purchase_return';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- PostgreSQL cannot drop individual enum values; this Down is a no-op.
SELECT 1;
-- +goose StatementEnd
