package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"proyecto-cursos/services/chat-api/internal/domain"
)

type fakeUsersAccess struct {
	getUserFn func(ctx context.Context, userID string) (*UserProfile, error)
}

func (f fakeUsersAccess) GetUser(ctx context.Context, userID string) (*UserProfile, error) {
	if f.getUserFn == nil {
		return nil, errors.New("unexpected call")
	}
	return f.getUserFn(ctx, userID)
}

func TestBuildMessageResponsesIncludesSenderNames(t *testing.T) {
	t.Parallel()

	lookups := 0
	handler := &Handler{
		users: fakeUsersAccess{
			getUserFn: func(_ context.Context, userID string) (*UserProfile, error) {
				lookups++
				switch userID {
				case "user-1":
					return &UserProfile{ID: "user-1", Name: "Ada Lovelace"}, nil
				case "user-2":
					return &UserProfile{ID: "user-2", Name: "Alan Turing"}, nil
				default:
					return nil, ErrUserNotFound
				}
			},
		},
	}

	now := time.Date(2026, time.March, 11, 3, 20, 0, 0, time.UTC)
	response := handler.buildMessageResponses(context.Background(), []domain.Message{
		{ID: "1", RoomID: "class_course-1", SenderID: "user-1", SenderRole: "teacher", Content: "hola", CreatedAt: now},
		{ID: "2", RoomID: "class_course-1", SenderID: "user-2", SenderRole: "student", Content: "buenas", CreatedAt: now},
		{ID: "3", RoomID: "class_course-1", SenderID: "user-1", SenderRole: "teacher", Content: "seguimos", CreatedAt: now},
	})

	require.Len(t, response, 3)
	require.Equal(t, "Ada Lovelace", response[0].SenderName)
	require.Equal(t, "Alan Turing", response[1].SenderName)
	require.Equal(t, "Ada Lovelace", response[2].SenderName)
	require.Equal(t, 2, lookups)
}

func TestBuildMessageResponseFallsBackWhenUserLookupFails(t *testing.T) {
	t.Parallel()

	handler := &Handler{
		users: fakeUsersAccess{
			getUserFn: func(context.Context, string) (*UserProfile, error) {
				return nil, errors.New("users api unavailable")
			},
		},
	}

	response := handler.buildMessageResponse(context.Background(), domain.Message{
		ID:         "1",
		RoomID:     "class_course-1",
		SenderID:   "user-1",
		SenderRole: "student",
		Content:    "hola",
		CreatedAt:  time.Date(2026, time.March, 11, 3, 21, 0, 0, time.UTC),
	})

	require.Equal(t, "", response.SenderName)
	require.Equal(t, "user-1", response.SenderID)
}
