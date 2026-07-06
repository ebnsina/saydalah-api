-- +goose Up
-- +goose StatementBegin
-- Record who performed each stock movement. Nullable: historical rows and
-- system-initiated movements have no acting user.
ALTER TABLE stock_movements
    ADD COLUMN created_by UUID REFERENCES users (id) ON DELETE SET NULL;
CREATE INDEX idx_movements_created_by ON stock_movements (created_by);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_movements_created_by;
ALTER TABLE stock_movements DROP COLUMN IF EXISTS created_by;
-- +goose StatementEnd
