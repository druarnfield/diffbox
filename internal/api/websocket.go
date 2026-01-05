package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for self-hosted
	},
}

// WebSocket message types
type WSMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

type JobProgress struct {
	JobID    string  `json:"job_id"`
	Progress float64 `json:"progress"`
	Stage    string  `json:"stage"`
	Preview  string  `json:"preview,omitempty"` // base64 preview frame
}

type JobComplete struct {
	JobID  string    `json:"job_id"`
	Output JobOutput `json:"output"`
}

type JobError struct {
	JobID string `json:"job_id"`
	Error string `json:"error"`
}

type DownloadProgress struct {
	DownloadID string  `json:"download_id"`
	ModelID    string  `json:"model_id"`
	Progress   float64 `json:"progress"`
	Speed      string  `json:"speed"`
}

type SubscribeMessage struct {
	JobIDs []string `json:"job_ids"`
}

// WebSocket Hub manages all client connections
type WebSocketHub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

type Client struct {
	hub          *WebSocketHub
	conn         *websocket.Conn
	send         chan []byte
	subscribedTo map[string]bool
	mu           sync.RWMutex
}

func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *WebSocketHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastJobProgress sends job progress to subscribed clients
func (h *WebSocketHub) BroadcastJobProgress(progress JobProgress) {
	data, _ := json.Marshal(progress)
	msg := WSMessage{
		Type: "job:progress",
		Data: data,
	}
	msgBytes, _ := json.Marshal(msg)
	h.broadcast <- msgBytes
}

// BroadcastJobComplete sends job completion to subscribed clients
func (h *WebSocketHub) BroadcastJobComplete(complete JobComplete) {
	data, _ := json.Marshal(complete)
	msg := WSMessage{
		Type: "job:complete",
		Data: data,
	}
	msgBytes, _ := json.Marshal(msg)
	h.broadcast <- msgBytes
}

// BroadcastJobError sends job error to subscribed clients
func (h *WebSocketHub) BroadcastJobError(jobError JobError) {
	data, _ := json.Marshal(jobError)
	msg := WSMessage{
		Type: "job:error",
		Data: data,
	}
	msgBytes, _ := json.Marshal(msg)
	h.broadcast <- msgBytes
}

// BroadcastDownloadProgress sends download progress
func (h *WebSocketHub) BroadcastDownloadProgress(progress DownloadProgress) {
	data, _ := json.Marshal(progress)
	msg := WSMessage{
		Type: "download:progress",
		Data: data,
	}
	msgBytes, _ := json.Marshal(msg)
	h.broadcast <- msgBytes
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:          s.hub,
		conn:         conn,
		send:         make(chan []byte, 256),
		subscribedTo: make(map[string]bool),
	}

	s.hub.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		var msg WSMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "subscribe":
			var sub SubscribeMessage
			json.Unmarshal(msg.Data, &sub)
			c.mu.Lock()
			for _, jobID := range sub.JobIDs {
				c.subscribedTo[jobID] = true
			}
			c.mu.Unlock()

		case "unsubscribe":
			var sub SubscribeMessage
			json.Unmarshal(msg.Data, &sub)
			c.mu.Lock()
			for _, jobID := range sub.JobIDs {
				delete(c.subscribedTo, jobID)
			}
			c.mu.Unlock()
		}
	}
}

func (c *Client) writePump() {
	defer c.conn.Close()

	for message := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
			break
		}
	}
}
