package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ContainerSolutions/todo-app/auth"
	"github.com/ContainerSolutions/todo-app/db"
	"github.com/ContainerSolutions/todo-app/handlers"
	"github.com/gin-gonic/gin"
)

//go:embed all:frontend/dist
var frontendFS embed.FS

var (
	initDBFunc    = db.InitDBWithConfig
	startServerFn = func(r *gin.Engine, addr string) error {
		return r.Run(addr)
	}
)

// SetupRouter creates and configures the Gin router with all routes
func SetupRouter() *gin.Engine {
	r := gin.New()

	// Request audit logging and CORS middleware
	r.Use(AuditLogMiddleware())
	r.Use(gin.Recovery())
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

	// Serve embedded frontend SPA; fallback to index.html for client-side routing
	dist, err := fs.Sub(frontendFS, "frontend/dist")
	if err != nil {
		log.Fatalf("failed to create frontend sub-FS: %v", err)
	}
	fileServer := http.FileServer(http.FS(dist))
	r.NoRoute(func(c *gin.Context) {
		path := strings.TrimPrefix(c.Request.URL.Path, "/")
		if c.Request.Method != http.MethodGet || path == "api" || strings.HasPrefix(path, "api/") {
			c.Status(http.StatusNotFound)
			return
		}

		if _, err := dist.Open(path); err == nil && path != "" {
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}

		segments := strings.Split(path, "/")
		if len(segments) > 0 && strings.Contains(segments[len(segments)-1], ".") {
			c.Status(http.StatusNotFound)
			return
		}

		if !strings.Contains(c.GetHeader("Accept"), "text/html") {
			c.Status(http.StatusNotFound)
			return
		}

		indexHTML, err := fs.ReadFile(dist, "index.html")
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
	})

	return r
}

// CORSMiddleware returns a middleware that handles CORS
func CORSMiddleware() gin.HandlerFunc {
	allowedOrigin := GetAllowedOrigin()
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && allowedOrigin != "" && allowedOrigin != "*" {
			c.Writer.Header().Set("Vary", "Origin")
		}
		if allowedOrigin != "" && (allowedOrigin == origin || allowedOrigin == "*") {
			c.Writer.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
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
		if userID, ok := auth.GetUserIDFromContext(c); ok {
			entry["user_id"] = userID
		}

		data, err := json.Marshal(entry)
		if err != nil {
			log.Printf("failed to marshal audit log: %v", err)
			return
		}
		fmt.Println(string(data))
	}
}

// HealthCheck handles the health check endpoint
func HealthCheck(c *gin.Context) {
	c.JSON(200, gin.H{"status": "healthy"})
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
