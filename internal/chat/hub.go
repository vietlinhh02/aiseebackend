package chat

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"nhooyr.io/websocket"
)

type Hub struct {
	mu    sync.RWMutex
	conns map[string]*websocket.Conn // userID -> conn
}

func NewHub() *Hub {
	return &Hub{conns: make(map[string]*websocket.Conn)}
}

func (h *Hub) Add(userID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if old, ok := h.conns[userID]; ok {
		old.CloseNow()
	}
	h.conns[userID] = conn
}

func (h *Hub) Remove(userID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.conns, userID)
}

type WSMessage struct {
	Type    string `json:"type"`
	Message any    `json:"message"`
}

func (h *Hub) SendToUser(userID string, msg WSMessage) {
	h.mu.RLock()
	conn, ok := h.conns[userID]
	h.mu.RUnlock()

	if !ok {
		return
	}

	data, err := json.Marshal(msg)
	if err != nil {
		slog.Error("marshal ws message", "error", err)
		return
	}

	if err := conn.Write(
		context.Background(), websocket.MessageText, data,
	); err != nil {
		slog.Error(
			"write ws message", "error", err, "user_id", userID,
		)
		h.Remove(userID)
	}
}

func (h *Hub) IsOnline(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.conns[userID]
	return ok
}
