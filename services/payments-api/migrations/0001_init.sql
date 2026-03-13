CREATE TABLE IF NOT EXISTS orders (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    course_id UUID NOT NULL,
    amount NUMERIC(12,2) NOT NULL,
    currency TEXT NOT NULL,
    provider TEXT NOT NULL CHECK (provider IN ('mercadopago', 'stripe')),
    provider_payment_id TEXT NULL,
    status TEXT NOT NULL CHECK (status IN ('created', 'paid', 'failed', 'refunded')),
    idempotency_key TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_orders_user_id
    ON orders (user_id);

CREATE INDEX IF NOT EXISTS idx_orders_course_id
    ON orders (course_id);

CREATE INDEX IF NOT EXISTS idx_orders_status
    ON orders (status);
