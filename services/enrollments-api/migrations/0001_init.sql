CREATE TABLE IF NOT EXISTS enrollments (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    course_id UUID NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'active', 'cancelled', 'refunded')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, course_id)
);

CREATE INDEX IF NOT EXISTS idx_enrollments_course_status
    ON enrollments (course_id, status);

CREATE INDEX IF NOT EXISTS idx_enrollments_user_id
    ON enrollments (user_id);
