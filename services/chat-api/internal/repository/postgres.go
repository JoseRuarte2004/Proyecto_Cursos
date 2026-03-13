package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/lib/pq"

	"proyecto-cursos/services/chat-api/internal/domain"
	"proyecto-cursos/services/chat-api/internal/service"
)

type PostgresMessageRepository struct {
	db *sql.DB
}

type dbtx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func NewPostgresMessageRepository(db *sql.DB) *PostgresMessageRepository {
	return &PostgresMessageRepository{db: db}
}

func (r *PostgresMessageRepository) SaveMessage(ctx context.Context, message domain.Message) (*domain.Message, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	const query = `
		INSERT INTO messages (id, room_id, sender_id, sender_role, content, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id::text, room_id, sender_id::text, sender_role, content, created_at
	`

	var stored domain.Message
	if err := tx.QueryRowContext(
		ctx,
		query,
		message.ID,
		message.RoomID,
		message.SenderID,
		message.SenderRole,
		message.Content,
		message.CreatedAt,
	).Scan(
		&stored.ID,
		&stored.RoomID,
		&stored.SenderID,
		&stored.SenderRole,
		&stored.Content,
		&stored.CreatedAt,
	); err != nil {
		return nil, err
	}

	if err := insertMessageAttachments(ctx, tx, message.Attachments); err != nil {
		return nil, err
	}
	stored.Attachments = cloneAttachmentsWithoutData(message.Attachments)

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &stored, nil
}

func (r *PostgresMessageRepository) GetMessagesByRoom(ctx context.Context, roomID string, limit int) ([]domain.Message, error) {
	roomID = strings.TrimSpace(roomID)
	if roomID == "" {
		return []domain.Message{}, nil
	}
	if limit <= 0 {
		limit = 50
	}

	const query = `
		SELECT id::text, room_id, sender_id::text, sender_role, content, created_at
		FROM messages
		WHERE room_id = $1
		ORDER BY created_at DESC, sort_index DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, roomID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]domain.Message, 0, limit)
	messageIDs := make([]string, 0, limit)
	for rows.Next() {
		var message domain.Message
		if err := rows.Scan(
			&message.ID,
			&message.RoomID,
			&message.SenderID,
			&message.SenderRole,
			&message.Content,
			&message.CreatedAt,
		); err != nil {
			return nil, err
		}
		messages = append(messages, message)
		messageIDs = append(messageIDs, message.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	attachmentsByMessage, err := listMessageAttachments(ctx, r.db, messageIDs)
	if err != nil {
		return nil, err
	}
	for index := range messages {
		messages[index].Attachments = attachmentsByMessage[messages[index].ID]
	}

	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}

	return messages, nil
}

func (r *PostgresMessageRepository) GetAttachment(ctx context.Context, roomID, attachmentID string) (*domain.Attachment, error) {
	const query = `
		SELECT id, message_id::text, room_id, kind, file_name, content_type, size_bytes, data, created_at
		FROM message_attachments
		WHERE room_id = $1 AND id = $2
	`

	var attachment domain.Attachment
	if err := r.db.QueryRowContext(ctx, query, roomID, attachmentID).Scan(
		&attachment.ID,
		&attachment.MessageID,
		&attachment.RoomID,
		&attachment.Kind,
		&attachment.FileName,
		&attachment.ContentType,
		&attachment.SizeBytes,
		&attachment.Data,
		&attachment.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrAttachmentNotFound
		}
		return nil, err
	}

	return &attachment, nil
}

func insertMessageAttachments(ctx context.Context, execer dbtx, attachments []domain.Attachment) error {
	for _, attachment := range attachments {
		if len(attachment.Data) == 0 {
			continue
		}

		if _, err := execer.ExecContext(ctx, `
			INSERT INTO message_attachments (
				id, message_id, room_id, kind, file_name, content_type, size_bytes, data, created_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`,
			attachment.ID,
			attachment.MessageID,
			attachment.RoomID,
			attachment.Kind,
			attachment.FileName,
			attachment.ContentType,
			attachment.SizeBytes,
			attachment.Data,
			attachment.CreatedAt,
		); err != nil {
			return err
		}
	}

	return nil
}

func listMessageAttachments(ctx context.Context, queryable dbtx, messageIDs []string) (map[string][]domain.Attachment, error) {
	attachmentsByMessage := make(map[string][]domain.Attachment, len(messageIDs))
	if len(messageIDs) == 0 {
		return attachmentsByMessage, nil
	}

	rows, err := queryable.QueryContext(ctx, `
		SELECT id, message_id::text, room_id, kind, file_name, content_type, size_bytes, created_at
		FROM message_attachments
		WHERE message_id = ANY($1)
		ORDER BY created_at ASC, id ASC
	`, pq.Array(messageIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var attachment domain.Attachment
		if err := rows.Scan(
			&attachment.ID,
			&attachment.MessageID,
			&attachment.RoomID,
			&attachment.Kind,
			&attachment.FileName,
			&attachment.ContentType,
			&attachment.SizeBytes,
			&attachment.CreatedAt,
		); err != nil {
			return nil, err
		}
		attachmentsByMessage[attachment.MessageID] = append(
			attachmentsByMessage[attachment.MessageID],
			attachment,
		)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return attachmentsByMessage, nil
}

func cloneAttachmentsWithoutData(attachments []domain.Attachment) []domain.Attachment {
	if len(attachments) == 0 {
		return nil
	}

	cloned := make([]domain.Attachment, 0, len(attachments))
	for _, attachment := range attachments {
		cloned = append(cloned, domain.Attachment{
			ID:          attachment.ID,
			MessageID:   attachment.MessageID,
			RoomID:      attachment.RoomID,
			Kind:        attachment.Kind,
			FileName:    attachment.FileName,
			ContentType: attachment.ContentType,
			SizeBytes:   attachment.SizeBytes,
			CreatedAt:   attachment.CreatedAt,
		})
	}

	return cloned
}
