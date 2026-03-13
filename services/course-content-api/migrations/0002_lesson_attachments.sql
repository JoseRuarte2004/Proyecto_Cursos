CREATE TABLE IF NOT EXISTS lesson_attachments (
    id TEXT PRIMARY KEY,
    lesson_id UUID NOT NULL REFERENCES lessons(id) ON DELETE CASCADE,
    course_id UUID NOT NULL,
    kind TEXT NOT NULL,
    file_name TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    data BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_lesson_attachments_lesson_id
    ON lesson_attachments (lesson_id, created_at ASC);

CREATE INDEX IF NOT EXISTS idx_lesson_attachments_course_id
    ON lesson_attachments (course_id);
