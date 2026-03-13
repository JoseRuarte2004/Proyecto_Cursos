CREATE TABLE IF NOT EXISTS message_attachments (
    id TEXT PRIMARY KEY,
    message_id UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    room_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    file_name TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    data BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_message_attachments_message_id
    ON message_attachments (message_id, created_at ASC);

CREATE INDEX IF NOT EXISTS idx_message_attachments_room_id
    ON message_attachments (room_id);
