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
        "string", // List of usernames
        "string",
        ...
      ]
    }
    ```
*   **Error Responses:** 500 Internal Server Error.

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