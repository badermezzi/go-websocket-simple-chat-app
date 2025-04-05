# API Reference

This document outlines the API endpoints and WebSocket communication for the chat application backend.

## HTTP Endpoints

Base URL: `http://localhost:8080` (or configured port)

### 1. Create User

*   **Endpoint:** `POST /users`
*   **Description:** Creates a new user account.
*   **Headers:**
    *   `Content-Type: application/json`
*   **Request Body (JSON):**
    ```json
    {
      "username": "string",  // Desired username
      "password": "string"   // Desired password
    }
    ```
*   **Success Response (200 OK):**
    ```json
    {
      "message": "User created",
      "user_id": number // Integer ID of the newly created user
    }
    ```
*   **Error Responses:** 400 Bad Request (invalid input), 500 Internal Server Error.

### 2. Login User

*   **Endpoint:** `POST /login`
*   **Description:** Authenticates a user and returns a Paseto token.
*   **Headers:**
    *   `Content-Type: application/json`
*   **Request Body (JSON):**
    ```json
    {
      "username": "string", // Existing username
      "password": "string"  // Correct password
    }
    ```
*   **Success Response (200 OK):**
    ```json
    {
      "message": "Logged in successfully",
      "token": "string", // Paseto token (v2.local...)
      "payload": {
        "id": "string", // UUID of the token
        "user_id": number, // Integer ID of the logged-in user
        "username": "string", // Username of the logged-in user
        "issued_at": "string", // Timestamp (RFC3339)
        "expired_at": "string" // Timestamp (RFC3339)
      }
    }
    ```
*   **Error Responses:** 400 Bad Request, 401 Unauthorized (invalid credentials), 500 Internal Server Error.

### 3. List Online Users

*   **Endpoint:** `GET /users/online`
*   **Description:** Returns a list of usernames currently marked as online.
*   **Headers:** None required.
*   **Request Body:** None.
*   **Success Response (200 OK):**
    ```json
    {
      "online_users": [
        {
          "id": number,       // Integer ID of the online user
          "username": "string" // Username of the online user
        },
        // ... more users
      ]
    }
    ```
*   **Error Responses:** 500 Internal Server Error.

### 4. List Offline Users

*   **Endpoint:** `GET /users/offline`
*   **Description:** Returns a list of users currently marked as offline. Useful for populating user lists alongside online users.
*   **Headers:** None required.
*   **Request Body:** None.
*   **Success Response (200 OK):**
    ```json
    {
      "offline_users": [
        {
          "id": number,       // Integer ID of the offline user
          "username": "string" // Username of the offline user
        },
        // ... more users
      ]
    }
    ```
*   **Error Responses:** 500 Internal Server Error.

### 5. Get Messages Between Users

*   **Endpoint:** `GET /messages`
*   **Description:** Retrieves the message history between the logged-in user and a specified partner user, ordered by newest first, with pagination.
*   **Headers:**
    *   `Authorization: Bearer <your_paseto_token>` (Required)
*   **Query Parameters:**
    *   `partner_id` (integer, Required): The ID of the user whose conversation history you want to fetch.
    *   `page` (integer, Optional, Default: `1`): The page number of messages to retrieve.
    *   `limit` (integer, Optional, Default: `20`): The maximum number of messages to return per page.
*   **Request Body:** None.
*   **Success Response (200 OK):**
    ```json
    [
      {
        "id": number,          // Message ID
        "sender_id": number,   // Sender's user ID
        "receiver_id": number, // Receiver's user ID
        "content": "string",   // Message content
        "created_at": "string" // Timestamp (RFC3339 or similar)
      },
      // ... more messages (up to limit), ordered newest first
    ]
    ```
    *   Returns an empty array `[]` if no messages are found.
*   **Error Responses:** 400 Bad Request (invalid parameters), 401 Unauthorized (invalid/missing token), 500 Internal Server Error.

## WebSocket Communication

*   **Endpoint:** `GET /ws?token=<your_paseto_token>` (Upgrades to WebSocket connection)
*   **Description:** Establishes a persistent WebSocket connection for real-time communication. The authentication token obtained from `/login` must be provided as the `token` query parameter in the connection URL.
*   **Example URL:** `wss://your.api.domain/ws?token=YOUR_ACTUAL_TOKEN` (Replace `wss://your.api.domain` with the actual server address and `YOUR_ACTUAL_TOKEN` with the token)
*   **Connection:** Once established, the connection stays open for bidirectional communication.

### WebSocket Messages (Client -> Server)

*   **Type:** `private_message`
*   **Format (JSON Text Message):**
    ```json
    {
      "type": "private_message",
      "recipient_id": number, // Integer ID of the recipient user
      "content": "string"     // The message text
    }
    ```

*   **Type:** `typing_start`
*   **Format (JSON Text Message):**
    ```json
    {
      "type": "typing_start",
      "recipient_id": number // Integer ID of the user being typed to
    }
    ```
*   **Description:** Sent when the client user starts typing a message to the recipient.

*   **Type:** `typing_stop`
*   **Format (JSON Text Message):**
    ```json
    {
      "type": "typing_stop",
      "recipient_id": number // Integer ID of the user being typed to
    }
    ```
*   **Description:** Sent when the client user stops typing a message to the recipient.

*   **Type:** `message_read`
*   **Format (JSON Text Message):**
    ```json
    {
      "type": "message_read",
      "sender_id": number // Integer ID of the user whose messages were just read by the client
    }
    ```
*   **Description:** Sent when the client user views messages from a specific sender in a chat window.

### WebSocket Messages (Server -> Client)

*   **Type:** `incoming_message`
*   **Format (JSON Text Message):**
    ```json
    {
      "type": "incoming_message",
      "sender_id": number,       // Integer ID of the user who sent the message
      "sender_username": "string", // Username of the sender
      "content": "string"          // The message text received
    }
    ```

*   **Type:** `user_online`
*   **Format (JSON Text Message):**
    ```json
    {
      "type": "user_online",
      "userId": number // Integer ID of the user who just came online
    }
    ```
*   **Description:** Broadcast to all *other* connected clients when a user establishes their first WebSocket connection.

*   **Type:** `user_offline`
*   **Format (JSON Text Message):**
    ```json
    {
      "type": "user_offline",
      "userId": number // Integer ID of the user who just disconnected their last session
    }
    ```
*   **Description:** Broadcast to all *remaining* connected clients when a user disconnects their last WebSocket connection.

*   **Type:** `typing_start` (Forwarded)
*   **Format (JSON Text Message):**
    ```json
    {
      "type": "typing_start",
      "sender_id": number,    // Integer ID of the user who started typing
      "recipient_id": number  // Integer ID of the user being typed to (the client receiving this)
    }
    ```
*   **Description:** Sent to the recipient when the sender starts typing.

*   **Type:** `typing_stop` (Forwarded)
*   **Format (JSON Text Message):**
    ```json
    {
      "type": "typing_stop",
      "sender_id": number,    // Integer ID of the user who stopped typing
      "recipient_id": number  // Integer ID of the user being typed to (the client receiving this)
    }
    ```
*   **Description:** Sent to the recipient when the sender stops typing.

*   **Type:** `read_receipt_update`
*   **Format (JSON Text Message):**
    ```json
    {
      "type": "read_receipt_update",
      "reader_id": number, // Integer ID of the user who read the messages
      "sender_id": number  // Integer ID of the user whose messages were read (the client receiving this)
    }
    ```
*   **Description:** Sent to the original sender when the recipient reads their messages.