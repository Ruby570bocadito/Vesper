package api

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ruby570bocadito/vesper/pkg/shared/logger"
)

// WSMessage represents a WebSocket message sent to clients.
type WSMessage struct {
	Type       string      `json:"type"`
	CampaignID string      `json:"campaign_id,omitempty"`
	AgentID    string      `json:"agent_id,omitempty"`
	Timestamp  time.Time   `json:"timestamp"`
	Data       interface{} `json:"data,omitempty"`
}

// WSClient represents a connected WebSocket client.
type WSClient struct {
	id         string
	conn       *websocket.Conn
	campaignID string
	hub        *WSHub
	send       chan []byte
	mu         sync.Mutex
}

// Send sends a message to this client.
func (c *WSClient) Send(msg WSMessage) {
	msg.Timestamp = time.Now()
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case c.send <- data:
	default:
		// Client buffer full — skip message
	}
}

// WSHub manages all WebSocket connections and broadcasts.
type WSHub struct {
	log        *logger.Logger
	clients    map[string]*WSClient
	register   chan *WSClient
	unregister chan *WSClient
	broadcast  chan WSMessage
	mu         sync.RWMutex
	idCounter  int
	running    bool
}

// NewWSHub creates a new WebSocket hub.
func NewWSHub(log *logger.Logger) *WSHub {
	hub := &WSHub{
		log:        log,
		clients:    make(map[string]*WSClient),
		register:   make(chan *WSClient, 32),
		unregister: make(chan *WSClient, 32),
		broadcast:  make(chan WSMessage, 256),
	}

	go hub.run()
	return hub
}

func (h *WSHub) run() {
	h.running = true
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.id] = client
			count := len(h.clients)
			h.mu.Unlock()
			h.log.Debugf("ws client connected (id=%s, total=%d)", client.id, count)

		case client := <-h.unregister:
			h.mu.Lock()
			delete(h.clients, client.id)
			count := len(h.clients)
			h.mu.Unlock()
			close(client.send)
			client.conn.Close()
			h.log.Debugf("ws client disconnected (id=%s, total=%d)", client.id, count)

		case msg := <-h.broadcast:
			h.mu.RLock()
			for _, client := range h.clients {
				// Filter by campaign if set
				if msg.CampaignID != "" && client.campaignID != "" && client.campaignID != msg.CampaignID {
					continue
				}
				client.Send(msg)
			}
			h.mu.RUnlock()
		}
	}
}

// Register adds a new WebSocket client.
func (h *WSHub) Register(conn *websocket.Conn, campaignID string) *WSClient {
	h.idCounter++
	client := &WSClient{
		id:         generateClientID(h.idCounter),
		conn:       conn,
		campaignID: campaignID,
		hub:        h,
		send:       make(chan []byte, 64),
	}

	h.register <- client

	// Start write pump
	go func() {
		for msg := range client.send {
			client.mu.Lock()
			err := client.conn.WriteMessage(websocket.TextMessage, msg)
			client.mu.Unlock()
			if err != nil {
				break
			}
		}
	}()

	return client
}

// Unregister removes a WebSocket client.
func (h *WSHub) Unregister(client *WSClient) {
	h.unregister <- client
}

// Broadcast sends a message to all connected clients.
func (h *WSHub) Broadcast(msg WSMessage) {
	select {
	case h.broadcast <- msg:
	default:
		// Broadcast buffer full — drop message
	}
}

// ClientCount returns the number of connected clients.
func (h *WSHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Stop shuts down the hub.
func (h *WSHub) Stop() {
	h.running = false
	h.mu.Lock()
	for id, client := range h.clients {
		client.conn.Close()
		delete(h.clients, id)
	}
	h.mu.Unlock()
}

func generateClientID(counter int) string {
	return "ws-" + time.Now().Format("150405") + "-" + string(rune('A'+counter%26))
}
