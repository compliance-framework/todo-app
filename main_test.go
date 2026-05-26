package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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
	var response struct {
		OIDCConfigured bool `json:"oidc_configured"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal auth config response: %v", err)
	}
	if response.OIDCConfigured {
		t.Errorf("Expected OIDC disabled config, got %s", w.Body.String())
	}
}

func Test_Main_P_002C_NoRouteSPAFallbackOnlyForBrowserNavigation(t *testing.T) {
	setupTestDB(t)
	router := SetupRouter()

	tests := []struct {
		name       string
		method     string
		path       string
		accept     string
		wantStatus int
	}{
		{
			name:       "SPA route with HTML accept header",
			method:     http.MethodGet,
			path:       "/todos/123",
			accept:     "text/html",
			wantStatus: http.StatusOK,
		},
		{
			name:       "missing API route",
			method:     http.MethodGet,
			path:       "/api/missing",
			accept:     "text/html",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "API namespace root",
			method:     http.MethodGet,
			path:       "/api",
			accept:     "text/html",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "missing static asset",
			method:     http.MethodGet,
			path:       "/assets/missing.js",
			accept:     "text/html",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "existing static index redirect",
			method:     http.MethodGet,
			path:       "/index.html",
			accept:     "application/json",
			wantStatus: http.StatusMovedPermanently,
		},
		{
			name:       "non navigation request",
			method:     http.MethodGet,
			path:       "/todos/123",
			accept:     "application/json",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "non GET request",
			method:     http.MethodPost,
			path:       "/todos/123",
			accept:     "text/html",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequestWithContext(t.Context(), tc.method, tc.path, nil)
			req.Header.Set("Accept", tc.accept)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("Expected status %d, got %d", tc.wantStatus, w.Code)
			}
		})
	}
}

// Test_Main_P_002C_AuditLogMiddlewareLogsRecoveredPanic verifies panic responses are audited.
func Test_Main_P_002D_AuditLogMiddlewareLogsRecoveredPanic(t *testing.T) {
	originalStdout := os.Stdout
	defer func() {
		os.Stdout = originalStdout
	}()
	readCapturedStdout := func(stdoutReader *os.File) []byte {
		t.Helper()
		defer stdoutReader.Close()
		logs, err := io.ReadAll(stdoutReader)
		if err != nil {
			t.Fatalf("Failed to read captured stdout: %v", err)
		}
		return logs
	}

	firstStdoutReader, firstStdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to capture stdout: %v", err)
	}
	os.Stdout = firstStdoutWriter

	router := SetupRouter()
	router.GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	req, _ := http.NewRequestWithContext(t.Context(), "GET", "/panic", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
	if err := firstStdoutWriter.Close(); err != nil {
		t.Fatalf("Failed to close stdout writer: %v", err)
	}
	logs := readCapturedStdout(firstStdoutReader)
	logOutput := string(logs)
	if !strings.Contains(logOutput, `"method":"GET"`) ||
		!strings.Contains(logOutput, `"path":"/panic"`) ||
		!strings.Contains(logOutput, `"status":500`) ||
		!strings.HasPrefix(logOutput, "{") {
		t.Errorf("Expected raw JSON audit log on stdout, got %q", logOutput)
	}

	secondStdoutReader, secondStdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to capture stdout: %v", err)
	}
	os.Stdout = secondStdoutWriter
	router.GET("/audit-user", func(c *gin.Context) {
		c.Set("user_id", uint(42))
		c.Status(http.StatusNoContent)
	})
	req, _ = http.NewRequestWithContext(t.Context(), "GET", "/audit-user", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, w.Code)
	}
	if err := secondStdoutWriter.Close(); err != nil {
		t.Fatalf("Failed to close stdout writer: %v", err)
	}
	logs = readCapturedStdout(secondStdoutReader)
	logOutput = string(logs)
	if !strings.Contains(logOutput, `"path":"/audit-user"`) ||
		!strings.Contains(logOutput, `"user_id":42`) ||
		!strings.HasPrefix(logOutput, "{") {
		t.Errorf("Expected raw JSON audit log with user ID on stdout, got %q", logOutput)
	}

	thirdStdoutReader, thirdStdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to capture stdout: %v", err)
	}
	os.Stdout = thirdStdoutWriter
	router.GET("/audit-invalid-user", func(c *gin.Context) {
		c.Set("user_id", make(chan int))
		c.Status(http.StatusNoContent)
	})
	req, _ = http.NewRequestWithContext(t.Context(), "GET", "/audit-invalid-user", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status %d, got %d", http.StatusNoContent, w.Code)
	}
	if err := thirdStdoutWriter.Close(); err != nil {
		t.Fatalf("Failed to close stdout writer: %v", err)
	}
	logs = readCapturedStdout(thirdStdoutReader)
	logOutput = string(logs)
	if !strings.Contains(logOutput, `"path":"/audit-invalid-user"`) ||
		strings.Contains(logOutput, `"user_id"`) ||
		!strings.HasPrefix(logOutput, "{") {
		t.Errorf("Expected raw JSON audit log without invalid user ID on stdout, got %q", logOutput)
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
	if w.Header().Get("Vary") != "Origin" {
		t.Error("Expected Vary: Origin header for configured Access-Control-Allow-Origin")
	}
}

// Test_Main_P_003D_CORSMiddlewareDisallowedOriginVary verifies disallowed origins vary cached responses.
func Test_Main_P_003D_CORSMiddlewareDisallowedOriginVary(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGIN", "https://app.example.com")
	setupTestDB(t)
	router := SetupRouter()

	req, _ := http.NewRequestWithContext(t.Context(), "GET", "/health", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("Expected Access-Control-Allow-Origin header to be empty for disallowed origin")
	}
	if w.Header().Get("Vary") != "Origin" {
		t.Error("Expected Vary: Origin header for disallowed origin with configured CORS")
	}
}

// Test_Main_P_003C_CORSMiddlewareWildcardDoesNotVary verifies wildcard CORS avoids unnecessary vary.
func Test_Main_P_003C_CORSMiddlewareWildcardDoesNotVary(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGIN", "*")
	setupTestDB(t)
	router := SetupRouter()

	req, _ := http.NewRequestWithContext(t.Context(), "GET", "/health", nil)
	req.Header.Set("Origin", "https://app.example.com")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Expected wildcard Access-Control-Allow-Origin header")
	}
	if w.Header().Get("Vary") != "" {
		t.Errorf("Expected no Vary header for wildcard CORS, got %q", w.Header().Get("Vary"))
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

// Test_Main_P_010_MainSuccess verifies main exits normally when startup succeeds.
func Test_Main_P_010_MainSuccess(t *testing.T) {
	originalInitDB := initDBFunc
	originalStartServer := startServerFn
	defer func() {
		initDBFunc = originalInitDB
		startServerFn = originalStartServer
	}()

	t.Setenv("APP_ENV", "development")
	t.Setenv("PORT", "7070")

	initDBFunc = func(context.Context, db.Config) error {
		return nil
	}
	startServerFn = func(_ *gin.Engine, addr string) error {
		if addr != ":7070" {
			t.Errorf("Expected addr :7070, got %s", addr)
		}
		return nil
	}

	main()
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
