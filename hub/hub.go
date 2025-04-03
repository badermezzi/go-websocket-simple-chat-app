package hub

import (
	"sync"

	"github.com/gorilla/websocket"
)

type Hub struct {
	clients map[int32]map[*websocket.Conn]bool

	mu sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[int32]map[*websocket.Conn]bool),
	}
}

// Register adds a new connection for a given user.
// It returns true if this was the user's first connection (meaning they just came online).
func (h *Hub) Register(userID int32, conn *websocket.Conn) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	userConnections, ok := h.clients[userID]
	isFirstConnection := !ok || len(userConnections) == 0

	if !ok {
		userConnections = make(map[*websocket.Conn]bool)
		h.clients[userID] = userConnections
	}
	userConnections[conn] = true

	return isFirstConnection
}

// Unregister removes a connection for a given user.
// It returns true if this was the user's last connection (meaning they just went offline).
func (h *Hub) Unregister(userID int32, conn *websocket.Conn) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	userConnections, ok := h.clients[userID]
	if !ok {
		return false
	}

	delete(userConnections, conn)

	isLastConnection := len(userConnections) == 0
	if isLastConnection {
		delete(h.clients, userID)
	}

	return isLastConnection
}

// GetUserConnections returns a slice of active connections for a given user.
// It returns an empty slice if the user is not connected or not found.
func (h *Hub) GetUserConnections(userID int32) []*websocket.Conn {
	h.mu.RLock() // Use Read Lock for reading
	defer h.mu.RUnlock()

	userConnectionsMap, ok := h.clients[userID]
	if !ok {
		return []*websocket.Conn{} // Return empty slice if user not found
	}

	// Create a slice to hold the connections
	connections := make([]*websocket.Conn, 0, len(userConnectionsMap))
	for conn := range userConnectionsMap {
		connections = append(connections, conn)
	}
	return connections
}

// TODO: Add methods for broadcasting messages if needed later.
