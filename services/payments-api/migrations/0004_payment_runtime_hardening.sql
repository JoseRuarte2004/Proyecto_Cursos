ALTER TABLE orders
    DROP CONSTRAINT IF EXISTS orders_idempotency_key_key;

DROP INDEX IF EXISTS idx_orders_scoped_idempotency;
CREATE UNIQUE INDEX idx_orders_scoped_idempotency
    ON orders (user_id, course_id, provider, idempotency_key);

UPDATE orders
SET provider_status = 'approved'
WHERE status = 'paid'
  AND (provider_status IS NULL OR provider_status = '');

CREATE TABLE IF NOT EXISTS payment_webhook_jobs (
    id UUID PRIMARY KEY,
    provider TEXT NOT NULL CHECK (provider IN ('mercadopago', 'stripe')),
    dedupe_key TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    request_id TEXT NULL,
    signature_ts TEXT NULL,
    topic TEXT NULL,
    action TEXT NULL,
    payload TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ NULL,
    locked_at TIMESTAMPTZ NULL,
    locked_by TEXT NULL,
    last_error TEXT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_webhook_jobs_provider_dedupe
    ON payment_webhook_jobs (provider, dedupe_key);

CREATE INDEX IF NOT EXISTS idx_payment_webhook_jobs_pending
    ON payment_webhook_jobs (processed_at, available_at, received_at);

CREATE TABLE IF NOT EXISTS payment_order_issues (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    issue_key TEXT NOT NULL UNIQUE,
    issue_type TEXT NOT NULL,
    provider_payment_id TEXT NULL,
    details TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_payment_order_issues_order
    ON payment_order_issues (order_id, status, updated_at DESC);

CREATE OR REPLACE VIEW payment_order_health_issues AS
SELECT
    order_id,
    issue_type,
    details,
    created_at,
    updated_at
FROM payment_order_issues
UNION ALL
SELECT
    id AS order_id,
    'legacy_missing_provider_status' AS issue_type,
    'status=paid but provider_status was blank before remediation' AS details,
    created_at,
    updated_at
FROM orders
WHERE status = 'paid'
  AND (provider_status IS NULL OR provider_status = '');
