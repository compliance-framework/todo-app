package handlers

import (
	"net/http"
	"strconv"

	"github.com/ContainerSolutions/todo-app/auth"
	"github.com/ContainerSolutions/todo-app/db"
	"github.com/ContainerSolutions/todo-app/models"
	"github.com/gin-gonic/gin"
)

// CreateTodoRequest represents the request body for creating a todo
type CreateTodoRequest struct {
	Title       string `json:"title" binding:"required,min=1,max=255"`
	Description string `json:"description" binding:"max=1000"`
}

// UpdateTodoRequest represents the request body for updating a todo
type UpdateTodoRequest struct {
	Title       string `json:"title" binding:"omitempty,min=1,max=255"`
	Description string `json:"description" binding:"omitempty,max=1000"`
	Completed   *bool  `json:"completed"`
}

// ListTodos returns all todos
// REQ03: Users should be able to see all todo lists
func ListTodos(c *gin.Context) {
	var todos []models.Todo
	if err := db.GetDB().Preload("User").Find(&todos).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch todos"})
		return
	}

	c.JSON(http.StatusOK, todos)
}

// GetTodo returns a single todo by ID
// REQ03: Users should be able to see all todo lists
func GetTodo(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid todo ID"})
		return
	}

	var todo models.Todo
	if err := db.GetDB().Preload("User").First(&todo, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}

	c.JSON(http.StatusOK, todo)
}

// CreateTodo creates a new todo
// REQ02: Users should be able to create new TODOs
func CreateTodo(c *gin.Context) {
	userID, ok := auth.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req CreateTodoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	todo := models.Todo{
		Title:       req.Title,
		Description: req.Description,
		UserID:      userID,
	}

	if err := db.GetDB().Create(&todo).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create todo"})
		return
	}

	// Reload with user info
	db.GetDB().Preload("User").First(&todo, todo.ID)

	c.JSON(http.StatusCreated, todo)
}

// UpdateTodo updates an existing todo
// REQ04: Users should NOT be able to modify TODOs they did not create
func UpdateTodo(c *gin.Context) {
	userID, ok := auth.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid todo ID"})
		return
	}

	var todo models.Todo
	if err := db.GetDB().First(&todo, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}

	// REQ04: Check ownership - users can only modify their own todos
	if todo.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only modify your own todos"})
		return
	}

	var req UpdateTodoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update fields if provided
	if req.Title != "" {
		todo.Title = req.Title
	}
	if req.Description != "" {
		todo.Description = req.Description
	}
	if req.Completed != nil {
		todo.Completed = *req.Completed
	}

	if err := db.GetDB().Save(&todo).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update todo"})
		return
	}

	// Reload with user info
	db.GetDB().Preload("User").First(&todo, todo.ID)

	c.JSON(http.StatusOK, todo)
}

// DeleteTodo deletes a todo
// REQ04: Users should NOT be able to delete TODOs they did not create
func DeleteTodo(c *gin.Context) {
	userID, ok := auth.GetUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid todo ID"})
		return
	}

	var todo models.Todo
	if err := db.GetDB().First(&todo, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}

	// REQ04: Check ownership - users can only delete their own todos
	if todo.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only delete your own todos"})
		return
	}

	if err := db.GetDB().Delete(&todo).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete todo"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Todo deleted successfully"})
}
