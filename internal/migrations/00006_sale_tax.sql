-- +goose Up
-- +goose StatementBegin
-- Tax amount charged on a sale (subtotal minus discount, times the configured
-- tax rate). Defaults to 0 so existing rows and tax-free deployments are valid.
ALTER TABLE sales
    ADD COLUMN tax NUMERIC(12, 2) NOT NULL DEFAULT 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sales
    DROP COLUMN IF EXISTS tax;
-- +goose StatementEnd
