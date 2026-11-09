package approvals

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// InAppNotifier manages in-app notifications
type InAppNotifier struct {
	store NotificationStore
	hub   *WebSocketHub
}

// NewInAppNotifier creates a new in-app notifier
func NewInAppNotifier(store NotificationStore, hub *WebSocketHub) *InAppNotifier {
	return &InAppNotifier{
		store: store,
		hub:   hub,
	}
}

// Send sends an in-app notification
func (n *InAppNotifier) Send(ctx context.Context, recipient string, event NotificationEvent) error {
	// Record the notification
	record := &NotificationRecord{
		ID:        generateID("notif"),
		Recipient: recipient,
		Channel:   ChannelInApp,
		Event:     event,
		Read:      false,
		CreatedAt: time.Now(),
	}

	if n.store != nil {
		if err := n.store.RecordNotification(recipient, ChannelInApp, event); err != nil {
			return err
		}
	}

	// Push via WebSocket if connected
	if n.hub != nil {
		n.hub.SendToUser(recipient, map[string]interface{}{
			"type":       "notification",
			"id":         record.ID,
			"event":      event,
			"created_at": record.CreatedAt,
		})
	}

	return nil
}

// Channel returns the channel type
func (n *InAppNotifier) Channel() NotificationChannel {
	return ChannelInApp
}

// WebSocketHub manages WebSocket connections for real-time notifications
type WebSocketHub struct {
	clients    map[string][]*WebSocketClient
	broadcast  chan *BroadcastMessage
	register   chan *WebSocketClient
	unregister chan *WebSocketClient
	mu         sync.RWMutex
}

// WebSocketClient represents a connected WebSocket client
type WebSocketClient struct {
	UserID string
	Send   chan []byte
}

// BroadcastMessage represents a message to broadcast
type BroadcastMessage struct {
	UserID string
	Data   interface{}
}

// NewWebSocketHub creates a new WebSocket hub
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[string][]*WebSocketClient),
		broadcast:  make(chan *BroadcastMessage, 256),
		register:   make(chan *WebSocketClient),
		unregister: make(chan *WebSocketClient),
	}
}

// Run starts the hub
func (h *WebSocketHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.UserID] = append(h.clients[client.UserID], client)
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			clients := h.clients[client.UserID]
			for i, c := range clients {
				if c == client {
					h.clients[client.UserID] = append(clients[:i], clients[i+1:]...)
					close(client.Send)
					break
				}
			}
			if len(h.clients[client.UserID]) == 0 {
				delete(h.clients, client.UserID)
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			data, err := json.Marshal(msg.Data)
			if err != nil {
				continue
			}
			h.mu.RLock()
			clients := h.clients[msg.UserID]
			for _, client := range clients {
				select {
				case client.Send <- data:
				default:
					// Client buffer full, skip
				}
			}
			h.mu.RUnlock()
		}
	}
}

// SendToUser sends a message to all connections of a user
func (h *WebSocketHub) SendToUser(userID string, data interface{}) {
	h.broadcast <- &BroadcastMessage{
		UserID: userID,
		Data:   data,
	}
}

// Register registers a new WebSocket client
func (h *WebSocketHub) Register(client *WebSocketClient) {
	h.register <- client
}

// Unregister unregisters a WebSocket client
func (h *WebSocketHub) Unregister(client *WebSocketClient) {
	h.unregister <- client
}
