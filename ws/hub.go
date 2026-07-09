// Package ws implements the WebSocket hub for real-time direct messages.
//
// Architecture:
//
//	Client connects  →  hub.Register(client)
//	Client sends msg →  hub.Broadcast(conversationID, payload)
//	Client leaves    →  hub.Unregister(client)
//
// The hub keeps a map:  conversationID → set of *Client
// so a message is only pushed to the two participants of that conversation.
package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ─── Upgrader ─────────────────────────────────────────────────────────────────

var Upgrader = websocket.Upgrader{
	HandshakeTimeout: 10 * time.Second,
	ReadBufferSize:   1024,
	WriteBufferSize:  1024,
	// Allow all origins; tighten this in production via config / env.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ─── Client ───────────────────────────────────────────────────────────────────

// Client represents one WebSocket connection.
type Client struct {
	Hub            *Hub
	Conn           *websocket.Conn
	Send           chan []byte
	UserID         int64
	ConversationID int64
}

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
	maxMsgSize = 4096
)

// ReadPump pumps messages from the WebSocket connection to the hub.
// Each connection gets its own goroutine running ReadPump.
func (c *Client) ReadPump(onMessage func(client *Client, raw []byte)) {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMsgSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, raw, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[ws] read error uid=%d: %v", c.UserID, err)
			}
			break
		}
		onMessage(c, raw)
	}
}

// WritePump pumps messages from the hub to the WebSocket connection.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(msg)
			// Flush any queued messages in the same write frame.
			n := len(c.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.Send)
			}
			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ─── Hub ──────────────────────────────────────────────────────────────────────

// Hub maintains the set of active clients per conversation.
type Hub struct {
	mu sync.RWMutex
	// rooms: conversationID → set of clients
	rooms      map[int64]map[*Client]struct{}
	Register   chan *Client
	Unregister chan *Client
	broadcast  chan broadcastMsg
}

type broadcastMsg struct {
	ConversationID int64
	Payload        []byte
}

// NewHub creates a Hub and starts its run loop.
func NewHub() *Hub {
	h := &Hub{
		rooms:      make(map[int64]map[*Client]struct{}),
		Register:   make(chan *Client, 64),
		Unregister: make(chan *Client, 64),
		broadcast:  make(chan broadcastMsg, 256),
	}
	go h.run()
	return h
}

func (h *Hub) run() {
	for {
		select {
		case c := <-h.Register:
			h.mu.Lock()
			if h.rooms[c.ConversationID] == nil {
				h.rooms[c.ConversationID] = make(map[*Client]struct{})
			}
			h.rooms[c.ConversationID][c] = struct{}{}
			h.mu.Unlock()
			log.Printf("[ws] client registered uid=%d conv=%d", c.UserID, c.ConversationID)

		case c := <-h.Unregister:
			h.mu.Lock()
			if room, ok := h.rooms[c.ConversationID]; ok {
				delete(room, c)
				if len(room) == 0 {
					delete(h.rooms, c.ConversationID)
				}
			}
			close(c.Send)
			h.mu.Unlock()
			log.Printf("[ws] client unregistered uid=%d conv=%d", c.UserID, c.ConversationID)

		case bm := <-h.broadcast:
			h.mu.RLock()
			for c := range h.rooms[bm.ConversationID] {
				select {
				case c.Send <- bm.Payload:
				default:
					// slow client — drop and close
					close(c.Send)
					delete(h.rooms[bm.ConversationID], c)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends a JSON-serialisable payload to all clients in a conversation.
func (h *Hub) Broadcast(conversationID int64, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[ws] marshal error: %v", err)
		return
	}
	h.broadcast <- broadcastMsg{ConversationID: conversationID, Payload: raw}
}
