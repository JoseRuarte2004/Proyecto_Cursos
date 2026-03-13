CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('student', 'teacher', 'admin')),
    phone TEXT NOT NULL DEFAULT '',
    dni TEXT NOT NULL DEFAULT '',
    address TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY,
    admin_id UUID NOT NULL REFERENCES users(id),
    action TEXT NOT NULL CHECK (action IN ('VIEW_SENSITIVE', 'CHANGE_ROLE')),
    target_user_id UUID NOT NULL REFERENCES users(id),
    "timestamp" TIMESTAMPTZ NOT NULL,
    request_id TEXT NOT NULL,
    ip TEXT NOT NULL
);
