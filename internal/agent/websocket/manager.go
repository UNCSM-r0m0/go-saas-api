package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/r0lm0/go-saas-api/internal/agent/runtime"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // allow all origins in dev; restrict in production
	},
}

// Client represents a single WebSocket connection.
type Client struct {
	conn    *websocket.Conn
	userID  uuid.UUID
	sendCh  chan []byte
	manager *Manager

	// active generation cancellation
	mu        sync.Mutex
	cancelGen context.CancelFunc
}

// Manager manages WebSocket clients.
type Manager struct {
	clients    map[string]*Client // key: userID
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	orch       *runtime.Orchestrator
	log        logger.Logger
}

// NewManager creates a new WebSocket manager.
func NewManager(orch *runtime.Orchestrator, log logger.Logger) *Manager {
	m := &Manager{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		orch:       orch,
		log:        log,
	}
	go m.run()
	return m
}

func (m *Manager) run() {
	for {
		select {
		case client := <- m.register:
			key := clientKey(client.userID)
			m.mu.Lock()
			// Close existing connection for same user
			if old, ok := m.clients[key]; ok {
				close(old.sendCh)
				old.conn.Close()
			}
			m.clients[key] = client
			m.mu.Unlock()
			m.log.Info("websocket client connected", logger.String("user", client.userID.String()))

		case client := <- m.unregister:
			key := clientKey(client.userID)
			m.mu.Lock()
			if _, ok := m.clients[key]; ok {
				delete(m.clients, key)
				close(client.sendCh)
				client.conn.Close()
			}
			m.mu.Unlock()
			m.log.Info("websocket client disconnected", logger.String("user", client.userID.String()))
		}
	}
}

func clientKey(userID uuid.UUID) string {
	return userID.String()
}

// HandleUpgrade upgrades an HTTP connection to WebSocket.
func (m *Manager) HandleUpgrade(c *gin.Context) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing X-User-ID"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		m.log.Error("websocket upgrade failed", logger.Error(err))
		return
	}

	client := &Client{
		conn:    conn,
		userID:  userID,
		sendCh:  make(chan []byte, 256),
		manager: m,
	}

	m.register <- client

	go client.writePump()
	go client.readPump()
}

// readPump reads messages from the WebSocket connection.
func (c *Client) readPump() {
	defer func() {
		c.manager.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512 * 1024) // 512KB max message size
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, msgBytes, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.manager.log.Error("websocket read error", logger.Error(err))
			}
			break
		}

		var msg Message
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			c.sendError("invalid message format")
			continue
		}

		switch msg.Type {
		case TypeChat:
			go c.handleChat(msg)
		case TypeStop:
			c.handleStop()
		case TypePing:
			c.send(Message{Type: TypePong})
		default:
			c.sendError("unknown message type: " + string(msg.Type))
		}
	}
}

// writePump writes messages to the WebSocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <- c.sendCh:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.conn.WriteMessage(websocket.TextMessage, message)

		case <- ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleChat(msg Message) {
	if msg.Content == "" {
		c.sendError("content is required")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.mu.Lock()
	if c.cancelGen != nil {
		c.cancelGen()
	}
	c.cancelGen = cancel
	c.mu.Unlock()

	streamCh, err := c.manager.orch.Chat(ctx, c.userID, msg.ConversationID, msg.Content, msg.FileIDs, msg.Model)
	if err != nil {
		c.sendError(fmt.Sprintf("chat failed: %v", err))
		c.clearCancel()
		return
	}

	messageID := uuid.New().String()
	var tokens int
	for chunk := range streamCh {
		tokens += len(chunk.Content)

		switch chunk.Event {
		case "tool_start":
			c.send(Message{
				Type:       TypeToolStart,
				MessageID:  messageID,
				ToolName:   chunk.ToolName,
				ToolCallID: chunk.ToolCall.ID,
				ToolArgs:   chunk.ToolCall.Arguments,
			})
		case "tool_result":
			c.send(Message{
				Type:       TypeToolResult,
				MessageID:  messageID,
				ToolName:   chunk.ToolName,
				Content:    chunk.Content,
			})
		case "error":
			c.send(Message{
				Type:      TypeError,
				MessageID: messageID,
				Error:     chunk.Content,
			})
		default:
			resp := Message{
				Type:      TypeChunk,
				MessageID: messageID,
				Content:   chunk.Content,
				Done:      chunk.Done,
			}
			if !c.send(resp) {
				break
			}
		}

		if chunk.Done {
			break
		}
	}

	c.send(Message{
		Type:       TypeDone,
		MessageID:  messageID,
		Done:       true,
		TokensUsed: tokens,
	})

	c.clearCancel()
}

func (c *Client) handleStop() {
	c.mu.Lock()
	if c.cancelGen != nil {
		c.cancelGen()
	}
	c.mu.Unlock()
}

func (c *Client) clearCancel() {
	c.mu.Lock()
	c.cancelGen = nil
	c.mu.Unlock()
}

func (c *Client) send(msg Message) bool {
	data, err := json.Marshal(msg)
	if err != nil {
		return false
	}
	select {
	case c.sendCh <- data:
		return true
	default:
		return false
	}
}

func (c *Client) sendError(err string) {
	c.send(Message{Type: TypeError, Error: err})
}
