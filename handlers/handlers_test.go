package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ContainerSolutions/todo-app/auth"
	"github.com/ContainerSolutions/todo-app/db"
	"github.com/ContainerSolutions/todo-app/models"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
)

func setupTestDB(t *testing.T) {
	t.Helper()
	err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}
}

func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.Default()

	// Public routes
	r.POST("/api/register", Register)
	r.POST("/api/login", Login)
	r.GET("/api/todos", ListTodos)
	r.GET("/api/todos/:id", GetTodo)

	// Protected routes
	protected := r.Group("/api")
	protected.Use(auth.AuthMiddleware())
	{
		protected.POST("/todos", CreateTodo)
		protected.PUT("/todos/:id", UpdateTodo)
		protected.DELETE("/todos/:id", DeleteTodo)
	}

	return r
}

func mustNewRequest(t *testing.T, method, url string, body io.Reader) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, url, body)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	return req
}

func mustNewOIDCRequest(t *testing.T, client *http.Client, method, url string, body io.Reader) *http.Request {
	t.Helper()
	ctx := oidc.ClientContext(t.Context(), client)
	ctx = context.WithValue(ctx, oauth2.HTTPClient, client)
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	return req
}

func mustUnmarshalResponse[T any](t *testing.T, data []byte, target *T) {
	t.Helper()
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}

func createTestUser(t *testing.T, username, password string) models.User {
	t.Helper()
	hashedPassword, _ := auth.HashPassword(password)
	user := models.User{
		Username:     username,
		Password:     hashedPassword,
		AuthProvider: "password",
	}
	db.GetDB().Create(&user)
	return user
}

func getAuthToken(t *testing.T, userID uint, username string) string {
	t.Helper()
	token, err := auth.GenerateToken(userID, username)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}
	return token
}

// =============================================================================
// REQ01: Users should be able to LOGIN
// =============================================================================

// Test_REQ01_P_005_LoginSuccess verifies user can login with valid credentials
func Test_REQ01_P_005_LoginSuccess(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	// Create a test user
	createTestUser(t, "testuser", "password123")

	// Attempt login
	loginReq := LoginRequest{
		Username: "testuser",
		Password: "password123",
	}
	body, _ := json.Marshal(loginReq)

	req := mustNewRequest(t, "POST", "/api/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response LoginResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal login response: %v", err)
	}

	if response.Token == "" {
		t.Error("Expected token in response")
	}
}

// Test_REQ01_P_006_RegisterSuccess verifies user can register a new account
func Test_REQ01_P_006_RegisterSuccess(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	registerReq := RegisterRequest{
		Username: "newuser",
		Password: "password123",
	}
	body, _ := json.Marshal(registerReq)

	req := mustNewRequest(t, "POST", "/api/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var response struct {
		User models.User `json:"user"`
	}
	mustUnmarshalResponse(t, w.Body.Bytes(), &response)
	if response.User.AuthProvider != "password" {
		t.Errorf("Expected auth_provider password, got %q", response.User.AuthProvider)
	}
}

