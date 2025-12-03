package main

import (
	"log"
	"os"

	"github.com/ContainerSolutions/todo-app/auth"
	"github.com/ContainerSolutions/todo-app/db"
	"github.com/ContainerSolutions/todo-app/handlers"
	"github.com/gin-gonic/gin"
)

func main() {
	// Get database path from environment or use default
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "leo_app.db"
	}

	// Initialize database
	if err := db.InitDB(dbPath); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Create Gin router
	r := gin.Default()

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})

	// Public routes (no auth required)
	r.POST("/api/register", handlers.Register)
	r.POST("/api/login", handlers.Login)

	// REQ03: Users should be able to see all todo lists (public read)
	r.GET("/api/todos", handlers.ListTodos)
	r.GET("/api/todos/:id", handlers.GetTodo)

	// Protected routes (auth required)
	protected := r.Group("/api")
	protected.Use(auth.AuthMiddleware())
	{
		// REQ02: Users should be able to create new TODOs
		protected.POST("/todos", handlers.CreateTodo)

		// REQ04: Users can only modify/delete their own TODOs
		protected.PUT("/todos/:id", handlers.UpdateTodo)
		protected.DELETE("/todos/:id", handlers.DeleteTodo)
	}

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting Leo App on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
