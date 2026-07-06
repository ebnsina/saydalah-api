-- +goose NO TRANSACTION
-- ALTER TYPE ... ADD VALUE cannot run inside a transaction block, so this
-- migration is marked NO TRANSACTION. Adding values is idempotent via IF NOT
-- EXISTS, making re-runs safe.

-- +goose Up
-- +goose StatementBegin
ALTER TYPE movement_type ADD VALUE IF NOT EXISTS 'transfer_out';
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TYPE movement_type ADD VALUE IF NOT EXISTS 'transfer_in';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- PostgreSQL cannot drop individual enum values; this Down is intentionally a
-- no-op. Rolling back would require recreating movement_type and rewriting every
-- dependent column, which is not worth automating for a purely additive change.
SELECT 1;
-- +goose StatementEnd
