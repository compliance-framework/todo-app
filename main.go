package main

import (
	"log"
	"os"

	"github.com/ContainerSolutions/todo-app/auth"
	"github.com/ContainerSolutions/todo-app/db"
	"github.com/ContainerSolutions/todo-app/handlers"
	"github.com/gin-gonic/gin"
)

// SetupRouter creates and configures the Gin router with all routes
func SetupRouter() *gin.Engine {
	r := gin.Default()

	// CORS middleware
	r.Use(CORSMiddleware())

	// Health check
	r.GET("/health", HealthCheck)

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

	return r
}

// CORSMiddleware returns a middleware that handles CORS
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

// HealthCheck handles the health check endpoint
func HealthCheck(c *gin.Context) {
	c.JSON(200, gin.H{"status": "healthy"})
}

// GetDBPath returns the database path from environment or default
func GetDBPath() string {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "todo_app.db"
	}
	return dbPath
}

// GetPort returns the port from environment or default
func GetPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return port
}

func main() {
	dbPath := GetDBPath()

	// Initialize database
	if err := db.InitDB(dbPath); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	r := SetupRouter()

	port := GetPort()
	log.Printf("Starting todo App on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func unusedFunction(logger *log.Logger) {
	logger.Println("This function is unused")
}
