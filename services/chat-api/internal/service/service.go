package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"proyecto-cursos/services/chat-api/internal/domain"
)

var (
	ErrRoomIDRequired         = errors.New("room_id is required")
	ErrSenderIDRequired       = errors.New("sender_id is required")
	ErrInvalidSenderID        = errors.New("sender_id must be a valid UUID")
	ErrInvalidSenderRole      = errors.New("sender_role must be one of: admin, teacher, student")
	ErrContentRequired        = errors.New("content is required")
	ErrContentTooLong         = errors.New("content must be at most 2000 characters")
	ErrInvalidLimit           = errors.New("limit must be between 1 and 200")
	ErrAttachmentNotFound     = errors.New("attachment not found")
	ErrTooManyAttachments     = errors.New("too many attachments")
	ErrAttachmentTooLarge     = errors.New("attachment size must be at most 25MB")
	ErrAttachmentNameRequired = errors.New("attachment fileName is required")
	ErrAttachmentDataRequired = errors.New("attachment data is required")
)

type MessageRepository interface {
	SaveMessage(ctx context.Context, message domain.Message) (*domain.Message, error)
	GetMessagesByRoom(ctx context.Context, roomID string, limit int) ([]domain.Message, error)
	GetAttachment(ctx context.Context, roomID, attachmentID string) (*domain.Attachment, error)
}

type AttachmentInput struct {
	FileName    string
	ContentType string
	Data        []byte
}

type CreateMessageInput struct {
	RoomID      string
	SenderID    string
	SenderRole  string
	Content     string
	Attachments []AttachmentInput
}

type ListMessagesInput struct {
	RoomID string
	Limit  int
}

type ChatService struct {
	repo MessageRepository
	now  func() time.Time
}

const (
	maxMessageAttachments  = 5
	maxAttachmentSizeBytes = 25 << 20
)

func NewChatService(repo MessageRepository) *ChatService {
	return &ChatService{
		repo: repo,
		now:  time.Now().UTC,
	}
}

func (s *ChatService) CreateMessage(ctx context.Context, input CreateMessageInput) (*domain.Message, error) {
	roomID := strings.TrimSpace(input.RoomID)
	if roomID == "" {
		return nil, ErrRoomIDRequired
	}

	senderID := strings.TrimSpace(input.SenderID)
	if senderID == "" {
		return nil, ErrSenderIDRequired
	}
	if _, err := uuid.Parse(senderID); err != nil {
		return nil, ErrInvalidSenderID
	}

	content := strings.TrimSpace(input.Content)
	if content == "" {
		return nil, ErrContentRequired
	}
	if len([]rune(content)) > 2000 {
		return nil, ErrContentTooLong
	}

	senderRole := strings.ToLower(strings.TrimSpace(input.SenderRole))
	switch senderRole {
	case "admin", "teacher", "student":
	default:
		return nil, ErrInvalidSenderRole
	}

	message := domain.Message{
		ID:         uuid.NewString(),
		RoomID:     roomID,
		SenderID:   senderID,
		SenderRole: senderRole,
		Content:    content,
		CreatedAt:  s.now(),
	}
	attachments, err := buildMessageAttachments(message.ID, roomID, input.Attachments, message.CreatedAt)
	if err != nil {
		return nil, err
	}
	message.Attachments = attachments

	return s.repo.SaveMessage(ctx, message)
}

func (s *ChatService) GetMessagesByRoom(ctx context.Context, input ListMessagesInput) ([]domain.Message, error) {
	roomID := strings.TrimSpace(input.RoomID)
	if roomID == "" {
		return nil, ErrRoomIDRequired
	}

	limit := input.Limit
	switch {
	case limit == 0:
		limit = 50
	case limit < 1 || limit > 200:
		return nil, ErrInvalidLimit
	}

	return s.repo.GetMessagesByRoom(ctx, roomID, limit)
}

func (s *ChatService) GetAttachment(ctx context.Context, roomID, attachmentID string) (*domain.Attachment, error) {
	roomID = strings.TrimSpace(roomID)
	if roomID == "" {
		return nil, ErrRoomIDRequired
	}

	attachmentID = strings.TrimSpace(attachmentID)
	if attachmentID == "" {
		return nil, ErrAttachmentNotFound
	}

	return s.repo.GetAttachment(ctx, roomID, attachmentID)
}

func buildMessageAttachments(messageID, roomID string, inputs []AttachmentInput, now time.Time) ([]domain.Attachment, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	if len(inputs) > maxMessageAttachments {
		return nil, ErrTooManyAttachments
	}

	attachments := make([]domain.Attachment, 0, len(inputs))
	for _, input := range inputs {
		fileName := strings.TrimSpace(input.FileName)
		if fileName == "" {
			return nil, ErrAttachmentNameRequired
		}
		if len(input.Data) == 0 {
			return nil, ErrAttachmentDataRequired
		}
		if len(input.Data) > maxAttachmentSizeBytes {
			return nil, ErrAttachmentTooLarge
		}

		attachments = append(attachments, domain.Attachment{
			ID:          uuid.NewString(),
			MessageID:   messageID,
			RoomID:      roomID,
			Kind:        normalizeAttachmentKind(input.ContentType),
			FileName:    fileName,
			ContentType: strings.TrimSpace(input.ContentType),
			SizeBytes:   int64(len(input.Data)),
			CreatedAt:   now,
			Data:        append([]byte(nil), input.Data...),
		})
	}

	return attachments, nil
}

func normalizeAttachmentKind(contentType string) string {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	switch {
	case strings.HasPrefix(contentType, "image/"):
		return "image"
	case strings.HasPrefix(contentType, "video/"):
		return "video"
	default:
		return "file"
	}
}
