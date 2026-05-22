package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/ContainerSolutions/todo-app/db"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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

	req, _ := http.NewRequestWithContext(t.Context(), "GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}

// Test_Main_P_002B_AuthConfig verifies auth config endpoint exposes OIDC state.
func Test_Main_P_002B_AuthConfig(t *testing.T) {
	t.Setenv("OIDC_ISSUER_URL", "")
	t.Setenv("OIDC_CLIENT_ID", "")
	t.Setenv("OIDC_CLIENT_SECRET", "")
	t.Setenv("OIDC_REDIRECT_URL", "")
	setupTestDB(t)
	router := SetupRouter()

	req, _ := http.NewRequestWithContext(t.Context(), "GET", "/api/auth/config", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
	if w.Body.String() != "{\"oidc_configured\":false}" {
		t.Errorf("Expected OIDC disabled config, got %s", w.Body.String())
	}
}

// Test_Main_P_003_CORSMiddlewareDefaultSameOrigin verifies CORS does not allow all origins by default.
func Test_Main_P_003_CORSMiddlewareDefaultSameOrigin(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGIN", "")
	setupTestDB(t)
	router := SetupRouter()

	req, _ := http.NewRequestWithContext(t.Context(), "GET", "/health", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("Expected Access-Control-Allow-Origin header to be empty by default")
	}
}

// Test_Main_P_003B_CORSMiddlewareConfigured verifies configured CORS origin is allowed.
func Test_Main_P_003B_CORSMiddlewareConfigured(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGIN", "https://app.example.com")
	setupTestDB(t)
	router := SetupRouter()

	req, _ := http.NewRequestWithContext(t.Context(), "GET", "/health", nil)
	req.Header.Set("Origin", "https://app.example.com")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "https://app.example.com" {
		t.Error("Expected configured Access-Control-Allow-Origin header")
	}
}

// Test_Main_P_004_CORSMiddlewareOptions verifies OPTIONS request handling
func Test_Main_P_004_CORSMiddlewareOptions(t *testing.T) {
	setupTestDB(t)
	router := SetupRouter()
	req, _ := http.NewRequestWithContext(t.Context(), "OPTIONS", "/api/todos", nil)
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
	if path != "todo_app.db" {
		t.Errorf("Expected default path 'todo_app.db', got '%s'", path)
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

// Test_Main_P_009_RunSuccess verifies startup orchestration succeeds.
func Test_Main_P_009_RunSuccess(t *testing.T) {
	originalInitDB := initDBFunc
	originalStartServer := startServerFn
	defer func() {
		initDBFunc = originalInitDB
		startServerFn = originalStartServer
	}()

	t.Setenv("APP_ENV", "development")
	t.Setenv("PORT", "9090")

	initDBFunc = func(context.Context, db.Config) error {
		return nil
	}
	startServerFn = func(_ *gin.Engine, addr string) error {
		if addr != ":9090" {
			t.Errorf("Expected addr :9090, got %s", addr)
		}
		return nil
	}

	if err := run(t.Context()); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
}

// Test_Main_N_001_RunAuthConfigError verifies startup fails on auth config errors.
func Test_Main_N_001_RunAuthConfigError(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "")

	if err := run(t.Context()); err == nil {
		t.Error("Expected auth config error")
	}
}

// Test_Main_N_002_RunDBError verifies startup fails on database errors.
func Test_Main_N_002_RunDBError(t *testing.T) {
	originalInitDB := initDBFunc
	defer func() { initDBFunc = originalInitDB }()

	t.Setenv("APP_ENV", "development")
	initDBFunc = func(context.Context, db.Config) error {
		return gorm.ErrInvalidDB
	}

	if err := run(t.Context()); err == nil {
		t.Error("Expected database error")
	}
}

// Test_Main_N_003_RunServerError verifies startup fails on server errors.
func Test_Main_N_003_RunServerError(t *testing.T) {
	originalInitDB := initDBFunc
	originalStartServer := startServerFn
	defer func() {
		initDBFunc = originalInitDB
		startServerFn = originalStartServer
	}()

	t.Setenv("APP_ENV", "development")
	initDBFunc = func(context.Context, db.Config) error {
		return nil
	}
	startServerFn = func(*gin.Engine, string) error {
		return errors.New("server error")
	}

	if err := run(t.Context()); err == nil {
		t.Error("Expected server error")
	}
}