// Test_REQ01_N_009_LoginInvalidPassword verifies login fails with wrong password
func Test_REQ01_N_009_LoginInvalidPassword(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	createTestUser(t, "testuser", "password123")

	loginReq := LoginRequest{
		Username: "testuser",
		Password: "wrongpassword",
	}
	body, _ := json.Marshal(loginReq)

	req := mustNewRequest(t, "POST", "/api/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// Test_REQ01_N_010_LoginNonexistentUser verifies login fails for non-existent user
func Test_REQ01_N_010_LoginNonexistentUser(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	loginReq := LoginRequest{
		Username: "nonexistent",
		Password: "password123",
	}
	body, _ := json.Marshal(loginReq)

	req := mustNewRequest(t, "POST", "/api/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// Test_REQ01_N_011_RegisterDuplicateUsername verifies registration fails for duplicate username
func Test_REQ01_N_011_RegisterDuplicateUsername(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	createTestUser(t, "existinguser", "password123")

	registerReq := RegisterRequest{
		Username: "existinguser",
		Password: "password123",
	}
	body, _ := json.Marshal(registerReq)

	req := mustNewRequest(t, "POST", "/api/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected status %d, got %d", http.StatusConflict, w.Code)
	}
}

// =============================================================================
// REQ02: Users should be able to create new TODOs
// =============================================================================

// Test_REQ02_P_001_CreateTodoSuccess verifies authenticated user can create a todo
func Test_REQ02_P_001_CreateTodoSuccess(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	user := createTestUser(t, "testuser", "password123")
	token := getAuthToken(t, user.ID, user.Username)

	todoReq := CreateTodoRequest{
		Title:       "Test Todo",
		Description: "Test Description",
	}
	body, _ := json.Marshal(todoReq)

	req := mustNewRequest(t, "POST", "/api/todos", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var todo models.Todo
	mustUnmarshalResponse(t, w.Body.Bytes(), &todo)

	if todo.Title != "Test Todo" {
		t.Errorf("Expected title 'Test Todo', got '%s'", todo.Title)
	}
	if todo.UserID != user.ID {
		t.Errorf("Expected user_id %d, got %d", user.ID, todo.UserID)
	}
}

// Test_REQ02_N_001_CreateTodoUnauthenticated verifies unauthenticated user cannot create todo
func Test_REQ02_N_001_CreateTodoUnauthenticated(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	todoReq := CreateTodoRequest{
		Title: "Test Todo",
	}
	body, _ := json.Marshal(todoReq)

	req := mustNewRequest(t, "POST", "/api/todos", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// Test_REQ02_N_002_CreateTodoEmptyTitle verifies todo creation fails without title
func Test_REQ02_N_002_CreateTodoEmptyTitle(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	user := createTestUser(t, "testuser", "password123")
	token := getAuthToken(t, user.ID, user.Username)

	todoReq := CreateTodoRequest{
		Title: "",
	}
	body, _ := json.Marshal(todoReq)

	req := mustNewRequest(t, "POST", "/api/todos", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// =============================================================================
// REQ03: Users should be able to see all todo lists
// =============================================================================

// Test_REQ03_P_001_ListAllTodos verifies all todos are returned
func Test_REQ03_P_001_ListAllTodos(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	user := createTestUser(t, "testuser", "password123")

	// Create some todos
	db.GetDB().Create(&models.Todo{Title: "Todo 1", UserID: user.ID})
	db.GetDB().Create(&models.Todo{Title: "Todo 2", UserID: user.ID})

	req := mustNewRequest(t, "GET", "/api/todos", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var todos []models.Todo
	mustUnmarshalResponse(t, w.Body.Bytes(), &todos)

	if len(todos) != 2 {
		t.Errorf("Expected 2 todos, got %d", len(todos))
	}
}

// Test_REQ03_P_002_GetTodoById verifies single todo can be retrieved
func Test_REQ03_P_002_GetTodoById(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	user := createTestUser(t, "testuser", "password123")
	todo := models.Todo{Title: "Test Todo", UserID: user.ID}
	db.GetDB().Create(&todo)

	req := mustNewRequest(t, "GET", "/api/todos/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var returnedTodo models.Todo
	mustUnmarshalResponse(t, w.Body.Bytes(), &returnedTodo)

	if returnedTodo.Title != "Test Todo" {
		t.Errorf("Expected title 'Test Todo', got '%s'", returnedTodo.Title)
	}
}

// Test_REQ03_P_003_ListTodosUnauthenticated verifies unauthenticated users can view todos
func Test_REQ03_P_003_ListTodosUnauthenticated(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	user := createTestUser(t, "testuser", "password123")
	db.GetDB().Create(&models.Todo{Title: "Public Todo", UserID: user.ID})

	req := mustNewRequest(t, "GET", "/api/todos", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}

// Test_REQ03_N_001_GetNonexistentTodo verifies 404 for non-existent todo
func Test_REQ03_N_001_GetNonexistentTodo(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	req := mustNewRequest(t, "GET", "/api/todos/999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// Test_REQ03_E_001_ListEmptyTodos verifies empty list returned when no todos
func Test_REQ03_E_001_ListEmptyTodos(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	req := mustNewRequest(t, "GET", "/api/todos", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var todos []models.Todo
	mustUnmarshalResponse(t, w.Body.Bytes(), &todos)

	if len(todos) != 0 {
		t.Errorf("Expected 0 todos, got %d", len(todos))
	}
}

// =============================================================================
// REQ04: Users should NOT be able to modify/delete TODOs they did not create
// =============================================================================

// Test_REQ04_P_001_UpdateOwnTodo verifies user can update their own todo
func Test_REQ04_P_001_UpdateOwnTodo(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	user := createTestUser(t, "testuser", "password123")
	token := getAuthToken(t, user.ID, user.Username)

	todo := models.Todo{Title: "Original Title", UserID: user.ID}
	db.GetDB().Create(&todo)

	updateReq := UpdateTodoRequest{
		Title: "Updated Title",
	}
	body, _ := json.Marshal(updateReq)

	req := mustNewRequest(t, "PUT", "/api/todos/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var updatedTodo models.Todo
	mustUnmarshalResponse(t, w.Body.Bytes(), &updatedTodo)

	if updatedTodo.Title != "Updated Title" {
		t.Errorf("Expected title 'Updated Title', got '%s'", updatedTodo.Title)
	}
}

// Test_REQ04_P_002_DeleteOwnTodo verifies user can delete their own todo
func Test_REQ04_P_002_DeleteOwnTodo(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	user := createTestUser(t, "testuser", "password123")
	token := getAuthToken(t, user.ID, user.Username)

	todo := models.Todo{Title: "To Delete", UserID: user.ID}
	db.GetDB().Create(&todo)

	req := mustNewRequest(t, "DELETE", "/api/todos/1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}

// Test_REQ04_N_001_UpdateOtherUserTodo verifies user cannot update another user's todo
func Test_REQ04_N_001_UpdateOtherUserTodo(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	// Create two users
	user1 := createTestUser(t, "user1", "password123")
	user2 := createTestUser(t, "user2", "password123")
	token2 := getAuthToken(t, user2.ID, user2.Username)

	// Create todo owned by user1
	todo := models.Todo{Title: "User1's Todo", UserID: user1.ID}
	db.GetDB().Create(&todo)

	// User2 tries to update user1's todo
	updateReq := UpdateTodoRequest{
		Title: "Hacked Title",
	}
	body, _ := json.Marshal(updateReq)

	req := mustNewRequest(t, "PUT", "/api/todos/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token2)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

// Test_REQ04_N_002_DeleteOtherUserTodo verifies user cannot delete another user's todo
func Test_REQ04_N_002_DeleteOtherUserTodo(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	// Create two users
	user1 := createTestUser(t, "user1", "password123")
	user2 := createTestUser(t, "user2", "password123")
	token2 := getAuthToken(t, user2.ID, user2.Username)

	// Create todo owned by user1
	todo := models.Todo{Title: "User1's Todo", UserID: user1.ID}
	db.GetDB().Create(&todo)

	// User2 tries to delete user1's todo
	req := mustNewRequest(t, "DELETE", "/api/todos/1", nil)
	req.Header.Set("Authorization", "Bearer "+token2)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

// Test_REQ04_N_003_UpdateTodoUnauthenticated verifies unauthenticated user cannot update
func Test_REQ04_N_003_UpdateTodoUnauthenticated(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	user := createTestUser(t, "testuser", "password123")
	todo := models.Todo{Title: "Test Todo", UserID: user.ID}
	db.GetDB().Create(&todo)

	updateReq := UpdateTodoRequest{
		Title: "Hacked Title",
	}
	body, _ := json.Marshal(updateReq)

	req := mustNewRequest(t, "PUT", "/api/todos/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// Test_REQ04_N_004_DeleteTodoUnauthenticated verifies unauthenticated user cannot delete
func Test_REQ04_N_004_DeleteTodoUnauthenticated(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	user := createTestUser(t, "testuser", "password123")
	todo := models.Todo{Title: "Test Todo", UserID: user.ID}
	db.GetDB().Create(&todo)

	req := mustNewRequest(t, "DELETE", "/api/todos/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// =============================================================================
// Additional Edge Case Tests for 100% Coverage
// =============================================================================

// Test_REQ03_N_002_GetTodoInvalidID verifies invalid todo ID returns bad request
func Test_REQ03_N_002_GetTodoInvalidID(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	req := mustNewRequest(t, "GET", "/api/todos/invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// Test_REQ04_N_005_UpdateTodoInvalidID verifies invalid todo ID returns bad request
func Test_REQ04_N_005_UpdateTodoInvalidID(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	user := createTestUser(t, "testuser", "password123")
	token := getAuthToken(t, user.ID, user.Username)

	updateReq := UpdateTodoRequest{
		Title: "Updated Title",
	}
	body, _ := json.Marshal(updateReq)

	req := mustNewRequest(t, "PUT", "/api/todos/invalid", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// Test_REQ04_N_006_UpdateNonexistentTodo verifies updating non-existent todo returns not found
func Test_REQ04_N_006_UpdateNonexistentTodo(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	user := createTestUser(t, "testuser", "password123")
	token := getAuthToken(t, user.ID, user.Username)

	updateReq := UpdateTodoRequest{
		Title: "Updated Title",
	}
	body, _ := json.Marshal(updateReq)

	req := mustNewRequest(t, "PUT", "/api/todos/999", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// Test_REQ04_N_007_DeleteTodoInvalidID verifies invalid todo ID returns bad request
func Test_REQ04_N_007_DeleteTodoInvalidID(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	user := createTestUser(t, "testuser", "password123")
	token := getAuthToken(t, user.ID, user.Username)

	req := mustNewRequest(t, "DELETE", "/api/todos/invalid", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// Test_REQ04_N_008_DeleteNonexistentTodo verifies deleting non-existent todo returns not found
func Test_REQ04_N_008_DeleteNonexistentTodo(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	user := createTestUser(t, "testuser", "password123")
	token := getAuthToken(t, user.ID, user.Username)

	req := mustNewRequest(t, "DELETE", "/api/todos/999", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

// Test_REQ01_N_012_LoginInvalidJSON verifies login fails with invalid JSON
func Test_REQ01_N_012_LoginInvalidJSON(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	req := mustNewRequest(t, "POST", "/api/login", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// Test_REQ01_N_013_RegisterInvalidJSON verifies registration fails with invalid JSON
func Test_REQ01_N_013_RegisterInvalidJSON(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	req := mustNewRequest(t, "POST", "/api/register", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// Test_REQ02_N_003_CreateTodoInvalidJSON verifies create todo fails with invalid JSON
func Test_REQ02_N_003_CreateTodoInvalidJSON(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	user := createTestUser(t, "testuser", "password123")
	token := getAuthToken(t, user.ID, user.Username)

	req := mustNewRequest(t, "POST", "/api/todos", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// Test_REQ04_P_003_UpdateTodoCompleted verifies updating todo completed status
func Test_REQ04_P_003_UpdateTodoCompleted(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	user := createTestUser(t, "testuser", "password123")
	token := getAuthToken(t, user.ID, user.Username)

	todo := models.Todo{Title: "Test Todo", UserID: user.ID, Completed: false}
	db.GetDB().Create(&todo)

	completed := true
	updateReq := UpdateTodoRequest{
		Completed: &completed,
	}
	body, _ := json.Marshal(updateReq)

	req := mustNewRequest(t, "PUT", "/api/todos/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var updatedTodo models.Todo
	mustUnmarshalResponse(t, w.Body.Bytes(), &updatedTodo)

	if !updatedTodo.Completed {
		t.Error("Expected todo to be completed")
	}
}

// Test_REQ04_P_004_UpdateTodoDescription verifies updating todo description
func Test_REQ04_P_004_UpdateTodoDescription(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	user := createTestUser(t, "testuser", "password123")
	token := getAuthToken(t, user.ID, user.Username)

	todo := models.Todo{Title: "Test Todo", Description: "Original", UserID: user.ID}
	db.GetDB().Create(&todo)

	updateReq := UpdateTodoRequest{
		Description: "Updated Description",
	}
	body, _ := json.Marshal(updateReq)

	req := mustNewRequest(t, "PUT", "/api/todos/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var updatedTodo models.Todo
	err := json.Unmarshal(w.Body.Bytes(), &updatedTodo)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
	if updatedTodo.Description != "Updated Description" {
		t.Errorf("Expected description 'Updated Description', got '%s'", updatedTodo.Description)
	}
}

// Test_REQ04_N_009_UpdateTodoInvalidJSON verifies update todo fails with invalid JSON
func Test_REQ04_N_009_UpdateTodoInvalidJSON(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	user := createTestUser(t, "testuser", "password123")
	token := getAuthToken(t, user.ID, user.Username)

	todo := models.Todo{Title: "Test Todo", UserID: user.ID}
	db.GetDB().Create(&todo)

	req := mustNewRequest(t, "PUT", "/api/todos/1", bytes.NewBuffer([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// =============================================================================
// Database Error Tests (using closed DB connection)
// =============================================================================

// Test_REQ03_E_002_ListTodosDBError verifies ListTodos handles DB error
func Test_REQ03_E_002_ListTodosDBError(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	// Close the underlying SQL connection to force an error
	sqlDB, _ := db.GetDB().DB()
	sqlDB.Close()

	req := mustNewRequest(t, "GET", "/api/todos", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	// Re-initialize DB for other tests
	setupTestDB(t)
}

// Test_REQ02_E_001_CreateTodoDBError verifies CreateTodo handles DB error
func Test_REQ02_E_001_CreateTodoDBError(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	user := createTestUser(t, "testuser", "password123")
	token := getAuthToken(t, user.ID, user.Username)

	// Rename the todos table to cause create to fail
	db.GetDB().Exec("ALTER TABLE todos RENAME TO todos_backup")
	db.GetDB().Exec("CREATE VIEW todos AS SELECT * FROM todos_backup")

	todoReq := CreateTodoRequest{
		Title: "Test Todo",
	}
	body, _ := json.Marshal(todoReq)

	req := mustNewRequest(t, "POST", "/api/todos", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	// Re-initialize DB for other tests
	setupTestDB(t)
}

// Test_REQ04_E_001_UpdateTodoDBErrorOnSave verifies UpdateTodo handles DB error on save
func Test_REQ04_E_001_UpdateTodoDBErrorOnSave(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	user := createTestUser(t, "testuser", "password123")
	token := getAuthToken(t, user.ID, user.Username)

	todo := models.Todo{Title: "Test Todo", UserID: user.ID}
	db.GetDB().Create(&todo)

	// Rename the todos table to cause save to fail but allow read
	db.GetDB().Exec("ALTER TABLE todos RENAME TO todos_backup")
	db.GetDB().Exec("CREATE VIEW todos AS SELECT * FROM todos_backup")

	updateReq := UpdateTodoRequest{
		Title: "Updated Title",
	}
	body, _ := json.Marshal(updateReq)

	req := mustNewRequest(t, "PUT", "/api/todos/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should be InternalServerError (can't update a view)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	// Re-initialize DB for other tests
	setupTestDB(t)
}

// Test_REQ04_E_002_DeleteTodoDBErrorOnDelete verifies DeleteTodo handles DB error on delete
func Test_REQ04_E_002_DeleteTodoDBErrorOnDelete(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	user := createTestUser(t, "testuser", "password123")
	token := getAuthToken(t, user.ID, user.Username)

	todo := models.Todo{Title: "Test Todo", UserID: user.ID}
	db.GetDB().Create(&todo)

	// Rename the todos table to cause delete to fail but allow read
	db.GetDB().Exec("ALTER TABLE todos RENAME TO todos_backup")
	db.GetDB().Exec("CREATE VIEW todos AS SELECT * FROM todos_backup")

	req := mustNewRequest(t, "DELETE", "/api/todos/1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should be InternalServerError (can't delete from a view)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	// Re-initialize DB for other tests
	setupTestDB(t)
}

// Test_REQ01_E_002_RegisterDBError verifies Register handles DB error on create
func Test_REQ01_E_002_RegisterDBError(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	// Rename the users table to cause create to fail but allow check
	db.GetDB().Exec("ALTER TABLE users RENAME TO users_backup")
	db.GetDB().Exec("CREATE VIEW users AS SELECT * FROM users_backup")

	registerReq := RegisterRequest{
		Username: "newuser",
		Password: "password123",
	}
	body, _ := json.Marshal(registerReq)

	req := mustNewRequest(t, "POST", "/api/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	// Re-initialize DB for other tests
	setupTestDB(t)
}

// Test_REQ01_E_003_LoginDBError verifies Login handles DB error
func Test_REQ01_E_003_LoginDBError(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	// Close the underlying SQL connection to force an error
	sqlDB, _ := db.GetDB().DB()
	sqlDB.Close()

	loginReq := LoginRequest{
		Username: "testuser",
		Password: "password123",
	}
	body, _ := json.Marshal(loginReq)

	req := mustNewRequest(t, "POST", "/api/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return Unauthorized since user can't be found
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	// Re-initialize DB for other tests
	setupTestDB(t)
}

// =============================================================================
// Mock-based Error Tests
// =============================================================================

// Test_REQ01_E_004_RegisterHashPasswordError verifies Register handles hash password error
func Test_REQ01_E_004_RegisterHashPasswordError(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	// Mock HashPassword to return an error
	originalFunc := auth.HashPasswordFunc
	auth.HashPasswordFunc = func(password string) (string, error) {
		return "", errors.New("mock hash error")
	}
	defer func() { auth.HashPasswordFunc = originalFunc }()

	registerReq := RegisterRequest{
		Username: "newuser",
		Password: "password123",
	}
	body, _ := json.Marshal(registerReq)

	req := mustNewRequest(t, "POST", "/api/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// Test_REQ01_E_005_LoginGenerateTokenError verifies Login handles token generation error
func Test_REQ01_E_005_LoginGenerateTokenError(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	// Create a test user
	createTestUser(t, "testuser", "password123")

	// Mock GenerateToken to return an error
	originalFunc := auth.GenerateTokenFunc
	auth.GenerateTokenFunc = func(userID uint, username string) (string, error) {
		return "", errors.New("mock token error")
	}
	defer func() { auth.GenerateTokenFunc = originalFunc }()

	loginReq := LoginRequest{
		Username: "testuser",
		Password: "password123",
	}
	body, _ := json.Marshal(loginReq)

	req := mustNewRequest(t, "POST", "/api/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// Test_REQ02_E_002_CreateTodoNoUserInContext verifies CreateTodo handles missing user in context
func Test_REQ02_E_002_CreateTodoNoUserInContext(t *testing.T) {
	setupTestDB(t)
	gin.SetMode(gin.TestMode)

	// Create a router WITHOUT auth middleware to test the handler directly
	router := gin.New()
	router.POST("/api/todos", CreateTodo)

	todoReq := CreateTodoRequest{
		Title: "Test Todo",
	}
	body, _ := json.Marshal(todoReq)

	req := mustNewRequest(t, "POST", "/api/todos", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// Test_REQ04_E_003_UpdateTodoNoUserInContext verifies UpdateTodo handles missing user in context
func Test_REQ04_E_003_UpdateTodoNoUserInContext(t *testing.T) {
	setupTestDB(t)
	gin.SetMode(gin.TestMode)

	// Create a router WITHOUT auth middleware
	router := gin.New()
	router.PUT("/api/todos/:id", UpdateTodo)

	updateReq := UpdateTodoRequest{
		Title: "Updated Title",
	}
	body, _ := json.Marshal(updateReq)

	req := mustNewRequest(t, "PUT", "/api/todos/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// Test_REQ04_E_004_DeleteTodoNoUserInContext verifies DeleteTodo handles missing user in context
func Test_REQ04_E_004_DeleteTodoNoUserInContext(t *testing.T) {
	setupTestDB(t)
	gin.SetMode(gin.TestMode)

	// Create a router WITHOUT auth middleware
	router := gin.New()
	router.DELETE("/api/todos/:id", DeleteTodo)

	req := mustNewRequest(t, "DELETE", "/api/todos/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// Test_REQ01_P_007_AuthConfig verifies auth config reports OIDC availability.
func Test_REQ01_P_007_AuthConfig(t *testing.T) {
	t.Setenv("OIDC_ISSUER_URL", "https://issuer.example.com")
	t.Setenv("OIDC_CLIENT_ID", "client-id")
	t.Setenv("OIDC_CLIENT_SECRET", "client-secret")
	t.Setenv("OIDC_REDIRECT_URL", "https://app.example.com/callback")

	router := gin.New()
	router.GET("/api/auth/config", AuthConfig)

	req := mustNewRequest(t, "GET", "/api/auth/config", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
	var response AuthConfigResponse
	mustUnmarshalResponse(t, w.Body.Bytes(), &response)
	if !response.OIDCConfigured {
		t.Error("Expected OIDC to be configured")
	}
}

// Test_REQ01_N_014_OIDCLoginNotConfigured verifies OIDC login fails when disabled.
func Test_REQ01_N_014_OIDCLoginNotConfigured(t *testing.T) {
	t.Setenv("OIDC_ISSUER_URL", "")
	t.Setenv("OIDC_CLIENT_ID", "")
	t.Setenv("OIDC_CLIENT_SECRET", "")
	t.Setenv("OIDC_REDIRECT_URL", "")

	router := gin.New()
	router.GET("/api/auth/oidc/login", OIDCLogin)

	req := mustNewRequest(t, "GET", "/api/auth/oidc/login", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}
}

// Test_REQ01_N_014A_OIDCLoginRandomStateError verifies OIDC login handles random source failures.
func Test_REQ01_N_014A_OIDCLoginRandomStateError(t *testing.T) {
	issuer, oidcClient := installTestOIDCProvider(t, testOIDCProvider{
		key: mustGenerateRSAKey(t),
	})
	setTestOIDCEnv(t, issuer)

	originalReader := randomReader
	t.Cleanup(func() { randomReader = originalReader })

	for _, tt := range []struct {
		name            string
		successfulReads int
	}{
		{name: "state", successfulReads: 0},
		{name: "code_verifier", successfulReads: 1},
		{name: "nonce", successfulReads: 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			randomReader = &failingRandomReader{successfulReads: tt.successfulReads}

			router := gin.New()
			router.GET("/api/auth/oidc/login", OIDCLogin)

			req := mustNewOIDCRequest(t, oidcClient, "GET", "/api/auth/oidc/login", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusInternalServerError {
				t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
			}
		})
	}
}

// Test_REQ01_P_011_OIDCLoginRedirect verifies OIDC login redirects to provider.
func Test_REQ01_P_011_OIDCLoginRedirect(t *testing.T) {
	issuer, oidcClient := installTestOIDCProvider(t, testOIDCProvider{
		key: mustGenerateRSAKey(t),
	})
	t.Setenv("APP_ENV", "test")
	t.Setenv("OIDC_ISSUER_URL", issuer)
	t.Setenv("OIDC_CLIENT_ID", "client-id")
	t.Setenv("OIDC_CLIENT_SECRET", "client-secret")
	t.Setenv("OIDC_REDIRECT_URL", "https://app.example.com/callback")

	router := gin.New()
	router.GET("/api/auth/oidc/login", OIDCLogin)

	req := mustNewOIDCRequest(t, oidcClient, "GET", "/api/auth/oidc/login", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("Expected status %d, got %d", http.StatusFound, w.Code)
	}
	if !strings.HasPrefix(w.Header().Get("Location"), issuer+"/auth") {
		t.Errorf("Expected redirect to provider auth endpoint, got %q", w.Header().Get("Location"))
	}
	location, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatalf("Parse redirect location: %v", err)
	}
	query := location.Query()
	if query.Get("code_challenge_method") != "S256" {
		t.Errorf("Expected S256 PKCE challenge method, got %q", query.Get("code_challenge_method"))
	}
	if query.Get("code_challenge") == "" {
		t.Error("Expected PKCE code challenge")
	}
	if query.Get("nonce") == "" {
		t.Error("Expected OIDC nonce")
	}
	result := w.Result()
	defer result.Body.Close()
	if len(result.Cookies()) == 0 {
		t.Error("Expected OIDC state cookie")
	}
	if result.Cookies()[0].MaxAge != int(oidcLoginStateTTL/time.Second) {
		t.Fatalf("Expected OIDC state cookie MaxAge to match TTL, got %d", result.Cookies()[0].MaxAge)
	}
	loginState, err := decodeOIDCLoginState(result.Cookies()[0])
	if err != nil {
		t.Fatalf("Decode OIDC state cookie: %v", err)
	}
	if loginState.State != query.Get("state") || loginState.Nonce != query.Get("nonce") {
		t.Fatalf("OIDC state cookie does not match redirect parameters: %+v", loginState)
	}
	codeVerifier, ok := takeOIDCCodeVerifier(loginState.State)
	if !ok {
		t.Fatal("Expected OIDC code verifier to be stored server-side")
	}
	if pkceChallenge(codeVerifier) != query.Get("code_challenge") {
		t.Error("Stored OIDC code verifier does not match redirect PKCE challenge")
	}
}

// Test_REQ01_N_015_OIDCCallbackInvalidState verifies callback rejects invalid state.
func Test_REQ01_N_015_OIDCCallbackInvalidState(t *testing.T) {
	router := gin.New()
	router.GET("/api/auth/oidc/callback", OIDCCallback)

	req := mustNewRequest(t, "GET", "/api/auth/oidc/callback?state=bad", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// Test_REQ01_N_015A_OIDCCallbackStateMismatchClearsVerifier verifies mismatched states clear OIDC state.
func Test_REQ01_N_015A_OIDCCallbackStateMismatchClearsVerifier(t *testing.T) {
	router := gin.New()
	router.GET("/api/auth/oidc/callback", OIDCCallback)

	req := mustNewRequest(t, "GET", "/api/auth/oidc/callback?state=other-state", nil)
	req.AddCookie(testOIDCStateCookie(t, "state", "nonce"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
	if _, ok := takeOIDCCodeVerifier("state"); ok {
		t.Error("Expected mismatched callback to clear stored OIDC code verifier")
	}
	result := w.Result()
	defer result.Body.Close()
	if len(result.Cookies()) == 0 || result.Cookies()[0].Name != "oidc_state" || result.Cookies()[0].MaxAge != -1 {
		t.Error("Expected mismatched callback to clear OIDC state cookie")
	}
}

// Test_REQ01_N_015C_OIDCCallbackMalformedStateCookieClearsCookie verifies bad state cookies are cleared.
func Test_REQ01_N_015C_OIDCCallbackMalformedStateCookieClearsCookie(t *testing.T) {
	router := gin.New()
	router.GET("/api/auth/oidc/callback", OIDCCallback)

	req := mustNewRequest(t, "GET", "/api/auth/oidc/callback?state=state", nil)
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "not-a-valid-signed-state"})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
	result := w.Result()
	defer result.Body.Close()
	if len(result.Cookies()) == 0 || result.Cookies()[0].Name != "oidc_state" || result.Cookies()[0].MaxAge != -1 {
		t.Error("Expected malformed state cookie to be cleared")
	}
}

// Test_REQ01_N_015B_OIDCCallbackMissingStoredCodeVerifier verifies callback requires server-side PKCE state.
func Test_REQ01_N_015B_OIDCCallbackMissingStoredCodeVerifier(t *testing.T) {
	t.Setenv("APP_ENV", "test")

	router := gin.New()
	router.GET("/api/auth/oidc/callback", OIDCCallback)

	value, err := encodeOIDCLoginState(oidcLoginState{State: "state", Nonce: "nonce"})
	if err != nil {
		t.Fatalf("encode OIDC login state: %v", err)
	}
	req := mustNewRequest(t, "GET", "/api/auth/oidc/callback?state=state&code=code", nil)
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: value})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

// Test_REQ01_N_016_OIDCCallbackMissingCode verifies callback requires an auth code.
func Test_REQ01_N_016_OIDCCallbackMissingCode(t *testing.T) {
	router := gin.New()
	router.GET("/api/auth/oidc/callback", OIDCCallback)

	req := mustNewRequest(t, "GET", "/api/auth/oidc/callback?state=state", nil)
	req.AddCookie(testOIDCStateCookie(t, "state", "nonce"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
	result := w.Result()
	defer result.Body.Close()
	if result.Cookies()[0].MaxAge != -1 {
		t.Error("Expected OIDC state cookie to be cleared")
	}
}

// Test_REQ01_N_017_OIDCCallbackTokenExchangeFailed verifies token exchange failures.
func Test_REQ01_N_017_OIDCCallbackTokenExchangeFailed(t *testing.T) {
	issuer, oidcClient := installTestOIDCProvider(t, testOIDCProvider{
		key:         mustGenerateRSAKey(t),
		tokenStatus: http.StatusInternalServerError,
		tokenBody:   func() string { return `{"error":"server_error"}` },
	})
	setTestOIDCEnv(t, issuer)

	router := gin.New()
	router.GET("/api/auth/oidc/callback", OIDCCallback)

	req := mustNewOIDCRequest(t, oidcClient, "GET", "/api/auth/oidc/callback?state=state&code=code", nil)
	req.AddCookie(testOIDCStateCookie(t, "state", "nonce"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// Test_REQ01_N_018_OIDCCallbackMissingIDToken verifies callback requires an ID token.
func Test_REQ01_N_018_OIDCCallbackMissingIDToken(t *testing.T) {
	issuer, oidcClient := installTestOIDCProvider(t, testOIDCProvider{
		key:       mustGenerateRSAKey(t),
		tokenBody: func() string { return `{"access_token":"access","token_type":"Bearer"}` },
	})
	setTestOIDCEnv(t, issuer)

	router := gin.New()
	router.GET("/api/auth/oidc/callback", OIDCCallback)

	req := mustNewOIDCRequest(t, oidcClient, "GET", "/api/auth/oidc/callback?state=state&code=code", nil)
	req.AddCookie(testOIDCStateCookie(t, "state", "nonce"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// Test_REQ01_N_019_OIDCCallbackInvalidIDToken verifies callback verifies ID tokens.
func Test_REQ01_N_019_OIDCCallbackInvalidIDToken(t *testing.T) {
	issuer, oidcClient := installTestOIDCProvider(t, testOIDCProvider{
		key:       mustGenerateRSAKey(t),
		tokenBody: func() string { return `{"access_token":"access","token_type":"Bearer","id_token":"invalid"}` },
	})
	setTestOIDCEnv(t, issuer)

	router := gin.New()
	router.GET("/api/auth/oidc/callback", OIDCCallback)

	req := mustNewOIDCRequest(t, oidcClient, "GET", "/api/auth/oidc/callback?state=state&code=code", nil)
	req.AddCookie(testOIDCStateCookie(t, "state", "nonce"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// Test_REQ01_P_012_OIDCCallbackSuccess verifies callback creates user and returns app JWT.
func Test_REQ01_P_012_OIDCCallbackSuccess(t *testing.T) {
	setupTestDB(t)
	key := mustGenerateRSAKey(t)
	var issuer string
	var oidcClient *http.Client
	issuer, oidcClient = installTestOIDCProvider(t, testOIDCProvider{
		key: key,
		tokenBody: func() string {
			return `{"access_token":"access","token_type":"Bearer","id_token":"` +
				mustSignIDToken(t, key, issuer, "client-id", "subject-success", "success@example.com") + `"}`
		},
	})
	setTestOIDCEnv(t, issuer)

	router := gin.New()
	router.GET("/api/auth/oidc/callback", OIDCCallback)

	req := mustNewOIDCRequest(t, oidcClient, "GET", "/api/auth/oidc/callback?state=state&code=code", nil)
	req.AddCookie(testOIDCStateCookie(t, "state", "nonce"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status %d, got %d with body %s", http.StatusOK, w.Code, w.Body.String())
	}
	var response LoginResponse
	mustUnmarshalResponse(t, w.Body.Bytes(), &response)
	if response.Token == "" || response.User.Username != "success@example.com" {
		t.Fatalf("Unexpected OIDC login response: %+v", response)
	}
}

// Test_REQ01_N_020_OIDCCallbackProviderConfigError verifies provider config errors.
func Test_REQ01_N_020_OIDCCallbackProviderConfigError(t *testing.T) {
	t.Setenv("OIDC_ISSUER_URL", "://bad issuer")
	t.Setenv("OIDC_CLIENT_ID", "client-id")
	t.Setenv("OIDC_CLIENT_SECRET", "client-secret")
	t.Setenv("OIDC_REDIRECT_URL", "https://app.example.com/callback")

	router := gin.New()
	router.GET("/api/auth/oidc/callback", OIDCCallback)

	req := mustNewRequest(t, "GET", "/api/auth/oidc/callback?state=state&code=code", nil)
	req.AddCookie(testOIDCStateCookie(t, "state", "nonce"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
	}
}

// Test_REQ01_N_021_OIDCCallbackInvalidClaims verifies claim decoding errors.
func Test_REQ01_N_021_OIDCCallbackInvalidClaims(t *testing.T) {
	setupTestDB(t)
	key := mustGenerateRSAKey(t)
	var issuer string
	var oidcClient *http.Client
	issuer, oidcClient = installTestOIDCProvider(t, testOIDCProvider{
		key: key,
		tokenBody: func() string {
			return `{"access_token":"access","token_type":"Bearer","id_token":"` +
				mustSignIDTokenClaims(t, key, jwt.MapClaims{
					"iss":   issuer,
					"aud":   "client-id",
					"sub":   "subject-invalid-claims",
					"email": 123,
					"exp":   time.Now().Add(time.Hour).Unix(),
					"iat":   time.Now().Unix(),
				}) + `"}`
		},
	})
	setTestOIDCEnv(t, issuer)

	router := gin.New()
	router.GET("/api/auth/oidc/callback", OIDCCallback)

	req := mustNewOIDCRequest(t, oidcClient, "GET", "/api/auth/oidc/callback?state=state&code=code", nil)
	req.AddCookie(testOIDCStateCookie(t, "state", "nonce"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// Test_REQ01_N_023_OIDCCallbackInvalidNonce verifies callback binds ID tokens to the login nonce.
func Test_REQ01_N_023_OIDCCallbackInvalidNonce(t *testing.T) {
	setupTestDB(t)
	key := mustGenerateRSAKey(t)
	var issuer string
	var oidcClient *http.Client
	issuer, oidcClient = installTestOIDCProvider(t, testOIDCProvider{
		key: key,
		tokenBody: func() string {
			return `{"access_token":"access","token_type":"Bearer","id_token":"` +
				mustSignIDToken(t, key, issuer, "client-id", "subject-bad-nonce", "bad-nonce@example.com", "wrong-nonce") + `"}`
		},
	})
	setTestOIDCEnv(t, issuer)

	router := gin.New()
	router.GET("/api/auth/oidc/callback", OIDCCallback)

	req := mustNewOIDCRequest(t, oidcClient, "GET", "/api/auth/oidc/callback?state=state&code=code", nil)
	req.AddCookie(testOIDCStateCookie(t, "state", "nonce"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// Test_REQ01_N_022_OIDCCallbackAccountLinkingConflict verifies semantic linking conflicts return 409.
func Test_REQ01_N_022_OIDCCallbackAccountLinkingConflict(t *testing.T) {
	setupTestDB(t)
	existing := createTestUser(t, "linked-callback@example.com", "password123")
	if _, err := attachOIDCIdentity(existing, "https://existing-issuer.example.com", auth.OIDCClaims{Subject: "existing-subject"}); err != nil {
		t.Fatalf("attachOIDCIdentity returned error: %v", err)
	}

	key := mustGenerateRSAKey(t)
	var issuer string
	var oidcClient *http.Client
	issuer, oidcClient = installTestOIDCProvider(t, testOIDCProvider{
		key: key,
		tokenBody: func() string {
			return `{"access_token":"access","token_type":"Bearer","id_token":"` +
				mustSignIDToken(t, key, issuer, "client-id", "conflicting-subject", "linked-callback@example.com") + `"}`
		},
	})
	setTestOIDCEnv(t, issuer)

	router := gin.New()
	router.GET("/api/auth/oidc/callback", OIDCCallback)

	req := mustNewOIDCRequest(t, oidcClient, "GET", "/api/auth/oidc/callback?state=state&code=code", nil)
	req.AddCookie(testOIDCStateCookie(t, "state", "nonce"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("Expected status %d, got %d with body %s", http.StatusConflict, w.Code, w.Body.String())
	}
}

// Test_REQ01_E_006_OIDCCallbackGenerateTokenError verifies app token generation errors.
func Test_REQ01_E_006_OIDCCallbackGenerateTokenError(t *testing.T) {
	setupTestDB(t)
	originalGenerateToken := auth.GenerateTokenFunc
	auth.GenerateTokenFunc = func(uint, string) (string, error) {
		return "", errors.New("token error")
	}
	defer func() { auth.GenerateTokenFunc = originalGenerateToken }()

	key := mustGenerateRSAKey(t)
	var issuer string
	var oidcClient *http.Client
	issuer, oidcClient = installTestOIDCProvider(t, testOIDCProvider{
		key: key,
		tokenBody: func() string {
			return `{"access_token":"access","token_type":"Bearer","id_token":"` +
				mustSignIDToken(t, key, issuer, "client-id", "subject-token-error", "token-error@example.com") + `"}`
		},
	})
	setTestOIDCEnv(t, issuer)

	router := gin.New()
	router.GET("/api/auth/oidc/callback", OIDCCallback)

	req := mustNewOIDCRequest(t, oidcClient, "GET", "/api/auth/oidc/callback?state=state&code=code", nil)
	req.AddCookie(testOIDCStateCookie(t, "state", "nonce"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// Test_REQ01_P_008_UpsertOIDCUserCreateAndFind verifies OIDC users are created and reused.
func Test_REQ01_P_008_UpsertOIDCUserCreateAndFind(t *testing.T) {
	setupTestDB(t)

	claims := auth.OIDCClaims{
		Subject:       "subject-1",
		Email:         "oidc@example.com",
		EmailVerified: true,
	}

	user, err := upsertOIDCUser("https://issuer.example.com", claims)
	if err != nil {
		t.Fatalf("upsertOIDCUser returned error: %v", err)
	}
	if user.ID == 0 || user.Username != claims.Email || user.Email == nil || *user.Email != claims.Email {
		t.Fatalf("Unexpected OIDC user: %+v", user)
	}
	if user.Password == "" || user.Password == "OIDC_LOGIN_ONLY" || !strings.HasPrefix(user.Password, "$2") {
		t.Fatalf("Expected OIDC-only password to be stored as a bcrypt hash, got %q", user.Password)
	}
	if auth.CheckPasswordHash("OIDC_LOGIN_ONLY", user.Password) {
		t.Fatal("OIDC-only password hash must not authenticate the previous sentinel value")
	}

	sameUser, err := upsertOIDCUser("https://issuer.example.com", claims)
	if err != nil {
		t.Fatalf("upsertOIDCUser returned error on existing user: %v", err)
	}
	if sameUser.ID != user.ID {
		t.Errorf("Expected existing user ID %d, got %d", user.ID, sameUser.ID)
	}
}

// Test_REQ01_P_008A_UpsertOIDCUserBackfillsVerifiedEmail verifies later verified email claims are persisted.
func Test_REQ01_P_008A_UpsertOIDCUserBackfillsVerifiedEmail(t *testing.T) {
	setupTestDB(t)

	issuer := "https://issuer.example.com"
	claims := auth.OIDCClaims{
		Subject:       "subject-backfill",
		Email:         "missing@example.com",
		EmailVerified: false,
	}

	user, err := upsertOIDCUser(issuer, claims)
	if err != nil {
		t.Fatalf("upsertOIDCUser returned error: %v", err)
	}
	if user.Email != nil {
		t.Fatalf("Expected initial OIDC user to have no email, got %+v", user.Email)
	}

	claims.Email = " Verified@Example.com "
	claims.EmailVerified = true
	updatedUser, err := upsertOIDCUser(issuer, claims)
	if err != nil {
		t.Fatalf("upsertOIDCUser returned error on existing user: %v", err)
	}
	if updatedUser.ID != user.ID {
		t.Fatalf("Expected existing user ID %d, got %d", user.ID, updatedUser.ID)
	}
	if updatedUser.Email == nil || *updatedUser.Email != "verified@example.com" {
		t.Fatalf("Expected verified email to be backfilled, got %+v", updatedUser.Email)
	}

	var reloaded models.User
	if err := db.GetDB().First(&reloaded, user.ID).Error; err != nil {
		t.Fatalf("Failed to reload user: %v", err)
	}
	if reloaded.Email == nil || *reloaded.Email != "verified@example.com" {
		t.Fatalf("Expected persisted verified email, got %+v", reloaded.Email)
	}
}

func Test_REQ01_N_008A_UpsertOIDCUserBackfillEmailConflict(t *testing.T) {
	setupTestDB(t)

	issuer := "https://issuer.example.com"
	claims := auth.OIDCClaims{
		Subject:       "subject-backfill-conflict",
		Email:         "missing@example.com",
		EmailVerified: false,
	}

	user, err := upsertOIDCUser(issuer, claims)
	if err != nil {
		t.Fatalf("upsertOIDCUser returned error: %v", err)
	}
	if user.Email != nil {
		t.Fatalf("Expected initial OIDC user to have no email, got %+v", user.Email)
	}

	conflictingEmail := "verified@example.com"
	conflictingUser := createTestUser(t, "email-owner", "password123")
	if err := db.GetDB().Model(&conflictingUser).Update("email", conflictingEmail).Error; err != nil {
		t.Fatalf("Failed to set conflicting email: %v", err)
	}

	claims.Email = " Verified@Example.com "
	claims.EmailVerified = true
	_, err = upsertOIDCUser(issuer, claims)
	if !errors.Is(err, errOIDCUserAlreadyLinked) {
		t.Fatalf("Expected email backfill conflict to fail with errOIDCUserAlreadyLinked, got %v", err)
	}

	var reloaded models.User
	if err := db.GetDB().First(&reloaded, user.ID).Error; err != nil {
		t.Fatalf("Failed to reload user: %v", err)
	}
	if reloaded.Email != nil {
		t.Fatalf("Expected OIDC user email to remain unset after conflict, got %+v", reloaded.Email)
	}
}

func Test_REQ01_P_008C_UpsertOIDCUserTreatsOverlongEmailAsAbsent(t *testing.T) {
	setupTestDB(t)

	claims := auth.OIDCClaims{
		Subject:       "subject-overlong-email",
		Email:         strings.Repeat("a", maxOIDCEmailLength-len("@example.com")+1) + "@example.com",
		EmailVerified: true,
	}

	user, err := upsertOIDCUser("https://issuer.example.com", claims)
	if err != nil {
		t.Fatalf("upsertOIDCUser returned error: %v", err)
	}
	if user.Email != nil {
		t.Fatalf("Expected overlong email claim to be ignored, got %+v", user.Email)
	}
	if !strings.HasPrefix(user.Username, "oidc-") {
		t.Fatalf("Expected generated OIDC username for ignored overlong email, got %q", user.Username)
	}
}

func Test_REQ01_P_008D_UpsertOIDCUserStoresLongEmailButDoesNotUseItAsUsername(t *testing.T) {
	setupTestDB(t)

	longEmail := strings.Repeat("a", maxOIDCUsernameLength-len("@example.com")+1) + "@example.com"
	claims := auth.OIDCClaims{
		Subject:       "subject-long-email",
		Email:         longEmail,
		EmailVerified: true,
	}

	user, err := upsertOIDCUser("https://issuer.example.com", claims)
	if err != nil {
		t.Fatalf("upsertOIDCUser returned error: %v", err)
	}
	if user.Email == nil || *user.Email != longEmail {
		t.Fatalf("Expected long verified email to be stored, got %+v", user.Email)
	}
	if !strings.HasPrefix(user.Username, "oidc-") {
		t.Fatalf("Expected generated OIDC username for long email, got %q", user.Username)
	}
	if user.Username == longEmail {
		t.Fatal("Expected long email not to be reused as username")
	}
}

// Test_REQ01_P_008B_UpsertOIDCUserRetriesAfterCreateRace verifies concurrent creates are idempotent.
func Test_REQ01_P_008B_UpsertOIDCUserRetriesAfterCreateRace(t *testing.T) {
	setupTestDB(t)

	issuer := "https://issuer.example.com"
	claims := auth.OIDCClaims{
		Subject:       "subject-race",
		Email:         "race@example.com",
		EmailVerified: true,
	}
	originalCreateOIDCUser := createOIDCUser
	t.Cleanup(func() { createOIDCUser = originalCreateOIDCUser })

	createOIDCUser = func(user *models.User) error {
		competingUser := *user
		if err := originalCreateOIDCUser(&competingUser); err != nil {
			t.Fatalf("Failed to create competing OIDC user: %v", err)
		}
		return errors.New("UNIQUE constraint failed: users.oidc_issuer, users.oidc_subject")
	}

	user, err := upsertOIDCUser(issuer, claims)
	if err != nil {
		t.Fatalf("upsertOIDCUser returned error after create race: %v", err)
	}
	if user.ID == 0 || user.OIDCIssuer == nil || *user.OIDCIssuer != issuer || user.OIDCSubject == nil || *user.OIDCSubject != claims.Subject {
		t.Fatalf("Expected existing OIDC user after create race, got %+v", user)
	}
}

func Test_REQ01_P_008E_UpsertOIDCUserRetriesEmailAttachAfterCreateRace(t *testing.T) {
	setupTestDB(t)

	issuer := "https://issuer.example.com"
	claims := auth.OIDCClaims{
		Subject:       "subject-email-race",
		Email:         "email-race@example.com",
		EmailVerified: true,
	}
	originalCreateOIDCUser := createOIDCUser
	t.Cleanup(func() { createOIDCUser = originalCreateOIDCUser })

	createOIDCUser = func(user *models.User) error {
		competingUser := createTestUser(t, "email-race-owner", "password123")
		competingUser.Email = user.Email
		if err := db.GetDB().Save(&competingUser).Error; err != nil {
			t.Fatalf("Failed to create competing email user: %v", err)
		}
		return errors.New("UNIQUE constraint failed: users.email")
	}

	user, err := upsertOIDCUser(issuer, claims)
	if err != nil {
		t.Fatalf("upsertOIDCUser returned error after email create race: %v", err)
	}
	if user.Username != "email-race-owner" || user.OIDCIssuer == nil || *user.OIDCIssuer != issuer || user.OIDCSubject == nil || *user.OIDCSubject != claims.Subject {
		t.Fatalf("Expected competing email user to receive OIDC identity, got %+v", user)
	}
}

func Test_REQ01_P_008F_UpsertOIDCUserRetriesUsernameAttachAfterCreateRace(t *testing.T) {
	setupTestDB(t)

	issuer := "https://issuer.example.com"
	claims := auth.OIDCClaims{
		Subject:       "subject-username-race",
		Email:         "username-race@example.com",
		EmailVerified: true,
	}
	originalCreateOIDCUser := createOIDCUser
	t.Cleanup(func() { createOIDCUser = originalCreateOIDCUser })

	createOIDCUser = func(user *models.User) error {
		createTestUser(t, user.Username, "password123")
		return errors.New("UNIQUE constraint failed: users.username")
	}

	user, err := upsertOIDCUser(issuer, claims)
	if err != nil {
		t.Fatalf("upsertOIDCUser returned error after username create race: %v", err)
	}
	if user.Username != claims.Email || user.OIDCIssuer == nil || *user.OIDCIssuer != issuer || user.OIDCSubject == nil || *user.OIDCSubject != claims.Subject {
		t.Fatalf("Expected competing username user to receive OIDC identity, got %+v", user)
	}
}

func Test_REQ01_N_008E_UpsertOIDCUserCreateRacePreservesUsernameConflict(t *testing.T) {
	setupTestDB(t)

	claims := auth.OIDCClaims{
		Subject:       "subject-username-conflict-race",
		Email:         "username-conflict-race@example.com",
		EmailVerified: true,
	}
	originalCreateOIDCUser := createOIDCUser
	t.Cleanup(func() { createOIDCUser = originalCreateOIDCUser })

	createOIDCUser = func(user *models.User) error {
		competingUser := createTestUser(t, user.Username, "password123")
		competingUser.AuthProvider = "oidc"
		if err := db.GetDB().Save(&competingUser).Error; err != nil {
			t.Fatalf("Failed to create competing non-password user: %v", err)
		}
		return errors.New("UNIQUE constraint failed: users.username")
	}

	_, err := upsertOIDCUser("https://issuer.example.com", claims)
	if !errors.Is(err, errOIDCUsernameMatchNotPasswordUser) {
		t.Fatalf("Expected username conflict sentinel after create race, got %v", err)
	}
}

// Test_REQ01_P_009_UpsertOIDCUserAttachByEmail verifies OIDC identity attaches by email.
func Test_REQ01_P_009_UpsertOIDCUserAttachByEmail(t *testing.T) {
	setupTestDB(t)
	existing := createTestUser(t, "existing@example.com", "password123")

	user, err := upsertOIDCUser("https://issuer.example.com", auth.OIDCClaims{
		Subject:       "subject-2",
		Email:         " existing@example.com ",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("upsertOIDCUser returned error: %v", err)
	}
	if user.ID != existing.ID || user.OIDCIssuer == nil || user.OIDCSubject == nil {
		t.Fatalf("Expected existing user to receive OIDC identity, got %+v", user)
	}
	if user.AuthProvider != "password" {
		t.Fatalf("Expected linked password user to keep auth_provider password, got %q", user.AuthProvider)
	}
	if user.Email == nil || *user.Email != "existing@example.com" {
		t.Fatalf("Expected normalized email, got %+v", user.Email)
	}

	emailOwner := createTestUser(t, "email-owner", "password123")
	sharedEmail := "shared@example.com"
	emailOwner.Email = &sharedEmail
	if err := db.GetDB().Save(&emailOwner).Error; err != nil {
		t.Fatalf("Failed to set email owner email: %v", err)
	}
	usernameOwner := createTestUser(t, sharedEmail, "password123")

	user, err = upsertOIDCUser("https://issuer.example.com", auth.OIDCClaims{
		Subject:       "subject-2b",
		Email:         sharedEmail,
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("upsertOIDCUser returned error for shared email/username: %v", err)
	}
	if user.ID != emailOwner.ID {
		t.Fatalf("Expected email match user ID %d, got %d", emailOwner.ID, user.ID)
	}

	var reloadedUsernameOwner models.User
	if err := db.GetDB().First(&reloadedUsernameOwner, usernameOwner.ID).Error; err != nil {
		t.Fatalf("Failed to reload username owner: %v", err)
	}
	if reloadedUsernameOwner.OIDCIssuer != nil || reloadedUsernameOwner.OIDCSubject != nil {
		t.Fatalf("Expected username owner to remain unlinked, got %+v", reloadedUsernameOwner)
	}
}

func Test_REQ01_P_009A_UpsertOIDCUserAttachByEmailCaseInsensitive(t *testing.T) {
	setupTestDB(t)
	existing := createTestUser(t, "case-email-owner", "password123")
	storedEmail := "User@Example.com"
	existing.Email = &storedEmail
	if err := db.GetDB().Save(&existing).Error; err != nil {
		t.Fatalf("Failed to set existing user email: %v", err)
	}

	user, err := upsertOIDCUser("https://issuer.example.com", auth.OIDCClaims{
		Subject:       "subject-case-email",
		Email:         " user@example.com ",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("upsertOIDCUser returned error: %v", err)
	}
	if user.ID != existing.ID || user.OIDCIssuer == nil || user.OIDCSubject == nil {
		t.Fatalf("Expected existing mixed-case email user to receive OIDC identity, got %+v", user)
	}
	if user.Email == nil || *user.Email != "user@example.com" {
		t.Fatalf("Expected linked email to be normalized to lowercase, got %+v", user.Email)
	}
}

func Test_REQ01_N_027_UpsertOIDCUserRejectsAmbiguousCaseInsensitiveEmail(t *testing.T) {
	setupTestDB(t)
	first := createTestUser(t, "case-email-owner-one", "password123")
	firstEmail := "User@Example.com"
	first.Email = &firstEmail
	if err := db.GetDB().Save(&first).Error; err != nil {
		t.Fatalf("Failed to set first existing user email: %v", err)
	}
	second := createTestUser(t, "case-email-owner-two", "password123")
	secondEmail := "USER@example.com"
	second.Email = &secondEmail
	if err := db.GetDB().Save(&second).Error; err != nil {
		t.Fatalf("Failed to set second existing user email: %v", err)
	}

	_, err := upsertOIDCUser("https://issuer.example.com", auth.OIDCClaims{
		Subject:       "subject-ambiguous-email",
		Email:         " user@example.com ",
		EmailVerified: true,
	})
	if !errors.Is(err, errOIDCEmailMatchAmbiguous) {
		t.Fatalf("Expected ambiguous email error, got %v", err)
	}

	for _, existing := range []models.User{first, second} {
		var reloaded models.User
		if err := db.GetDB().First(&reloaded, existing.ID).Error; err != nil {
			t.Fatalf("Failed to reload user %d: %v", existing.ID, err)
		}
		if reloaded.OIDCIssuer != nil || reloaded.OIDCSubject != nil {
			t.Fatalf("Expected ambiguous email user %d to remain unlinked, got %+v", existing.ID, reloaded)
		}
	}
}

func Test_REQ01_P_009B_UpsertOIDCUserAttachByUsernameCaseInsensitive(t *testing.T) {
	setupTestDB(t)
	existing := createTestUser(t, "User@Example.com", "password123")

	user, err := upsertOIDCUser("https://issuer.example.com", auth.OIDCClaims{
		Subject:       "subject-case-username",
		Email:         " user@example.com ",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("upsertOIDCUser returned error: %v", err)
	}
	if user.ID != existing.ID || user.OIDCIssuer == nil || user.OIDCSubject == nil {
		t.Fatalf("Expected existing mixed-case username user to receive OIDC identity, got %+v", user)
	}
	if user.AuthProvider != "password" {
		t.Fatalf("Expected linked password user to keep auth_provider password, got %q", user.AuthProvider)
	}
	if user.Email == nil || *user.Email != "user@example.com" {
		t.Fatalf("Expected linked email to be normalized to lowercase, got %+v", user.Email)
	}
}

func Test_REQ01_N_028_UpsertOIDCUserRejectsAmbiguousCaseInsensitiveUsername(t *testing.T) {
	setupTestDB(t)
	first := createTestUser(t, "User@Example.com", "password123")
	second := createTestUser(t, "USER@example.com", "password123")

	_, err := upsertOIDCUser("https://issuer.example.com", auth.OIDCClaims{
		Subject:       "subject-ambiguous-username",
		Email:         " user@example.com ",
		EmailVerified: true,
	})
	if !errors.Is(err, errOIDCUsernameMatchAmbiguous) {
		t.Fatalf("Expected ambiguous username error, got %v", err)
	}

	for _, existing := range []models.User{first, second} {
		var reloaded models.User
		if err := db.GetDB().First(&reloaded, existing.ID).Error; err != nil {
			t.Fatalf("Failed to reload user %d: %v", existing.ID, err)
		}
		if reloaded.OIDCIssuer != nil || reloaded.OIDCSubject != nil {
			t.Fatalf("Expected ambiguous username user %d to remain unlinked, got %+v", existing.ID, reloaded)
		}
	}
}

// Test_REQ01_P_013_UpsertOIDCUserDoesNotAttachUnverifiedEmail verifies unverified emails do not link accounts.
func Test_REQ01_P_013_UpsertOIDCUserDoesNotAttachUnverifiedEmail(t *testing.T) {
	setupTestDB(t)
	existing := createTestUser(t, "existing@example.com", "password123")

	user, err := upsertOIDCUser("https://issuer.example.com", auth.OIDCClaims{
		Subject:       "subject-3",
		Email:         " existing@example.com ",
		EmailVerified: false,
	})
	if err != nil {
		t.Fatalf("upsertOIDCUser returned error: %v", err)
	}
	if user.ID == existing.ID {
		t.Fatalf("Expected a new OIDC-only user, got existing user ID %d", user.ID)
	}
	if user.Username == "existing@example.com" {
		t.Fatalf("Expected unverified email not to be used as username, got %q", user.Username)
	}
	if user.Email != nil {
		t.Fatalf("Expected unverified email not to be stored, got %+v", user.Email)
	}

	var reloadedExisting models.User
	if err := db.GetDB().First(&reloadedExisting, existing.ID).Error; err != nil {
		t.Fatalf("Failed to reload existing user: %v", err)
	}
	if reloadedExisting.OIDCIssuer != nil || reloadedExisting.OIDCSubject != nil {
		t.Fatalf("Expected existing user to remain unlinked, got %+v", reloadedExisting)
	}
}

// Test_REQ01_P_014_AttachOIDCIdentityRefusesRelink verifies linked accounts cannot be re-linked.
func Test_REQ01_P_014_AttachOIDCIdentityRefusesRelink(t *testing.T) {
	setupTestDB(t)
	user := createTestUser(t, "linked@example.com", "password123")

	linked, err := attachOIDCIdentity(user, "https://issuer.example.com", auth.OIDCClaims{Subject: "subject-1"})
	if err != nil {
		t.Fatalf("attachOIDCIdentity returned error: %v", err)
	}

	sameLinked, err := attachOIDCIdentity(linked, "https://issuer.example.com", auth.OIDCClaims{Subject: "subject-1"})
	if err != nil {
		t.Fatalf("attachOIDCIdentity returned error for same identity: %v", err)
	}
	if sameLinked.ID != linked.ID {
		t.Fatalf("Expected same linked user ID %d, got %d", linked.ID, sameLinked.ID)
	}

	if _, err := attachOIDCIdentity(linked, "https://issuer.example.com", auth.OIDCClaims{Subject: "subject-2"}); err == nil {
		t.Fatal("Expected relink to a different OIDC subject to fail")
	}

	var reloaded models.User
	if err := db.GetDB().First(&reloaded, linked.ID).Error; err != nil {
		t.Fatalf("Failed to reload linked user: %v", err)
	}
	if reloaded.OIDCSubject == nil || *reloaded.OIDCSubject != "subject-1" {
		t.Fatalf("Expected original OIDC subject to remain unchanged, got %+v", reloaded)
	}
}

// Test_REQ01_P_014B_AttachOIDCIdentityRejectsStaleConcurrentRelink verifies stale unlinked rows cannot overwrite links.
func Test_REQ01_P_014B_AttachOIDCIdentityRejectsStaleConcurrentRelink(t *testing.T) {
	setupTestDB(t)
	user := createTestUser(t, "stale-linked@example.com", "password123")
	staleUser := user

	linked, err := attachOIDCIdentity(user, "https://issuer-one.example.com", auth.OIDCClaims{Subject: "subject-1"})
	if err != nil {
		t.Fatalf("attachOIDCIdentity returned error: %v", err)
	}

	sameLinked, err := attachOIDCIdentity(staleUser, "https://issuer-one.example.com", auth.OIDCClaims{Subject: "subject-1"})
	if err != nil {
		t.Fatalf("Expected stale same-identity attach to return linked user, got %v", err)
	}
	if sameLinked.ID != linked.ID {
		t.Fatalf("Expected stale same-identity attach to return user ID %d, got %d", linked.ID, sameLinked.ID)
	}

	_, err = attachOIDCIdentity(staleUser, "https://issuer-two.example.com", auth.OIDCClaims{Subject: "subject-2"})
	if !errors.Is(err, errOIDCUserAlreadyLinked) {
		t.Fatalf("Expected stale relink to fail with errOIDCUserAlreadyLinked, got %v", err)
	}

	var reloaded models.User
	if err := db.GetDB().First(&reloaded, linked.ID).Error; err != nil {
		t.Fatalf("Failed to reload linked user: %v", err)
	}
	if reloaded.OIDCIssuer == nil || *reloaded.OIDCIssuer != "https://issuer-one.example.com" ||
		reloaded.OIDCSubject == nil || *reloaded.OIDCSubject != "subject-1" {
		t.Fatalf("Expected stale relink not to overwrite original OIDC identity, got %+v", reloaded)
	}
}

// Test_REQ01_P_014D_AttachOIDCIdentityMapsDuplicateIdentityToConflict verifies unique conflicts are semantic link conflicts.
func Test_REQ01_P_014D_AttachOIDCIdentityMapsDuplicateIdentityToConflict(t *testing.T) {
	setupTestDB(t)
	linkedOwner := createTestUser(t, "linked-owner@example.com", "password123")
	target := createTestUser(t, "link-target@example.com", "password123")

	if _, err := attachOIDCIdentity(linkedOwner, "https://issuer.example.com", auth.OIDCClaims{Subject: "subject-1"}); err != nil {
		t.Fatalf("attachOIDCIdentity returned error: %v", err)
	}

	_, err := attachOIDCIdentity(target, "https://issuer.example.com", auth.OIDCClaims{Subject: "subject-1"})
	if !errors.Is(err, errOIDCUserAlreadyLinked) {
		t.Fatalf("Expected duplicate identity attach to fail with errOIDCUserAlreadyLinked, got %v", err)
	}

	var reloaded models.User
	if err := db.GetDB().First(&reloaded, target.ID).Error; err != nil {
		t.Fatalf("Failed to reload target user: %v", err)
	}
	if reloaded.OIDCIssuer != nil || reloaded.OIDCSubject != nil {
		t.Fatalf("Expected target user to remain unlinked, got %+v", reloaded)
	}
}

// Test_REQ01_P_014C_AttachOIDCIdentityNormalizesEmptyAuthProvider verifies legacy password rows keep password auth.
func Test_REQ01_P_014C_AttachOIDCIdentityNormalizesEmptyAuthProvider(t *testing.T) {
	setupTestDB(t)
	user := createTestUser(t, "legacy-auth-provider@example.com", "password123")
	if err := db.GetDB().Model(&user).Update("auth_provider", "").Error; err != nil {
		t.Fatalf("Failed to clear auth provider: %v", err)
	}
	user.AuthProvider = ""

	linked, err := attachOIDCIdentity(user, "https://issuer.example.com", auth.OIDCClaims{Subject: "subject-legacy"})
	if err != nil {
		t.Fatalf("attachOIDCIdentity returned error: %v", err)
	}
	if linked.AuthProvider != "password" {
		t.Fatalf("Expected empty auth provider to be normalized to password, got %q", linked.AuthProvider)
	}
}

// Test_REQ01_P_010_OIDCUtilities verifies OIDC helper behavior.
func Test_REQ01_P_010_OIDCUtilities(t *testing.T) {
	t.Setenv("APP_ENV", "test")

	if oidcUsername("https://issuer.example.com", auth.OIDCClaims{Email: "user@example.com", Subject: "subject"}) != "user@example.com" {
		t.Error("Expected email username")
	}
	hashedUsername := oidcUsername("https://issuer.example.com", auth.OIDCClaims{Subject: "subject"})
	if !strings.HasPrefix(hashedUsername, "oidc-") {
		t.Errorf("Expected hashed OIDC username, got %q", hashedUsername)
	}
	otherIssuerUsername := oidcUsername("https://other-issuer.example.com", auth.OIDCClaims{Subject: "subject"})
	if hashedUsername == otherIssuerUsername {
		t.Error("Expected same subject from different issuers to produce different usernames")
	}

	state, err := randomState()
	if err != nil {
		t.Fatalf("randomState returned error: %v", err)
	}
	if state == "" {
		t.Error("Expected random state")
	}
	originalReader := randomReader
	t.Cleanup(func() { randomReader = originalReader })
	randomReader = &shortReader{data: bytes.Repeat([]byte{1}, 32)}
	state, err = randomState()
	if err != nil {
		t.Fatalf("randomState returned error for short reads: %v", err)
	}
	if state == "" {
		t.Error("Expected random state from short reads")
	}
	randomReader = errorReader{}
	if _, err := randomState(); err == nil {
		t.Error("Expected randomState error")
	}
	cookieValue, err := encodeOIDCLoginState(oidcLoginState{
		State: "state",
		Nonce: "nonce",
	})
	if err != nil {
		t.Fatalf("encodeOIDCLoginState returned error: %v", err)
	}
	loginState, err := decodeOIDCLoginState(&http.Cookie{Value: cookieValue})
	if err != nil {
		t.Fatalf("decodeOIDCLoginState returned error: %v", err)
	}
	if loginState.State != "state" || loginState.Nonce != "nonce" {
		t.Fatalf("Unexpected OIDC login state: %+v", loginState)
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.Split(cookieValue, ".")[0])
	if err != nil {
		t.Fatalf("Decode OIDC state payload: %v", err)
	}
	if strings.Contains(string(payload), "code_verifier") || strings.Contains(string(payload), "code-verifier") {
		t.Fatalf("OIDC state cookie leaked code verifier: %s", payload)
	}
	if _, err := decodeOIDCLoginState(&http.Cookie{Value: "not-base64"}); err == nil {
		t.Error("Expected invalid signed state cookie format error")
	}
	parts := strings.Split(cookieValue, ".")
	payloadReplacement := "A"
	if parts[0][:1] == payloadReplacement {
		payloadReplacement = "B"
	}
	tamperedPayload := payloadReplacement + parts[0][1:] + "." + parts[1]
	if _, err := decodeOIDCLoginState(&http.Cookie{Value: tamperedPayload}); err == nil {
		t.Error("Expected tampered OIDC state payload error")
	}
	replacement := "A"
	if parts[1][:1] == replacement {
		replacement = "B"
	}
	tamperedSignature := parts[0] + "." + replacement + parts[1][1:]
	if _, err := decodeOIDCLoginState(&http.Cookie{Value: tamperedSignature}); err == nil {
		t.Error("Expected tampered OIDC state signature error")
	}
	invalidJSONValue := base64.RawURLEncoding.EncodeToString([]byte("{"))
	invalidJSONSignature, err := signOIDCLoginStatePayload(invalidJSONValue)
	if err != nil {
		t.Fatalf("signOIDCLoginStatePayload returned error: %v", err)
	}
	if _, err := decodeOIDCLoginState(&http.Cookie{Value: invalidJSONValue + "." + invalidJSONSignature}); err == nil {
		t.Error("Expected invalid JSON state cookie error")
	}
	t.Setenv("APP_ENV", "production")
	t.Setenv("OIDC_STATE_SECRET", "")
	t.Setenv("JWT_SECRET", "")
	if _, err := encodeOIDCLoginState(oidcLoginState{State: "state", Nonce: "nonce"}); err == nil {
		t.Error("Expected missing OIDC state signing secret error outside development")
	}
	t.Setenv("APP_ENV", "test")

	storeOIDCCodeVerifier("stored-state", "stored-verifier", time.Now().Add(time.Minute))
	if verifier, ok := takeOIDCCodeVerifier("stored-state"); !ok || verifier != "stored-verifier" {
		t.Fatalf("Expected stored OIDC code verifier, got %q ok=%v", verifier, ok)
	}
	if _, ok := takeOIDCCodeVerifier("stored-state"); ok {
		t.Error("Expected OIDC code verifier to be removed after read")
	}
	storeOIDCCodeVerifier("expired-state", "expired-verifier", time.Now().Add(-time.Minute))
	if _, ok := takeOIDCCodeVerifier("expired-state"); ok {
		t.Error("Expected expired OIDC code verifier to be rejected")
	}
	storeOIDCCodeVerifier("expired-cleanup-state", "expired-verifier", time.Now().Add(-time.Minute))
	storeOIDCCodeVerifier("active-state", "active-verifier", time.Now().Add(time.Minute))
	if _, ok := takeOIDCCodeVerifier("expired-cleanup-state"); ok {
		t.Error("Expected expired OIDC code verifier to be removed during cleanup")
	}
	if verifier, ok := takeOIDCCodeVerifier("active-state"); !ok || verifier != "active-verifier" {
		t.Fatalf("Expected active OIDC code verifier after cleanup, got %q ok=%v", verifier, ok)
	}
	resetOIDCCodeVerifierStore(t)
	t.Setenv("OIDC_CODE_VERIFIER_STORE_MAX_ENTRIES", "2")
	storeOIDCCodeVerifier("oldest-state", "oldest-verifier", time.Now().Add(time.Minute))
	storeOIDCCodeVerifier("middle-state", "middle-verifier", time.Now().Add(2*time.Minute))
	storeOIDCCodeVerifier("newest-state", "newest-verifier", time.Now().Add(3*time.Minute))
	oidcCodeVerifierStore.Lock()
	storeLen := len(oidcCodeVerifierStore.entries)
	oidcCodeVerifierStore.Unlock()
	if storeLen != 2 {
		t.Fatalf("Expected capped OIDC code verifier store length 2, got %d", storeLen)
	}
	if _, ok := takeOIDCCodeVerifier("oldest-state"); ok {
		t.Fatal("Expected oldest OIDC code verifier to be evicted")
	}
	if verifier, ok := takeOIDCCodeVerifier("newest-state"); !ok || verifier != "newest-verifier" {
		t.Fatalf("Expected non-evicted OIDC code verifier, got %q ok=%v", verifier, ok)
	}

	t.Setenv("OIDC_COOKIE_SECURE", "")
	if !oidcCookieSecure() {
		t.Error("Expected OIDC cookie to be secure by default")
	}
	t.Setenv("OIDC_COOKIE_SECURE", "false")
	if oidcCookieSecure() {
		t.Error("Expected OIDC cookie secure override to be false")
	}
	t.Setenv("OIDC_COOKIE_SECURE", " false ")
	if oidcCookieSecure() {
		t.Error("Expected trimmed OIDC cookie secure override to be false")
	}
	t.Setenv("OIDC_COOKIE_SECURE", " true ")
	if !oidcCookieSecure() {
		t.Error("Expected trimmed OIDC cookie secure override to be true")
	}
	t.Setenv("OIDC_COOKIE_SECURE", "invalid")
	if !oidcCookieSecure() {
		t.Error("Expected invalid OIDC cookie secure override to fall back to true")
	}

	if stringPointerOrNil("") != nil {
		t.Error("Expected nil pointer for empty string")
	}
	if got := stringPointerOrNil("value"); got == nil || *got != "value" {
		t.Error("Expected pointer for non-empty string")
	}
}

// Test_REQ01_E_007_UpsertOIDCUserDBError verifies OIDC upsert handles DB errors.
func Test_REQ01_E_007_UpsertOIDCUserDBError(t *testing.T) {
	setupTestDB(t)
	sqlDB, err := db.GetDB().DB()
	if err != nil {
		t.Fatalf("DB returned error: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	_, err = upsertOIDCUser("https://issuer.example.com", auth.OIDCClaims{Subject: "subject"})
	if err == nil {
		t.Error("Expected DB error")
	}
}

// Test_REQ01_E_008_AttachOIDCIdentityDBError verifies OIDC attach handles DB errors.
func Test_REQ01_E_008_AttachOIDCIdentityDBError(t *testing.T) {
	setupTestDB(t)
	user := createTestUser(t, "attach@example.com", "password123")
	sqlDB, err := db.GetDB().DB()
	if err != nil {
		t.Fatalf("DB returned error: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	_, err = attachOIDCIdentity(user, "https://issuer.example.com", auth.OIDCClaims{Subject: "subject"})
	if err == nil {
		t.Error("Expected DB error")
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, errors.New("random error")
}

type failingRandomReader struct {
	successfulReads int
	reads           int
}

func (r *failingRandomReader) Read(p []byte) (int, error) {
	if r.reads >= r.successfulReads {
		return 0, errors.New("random error")
	}
	r.reads++
	for i := range p {
		p[i] = 1
	}
	return len(p), nil
}

type shortReader struct {
	data []byte
}

func (r *shortReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := 3
	if len(r.data) < n {
		n = len(r.data)
	}
	if len(p) < n {
		n = len(p)
	}
	copy(p, r.data[:n])
	r.data = r.data[n:]
	return n, nil
}

func setTestOIDCEnv(t *testing.T, issuer string) {
	t.Helper()
	t.Setenv("APP_ENV", "test")
	t.Setenv("OIDC_ISSUER_URL", issuer)
	t.Setenv("OIDC_CLIENT_ID", "client-id")
	t.Setenv("OIDC_CLIENT_SECRET", "client-secret")
	t.Setenv("OIDC_REDIRECT_URL", "https://app.example.com/callback")
}

func testOIDCStateCookie(t *testing.T, state, nonce string) *http.Cookie {
	t.Helper()
	t.Setenv("APP_ENV", "test")
	value, err := encodeOIDCLoginState(oidcLoginState{
		State: state,
		Nonce: nonce,
	})
	if err != nil {
		t.Fatalf("encode OIDC login state: %v", err)
	}
	storeOIDCCodeVerifier(state, "test-code-verifier", time.Now().Add(oidcLoginStateTTL))
	return &http.Cookie{Name: "oidc_state", Value: value}
}

func resetOIDCCodeVerifierStore(t *testing.T) {
	t.Helper()
	oidcCodeVerifierStore.Lock()
	defer oidcCodeVerifierStore.Unlock()
	oidcCodeVerifierStore.entries = make(map[string]oidcCodeVerifierEntry)
}

type testOIDCProvider struct {
	key         *rsa.PrivateKey
	tokenStatus int
	tokenBody   func() string
}

func installTestOIDCProvider(t *testing.T, provider testOIDCProvider) (string, *http.Client) {
	t.Helper()
	issuer := "https://issuer.example.com/" + strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		status := http.StatusOK
		body := ""
		switch {
		case strings.HasSuffix(req.URL.Path, "/.well-known/openid-configuration"):
			body = `{
				"issuer":"` + issuer + `",
				"authorization_endpoint":"` + issuer + `/auth",
				"token_endpoint":"` + issuer + `/token",
				"jwks_uri":"` + issuer + `/keys",
				"id_token_signing_alg_values_supported":["RS256"]
			}`
		case strings.HasSuffix(req.URL.Path, "/token"):
			status = provider.tokenStatus
			if status == 0 {
				status = http.StatusOK
			}
			if provider.tokenBody == nil {
				body = `{"access_token":"access","token_type":"Bearer"}`
			} else {
				body = provider.tokenBody()
			}
		case strings.HasSuffix(req.URL.Path, "/keys"):
			body = `{"keys":[` + jwkForKey(provider.key) + `]}`
		default:
			status = http.StatusNotFound
			body = `{}`
		}
		return jsonResponse(status, body), nil
	})}
	return issuer, client
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func mustGenerateRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return key
}

func mustSignIDToken(t *testing.T, key *rsa.PrivateKey, issuer, audience, subject, email string, nonce ...string) string {
	t.Helper()
	tokenNonce := "nonce"
	if len(nonce) > 0 {
		tokenNonce = nonce[0]
	}
	return mustSignIDTokenClaims(t, key, jwt.MapClaims{
		"iss":            issuer,
		"aud":            audience,
		"sub":            subject,
		"email":          email,
		"email_verified": true,
		"nonce":          tokenNonce,
		"exp":            time.Now().Add(time.Hour).Unix(),
		"iat":            time.Now().Unix(),
	})
}

func mustSignIDTokenClaims(t *testing.T, key *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "test-key"
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}
	return signed
}

func jwkForKey(key *rsa.PrivateKey) string {
	return `{
		"kty":"RSA",
		"use":"sig",
		"kid":"test-key",
		"alg":"RS256",
		"n":"` + base64.RawURLEncoding.EncodeToString(key.N.Bytes()) + `",
		"e":"` + base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()) + `"
	}`
}
