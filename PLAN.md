# Implementation Plan: Robust Online/Offline Status

**Goal:** Accurately reflect a user's online status based *only* on active WebSocket connections using Paseto tokens and a connection hub.

**Implementation Task List:**

1.  **Install Paseto Library:** Add the Go Paseto library (`github.com/o1egl/paseto`) to the project dependencies (`go get github.com/o1egl/paseto/v2` and `go mod tidy`). *Note: Using v2 is recommended.*
2.  **Token Generation/Verification Setup:**
    *   Create a new package (e.g., `token`) or add functions to handle Paseto token creation (using `V2.Encrypt` for symmetric keys) and verification (`V2.Decrypt`).
    *   Define a payload structure for the token (e.g., including `UserID`, `Username`, `IssuedAt`, `ExpiresAt`).
    *   Generate a 32-byte symmetric key for Paseto (store securely, perhaps via environment variable later, e.g., `PASETO_SYMMETRIC_KEY`).
3.  **Server Startup Cleanup (Optional but Recommended):**
    *   Add a function call at the beginning of `main()` to execute a SQL query that sets the `status` of *all* users in the `users` table to `'offline'`. This ensures a clean state on restart.
4.  **Connection Hub Implementation:**
    *   Create a new package (e.g., `hub`) or add to an existing suitable package.
    *   Define a struct (e.g., `Hub`) to manage connections.
    *   Inside the `Hub` struct, add a map like `clients map[int32]map[*websocket.Conn]bool`.
    *   Add a `sync.RWMutex` to the `Hub` struct to protect concurrent access to the `clients` map.
    *   Implement methods on the `Hub` struct:
        *   `Register(userID int32, conn *websocket.Conn) bool`: Adds a connection, checks if it's the first for the user, and returns `true` if the user just came online (status should be updated). Handles locking/unlocking.
        *   `Unregister(userID int32, conn *websocket.Conn) bool`: Removes a connection, checks if it's the last for the user, and returns `true` if the user just went offline (status should be updated). Handles locking/unlocking.
    *   Instantiate the `Hub` (e.g., `connectionHub := hub.NewHub()`) in `main` or make it a singleton.
5.  **Refactor `/login` Handler:**
    *   Remove the `store.UpdateUserStatus` call.
    *   Upon successful username/password verification, call the Paseto token creation function (from step 2) to generate a token containing the authenticated `user.ID` and other relevant payload data.
    *   Return the generated Paseto token in the JSON response (e.g., `{"token": "v2.local...."}`).
6.  **Refactor `/ws` Handler:**
    *   Modify the handler to expect the Paseto token, preferably from an `Authorization: Bearer <token>` header or alternatively a query parameter (`?token=...`).
    *   Call the Paseto token verification function (from step 2) to validate the token and extract the `userID` from the payload. Reject the connection if the token is invalid or expired.
    *   Call `connectionHub.Register(userID, conn)`.
    *   If `connectionHub.Register` returns `true`, call `store.UpdateUserStatus(userID, "online")`.
    *   In the `defer conn.Close()` block and/or the error handling part of the `conn.ReadMessage` loop:
        *   Call `connectionHub.Unregister(userID, conn)`.
        *   If `connectionHub.Unregister` returns `true`, call `store.UpdateUserStatus(userID, "offline")`.

This ordered list provides the steps to implement the planned logic.