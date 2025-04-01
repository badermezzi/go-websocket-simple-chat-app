package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"strings" // Import strings package

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	_ "github.com/lib/pq"

	"time" // Import time package for token duration
	db "websocket-simple-chat-app/db/sqlc"
	"websocket-simple-chat-app/hub"   // Import the hub package
	"websocket-simple-chat-app/token" // Import the token package
)

const dbDriverName = "postgres"
const dbDataSourceName = "postgres://postgres:159159@localhost:5432/chat_app_db?sslmode=disable"

// TODO: Load configuration properly (e.g., from env vars or file)
const pasetoSymmetricKey = "12345678901234567890123456789012" // 32 bytes! Replace with secure key management
const accessTokenDuration = time.Hour * 24                    // Example duration
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func main() {
	// Create a new connection hub
	connectionHub := hub.NewHub()

	// TODO: Load symmetric key from config/env variable
	// Key must be exactly 32 bytes.
	// Create Paseto Maker
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

	// Set all users to offline on startup
	_, err = dbConn.Exec("UPDATE users SET status = 'offline'")
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

		// Create Paseto token
		tokenDuration := time.Hour // Token valid for 1 hour
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
				// The deferred Unregister function will handle status updates
				break
			}
			log.Printf("Received message from %s (ID: %d): type=%d, payload=%s\n", username, userID, messageType, string(p))

			// TODO: Implement actual message handling/broadcasting logic here
			// For now, just echoing back
			if err := conn.WriteMessage(messageType, p); err != nil {
				log.Printf("WS write error for user %s (ID: %d): %v\n", username, userID, err)
				break
			}
		}
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	r.Run(":" + port)
}
