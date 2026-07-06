-- +goose Up
-- +goose StatementBegin
-- Voiding a sale restores its outstanding stock and refunds it. voided_at set
-- means the sale has been reversed; voided_by records who did it.
ALTER TABLE sales
    ADD COLUMN voided_at TIMESTAMPTZ,
    ADD COLUMN voided_by UUID REFERENCES users (id) ON DELETE SET NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE sales
    DROP COLUMN IF EXISTS voided_by,
    DROP COLUMN IF EXISTS voided_at;
-- +goose StatementEnd
