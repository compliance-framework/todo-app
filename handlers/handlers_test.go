package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ContainerSolutions/todo-app/auth"
	"github.com/ContainerSolutions/todo-app/db"
	"github.com/ContainerSolutions/todo-app/models"
	"github.com/gin-gonic/gin"
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

// Test_REQ01_P_001_LoginSuccess verifies user can login with valid credentials
func Test_REQ01_P_001_LoginSuccess(t *testing.T) {
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

	req, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response LoginResponse
	json.Unmarshal(w.Body.Bytes(), &response)

	if response.Token == "" {
		t.Error("Expected token in response")
	}
}

// Test_REQ01_P_002_RegisterSuccess verifies user can register a new account
func Test_REQ01_P_002_RegisterSuccess(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	registerReq := RegisterRequest{
		Username: "newuser",
		Password: "password123",
	}
	body, _ := json.Marshal(registerReq)

	req, _ := http.NewRequest("POST", "/api/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}
}

// Test_REQ01_N_001_LoginInvalidPassword verifies login fails with wrong password
func Test_REQ01_N_001_LoginInvalidPassword(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	createTestUser(t, "testuser", "password123")

	loginReq := LoginRequest{
		Username: "testuser",
		Password: "wrongpassword",
	}
	body, _ := json.Marshal(loginReq)

	req, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// Test_REQ01_N_002_LoginNonexistentUser verifies login fails for non-existent user
func Test_REQ01_N_002_LoginNonexistentUser(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	loginReq := LoginRequest{
		Username: "nonexistent",
		Password: "password123",
	}
	body, _ := json.Marshal(loginReq)

	req, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// Test_REQ01_N_003_RegisterDuplicateUsername verifies registration fails for duplicate username
func Test_REQ01_N_003_RegisterDuplicateUsername(t *testing.T) {
	setupTestDB(t)
	router := setupRouter()

	createTestUser(t, "existinguser", "password123")

	registerReq := RegisterRequest{
		Username: "existinguser",
		Password: "password123",
	}
	body, _ := json.Marshal(registerReq)

	req, _ := http.NewRequest("POST", "/api/register", bytes.NewBuffer(body))
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

	req, _ := http.NewRequest("POST", "/api/todos", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var todo models.Todo
	json.Unmarshal(w.Body.Bytes(), &todo)

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

	req, _ := http.NewRequest("POST", "/api/todos", bytes.NewBuffer(body))
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

	req, _ := http.NewRequest("POST", "/api/todos", bytes.NewBuffer(body))
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

	req, _ := http.NewRequest("GET", "/api/todos", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var todos []models.Todo
	json.Unmarshal(w.Body.Bytes(), &todos)

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

	req, _ := http.NewRequest("GET", "/api/todos/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var returnedTodo models.Todo
	json.Unmarshal(w.Body.Bytes(), &returnedTodo)

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

	req, _ := http.NewRequest("GET", "/api/todos", nil)
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

	req, _ := http.NewRequest("GET", "/api/todos/999", nil)
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

	req, _ := http.NewRequest("GET", "/api/todos", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var todos []models.Todo
	json.Unmarshal(w.Body.Bytes(), &todos)

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

	req, _ := http.NewRequest("PUT", "/api/todos/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var updatedTodo models.Todo
	json.Unmarshal(w.Body.Bytes(), &updatedTodo)

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

	req, _ := http.NewRequest("DELETE", "/api/todos/1", nil)
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

	req, _ := http.NewRequest("PUT", "/api/todos/1", bytes.NewBuffer(body))
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
	req, _ := http.NewRequest("DELETE", "/api/todos/1", nil)
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

	req, _ := http.NewRequest("PUT", "/api/todos/1", bytes.NewBuffer(body))
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

	req, _ := http.NewRequest("DELETE", "/api/todos/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}
