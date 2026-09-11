package api

import (
	"crypto/rand"
	"encoding/hex"
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
	mu         sync.Mutex // guards send channel close vs Send()
	closed     bool
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
	if c.closed {
		return
	}

	select {
	case c.send <- data:
	default:
		// Client buffer full — skip message
	}
}

// close marks the client as closed and shuts its channel + connection.
// Safe to call multiple times.
func (c *WSClient) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	close(c.send)
	_ = c.conn.Close()
}

// WSHub manages all WebSocket connections and broadcasts.
type WSHub struct {
	log        *logger.Logger
	clients    map[string]*WSClient
	register   chan *WSClient
	unregister chan *WSClient
	broadcast  chan WSMessage
	mu         sync.RWMutex
	idGen      func() string // unique client IDs (overridable in tests)
}

// NewWSHub creates a new WebSocket hub.
func NewWSHub(log *logger.Logger) *WSHub {
	hub := &WSHub{
		log:        log,
		clients:    make(map[string]*WSClient),
		register:   make(chan *WSClient, 32),
		unregister: make(chan *WSClient, 32),
		broadcast:  make(chan WSMessage, 256),
		idGen:      randomClientID,
	}

	go hub.run()
	return hub
}

func (h *WSHub) run() {
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
			client.close() // close(send) under client.mu — no send-on-closed race
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
	client := &WSClient{
		id:         h.uniqueID(),
		conn:       conn,
		campaignID: campaignID,
		hub:        h,
		send:       make(chan []byte, 64),
	}

	h.register <- client

	// Write pump: exits when client.send is closed by the hub; a write
	// error also deregisters so dead clients never linger.
	go func() {
		for msg := range client.send {
			client.mu.Lock()
			err := client.conn.WriteMessage(websocket.TextMessage, msg)
			client.mu.Unlock()
			if err != nil {
				h.Unregister(client)
				return
			}
		}
	}()

	return client
}

func (h *WSHub) uniqueID() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	for range 100 {
		id := h.idGen()
		if _, taken := h.clients[id]; !taken {
			return id
		}
	}
	return time.Now().Format("150405.000000000")
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

// Stop shuts down the hub and closes every client connection.
func (h *WSHub) Stop() {
	h.mu.Lock()
	clients := make([]*WSClient, 0, len(h.clients))
	for _, client := range h.clients {
		clients = append(clients, client)
		delete(h.clients, client.id)
	}
	h.mu.Unlock()
	for _, client := range clients {
		client.close()
	}
}

func randomClientID() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return "ws-" + hex.EncodeToString(buf)
}
