// Package main is the entry point for the websocket-simple-chat-app application.
package main

import (
	"database/sql" // Package sql provides a generic interface around SQL databases.
	"fmt"          // Package fmt implements formatted I/O with functions analogous to C's printf and scanf.
	"log"          // Package log implements a simple logging package.
	"os"           // Package os provides a platform-independent interface to operating system functionality.

	"github.com/gin-gonic/gin" // Gin is a HTTP web framework written in Go (Golang). It features a Martini-like API with much better performance -- up to 40 times faster. If you need smashing performance, get yourself some Gin.
	_ "github.com/lib/pq"      // postgres driver - _ "github.com/lib/pq" imports the postgres driver package, making its database/sql driver available.
)

// Constants for database connection details.
const dbDriverName = "postgres"
const dbDataSourceName = "postgres://postgres:159159@localhost:5432/chat_app_db?sslmode=disable"

// main is the main function, the entry point of the application.
func main() {
	// gin.Default() creates a Gin router with default middleware:
	// logger and recovery (crash-free) middleware.
	r := gin.Default()

	// sql.Open() opens a database specified by its database driver name and a
	// driver-specific data source name, usually consisting of at least a database name and
	// connection information.
	db, err := sql.Open(dbDriverName, dbDataSourceName)
	if err != nil {
		// log.Fatal() prints the error and then calls os.Exit(1).
		log.Fatal("cannot connect to db:", err)
	}
	// defer db.Close() schedules the call to db.Close() to be run after the surrounding
	// function exits. This is important to close the database connection and release resources.
	defer db.Close()

	// db.Ping() verifies if the database connection is still alive, establishing a connection if necessary.
	err = db.Ping()
	if err != nil {
		log.Fatal("cannot ping db:", err)
	}
	fmt.Println("DB connection successful") // Print to console if database connection is successful.

	// r.GET("/ping", ...) defines a handler for GET requests to the "/ping" path.
	r.GET("/ping", func(c *gin.Context) {
		// c.JSON(200, gin.H{...}) responds with a JSON payload and HTTP status code 200 (OK).
		c.JSON(200, gin.H{
			"message": "pong", // The JSON response body will be {"message": "pong"}.
		})
	})

	// --- Port Configuration ---
	// Get the port from the environment variable "PORT".
	// Environment variables are a set of dynamic named values that can affect the way running processes will behave on a computer.
	// Here, we are checking if an environment variable named "PORT" is set. This is a common practice in cloud environments
	// where the port for applications is often configured externally.
	port := os.Getenv("PORT")
	// If the PORT environment variable is not set (i.e., it's an empty string),
	// we default to port 8080. This is a common default port for web applications.
	if port == "" {
		port = "8080" // Default port if not specified in environment variables.
	}
	// r.Run(":" + port) starts the Gin HTTP server on the specified port.
	// The ":" before the port number in ":8080" tells Gin to listen on all available network interfaces (all IP addresses)
	// on port 8080. If you wanted to bind to a specific IP address, you would include it before the colon, e.g., "127.0.0.1:8080"
	r.Run(":" + port)
}
