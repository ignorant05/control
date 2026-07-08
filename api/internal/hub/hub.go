package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/ignorant05/control/api/cmd/model"
	"github.com/ignorant05/control/api/internal/store"
)

// Websocket client initialization
type Client struct {
	ID        string
	UserID    string
	ProjectID string
	Role      model.Role
	Conn      *websocket.Conn
	Send      chan []byte
	Hub       *Hub
	mu        sync.RWMutex
}

func NewClient(id, userID, projectID string, role model.Role, conn *websocket.Conn, hub *Hub) *Client {
	return &Client{
		ID: id, UserID: userID, ProjectID: projectID,
		Role: role, Conn: conn, Send: make(chan []byte, 256), Hub: hub,
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() { ticker.Stop(); c.Conn.Close() }()
	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.Conn.WriteMessage(websocket.TextMessage, message)
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) ReadPump() {
	defer func() { c.Hub.unregister <- c; c.Conn.Close() }()
	c.Conn.SetReadLimit(65536)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error { c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second)); return nil })
	for {
		if _, _, err := c.Conn.ReadMessage(); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
			}
			break
		}
	}
}

func (c *Client) SendMessage(msgType string, payload interface{}) error {
	msg := model.WSMessage{Type: msgType, ProjectID: c.ProjectID, Payload: payload, Timestamp: time.Now()}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	select {
	case c.Send <- data:
	default:
	}
	return nil
}

type Hub struct {
	clients    map[string]*Client
	projects   map[string]map[string]*Client
	register   chan *Client
	unregister chan *Client
	broadcast  chan *model.WSMessage
	redis      *store.RedisStore
	mu         sync.RWMutex
}

func NewHub(redis *store.RedisStore) *Hub {
	return &Hub{
		clients: make(map[string]*Client), projects: make(map[string]map[string]*Client),
		register: make(chan *Client), unregister: make(chan *Client),
		broadcast: make(chan *model.WSMessage, 256), redis: redis,
	}
}

func (h *Hub) Run(ctx context.Context) {
	if h.redis != nil {
		go h.listenRedis(ctx)
	}
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.ID] = client
			if h.projects[client.ProjectID] == nil {
				h.projects[client.ProjectID] = make(map[string]*Client)
			}
			h.projects[client.ProjectID][client.ID] = client
			h.mu.Unlock()
			if h.redis != nil {
				h.redis.AddClient(ctx, client.ProjectID, client.ID)
			}
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.ID]; ok {
				delete(h.clients, client.ID)
				close(client.Send)
				if h.projects[client.ProjectID] != nil {
					delete(h.projects[client.ProjectID], client.ID)
				}
			}
			h.mu.Unlock()
			if h.redis != nil {
				h.redis.RemoveClient(ctx, client.ProjectID, client.ID)
			}
		case msg := <-h.broadcast:
			h.broadcastToProject(ctx, msg)
		}
	}
}

func (h *Hub) broadcastToProject(ctx context.Context, msg *model.WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.mu.RLock()
	projectClients := h.projects[msg.ProjectID]
	clientCount := len(projectClients)
	h.mu.RUnlock()
	fmt.Printf("[HUB] broadcast to project %s: %d clients, type=%s\n", msg.ProjectID, clientCount, msg.Type)
	for _, client := range projectClients {
		select {
		case client.Send <- data:
			fmt.Printf("[HUB] sent to client %s\n", client.ID)
		default:
			fmt.Printf("[HUB] client %s send buffer full, dropped\n", client.ID)
		}
	}
	if h.redis != nil {
		h.redis.Publish(ctx, "ws:broadcast", msg)
	}
}

// listenRedis subscribe to redis db to access cached content on change
func (h *Hub) listenRedis(ctx context.Context) {
	pubsub := h.redis.Subscribe(ctx, "ws:broadcast")
	defer pubsub.Close()
	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-ch:
			if msg == nil {
				continue
			}
			var wsMsg model.WSMessage
			if err := json.Unmarshal([]byte(msg.Payload), &wsMsg); err != nil {
				continue
			}
			h.mu.RLock()
			projectClients := h.projects[wsMsg.ProjectID]
			h.mu.RUnlock()
			data, _ := json.Marshal(&wsMsg)
			for _, client := range projectClients {
				select {
				case client.Send <- data:
				default:
				}
			}
		}
	}
}

// Broadcast the new changes if any
func (h *Hub) Broadcast(projectID string, msgType string, payload interface{}) {
	h.broadcast <- &model.WSMessage{Type: msgType, ProjectID: projectID, Payload: payload, Timestamp: time.Now()}
}

func (h *Hub) BroadcastFlagChange(projectID string, flag *model.FeatureFlag, action model.AuditAction, user string) {
	h.Broadcast(projectID, "flag_change", map[string]interface{}{"flag": flag, "action": action, "user": user})
}

func (h *Hub) BroadcastUserChange(projectID string, user *model.User, action string) {
	h.Broadcast(projectID, "user_change", map[string]interface{}{"user": user, "action": action})
}

func (h *Hub) BroadcastAuditEntry(projectID string, entry *model.AuditEntry) {
	h.Broadcast(projectID, "audit", entry)
}

// ServeWS handles WebSocket upgrade requests (legacy — uses user.Project as projectID)
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request, user *model.User) {
	h.ServeWSByProjectID(w, r, user, user.Project)
}

// ServeWSByProjectID handles WebSocket upgrade with explicit project ID
func (h *Hub) ServeWSByProjectID(w http.ResponseWriter, r *http.Request, user *model.User, projectID string) {
	upgrader := websocket.Upgrader{
		ReadBufferSize: 1024, WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Could not upgrade connection", http.StatusBadRequest)
		return
	}
	clientID := uuid.New().String()
	fmt.Printf("[HUB] new client %s for user %s project %s\n", clientID, user.Name, projectID)
	client := NewClient(clientID, user.ID, projectID, user.Role, conn, h)
	h.register <- client
	go client.WritePump()
	go client.ReadPump()
}

// GetProjectClients returns the client's projects number (how much)
func (h *Hub) GetProjectClients(projectID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.projects[projectID])
}
