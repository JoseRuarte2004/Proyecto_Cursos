ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS amount_cents BIGINT,
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ NULL;

UPDATE orders
SET amount_cents = ROUND(amount * 100)::BIGINT
WHERE amount_cents IS NULL;

ALTER TABLE orders
    ALTER COLUMN amount_cents SET NOT NULL;

UPDATE orders
SET expires_at = CASE
    WHEN status = 'created' THEN COALESCE(expires_at, created_at + INTERVAL '30 minutes')
    WHEN status = 'pending' THEN COALESCE(expires_at, COALESCE(last_webhook_at, updated_at, created_at) + INTERVAL '72 hours')
    ELSE NULL
END
WHERE status IN ('created', 'pending');

WITH ranked_open_orders AS (
    SELECT
        id,
        ROW_NUMBER() OVER (
            PARTITION BY user_id, course_id
            ORDER BY
                CASE status WHEN 'pending' THEN 0 ELSE 1 END,
                COALESCE(last_webhook_at, updated_at, created_at) DESC,
                created_at DESC,
                id DESC
        ) AS row_number
    FROM orders
    WHERE status IN ('created', 'pending')
)
UPDATE orders
SET status = 'failed',
    provider_status = CASE
        WHEN provider_status IS NULL OR provider_status = '' OR provider_status IN ('created', 'pending')
            THEN 'superseded'
        ELSE provider_status
    END,
    failed_at = COALESCE(failed_at, NOW()),
    expires_at = NULL,
    updated_at = NOW()
WHERE id IN (
    SELECT id
    FROM ranked_open_orders
    WHERE row_number > 1
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_orders_user_course_open
    ON orders (user_id, course_id)
    WHERE status IN ('created', 'pending');

CREATE INDEX IF NOT EXISTS idx_orders_provider_open_reconcile
    ON orders (provider, status, updated_at)
    WHERE status IN ('created', 'pending');

CREATE INDEX IF NOT EXISTS idx_orders_open_expiration
    ON orders (expires_at)
    WHERE status IN ('created', 'pending');

CREATE TABLE IF NOT EXISTS payment_outbox_events (
    id UUID PRIMARY KEY,
    event_type TEXT NOT NULL,
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    payload TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ NULL,
    locked_at TIMESTAMPTZ NULL,
    locked_by TEXT NULL,
    last_error TEXT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_outbox_events_type_order
    ON payment_outbox_events (event_type, order_id);

CREATE INDEX IF NOT EXISTS idx_payment_outbox_events_pending
    ON payment_outbox_events (published_at, available_at, created_at);
