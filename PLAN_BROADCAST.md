# Plan: Implement User Online/Offline Broadcasts

This plan details the steps to add WebSocket broadcast functionality for user online/offline status updates and modify the `/users/online` API endpoint.

## 1. Enhance `hub.Hub` (`hub/hub.go`)

*   **Add `Broadcast` Method:**
    *   Define a new method: `Broadcast(message []byte, excludeUserID int32)`.
    *   This method will iterate through all connected user IDs (`h.clients`).
    *   For each user ID, if it does *not* match `excludeUserID`, iterate through all active connections (`*websocket.Conn`) for that user.
    *   Send the `message` to each connection using `conn.WriteMessage(websocket.TextMessage, message)`.
    *   Use `h.mu.RLock()` for safe iteration.
    *   Include error logging for `conn.WriteMessage`.

## 2. Modify `main.go`

*   **Define Structs:**
    *   Add `UserStatusBroadcast` struct for WebSocket messages:
      ```go
      type UserStatusBroadcast struct {
          Type   string `json:"type"` // "user_online" or "user_offline"
          UserID int32  `json:"userId"`
      }
      ```
    *   Add `OnlineUserInfo` struct for the `/users/online` response:
      ```go
      type OnlineUserInfo struct {
          ID       int32  `json:"id"`
          Username string `json:"username"`
      }
      ```

*   **Modify `/users/online` Handler:**
    *   Fetch `onlineUsers` from the store.
    *   Create a slice: `var userInfos []OnlineUserInfo`.
    *   Iterate `onlineUsers`, appending `OnlineUserInfo{ID: user.ID, Username: user.Username}` to `userInfos`.
    *   Return `userInfos` in the JSON response: `c.JSON(http.StatusOK, gin.H{"online_users": userInfos})`.

*   **Modify `/ws` Handler:**
    *   **On User Online (First Connection):**
        *   After the `if isFirstConnection { ... }` block (and DB update).
        *   Create `onlineMsg := UserStatusBroadcast{Type: "user_online", UserID: userID}`.
        *   Marshal `onlineMsg` to JSON (`jsonMsg`).
        *   Call `connectionHub.Broadcast(jsonMsg, userID)` (exclude the user who just connected).
    *   **On User Offline (Last Connection):**
        *   Inside the `defer func() { ... }`, after the `if isLastConnection { ... }` block (and DB update).
        *   Create `offlineMsg := UserStatusBroadcast{Type: "user_offline", UserID: userID}`.
        *   Marshal `offlineMsg` to JSON (`jsonMsg`).
        *   Call `connectionHub.Broadcast(jsonMsg, 0)` (send to all remaining clients).

## 3. Mermaid Diagram

```mermaid
sequenceDiagram
    participant Client
    participant Server (main.go /ws)
    participant Hub (hub.go)
    participant DB

    Client->>Server (main.go /ws): WebSocket Upgrade Request (with token)
    Server (main.go /ws)->>Server (main.go /ws): Verify Token
    alt Token Valid
        Server (main.go /ws)->>Hub (hub.go): Register(userID, conn)
        Hub (hub.go)-->>Server (main.go /ws): isFirstConnection = true
        Server (main.go /ws)->>DB: UpdateUserStatus(userID, 'online')
        DB-->>Server (main.go /ws): Success
        Server (main.go /ws)->>Server (main.go /ws): Create user_online JSON message
        Server (main.go /ws)->>Hub (hub.go): Broadcast(user_online_msg, excludeUserID=userID)
        Hub (hub.go)-->>Other Clients: Send user_online message
        Server (main.go /ws)-->>Client: Connection Established
    else Token Invalid
        Server (main.go /ws)-->>Client: Close Connection (Policy Violation)
    end

    Client->>Server (main.go /ws): Close WebSocket Connection
    Server (main.go /ws)->>Hub (hub.go): Unregister(userID, conn)
    Hub (hub.go)-->>Server (main.go /ws): isLastConnection = true
    Server (main.go /ws)->>DB: UpdateUserStatus(userID, 'offline')
    DB-->>Server (main.go /ws): Success
    Server (main.go /ws)->>Server (main.go /ws): Create user_offline JSON message
    Server (main.go /ws)->>Hub (hub.go): Broadcast(user_offline_msg, excludeUserID=0) // Send to all remaining
    Hub (hub.go)-->>Remaining Clients: Send user_offline message