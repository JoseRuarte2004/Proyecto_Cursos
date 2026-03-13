package ws

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"time"

	"github.com/gorilla/websocket"

	"proyecto-cursos/internal/platform/logger"
	"proyecto-cursos/services/chat-api/internal/domain"
	"proyecto-cursos/services/chat-api/internal/service"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
)

type incomingPayload struct {
	Content string `json:"content"`
}

type outgoingPayload struct {
	Type        string              `json:"type"`
	ID          string              `json:"id"`
	RoomID      string              `json:"roomId"`
	CourseID    string              `json:"courseId"`
	SenderID    string              `json:"senderId"`
	SenderName  string              `json:"senderName,omitempty"`
	SenderRole  string              `json:"senderRole"`
	Content     string              `json:"content"`
	Attachments []attachmentPayload `json:"attachments,omitempty"`
	CreatedAt   string              `json:"createdAt"`
}

type attachmentPayload struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
	URL         string `json:"url"`
	CreatedAt   string `json:"createdAt"`
}

type errorPayload struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type Client struct {
	hub      *Hub
	service  *service.ChatService
	conn     *websocket.Conn
	send     chan []byte
	roomID   string
	courseID string
	userID   string
	role     string
	userName string
	log      *logger.Logger
}

func NewClient(conn *websocket.Conn, hub *Hub, chatService *service.ChatService, roomID, courseID, userID, role, userName string, log *logger.Logger) *Client {
	return &Client{
		hub:      hub,
		service:  chatService,
		conn:     conn,
		send:     make(chan []byte, 64),
		roomID:   roomID,
		courseID: courseID,
		userID:   userID,
		role:     role,
		userName: userName,
		log:      log,
	}
}

func (c *Client) ReadPump(ctx context.Context) {
	defer func() {
		c.hub.Unregister(c)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		var incoming incomingPayload
		if err := c.conn.ReadJSON(&incoming); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.log.Error(ctx, "websocket read failed", map[string]any{
					"roomId": c.roomID,
					"userId": c.userID,
					"error":  err.Error(),
				})
			}
			return
		}

		message, err := c.service.CreateMessage(ctx, service.CreateMessageInput{
			RoomID:     c.roomID,
			SenderID:   c.userID,
			SenderRole: c.role,
			Content:    incoming.Content,
		})
		if err != nil {
			c.pushError(err)
			continue
		}

		payload, err := json.Marshal(outgoingPayload{
			Type:        "message",
			ID:          message.ID,
			RoomID:      message.RoomID,
			CourseID:    c.courseID,
			SenderID:    message.SenderID,
			SenderName:  c.userName,
			SenderRole:  message.SenderRole,
			Content:     message.Content,
			Attachments: buildAttachmentPayloads(message.RoomID, message.Attachments),
			CreatedAt:   message.CreatedAt.Format(time.RFC3339Nano),
		})
		if err != nil {
			c.pushError(err)
			continue
		}

		c.hub.Broadcast(c.roomID, payload)
	}
}

func buildAttachmentPayloads(roomID string, attachments []domain.Attachment) []attachmentPayload {
	if len(attachments) == 0 {
		return nil
	}

	payloads := make([]attachmentPayload, 0, len(attachments))
	for _, attachment := range attachments {
		payloads = append(payloads, attachmentPayload{
			ID:          attachment.ID,
			Kind:        attachment.Kind,
			FileName:    attachment.FileName,
			ContentType: attachment.ContentType,
			SizeBytes:   attachment.SizeBytes,
			URL:         "/chat/attachments/" + url.PathEscape(attachment.ID) + "?room=" + url.QueryEscape(roomID),
			CreatedAt:   attachment.CreatedAt.Format(time.RFC3339Nano),
		})
	}

	return payloads
}

func (c *Client) WritePump(ctx context.Context) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			_ = c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"))
			return
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "bye"))
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) pushError(err error) {
	payload, marshalErr := json.Marshal(errorPayload{
		Type:    "error",
		Message: cleanError(err),
	})
	if marshalErr != nil {
		return
	}

	select {
	case c.send <- payload:
	default:
	}
}

func cleanError(err error) string {
	if err == nil {
		return "unexpected error"
	}

	switch {
	case errors.Is(err, service.ErrRoomIDRequired),
		errors.Is(err, service.ErrContentRequired),
		errors.Is(err, service.ErrContentTooLong),
		errors.Is(err, service.ErrInvalidSenderRole),
		errors.Is(err, service.ErrInvalidSenderID),
		errors.Is(err, service.ErrTooManyAttachments),
		errors.Is(err, service.ErrAttachmentTooLarge),
		errors.Is(err, service.ErrAttachmentNameRequired),
		errors.Is(err, service.ErrAttachmentDataRequired):
		return err.Error()
	default:
		return "unexpected error"
	}
}
