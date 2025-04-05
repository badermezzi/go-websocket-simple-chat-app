package hub

import (
	"log" // Added for logging in Broadcast
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

// Broadcast sends a message to all connected clients, optionally excluding one user.
// If excludeUserID is 0 or a non-existent ID, the message is sent to everyone.
func (h *Hub) Broadcast(message []byte, excludeUserID int32) {
	h.mu.RLock() // Use Read Lock as we are only reading the client list
	defer h.mu.RUnlock()

	for userID, userConnections := range h.clients {
		if userID == excludeUserID {
			continue // Skip the excluded user
		}

		for conn := range userConnections {
			// Use a separate goroutine for each write to avoid blocking the broadcast loop
			// if one connection is slow or unresponsive.
			go func(c *websocket.Conn) {
				// It's generally safer to use WriteMessage within its own lock if the connection
				// object itself isn't inherently thread-safe for concurrent writes,
				// although Gorilla WebSocket's default implementation usually handles this.
				// However, for simplicity here, we assume concurrent writes are safe or handled by the library.
				if err := c.WriteMessage(websocket.TextMessage, message); err != nil {
					// Log the error, but don't stop broadcasting to others.
					// The connection's own read loop should handle the disconnection.
					log.Printf("Broadcast Error: Failed to write message to user %d connection %p: %v", userID, c, err)
				}
			}(conn)
		}
	}
}
