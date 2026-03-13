CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY,
    room_id VARCHAR(255) NOT NULL,
    sender_id UUID NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_messages_room_created_at
    ON messages (room_id, created_at DESC);
