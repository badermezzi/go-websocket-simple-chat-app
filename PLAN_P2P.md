# Plan: Peer-to-Peer (P2P) WebSocket Messaging

This document outlines the plan to implement private P2P messaging in the WebSocket chat application.

## 1. Goal Recap

Enable users to send private messages to other specific online users in real-time via WebSockets. Messages should also be stored in the database (`messages` table).

## 2. Proposed Architecture Overview

Leverage the existing `Hub` to find the recipient's active WebSocket connection(s) and the database to persist messages.

*   **Client:** Sends a message specifying the recipient's ID.
*   **Server (`/ws` handler):**
    *   Receives the message.
    *   Identifies the sender via the authenticated WebSocket connection's token payload.
    *   Parses the message to get the recipient ID and content.
    *   **Stores** the message in the `messages` table using `sqlc`.
    *   **Looks up** the recipient's active connection(s) in the `Hub`.
    *   **Forwards** the message directly to the recipient's connection(s) if they are online.

## 3. Detailed Plan

### Step 3.1: Define WebSocket Message Formats

**Client -> Server (Private Message):**

```json
{
  "type": "private_message",
  "recipient_id": 123,
  "content": "Hello there!"
}
```

**Server -> Client (Incoming Message):**

```json
{
  "type": "incoming_message",
  "sender_id": 456,
  "sender_username": "Alice",
  "content": "Hello there!"
}
```

### Step 3.2: Enhance `sqlc` Queries

1.  Create/edit `db/query/message.sql`.
2.  Add the `CreateMessage` query:
    ```sql
    -- name: CreateMessage :one
    INSERT INTO messages (
      sender_id,
      receiver_id,
      content
    ) VALUES (
      $1, $2, $3
    ) RETURNING *;
    ```
3.  Run `sqlc generate` in the terminal from the project root.

### Step 3.3: Enhance the `Hub`

Add the `GetUserConnections` method to `hub/hub.go`:

```go
package hub

import (
	"sync"
	"github.com/gorilla/websocket"
)

// Hub maintains the set of active clients and broadcasts messages.
type Hub struct {
	clients map[int32]map[*websocket.Conn]bool
	mu      sync.RWMutex
}

// NewHub creates a new Hub instance.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[int32]map[*websocket.Conn]bool),
	}
}

// Register adds a new connection for a given user.
// Returns true if this was the user's first connection.
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
// Returns true if this was the user's last connection.
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
```

### Step 3.4: Modify WebSocket Handler (`/ws` in `main.go`)

Inside the message read loop (`for { conn.ReadMessage() ... }`):

1.  **Unmarshal JSON:** Read message `p`, unmarshal into `map[string]interface{}` or a specific struct.
2.  **Check Type:** If `type` is `"private_message"`:
    *   Extract `recipient_id` (as `int32`) and `content` (as `string`). Validate.
    *   Get sender `userID` and `username` from the connection's `payload`.
    *   **Store Message:** Call `store.CreateMessage(...)` with sender ID, recipient ID, and content. Handle errors.
    *   **Route Message:**
        *   `recipientConnections := connectionHub.GetUserConnections(recipientID)`
        *   If `len(recipientConnections) > 0`:
            *   Construct the `incoming_message` JSON (map or struct).
            *   Iterate through `recipientConnections` and send the JSON using `recipientConn.WriteJSON(outgoingMsg)`. Handle write errors.
3.  **Handle Other Types:** Log errors or ignore unknown message types for now.
4.  **Error Handling:** Implement robust error handling throughout.

## 4. User Journey / Story

1.  **Alice Connects:** Logs in (`/login`), gets token, connects to `/ws` (auth via header), Hub registers connection, DB status updated.
2.  **Alice Sends:** Client sends `{"type": "private_message", "recipient_id": 123, "content": "Hi Bob!"}` via WebSocket.
3.  **Server Receives (Alice's `/ws` Goroutine):**
    *   Parses JSON. Identifies sender (Alice, 456) and recipient (Bob, 123).
    *   Calls `store.CreateMessage(...)`. Message saved to DB.
    *   Calls `connectionHub.GetUserConnections(123)`. Gets Bob's connection(s).
    *   Constructs `{"type": "incoming_message", ...}`.
    *   Sends the message to Bob's connection(s) via `WriteJSON`.
4.  **Bob Receives:** Bob's client receives the message and displays it.

## 5. Mermaid Sequence Diagram

```mermaid
sequenceDiagram
    participant AliceClient
    participant Server
    participant Hub
    participant Database
    participant BobClient

    AliceClient->>Server: Login Request (/login)
    Server->>Database: GetUserByUsername(Alice)
    Database-->>Server: User Alice Data
    Server->>Server: Generate Paseto Token (UserID: 456)
    Server-->>AliceClient: Login Success + Token

    AliceClient->>Server: WebSocket Connect (/ws + Auth Header)
    Server->>Server: Verify Token (Payload: UserID 456, Username Alice)
    Server->>Hub: Register(UserID: 456, AliceConn)
    Hub-->>Server: isFirstConnection = true
    Server->>Database: UpdateUserStatus(UserID: 456, Status: online)
    Database-->>Server: OK
    Server-->>AliceClient: WebSocket Connection Established

    BobClient->>Server: Login Request (/login)
    Server->>Database: GetUserByUsername(Bob)
    Database-->>Server: User Bob Data
    Server->>Server: Generate Paseto Token (UserID: 123)
    Server-->>BobClient: Login Success + Token

    BobClient->>Server: WebSocket Connect (/ws + Auth Header)
    Server->>Server: Verify Token (Payload: UserID 123, Username Bob)
    Server->>Hub: Register(UserID: 123, BobConn)
    Hub-->>Server: isFirstConnection = true
    Server->>Database: UpdateUserStatus(UserID: 123, Status: online)
    Database-->>Server: OK
    Server-->>BobClient: WebSocket Connection Established

    AliceClient->>Server: Send WS Message: {"type":"private_message", "recipient_id":123, "content":"Hi Bob!"} (via AliceConn)
    Server->>Server: Parse Message (Sender: 456, Recipient: 123)
    Server->>Database: CreateMessage(sender:456, receiver:123, content:"Hi Bob!")
    Database-->>Server: Message Saved (ID: 789)
    Server->>Hub: GetUserConnections(UserID: 123)
    Hub-->>Server: Connections: [BobConn]
    Server->>BobClient: Send WS Message: {"type":"incoming_message", "sender_id":456, "sender_username":"Alice", "content":"Hi Bob!"} (via BobConn)
    BobClient->>BobClient: Display "From Alice: Hi Bob!"