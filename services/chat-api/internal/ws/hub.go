package ws

import (
	"context"

	"proyecto-cursos/internal/platform/logger"
)

type broadcastMessage struct {
	roomID  string
	payload []byte
}

type Hub struct {
	rooms      map[string]map[*Client]struct{}
	register   chan *Client
	unregister chan *Client
	broadcast  chan broadcastMessage
	log        *logger.Logger
}

func NewHub(log *logger.Logger) *Hub {
	return &Hub{
		rooms:      make(map[string]map[*Client]struct{}),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan broadcastMessage, 128),
		log:        log,
	}
}

func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			h.closeAll()
			return
		case client := <-h.register:
			if h.rooms[client.roomID] == nil {
				h.rooms[client.roomID] = make(map[*Client]struct{})
			}
			h.rooms[client.roomID][client] = struct{}{}
			h.log.Info(ctx, "ws client registered", map[string]any{
				"roomId": client.roomID,
				"userId": client.userID,
				"count":  len(h.rooms[client.roomID]),
			})
		case client := <-h.unregister:
			h.removeClient(client)
		case event := <-h.broadcast:
			h.broadcastToRoom(event.roomID, event.payload)
		}
	}
}

func (h *Hub) Register(client *Client) {
	h.register <- client
}

func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

func (h *Hub) Broadcast(roomID string, payload []byte) {
	h.broadcast <- broadcastMessage{
		roomID:  roomID,
		payload: payload,
	}
}

func (h *Hub) removeClient(client *Client) {
	room, ok := h.rooms[client.roomID]
	if !ok {
		return
	}

	if _, exists := room[client]; exists {
		delete(room, client)
		close(client.send)
	}

	if len(room) == 0 {
		delete(h.rooms, client.roomID)
	}

	h.log.Info(context.Background(), "ws client unregistered", map[string]any{
		"roomId": client.roomID,
		"userId": client.userID,
		"count":  len(room),
	})
}

func (h *Hub) broadcastToRoom(roomID string, payload []byte) {
	room, ok := h.rooms[roomID]
	if !ok {
		return
	}

	for client := range room {
		select {
		case client.send <- payload:
		default:
			delete(room, client)
			close(client.send)
		}
	}

	if len(room) == 0 {
		delete(h.rooms, roomID)
	}
}

func (h *Hub) closeAll() {
	for roomID, room := range h.rooms {
		for client := range room {
			close(client.send)
		}
		delete(h.rooms, roomID)
	}
}
