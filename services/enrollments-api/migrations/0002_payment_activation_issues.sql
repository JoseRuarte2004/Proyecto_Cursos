CREATE TABLE IF NOT EXISTS payment_activation_issues (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL UNIQUE,
    user_id UUID NOT NULL,
    course_id UUID NOT NULL,
    reason_code TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('retryable', 'manual_review', 'resolved')),
    last_error TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ NULL,
    locked_at TIMESTAMPTZ NULL,
    locked_by TEXT NULL
);

CREATE INDEX IF NOT EXISTS idx_payment_activation_issues_pending
    ON payment_activation_issues (status, next_attempt_at, created_at)
    WHERE resolved_at IS NULL;
