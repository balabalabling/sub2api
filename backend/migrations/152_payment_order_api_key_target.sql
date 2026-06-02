ALTER TABLE payment_orders
    ADD COLUMN IF NOT EXISTS api_key_id BIGINT;

CREATE INDEX IF NOT EXISTS idx_payment_orders_api_key_id ON payment_orders(api_key_id);
