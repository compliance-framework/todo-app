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
	"strings"
	"testing"
	"time"

	"github.com/ContainerSolutions/todo-app/auth"
	"github.com/ContainerSolutions/todo-app/db"
	"github.com/ContainerSolutions/todo-app/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
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
		Username: username,
		Password: hashedPassword,
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

// Test_REQ01_P_011_OIDCLoginRedirect verifies OIDC login redirects to provider.
func Test_REQ01_P_011_OIDCLoginRedirect(t *testing.T) {
	issuer := installTestOIDCProvider(t, testOIDCProvider{
		key: mustGenerateRSAKey(t),
	})
	t.Setenv("OIDC_ISSUER_URL", issuer)
	t.Setenv("OIDC_CLIENT_ID", "client-id")
	t.Setenv("OIDC_CLIENT_SECRET", "client-secret")
	t.Setenv("OIDC_REDIRECT_URL", "https://app.example.com/callback")

	router := gin.New()
	router.GET("/api/auth/oidc/login", OIDCLogin)

	req := mustNewRequest(t, "GET", "/api/auth/oidc/login", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("Expected status %d, got %d", http.StatusFound, w.Code)
	}
	if !strings.HasPrefix(w.Header().Get("Location"), issuer+"/auth") {
		t.Errorf("Expected redirect to provider auth endpoint, got %q", w.Header().Get("Location"))
	}
	result := w.Result()
	defer result.Body.Close()
	if len(result.Cookies()) == 0 {
		t.Error("Expected OIDC state cookie")
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

// Test_REQ01_N_016_OIDCCallbackMissingCode verifies callback requires an auth code.
func Test_REQ01_N_016_OIDCCallbackMissingCode(t *testing.T) {
	router := gin.New()
	router.GET("/api/auth/oidc/callback", OIDCCallback)

	req := mustNewRequest(t, "GET", "/api/auth/oidc/callback?state=state", nil)
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "state"})
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
	issuer := installTestOIDCProvider(t, testOIDCProvider{
		key:         mustGenerateRSAKey(t),
		tokenStatus: http.StatusInternalServerError,
		tokenBody:   func() string { return `{"error":"server_error"}` },
	})
	setTestOIDCEnv(t, issuer)

	router := gin.New()
	router.GET("/api/auth/oidc/callback", OIDCCallback)

	req := mustNewRequest(t, "GET", "/api/auth/oidc/callback?state=state&code=code", nil)
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "state"})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// Test_REQ01_N_018_OIDCCallbackMissingIDToken verifies callback requires an ID token.
func Test_REQ01_N_018_OIDCCallbackMissingIDToken(t *testing.T) {
	issuer := installTestOIDCProvider(t, testOIDCProvider{
		key:       mustGenerateRSAKey(t),
		tokenBody: func() string { return `{"access_token":"access","token_type":"Bearer"}` },
	})
	setTestOIDCEnv(t, issuer)

	router := gin.New()
	router.GET("/api/auth/oidc/callback", OIDCCallback)

	req := mustNewRequest(t, "GET", "/api/auth/oidc/callback?state=state&code=code", nil)
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "state"})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// Test_REQ01_N_019_OIDCCallbackInvalidIDToken verifies callback verifies ID tokens.
func Test_REQ01_N_019_OIDCCallbackInvalidIDToken(t *testing.T) {
	issuer := installTestOIDCProvider(t, testOIDCProvider{
		key:       mustGenerateRSAKey(t),
		tokenBody: func() string { return `{"access_token":"access","token_type":"Bearer","id_token":"invalid"}` },
	})
	setTestOIDCEnv(t, issuer)

	router := gin.New()
	router.GET("/api/auth/oidc/callback", OIDCCallback)

	req := mustNewRequest(t, "GET", "/api/auth/oidc/callback?state=state&code=code", nil)
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "state"})
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
	issuer := installTestOIDCProvider(t, testOIDCProvider{
		key: key,
		tokenBody: func() string {
			return `{"access_token":"access","token_type":"Bearer","id_token":"` +
				mustSignIDToken(t, key, "https://issuer.example.com", "client-id", "subject-success", "success@example.com") + `"}`
		},
	})
	setTestOIDCEnv(t, issuer)

	router := gin.New()
	router.GET("/api/auth/oidc/callback", OIDCCallback)

	req := mustNewRequest(t, "GET", "/api/auth/oidc/callback?state=state&code=code", nil)
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "state"})
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
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "state"})
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
	issuer := installTestOIDCProvider(t, testOIDCProvider{
		key: key,
		tokenBody: func() string {
			return `{"access_token":"access","token_type":"Bearer","id_token":"` +
				mustSignIDTokenClaims(t, key, jwt.MapClaims{
					"iss":   "https://issuer.example.com",
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

	req := mustNewRequest(t, "GET", "/api/auth/oidc/callback?state=state&code=code", nil)
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "state"})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
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
	issuer := installTestOIDCProvider(t, testOIDCProvider{
		key: key,
		tokenBody: func() string {
			return `{"access_token":"access","token_type":"Bearer","id_token":"` +
				mustSignIDToken(t, key, "https://issuer.example.com", "client-id", "subject-token-error", "token-error@example.com") + `"}`
		},
	})
	setTestOIDCEnv(t, issuer)

	router := gin.New()
	router.GET("/api/auth/oidc/callback", OIDCCallback)

	req := mustNewRequest(t, "GET", "/api/auth/oidc/callback?state=state&code=code", nil)
	req.AddCookie(&http.Cookie{Name: "oidc_state", Value: "state"})
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

	sameUser, err := upsertOIDCUser("https://issuer.example.com", claims)
	if err != nil {
		t.Fatalf("upsertOIDCUser returned error on existing user: %v", err)
	}
	if sameUser.ID != user.ID {
		t.Errorf("Expected existing user ID %d, got %d", user.ID, sameUser.ID)
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
	if user.Email == nil || *user.Email != "existing@example.com" {
		t.Fatalf("Expected normalized email, got %+v", user.Email)
	}
}

// Test_REQ01_P_011_UpsertOIDCUserDoesNotAttachUnverifiedEmail verifies unverified emails do not link accounts.
func Test_REQ01_P_011_UpsertOIDCUserDoesNotAttachUnverifiedEmail(t *testing.T) {
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
	if user.Email == nil || *user.Email != "existing@example.com" {
		t.Fatalf("Expected normalized email on new OIDC user, got %+v", user.Email)
	}

	var reloadedExisting models.User
	if err := db.GetDB().First(&reloadedExisting, existing.ID).Error; err != nil {
		t.Fatalf("Failed to reload existing user: %v", err)
	}
	if reloadedExisting.OIDCIssuer != nil || reloadedExisting.OIDCSubject != nil {
		t.Fatalf("Expected existing user to remain unlinked, got %+v", reloadedExisting)
	}
}

// Test_REQ01_P_012_AttachOIDCIdentityRefusesRelink verifies linked accounts cannot be re-linked.
func Test_REQ01_P_012_AttachOIDCIdentityRefusesRelink(t *testing.T) {
	setupTestDB(t)
	user := createTestUser(t, "linked@example.com", "password123")

	linked, err := attachOIDCIdentity(user, "https://issuer.example.com", auth.OIDCClaims{Subject: "subject-1"})
	if err != nil {
		t.Fatalf("attachOIDCIdentity returned error: %v", err)
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

// Test_REQ01_P_010_OIDCUtilities verifies OIDC helper behavior.
func Test_REQ01_P_010_OIDCUtilities(t *testing.T) {
	if oidcUsername(auth.OIDCClaims{Email: "user@example.com", Subject: "subject"}) != "user@example.com" {
		t.Error("Expected email username")
	}
	hashedUsername := oidcUsername(auth.OIDCClaims{Subject: "subject"})
	if !strings.HasPrefix(hashedUsername, "oidc-") {
		t.Errorf("Expected hashed OIDC username, got %q", hashedUsername)
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

	t.Setenv("OIDC_COOKIE_SECURE", "")
	if !oidcCookieSecure() {
		t.Error("Expected OIDC cookie to be secure by default")
	}
	t.Setenv("OIDC_COOKIE_SECURE", "false")
	if oidcCookieSecure() {
		t.Error("Expected OIDC cookie secure override to be false")
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
	t.Setenv("OIDC_ISSUER_URL", issuer)
	t.Setenv("OIDC_CLIENT_ID", "client-id")
	t.Setenv("OIDC_CLIENT_SECRET", "client-secret")
	t.Setenv("OIDC_REDIRECT_URL", "https://app.example.com/callback")
}

type testOIDCProvider struct {
	key         *rsa.PrivateKey
	tokenStatus int
	tokenBody   func() string
}

func installTestOIDCProvider(t *testing.T, provider testOIDCProvider) string {
	t.Helper()
	issuer := "https://issuer.example.com"
	oldTransport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		status := http.StatusOK
		body := ""
		switch req.URL.Path {
		case "/.well-known/openid-configuration":
			body = `{
				"issuer":"` + issuer + `",
				"authorization_endpoint":"` + issuer + `/auth",
				"token_endpoint":"` + issuer + `/token",
				"jwks_uri":"` + issuer + `/keys",
				"id_token_signing_alg_values_supported":["RS256"]
			}`
		case "/token":
			status = provider.tokenStatus
			if status == 0 {
				status = http.StatusOK
			}
			if provider.tokenBody == nil {
				body = `{"access_token":"access","token_type":"Bearer"}`
			} else {
				body = provider.tokenBody()
			}
		case "/keys":
			body = `{"keys":[` + jwkForKey(provider.key) + `]}`
		default:
			status = http.StatusNotFound
			body = `{}`
		}
		return jsonResponse(status, body), nil
	})
	t.Cleanup(func() {
		http.DefaultTransport = oldTransport
	})
	return issuer
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

func mustSignIDToken(t *testing.T, key *rsa.PrivateKey, issuer, audience, subject, email string) string {
	t.Helper()
	return mustSignIDTokenClaims(t, key, jwt.MapClaims{
		"iss":            issuer,
		"aud":            audience,
		"sub":            subject,
		"email":          email,
		"email_verified": true,
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
