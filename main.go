package main

import (
	"context"
	"database/sql"
	"encoding/json" // Added for handling JSON messages
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	_ "github.com/lib/pq"

	"time"
	db "websocket-simple-chat-app/db/sqlc"
	"websocket-simple-chat-app/hub"
	"websocket-simple-chat-app/token"
)

const dbDriverName = "postgres"
const dbDataSourceName = "postgres://postgres:159159@localhost:5432/chat_app_db?sslmode=disable"

const pasetoSymmetricKey = "12345678901234567890123456789012"

var upgrader = websocket.Upgrader{
	//  This is okay for local development but a security risk in production. Normally, you'd check if the request origin is allowed.
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// --- WebSocket Message Structs ---

// IncomingWsMessage defines the structure for messages received from clients
type IncomingWsMessage struct {
	Type        string `json:"type"`
	RecipientID int32  `json:"recipient_id"` // Use int32 to match DB schema/sqlc types
	Content     string `json:"content"`
}

// OutgoingWsMessage defines the structure for messages sent to clients
type OutgoingWsMessage struct {
	Type           string `json:"type"`
	SenderID       int32  `json:"sender_id"`
	SenderUsername string `json:"sender_username"`
	Content        string `json:"content"`
}

func main() {
	connectionHub := hub.NewHub()

	pasetoMaker, err := token.NewPasetoMaker([]byte(pasetoSymmetricKey))
	if err != nil {
		log.Fatalf("cannot create paseto maker: %v", err)
	}

	r := gin.Default()

	dbConn, err := sql.Open(dbDriverName, dbDataSourceName)
	if err != nil {
		log.Fatal("cannot connect to db:", err)
	}
	defer dbConn.Close()

	// !!!!!!!!! dir update l query, "where status=online", but at the same time add status as index in the db table, this wil lower the job time and resources
	_, err = dbConn.Exec("UPDATE users SET status = 'offline' WHERE status = 'online'") // Only update users currently online
	if err != nil {
		// Log the error but don't necessarily stop the server
		log.Printf("Warning: Failed to set all users offline on startup: %v\n", err)
	}

	store := db.New(dbConn)

	// --- Setup Routes ---

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	r.POST("/users", func(c *gin.Context) {
		type createUserRequest struct {
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		var req createUserRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		user, err := store.CreateUser(context.Background(), db.CreateUserParams{
			Username:          req.Username,
			PasswordPlaintext: req.Password,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "User created", "user_id": user.ID})
	})

	r.POST("/login", func(c *gin.Context) {
		type loginUserRequest struct {
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		var req loginUserRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		user, err := store.GetUserByUsername(context.Background(), req.Username)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to login"})
			return
		}

		if user.PasswordPlaintext != req.Password {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		tokenDuration := time.Hour
		tokenStr, payload, err := pasetoMaker.CreateToken(
			user.ID,
			user.Username,
			tokenDuration,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Logged in successfully", "token": tokenStr, "payload": payload})
	})

	r.GET("/users/online", func(c *gin.Context) {
		onlineUsers, err := store.ListOnlineUsers(context.Background())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list online users"})
			return
		}

		var usernames []string
		for _, user := range onlineUsers {
			usernames = append(usernames, user.Username)
		}

		c.JSON(http.StatusOK, gin.H{"online_users": usernames})
	})

	r.GET("/ws", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Println("WebSocket upgrade error:", err)
			return
		}
		defer conn.Close() // Ensure connection is closed eventually

		// --- WebSocket Authentication via Authorization Header ---
		authHeader := c.Request.Header.Get("Authorization")
		if authHeader == "" {
			log.Println("WS Error: Authorization header not provided")
			conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "authorization header required"))
			return
		}

		fields := strings.Fields(authHeader)
		if len(fields) < 2 || strings.ToLower(fields[0]) != "bearer" {
			log.Println("WS Error: Invalid authorization header format")
			conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "invalid authorization header format"))
			return
		}

		tokenStr := fields[1]

		payload, err := pasetoMaker.VerifyToken(tokenStr)
		if err != nil {
			log.Printf("WS Error: Invalid token: %v\n", err)
			conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "invalid token"))
			return
		}

		// --- User Authenticated - Register Connection ---
		userID := payload.UserID
		username := payload.Username // Get username from token payload

		// Register connection with the hub
		isFirstConnection := connectionHub.Register(userID, conn)

		// Update status to online ONLY if it's the first connection for this user
		if isFirstConnection {
			err = store.UpdateUserStatus(context.Background(), db.UpdateUserStatusParams{
				ID:     userID,
				Status: "online",
			})
			if err != nil {
				log.Printf("WS Error: Failed to update user %d status to online: %v\n", userID, err)
				// Decide if we should close the connection here or just log
			} else {
				log.Printf("User %s (ID: %d) connected (first WS connection)\n", username, userID)
			}
		} else {
			log.Printf("User %s (ID: %d) connected (additional WS connection)\n", username, userID)
		}

		// --- Handle Disconnect ---
		defer func() {
			isLastConnection := connectionHub.Unregister(userID, conn)
			if isLastConnection {
				err = store.UpdateUserStatus(context.Background(), db.UpdateUserStatusParams{
					ID:     userID,
					Status: "offline",
				})
				if err != nil {
					log.Printf("WS Error: Failed to update user %d status to offline on disconnect: %v\n", userID, err)
				} else {
					log.Printf("User %s (ID: %d) disconnected (last WS connection)\n", username, userID)
				}
			} else {
				log.Printf("User %s (ID: %d) disconnected (still has other WS connections)\n", username, userID)
			}
		}()

		// --- Message Read Loop ---
		for {
			messageType, p, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("WS read error for user %s (ID: %d): %v\n", username, userID, err)
				} else {
					log.Printf("WS connection closed normally for user %s (ID: %d)\n", username, userID)
				}
				break
			}
			// --- Handle Incoming Messages ---
			if messageType == websocket.TextMessage {
				var msg IncomingWsMessage
				if err := json.Unmarshal(p, &msg); err != nil {
					log.Printf("WS Error: Failed to unmarshal message from %s (ID: %d): %v. Payload: %s", username, userID, err, string(p))
					// Optionally send an error back to the sender
					// conn.WriteJSON(map[string]string{"error": "Invalid message format"})
					continue // Skip this message
				}

				log.Printf("Parsed message from %s (ID: %d): Type=%s, RecipientID=%d", username, userID, msg.Type, msg.RecipientID)

				// --- Handle Private Messages ---
				if msg.Type == "private_message" {
					// Basic validation
					if msg.RecipientID <= 0 || msg.Content == "" {
						log.Printf("WS Warning: Invalid private message from %s (ID: %d): RecipientID=%d, Content empty=%t", username, userID, msg.RecipientID, msg.Content == "")
						// Optionally send an error back to the sender
						// conn.WriteJSON(map[string]string{"error": "Invalid message content or recipient"})
						continue
					}

					// 1. Store the message in the database
					// Use the 'store' variable initialized earlier (line 55)
					_, dbErr := store.CreateMessage(context.Background(), db.CreateMessageParams{
						SenderID:   userID,          // Sender is the authenticated user of this connection
						ReceiverID: msg.RecipientID, // Recipient from the message payload
						Content:    msg.Content,
					})
					if dbErr != nil {
						log.Printf("WS Error: Failed to store message from %d to %d: %v", userID, msg.RecipientID, dbErr)
						// Optionally notify sender of storage failure
						// conn.WriteJSON(map[string]string{"error": "Failed to save message"})
						continue // Decide if we should stop processing or just log
					}
					log.Printf("Message from %d (%s) to %d stored successfully.", userID, username, msg.RecipientID)

					// 2. Attempt real-time delivery if recipient is online
					// Use the 'connectionHub' variable initialized earlier (line 33)
					recipientConnections := connectionHub.GetUserConnections(msg.RecipientID)
					if len(recipientConnections) > 0 {
						outgoingMsg := OutgoingWsMessage{
							Type:           "incoming_message",
							SenderID:       userID,
							SenderUsername: username, // Username from the authenticated token payload
							Content:        msg.Content,
						}
						log.Printf("Attempting to send message from %d (%s) to %d (%d active connections)", userID, username, msg.RecipientID, len(recipientConnections))

						// Send to all active connections for the recipient
						for _, recipientConn := range recipientConnections {
							// Use WriteJSON for convenience as we are sending a struct
							if writeErr := recipientConn.WriteJSON(outgoingMsg); writeErr != nil {
								log.Printf("WS Error: Failed to send message via WebSocket to user %d connection %p: %v", msg.RecipientID, recipientConn, writeErr)
								// This specific connection might be broken. The read loop for *that* connection
								// will likely handle its closure and unregister it via its defer function.
								// We don't need to break the sender's loop here.
							}
						}
					} else {
						log.Printf("Recipient %d is offline. Message stored for later retrieval (feature not implemented).", msg.RecipientID)
					}

				} else {
					// Handle other message types if needed in the future
					log.Printf("WS Warning: Received unhandled message type '%s' from %s (ID: %d)", msg.Type, username, userID)
					// Optionally send an error back: conn.WriteJSON(map[string]string{"error": "Unsupported message type"})
				}

			} else {
				// Handle non-text messages (e.g., binary, ping, pong) if necessary
				log.Printf("WS Warning: Received non-text message type %d from %s (ID: %d). Ignoring.", messageType, username, userID)
			}
		}
	})

	r.Run(":8080")

	// port := os.Getenv("PORT")
	// if port == "" {
	// 	port = "8080"
	// }
	// r.Run(":" + port)
}
