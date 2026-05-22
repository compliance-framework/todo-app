package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/ContainerSolutions/todo-app/auth"
	"github.com/ContainerSolutions/todo-app/db"
	"github.com/ContainerSolutions/todo-app/handlers"
	"github.com/gin-gonic/gin"
)

var (
	initDBFunc    = db.InitDBWithConfig
	startServerFn = func(r *gin.Engine, addr string) error {
		return r.Run(addr)
	}
)

// SetupRouter creates and configures the Gin router with all routes
func SetupRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	// Request audit logging and CORS middleware
	r.Use(AuditLogMiddleware())
	r.Use(CORSMiddleware())

	// Health check
	r.GET("/health", HealthCheck)

	// Public routes (no auth required)
	r.POST("/api/register", handlers.Register)
	r.POST("/api/login", handlers.Login)
	r.GET("/api/auth/config", handlers.AuthConfig)
	r.GET("/api/auth/oidc/login", handlers.OIDCLogin)
	r.GET("/api/auth/oidc/callback", handlers.OIDCCallback)

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
	allowedOrigin := GetAllowedOrigin()
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if allowedOrigin != "" && (allowedOrigin == origin || allowedOrigin == "*") {
			c.Writer.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			c.Writer.Header().Set("Vary", "Origin")
		}
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

// AuditLogMiddleware emits structured request logs to stdout.
func AuditLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		entry := map[string]interface{}{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"method":    c.Request.Method,
			"path":      c.Request.URL.Path,
			"status":    c.Writer.Status(),
		}
		if userID, exists := c.Get("user_id"); exists {
			entry["user_id"] = userID
		}

		data, err := json.Marshal(entry)
		if err != nil {
			log.Printf("failed to marshal audit log: %v", err)
			return
		}
		log.Println(string(data))
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

// GetAllowedOrigin returns the configured CORS allowed origin.
func GetAllowedOrigin() string {
	return strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGIN"))
}

// GetPort returns the port from environment or default
func GetPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return port
}

func run(ctx context.Context) error {
	if err := auth.ConfigureJWTSecretFromEnv(); err != nil {
		return fmt.Errorf("invalid auth configuration: %w", err)
	}

	// Initialize database
	if err := initDBFunc(ctx, db.ConfigFromEnv()); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	r := SetupRouter()

	port := GetPort()
	log.Printf("Starting todo App on port %s", port)
	if err := startServerFn(r, ":"+port); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
