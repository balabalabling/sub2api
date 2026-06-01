-- Storefront products and orders.

CREATE TABLE IF NOT EXISTS store_products (
    id BIGSERIAL PRIMARY KEY,
    product_type VARCHAR(32) NOT NULL DEFAULT 'api_key',
    name VARCHAR(160) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    price DECIMAL(20,2) NOT NULL,
    currency VARCHAR(16) NOT NULL DEFAULT 'CNY',
    status VARCHAR(32) NOT NULL DEFAULT 'draft',
    visibility VARCHAR(32) NOT NULL DEFAULT 'public',
    sort_order INTEGER NOT NULL DEFAULT 0,
    stock_mode VARCHAR(32) NOT NULL DEFAULT 'unlimited',
    stock_count INTEGER NOT NULL DEFAULT 0,
    delivery_mode VARCHAR(32) NOT NULL DEFAULT 'auto',
    delivery_config JSONB NOT NULL DEFAULT '{}'::jsonb,
    sale_start_at TIMESTAMPTZ NULL,
    sale_end_at TIMESTAMPTZ NULL,
    deleted_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT store_products_type_check CHECK (product_type IN ('api_key', 'account', 'sms', 'manual')),
    CONSTRAINT store_products_status_check CHECK (status IN ('draft', 'active', 'inactive')),
    CONSTRAINT store_products_visibility_check CHECK (visibility IN ('public', 'hidden')),
    CONSTRAINT store_products_stock_mode_check CHECK (stock_mode IN ('unlimited', 'tracked')),
    CONSTRAINT store_products_delivery_mode_check CHECK (delivery_mode IN ('auto', 'manual'))
);

CREATE INDEX IF NOT EXISTS store_products_public_idx
    ON store_products (status, visibility, sort_order, id)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS store_orders (
    id BIGSERIAL PRIMARY KEY,
    order_no VARCHAR(64) NOT NULL UNIQUE,
    email VARCHAR(320) NOT NULL,
    product_id BIGINT NULL REFERENCES store_products(id),
    product_type VARCHAR(32) NOT NULL,
    product_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    amount DECIMAL(20,2) NOT NULL,
    currency VARCHAR(16) NOT NULL DEFAULT 'CNY',
    payment_order_id BIGINT NULL UNIQUE REFERENCES payment_orders(id),
    user_id BIGINT NULL REFERENCES users(id),
    api_key_id BIGINT NULL REFERENCES api_keys(id),
    delivery_status VARCHAR(32) NOT NULL DEFAULT 'pending',
    delivery_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    delivery_error TEXT NULL,
    email_sent_at TIMESTAMPTZ NULL,
    delivered_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT store_orders_delivery_status_check CHECK (delivery_status IN ('pending', 'paid', 'delivering', 'delivered', 'failed', 'manual_required'))
);

CREATE INDEX IF NOT EXISTS store_orders_email_idx ON store_orders (LOWER(email), created_at DESC);
CREATE INDEX IF NOT EXISTS store_orders_api_key_idx ON store_orders (api_key_id);
CREATE INDEX IF NOT EXISTS store_orders_payment_order_idx ON store_orders (payment_order_id);

CREATE TABLE IF NOT EXISTS store_email_codes (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(320) NOT NULL,
    code_hash VARCHAR(128) NOT NULL,
    purpose VARCHAR(32) NOT NULL DEFAULT 'query',
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS store_email_codes_lookup_idx
    ON store_email_codes (LOWER(email), purpose, created_at DESC);
