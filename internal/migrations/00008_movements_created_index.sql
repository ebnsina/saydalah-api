-- +goose Up
-- +goose StatementBegin
-- The movement ledger is filtered by branch and ordered by created_at DESC.
-- This composite index serves both, so paging it doesn't sort/scan the table.
CREATE INDEX IF NOT EXISTS idx_movements_branch_created
    ON stock_movements (branch_id, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_movements_branch_created;
-- +goose StatementEnd
