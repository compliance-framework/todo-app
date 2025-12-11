package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/ContainerSolutions/todo-app/db"
	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestDB(t *testing.T) {
	t.Helper()
	err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}
}

// Test_Main_P_001_SetupRouter verifies router setup works correctly
func Test_Main_P_001_SetupRouter(t *testing.T) {
	setupTestDB(t)
	router := SetupRouter()

	if router == nil {
		t.Error("SetupRouter should return non-nil router")
	}
}

// Test_Main_P_002_HealthCheck verifies health check endpoint works
func Test_Main_P_002_HealthCheck(t *testing.T) {
	setupTestDB(t)
	router := SetupRouter()

	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}

// Test_Main_P_003_CORSMiddleware verifies CORS headers are set
func Test_Main_P_003_CORSMiddleware(t *testing.T) {
	setupTestDB(t)
	router := SetupRouter()

	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Expected Access-Control-Allow-Origin header to be *")
	}
}

// Test_Main_P_004_CORSMiddlewareOptions verifies OPTIONS request handling
func Test_Main_P_004_CORSMiddlewareOptions(t *testing.T) {
	setupTestDB(t)
	router := SetupRouter()

	req, _ := http.NewRequest("OPTIONS", "/api/todos", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, w.Code)
	}
}

// Test_Main_P_005_GetDBPathDefault verifies default DB path
func Test_Main_P_005_GetDBPathDefault(t *testing.T) {
	os.Unsetenv("DB_PATH")
	path := GetDBPath()
	if path != "leo_app.db" {
		t.Errorf("Expected default path 'leo_app.db', got '%s'", path)
	}
}

// Test_Main_P_006_GetDBPathEnv verifies DB path from environment
func Test_Main_P_006_GetDBPathEnv(t *testing.T) {
	os.Setenv("DB_PATH", "/custom/path.db")
	defer os.Unsetenv("DB_PATH")

	path := GetDBPath()
	if path != "/custom/path.db" {
		t.Errorf("Expected path '/custom/path.db', got '%s'", path)
	}
}

// Test_Main_P_007_GetPortDefault verifies default port
func Test_Main_P_007_GetPortDefault(t *testing.T) {
	os.Unsetenv("PORT")
	port := GetPort()
	if port != "8080" {
		t.Errorf("Expected default port '8080', got '%s'", port)
	}
}

// Test_Main_P_008_GetPortEnv verifies port from environment
func Test_Main_P_008_GetPortEnv(t *testing.T) {
	os.Setenv("PORT", "3000")
	defer os.Unsetenv("PORT")

	port := GetPort()
	if port != "3000" {
		t.Errorf("Expected port '3000', got '%s'", port)
	}
}
