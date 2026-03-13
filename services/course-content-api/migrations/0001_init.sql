CREATE TABLE IF NOT EXISTS lessons (
    id UUID PRIMARY KEY,
    course_id UUID NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    order_index INT NOT NULL,
    video_url TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(course_id, order_index)
);
