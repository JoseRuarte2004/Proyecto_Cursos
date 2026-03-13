ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS checkout_url TEXT NULL,
    ADD COLUMN IF NOT EXISTS external_reference TEXT NULL,
    ADD COLUMN IF NOT EXISTS provider_preference_id TEXT NULL,
    ADD COLUMN IF NOT EXISTS provider_status TEXT NULL,
    ADD COLUMN IF NOT EXISTS paid_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS failed_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS last_webhook_at TIMESTAMPTZ NULL;

ALTER TABLE orders
    DROP CONSTRAINT IF EXISTS orders_status_check;

ALTER TABLE orders
    ADD CONSTRAINT orders_status_check
    CHECK (status IN ('created', 'pending', 'paid', 'failed', 'refunded'));

UPDATE orders
SET external_reference = id::text
WHERE external_reference IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_orders_provider_external_reference
    ON orders (provider, external_reference)
    WHERE external_reference IS NOT NULL;

CREATE TABLE IF NOT EXISTS payment_webhook_events (
    id UUID PRIMARY KEY,
    provider TEXT NOT NULL CHECK (provider IN ('mercadopago', 'stripe')),
    event_key TEXT NOT NULL,
    request_id TEXT NULL,
    topic TEXT NULL,
    action TEXT NULL,
    resource_id TEXT NULL,
    order_id UUID NULL REFERENCES orders(id) ON DELETE SET NULL,
    payload TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ NULL,
    UNIQUE (provider, event_key)
);

CREATE INDEX IF NOT EXISTS idx_payment_webhook_events_provider_resource
    ON payment_webhook_events (provider, resource_id);

CREATE INDEX IF NOT EXISTS idx_payment_webhook_events_order_id
    ON payment_webhook_events (order_id);
