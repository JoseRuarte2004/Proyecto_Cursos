ALTER TABLE messages
    ADD COLUMN IF NOT EXISTS sort_index BIGINT;

CREATE SEQUENCE IF NOT EXISTS messages_sort_index_seq;

WITH ordered_messages AS (
    SELECT
        ctid,
        ROW_NUMBER() OVER (
            ORDER BY created_at ASC, xmin::text::bigint ASC, ctid ASC
        ) AS next_sort_index
    FROM messages
    WHERE sort_index IS NULL
)
UPDATE messages AS target
SET sort_index = ordered_messages.next_sort_index
FROM ordered_messages
WHERE target.ctid = ordered_messages.ctid;

SELECT setval(
    'messages_sort_index_seq',
    COALESCE((SELECT MAX(sort_index) FROM messages), 0),
    true
);

ALTER TABLE messages
    ALTER COLUMN sort_index SET DEFAULT nextval('messages_sort_index_seq');

UPDATE messages
SET sort_index = nextval('messages_sort_index_seq')
WHERE sort_index IS NULL;

ALTER TABLE messages
    ALTER COLUMN sort_index SET NOT NULL;

ALTER SEQUENCE messages_sort_index_seq
    OWNED BY messages.sort_index;

CREATE INDEX IF NOT EXISTS idx_messages_room_created_sort
    ON messages (room_id, created_at DESC, sort_index DESC);
