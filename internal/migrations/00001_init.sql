-- +goose Up
-- +goose StatementBegin

-- Enumerations -----------------------------------------------------------------
CREATE TYPE user_role      AS ENUM ('admin', 'manager', 'pharmacist', 'cashier');
CREATE TYPE po_status      AS ENUM ('draft', 'ordered', 'received', 'cancelled');
CREATE TYPE movement_type  AS ENUM ('purchase', 'sale', 'adjustment', 'return');
CREATE TYPE payment_method AS ENUM ('cash', 'card', 'mobile');

-- Branches (stores) ------------------------------------------------------------
CREATE TABLE branches (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT        NOT NULL,
    address    TEXT        NOT NULL DEFAULT '',
    phone      TEXT        NOT NULL DEFAULT '',
    active     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Users. branch_id NULL means chain-wide access (admins).
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT        NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL,
    full_name     TEXT        NOT NULL DEFAULT '',
    role          user_role   NOT NULL DEFAULT 'cashier',
    branch_id     UUID        REFERENCES branches (id) ON DELETE SET NULL,
    active        BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_users_branch ON users (branch_id);

-- Shared master catalog (products/drugs) --------------------------------------
CREATE TABLE products (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          TEXT        NOT NULL,
    generic_name  TEXT        NOT NULL DEFAULT '',
    form          TEXT        NOT NULL DEFAULT '',   -- tablet, syrup, injection...
    strength      TEXT        NOT NULL DEFAULT '',   -- 500mg, 5ml...
    barcode       TEXT        UNIQUE,
    category      TEXT        NOT NULL DEFAULT '',
    unit          TEXT        NOT NULL DEFAULT 'unit',
    reorder_level INTEGER     NOT NULL DEFAULT 0 CHECK (reorder_level >= 0),
    active        BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_products_name ON products (name);

-- Suppliers --------------------------------------------------------------------
CREATE TABLE suppliers (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT        NOT NULL,
    contact    TEXT        NOT NULL DEFAULT '',
    phone      TEXT        NOT NULL DEFAULT '',
    email      TEXT        NOT NULL DEFAULT '',
    active     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Purchasing -------------------------------------------------------------------
CREATE TABLE purchase_orders (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    branch_id   UUID        NOT NULL REFERENCES branches (id)  ON DELETE RESTRICT,
    supplier_id UUID        NOT NULL REFERENCES suppliers (id) ON DELETE RESTRICT,
    status      po_status   NOT NULL DEFAULT 'draft',
    reference   TEXT        NOT NULL DEFAULT '',
    ordered_at  TIMESTAMPTZ,
    received_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_po_branch ON purchase_orders (branch_id);

CREATE TABLE purchase_order_items (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    po_id      UUID           NOT NULL REFERENCES purchase_orders (id) ON DELETE CASCADE,
    product_id UUID           NOT NULL REFERENCES products (id)        ON DELETE RESTRICT,
    qty        INTEGER        NOT NULL CHECK (qty > 0),
    unit_cost  NUMERIC(12, 2) NOT NULL CHECK (unit_cost >= 0)
);
CREATE INDEX idx_po_items_po ON purchase_order_items (po_id);

-- Inventory: per-branch, per-batch stock with expiry tracking ------------------
CREATE TABLE stock_batches (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id  UUID           NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
    branch_id   UUID           NOT NULL REFERENCES branches (id) ON DELETE RESTRICT,
    batch_no    TEXT           NOT NULL DEFAULT '',
    quantity    INTEGER        NOT NULL CHECK (quantity >= 0),
    cost_price  NUMERIC(12, 2) NOT NULL CHECK (cost_price >= 0),
    sale_price  NUMERIC(12, 2) NOT NULL CHECK (sale_price >= 0),
    expiry_date DATE           NOT NULL,
    received_at TIMESTAMPTZ    NOT NULL DEFAULT now()
);
CREATE INDEX idx_batches_branch_product ON stock_batches (branch_id, product_id);
-- Drives FEFO dispensing: cheapest to find the earliest-expiring batch in stock.
CREATE INDEX idx_batches_fefo ON stock_batches (branch_id, product_id, expiry_date)
    WHERE quantity > 0;

-- Append-only stock movement ledger (audit trail for every quantity change).
CREATE TABLE stock_movements (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID          NOT NULL REFERENCES products (id) ON DELETE RESTRICT,
    branch_id  UUID          NOT NULL REFERENCES branches (id) ON DELETE RESTRICT,
    batch_id   UUID          REFERENCES stock_batches (id) ON DELETE SET NULL,
    type       movement_type NOT NULL,
    qty        INTEGER       NOT NULL,          -- signed: +in, -out
    ref_type   TEXT          NOT NULL DEFAULT '',
    ref_id     UUID,
    note       TEXT          NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE INDEX idx_movements_branch_product ON stock_movements (branch_id, product_id);

-- Customers & prescriptions ----------------------------------------------------
CREATE TABLE customers (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT        NOT NULL,
    phone      TEXT        NOT NULL DEFAULT '',
    address    TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE prescriptions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id  UUID        NOT NULL REFERENCES customers (id) ON DELETE RESTRICT,
    branch_id    UUID        NOT NULL REFERENCES branches (id)  ON DELETE RESTRICT,
    doctor_name  TEXT        NOT NULL DEFAULT '',
    notes        TEXT        NOT NULL DEFAULT '',
    dispensed_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_prescriptions_customer ON prescriptions (customer_id);

CREATE TABLE prescription_items (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    prescription_id UUID    NOT NULL REFERENCES prescriptions (id) ON DELETE CASCADE,
    product_id      UUID    NOT NULL REFERENCES products (id)      ON DELETE RESTRICT,
    qty             INTEGER NOT NULL CHECK (qty > 0),
    dosage          TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX idx_prescription_items_rx ON prescription_items (prescription_id);

-- Sales / POS ------------------------------------------------------------------
CREATE TABLE sales (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    branch_id       UUID           NOT NULL REFERENCES branches (id)      ON DELETE RESTRICT,
    cashier_id      UUID           NOT NULL REFERENCES users (id)         ON DELETE RESTRICT,
    customer_id     UUID           REFERENCES customers (id)     ON DELETE SET NULL,
    prescription_id UUID           REFERENCES prescriptions (id) ON DELETE SET NULL,
    subtotal        NUMERIC(12, 2) NOT NULL CHECK (subtotal >= 0),
    discount        NUMERIC(12, 2) NOT NULL DEFAULT 0 CHECK (discount >= 0),
    total           NUMERIC(12, 2) NOT NULL CHECK (total >= 0),
    paid            NUMERIC(12, 2) NOT NULL DEFAULT 0 CHECK (paid >= 0),
    payment_method  payment_method NOT NULL DEFAULT 'cash',
    created_at      TIMESTAMPTZ    NOT NULL DEFAULT now()
);
CREATE INDEX idx_sales_branch_created ON sales (branch_id, created_at);

CREATE TABLE sale_items (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sale_id    UUID           NOT NULL REFERENCES sales (id)         ON DELETE CASCADE,
    batch_id   UUID           NOT NULL REFERENCES stock_batches (id) ON DELETE RESTRICT,
    product_id UUID           NOT NULL REFERENCES products (id)      ON DELETE RESTRICT,
    qty        INTEGER        NOT NULL CHECK (qty > 0),
    unit_price NUMERIC(12, 2) NOT NULL CHECK (unit_price >= 0)
);
CREATE INDEX idx_sale_items_sale ON sale_items (sale_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS sale_items;
DROP TABLE IF EXISTS sales;
DROP TABLE IF EXISTS prescription_items;
DROP TABLE IF EXISTS prescriptions;
DROP TABLE IF EXISTS customers;
DROP TABLE IF EXISTS stock_movements;
DROP TABLE IF EXISTS stock_batches;
DROP TABLE IF EXISTS purchase_order_items;
DROP TABLE IF EXISTS purchase_orders;
DROP TABLE IF EXISTS suppliers;
DROP TABLE IF EXISTS products;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS branches;
DROP TYPE IF EXISTS payment_method;
DROP TYPE IF EXISTS movement_type;
DROP TYPE IF EXISTS po_status;
DROP TYPE IF EXISTS user_role;
-- +goose StatementEnd
