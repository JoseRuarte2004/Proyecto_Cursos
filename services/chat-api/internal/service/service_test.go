package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"proyecto-cursos/services/chat-api/internal/domain"
)

type fakeRepo struct {
	saveFn          func(ctx context.Context, message domain.Message) (*domain.Message, error)
	listFn          func(ctx context.Context, roomID string, limit int) ([]domain.Message, error)
	getAttachmentFn func(ctx context.Context, roomID, attachmentID string) (*domain.Attachment, error)
}

func (f fakeRepo) SaveMessage(ctx context.Context, message domain.Message) (*domain.Message, error) {
	return f.saveFn(ctx, message)
}

func (f fakeRepo) GetMessagesByRoom(ctx context.Context, roomID string, limit int) ([]domain.Message, error) {
	return f.listFn(ctx, roomID, limit)
}

func (f fakeRepo) GetAttachment(ctx context.Context, roomID, attachmentID string) (*domain.Attachment, error) {
	return f.getAttachmentFn(ctx, roomID, attachmentID)
}

func TestCreateMessage(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	var captured domain.Message
	svc := NewChatService(fakeRepo{
		saveFn: func(_ context.Context, msg domain.Message) (*domain.Message, error) {
			captured = msg
			return &msg, nil
		},
		listFn:          func(context.Context, string, int) ([]domain.Message, error) { return nil, nil },
		getAttachmentFn: func(context.Context, string, string) (*domain.Attachment, error) { return nil, nil },
	})
	svc.now = func() time.Time { return now }

	msg, err := svc.CreateMessage(context.Background(), CreateMessageInput{
		RoomID:     "class_123",
		SenderID:   "11111111-1111-1111-1111-111111111111",
		SenderRole: "teacher",
		Content:    "hola",
		Attachments: []AttachmentInput{{
			FileName:    "captura.png",
			ContentType: "image/png",
			Data:        []byte("png"),
		}},
	})
	require.NoError(t, err)
	require.Equal(t, "class_123", msg.RoomID)
	require.Equal(t, "hola", captured.Content)
	require.Equal(t, "teacher", captured.SenderRole)
	require.Equal(t, now, captured.CreatedAt)
	require.Len(t, captured.Attachments, 1)
	require.Equal(t, "image", captured.Attachments[0].Kind)
}

func TestGetMessagesByRoom(t *testing.T) {
	t.Parallel()

	svc := NewChatService(fakeRepo{
		saveFn: func(context.Context, domain.Message) (*domain.Message, error) { return nil, nil },
		listFn: func(_ context.Context, roomID string, limit int) ([]domain.Message, error) {
			require.Equal(t, "class_1", roomID)
			require.Equal(t, 20, limit)
			return []domain.Message{{ID: "1", RoomID: roomID}}, nil
		},
		getAttachmentFn: func(context.Context, string, string) (*domain.Attachment, error) { return nil, nil },
	})

	result, err := svc.GetMessagesByRoom(context.Background(), ListMessagesInput{
		RoomID: "class_1",
		Limit:  20,
	})
	require.NoError(t, err)
	require.Len(t, result, 1)
}

func TestGetAttachment(t *testing.T) {
	t.Parallel()

	expected := &domain.Attachment{ID: "att-1", RoomID: "class_1", FileName: "guia.pdf"}
	svc := NewChatService(fakeRepo{
		saveFn: func(context.Context, domain.Message) (*domain.Message, error) { return nil, nil },
		listFn: func(context.Context, string, int) ([]domain.Message, error) { return nil, nil },
		getAttachmentFn: func(_ context.Context, roomID, attachmentID string) (*domain.Attachment, error) {
			require.Equal(t, "class_1", roomID)
			require.Equal(t, "att-1", attachmentID)
			return expected, nil
		},
	})

	attachment, err := svc.GetAttachment(context.Background(), "class_1", "att-1")
	require.NoError(t, err)
	require.Equal(t, expected, attachment)
}
