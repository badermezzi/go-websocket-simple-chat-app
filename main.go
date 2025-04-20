package main

import (
	"context"
	"database/sql"
	"encoding/json" // Added for handling JSON messages
	"errors"        // Added for error handling
	"fmt"           // Added for error formatting
	"log"
	"net/http"
	"strconv" // Added for query param conversion
	"strings" // Added for header parsing

	"github.com/gin-contrib/cors" // Import CORS middleware
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

// UserStatusBroadcast defines the structure for user online/offline notifications
type UserStatusBroadcast struct {
	Type   string `json:"type"` // "user_online" or "user_offline"
	UserID int32  `json:"userId"`
}

// OnlineUserInfo defines the structure for the /users/online endpoint response
type OnlineUserInfo struct {
	ID       int32  `json:"id"`
	Username string `json:"username"`
}

// --- Specific WebSocket Message Payloads ---

// TypingIndicatorMessage is used for both incoming and outgoing typing status
type TypingIndicatorMessage struct {
	Type        string `json:"type"`         // "typing_start" or "typing_stop"
	RecipientID int32  `json:"recipient_id"` // User receiving the indicator
	SenderID    int32  `json:"sender_id"`    // User sending the indicator (added for outgoing)
}

// MessageReadMessage is sent by the client when messages from a sender are read
type MessageReadMessage struct {
	Type     string `json:"type"`      // "message_read"
	SenderID int32  `json:"sender_id"` // ID of the user whose messages were read
}

// ReadReceiptUpdateMessage is sent by the server to the original sender
type ReadReceiptUpdateMessage struct {
	Type     string `json:"type"`      // "read_receipt_update"
	ReaderID int32  `json:"reader_id"` // ID of the user who read the messages (the current user)
	SenderID int32  `json:"sender_id"` // ID of the user whose messages were read
}

// --- Gin Context Keys ---
const (
	authorizationHeaderKey  = "authorization"
	authorizationTypeBearer = "bearer"
	authorizationPayloadKey = "authorization_payload"
)

// --- Authentication Middleware ---

// authMiddleware creates a gin middleware for authorization
func authMiddleware(tokenMaker token.Maker) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authorizationHeader := ctx.GetHeader(authorizationHeaderKey)

		if len(authorizationHeader) == 0 {
			err := errors.New("authorization header is not provided")
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		fields := strings.Fields(authorizationHeader)
		if len(fields) < 2 {
			err := errors.New("invalid authorization header format")
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		authorizationType := strings.ToLower(fields[0])
		if authorizationType != authorizationTypeBearer {
			err := fmt.Errorf("unsupported authorization type %s", authorizationType)
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		accessToken := fields[1]
		payload, err := tokenMaker.VerifyToken(accessToken)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		ctx.Set(authorizationPayloadKey, payload)
		ctx.Next()
	}
}

// --- Main Function ---

func main() {
	connectionHub := hub.NewHub()

	pasetoMaker, err := token.NewPasetoMaker([]byte(pasetoSymmetricKey))
	if err != nil {
		log.Fatalf("cannot create paseto maker: %v", err)
	}

	r := gin.Default()

	// --- CORS Middleware Configuration ---
	config := cors.Config{
		// Allow requests from your frontend origin
		// Allow requests from any origin (useful for development with file:// URLs)
		AllowAllOrigins: true,
		// Allow common methods
		AllowMethods: []string{"GET", "POST", "OPTIONS"},
		// Allow common headers, including Authorization for WebSocket
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
		// Allow credentials if needed (e.g., cookies, though not used here yet)
		AllowCredentials: true,
		// MaxAge specifies how long the result of a preflight request can be cached
		MaxAge: 12 * time.Hour,
	}
	r.Use(cors.New(config)) // Apply CORS middleware globally

	dbConn, err := sql.Open(dbDriverName, dbDataSourceName)
	if err != nil {
		log.Fatal("cannot connect to db:", err)
	}
	defer dbConn.Close()

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

		// Create a slice to hold the user info objects
		var userInfos []OnlineUserInfo
		for _, user := range onlineUsers {
			userInfos = append(userInfos, OnlineUserInfo{
				ID:       user.ID,
				Username: user.Username,
			})
		}

		c.JSON(http.StatusOK, gin.H{"online_users": userInfos})
	})

	// Endpoint to list offline users
	r.GET("/users/offline", getOfflineUsersHandler(store))

	// --- Authenticated Routes ---
	authRoutes := r.Group("/").Use(authMiddleware(pasetoMaker))

	authRoutes.GET("/messages", getMessagesHandler(store)) // Pass store here for closure

	// --- WebSocket Route (Separate Auth) ---
	r.GET("/ws", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Println("WebSocket upgrade error:", err)
			return
		}
		defer conn.Close() // Ensure connection is closed eventually

		// --- WebSocket Authentication via Query Parameter ---
		tokenStr := c.Query("token") // Read token from query parameter
		if tokenStr == "" {
			log.Println("WS Error: 'token' query parameter not provided")
			conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "'token' query parameter required"))
			return
		}

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

				// --- Broadcast User Online Status ---
				onlineMsg := UserStatusBroadcast{Type: "user_online", UserID: userID}
				jsonMsg, marshalErr := json.Marshal(onlineMsg)
				if marshalErr != nil {
					log.Printf("WS Error: Failed to marshal user_online message for user %d: %v", userID, marshalErr)
				} else {
					// Broadcast to everyone *except* the user who just connected
					connectionHub.Broadcast(jsonMsg, userID)
					log.Printf("Broadcasted user_online for User %s (ID: %d)", username, userID)
				}
				// --- End Broadcast ---
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

					// --- Broadcast User Offline Status ---
					offlineMsg := UserStatusBroadcast{Type: "user_offline", UserID: userID}
					jsonMsg, marshalErr := json.Marshal(offlineMsg)
					if marshalErr != nil {
						log.Printf("WS Error: Failed to marshal user_offline message for user %d: %v", userID, marshalErr)
					} else {
						// Broadcast to all remaining clients (no exclusion needed)
						connectionHub.Broadcast(jsonMsg, 0) // excludeUserID 0 means no exclusion
						log.Printf("Broadcasted user_offline for User %s (ID: %d)", username, userID)
					}
					// --- End Broadcast ---
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
				// 1. Unmarshal into a generic map to check the type first
				var genericMsg map[string]any
				if err := json.Unmarshal(p, &genericMsg); err != nil {
					log.Printf("WS Error: Failed to unmarshal generic message from %s (ID: %d): %v. Payload: %s", username, userID, err, string(p))
					continue
				}

				// 2. Check the message type
				msgType, ok := genericMsg["type"].(string)
				if !ok {
					log.Printf("WS Error: Message type is missing or not a string from %s (ID: %d). Payload: %s", username, userID, string(p))
					continue
				}

				log.Printf("Received message type '%s' from %s (ID: %d)", msgType, username, userID)

				// 3. Handle based on type
				switch msgType {
				case "private_message":
					var msg IncomingWsMessage
					if err := json.Unmarshal(p, &msg); err != nil { // Unmarshal again into specific struct
						log.Printf("WS Error: Failed to unmarshal private_message: %v. Payload: %s", err, string(p))
						continue
					}
					// Basic validation
					if msg.RecipientID <= 0 || msg.Content == "" {
						log.Printf("WS Warning: Invalid private message from %s (ID: %d): RecipientID=%d, Content empty=%t", username, userID, msg.RecipientID, msg.Content == "")
						continue
					}
					// 1. Store the message in the database
					_, dbErr := store.CreateMessage(context.Background(), db.CreateMessageParams{
						SenderID:   userID,
						ReceiverID: msg.RecipientID,
						Content:    msg.Content,
					})
					if dbErr != nil {
						log.Printf("WS Error: Failed to store message from %d to %d: %v", userID, msg.RecipientID, dbErr)
						continue
					}
					log.Printf("Message from %d (%s) to %d stored successfully.", userID, username, msg.RecipientID)
					// 2. Attempt real-time delivery if recipient is online
					recipientConnections := connectionHub.GetUserConnections(msg.RecipientID)
					if len(recipientConnections) > 0 {
						outgoingMsg := OutgoingWsMessage{
							Type:           "incoming_message",
							SenderID:       userID,
							SenderUsername: username,
							Content:        msg.Content,
						}
						jsonMsg, marshalErr := json.Marshal(outgoingMsg)
						if marshalErr != nil {
							log.Printf("WS Error: Failed to marshal outgoing private message: %v", marshalErr)
							continue // Skip sending if marshalling fails
						}
						log.Printf("Attempting to send message from %d (%s) to %d (%d active connections)", userID, username, msg.RecipientID, len(recipientConnections))
						for _, recipientConn := range recipientConnections {
							if writeErr := recipientConn.WriteMessage(websocket.TextMessage, jsonMsg); writeErr != nil {
								log.Printf("WS Error: Failed to send message via WebSocket to user %d connection %p: %v", msg.RecipientID, recipientConn, writeErr)
							}
						}
					} else {
						log.Printf("Recipient %d is offline. Message stored.", msg.RecipientID)
					}

				case "typing_start", "typing_stop":
					var msg TypingIndicatorMessage
					if err := json.Unmarshal(p, &msg); err != nil {
						log.Printf("WS Error: Failed to unmarshal typing indicator: %v. Payload: %s", err, string(p))
						continue
					}
					// Basic validation
					if msg.RecipientID <= 0 {
						log.Printf("WS Warning: Invalid typing indicator from %s (ID: %d): RecipientID=%d", username, userID, msg.RecipientID)
						continue
					}
					// Add SenderID for forwarding
					msg.SenderID = userID
					// Marshal for sending
					jsonMsg, marshalErr := json.Marshal(msg)
					if marshalErr != nil {
						log.Printf("WS Error: Failed to marshal outgoing typing indicator: %v", marshalErr)
						continue
					}
					// Get recipient connections
					recipientConnections := connectionHub.GetUserConnections(msg.RecipientID)
					// Send to recipient
					for _, recipientConn := range recipientConnections {
						if writeErr := recipientConn.WriteMessage(websocket.TextMessage, jsonMsg); writeErr != nil {
							log.Printf("WS Error: Failed to send typing indicator to user %d: %v", msg.RecipientID, writeErr)
						}
					}
					log.Printf("Forwarded %s indicator from %d to %d", msg.Type, userID, msg.RecipientID)

				case "message_read":
					var msg MessageReadMessage
					if err := json.Unmarshal(p, &msg); err != nil {
						log.Printf("WS Error: Failed to unmarshal message_read: %v. Payload: %s", err, string(p))
						continue
					}
					// Basic validation
					if msg.SenderID <= 0 {
						log.Printf("WS Warning: Invalid message_read from %s (ID: %d): SenderID=%d", username, userID, msg.SenderID)
						continue
					}
					// Prepare the update message for the original sender
					updateMsg := ReadReceiptUpdateMessage{
						Type:     "read_receipt_update",
						ReaderID: userID,       // The current user read the message
						SenderID: msg.SenderID, // The user whose messages were read
					}
					// Marshal for sending
					jsonMsg, marshalErr := json.Marshal(updateMsg)
					if marshalErr != nil {
						log.Printf("WS Error: Failed to marshal read_receipt_update: %v", marshalErr)
						continue
					}
					// Get original sender's connections
					senderConnections := connectionHub.GetUserConnections(msg.SenderID)
					// Send update to original sender
					for _, senderConn := range senderConnections {
						if writeErr := senderConn.WriteMessage(websocket.TextMessage, jsonMsg); writeErr != nil {
							log.Printf("WS Error: Failed to send read receipt update to user %d: %v", msg.SenderID, writeErr)
						}
					}
					log.Printf("Sent read receipt update for sender %d from reader %d", msg.SenderID, userID)

				default:
					log.Printf("WS Warning: Received unhandled message type '%s' from %s (ID: %d)", msgType, username, userID)
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

// --- Handler Functions ---

// getMessagesHandler uses closure to access the store variable from main
// Use the concrete type *db.Queries (assuming this is what db.New returns)
func getMessagesHandler(store *db.Queries) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Get authenticated user from context
		authPayload, exists := c.Get(authorizationPayloadKey)
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Authorization payload not found in context"}) // Should not happen if middleware is correct
			return
		}
		payload := authPayload.(*token.Payload) // Type assertion
		loggedInUserID := payload.UserID

		// 2. Get partner_id from query string
		partnerIDStr := c.Query("partner_id")
		if partnerIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing 'partner_id' query parameter"})
			return
		}
		partnerID, err := strconv.ParseInt(partnerIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid 'partner_id' format"})
			return
		}

		// 3. Get pagination parameters (page, limit)
		pageStr := c.DefaultQuery("page", "1")
		limitStr := c.DefaultQuery("limit", "20") // Default limit 20 messages

		page, err := strconv.ParseInt(pageStr, 10, 32)
		if err != nil || page < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid 'page' format"})
			return
		}

		limit, err := strconv.ParseInt(limitStr, 10, 32)
		if err != nil || limit < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid 'limit' format"})
			return
		}

		// 4. Calculate offset
		offset := (int32(page) - 1) * int32(limit)

		// 5. Call store function
		// Use the 'store' variable captured by the closure
		messages, err := store.GetMessagesBetweenUsers(context.Background(), db.GetMessagesBetweenUsersParams{
			SenderID:   loggedInUserID,
			ReceiverID: int32(partnerID),
			Limit:      int32(limit),
			Offset:     offset, // Use the calculated offset
		})
		if err != nil {
			if err == sql.ErrNoRows {
				// Return empty list if no messages found, not an error
				c.JSON(http.StatusOK, []db.Message{})
				return
			}
			log.Printf("Error fetching messages between %d and %d: %v", loggedInUserID, partnerID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve messages"})
			return
		}

		// Handle case where messages might be nil from the DB query if no rows found
		if messages == nil {
			messages = []db.Message{}
		}

		// 6. Return messages
		c.JSON(http.StatusOK, messages)
	}
}

// --- Handler for listing offline users ---
func getOfflineUsersHandler(store *db.Queries) gin.HandlerFunc {
	return func(c *gin.Context) {
		offlineUsers, err := store.ListOfflineUsers(context.Background())
		if err != nil {
			log.Printf("Error fetching offline users: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list offline users"})
			return
		}

		// Format response similar to /users/online
		var userInfos []OnlineUserInfo // Re-use the same struct
		for _, user := range offlineUsers {
			userInfos = append(userInfos, OnlineUserInfo{
				ID:       user.ID,
				Username: user.Username,
			})
		}

		// Handle case where userInfos might be nil if no offline users found
		if userInfos == nil {
			userInfos = []OnlineUserInfo{}
		}

		c.JSON(http.StatusOK, gin.H{"offline_users": userInfos})
	}
}
