CREATE TABLE IF NOT EXISTS courses (
    id UUID PRIMARY KEY,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    category TEXT NOT NULL,
    image_url TEXT NULL,
    price NUMERIC(12,2) NOT NULL,
    currency TEXT NOT NULL,
    capacity INT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('draft', 'published')),
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS course_teachers (
    course_id UUID NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    teacher_id UUID NOT NULL,
    UNIQUE(course_id, teacher_id)
);
