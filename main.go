// Package main is the entry point for the websocket-simple-chat-app application.
package main

import (
	"context"      // Package context defines the Context type.
	"database/sql" // Package sql provides a generic interface around SQL databases.
	"log"          // Package log implements a simple logging package.
	"net/http"     // Package http provides HTTP client and server implementations.
	"os"           // Package os provides a platform-independent interface to operating system functionality.

	"github.com/gin-gonic/gin" // Gin is a HTTP web framework written in Go.
	_ "github.com/lib/pq"      // postgres driver.

	db "websocket-simple-chat-app/db/sqlc" // Import the generated db package.
)

// Constants for database connection details.
const dbDriverName = "postgres"
const dbDataSourceName = "postgres://postgres:159159@localhost:5432/chat_app_db?sslmode=disable"

// main is the main function, the entry point of the application.
func main() {
	// gin.Default() creates a Gin router with default middleware.
	r := gin.Default()

	// sql.Open() opens a database connection.
	dbConn, err := sql.Open(dbDriverName, dbDataSourceName)
	if err != nil {
		log.Fatal("cannot connect to db:", err)
	}
	defer dbConn.Close() // Close the database connection when main function exits.

	// db.New function creates a new Queries struct for database queries.
	store := db.New(dbConn)

	// Define route for ping test.
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	// Define route for user registration.
	r.POST("/users", func(c *gin.Context) {
		// Define struct for registration request data.
		type createUserRequest struct {
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		var req createUserRequest
		// Bind request body to createUserRequest struct.
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Create user in the database.
		user, err := store.CreateUser(context.Background(), db.CreateUserParams{
			Username:          req.Username,
			PasswordPlaintext: req.Password, // TODO: Hash the password in real application.
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "User created", "user_id": user.ID})
	})

	// Define route for user login.
	r.POST("/login", func(c *gin.Context) {
		// Define struct for login request data.
		type loginUserRequest struct {
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		var req loginUserRequest
		// Bind request body to loginUserRequest struct.
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Get user by username from database.
		user, err := store.GetUserByUsername(context.Background(), req.Username)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to login"})
			return
		}

		// Check password (plaintext comparison for now).
		if user.PasswordPlaintext != req.Password {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Logged in successfully", "user_id": user.ID})
	})

	// Get the port from the environment variable "PORT".
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port if not specified in environment variables.
	}
	// Start the Gin HTTP server.
	r.Run(":" + port)
}
