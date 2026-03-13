ALTER TABLE messages
    ADD COLUMN IF NOT EXISTS sender_role TEXT NOT NULL DEFAULT 'student';

UPDATE messages
SET sender_role = 'student'
WHERE sender_role IS NULL OR sender_role = '';
