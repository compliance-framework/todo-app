package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Test_REQ01_P_001_HashAndCheckPassword verifies password hashing works correctly
func Test_REQ01_P_001_HashAndCheckPassword(t *testing.T) {
	password := "testpassword123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	if hash == password {
		t.Error("Hash should not equal plain password")
	}

	if !CheckPasswordHash(password, hash) {
		t.Error("Password check should return true for correct password")
	}
}

// Test_REQ01_N_001_CheckWrongPassword verifies wrong password fails check
func Test_REQ01_N_001_CheckWrongPassword(t *testing.T) {
	password := "testpassword123"
	wrongPassword := "wrongpassword"

	hash, _ := HashPassword(password)

	if CheckPasswordHash(wrongPassword, hash) {
		t.Error("Password check should return false for wrong password")
	}
}

// Test_REQ01_P_002_GenerateAndValidateToken verifies token generation and validation
func Test_REQ01_P_002_GenerateAndValidateToken(t *testing.T) {
	userID := uint(1)
	username := "testuser"

	token, err := GenerateToken(userID, username)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	if token == "" {
		t.Error("Token should not be empty")
	}

	claims, err := ValidateToken(token)
	if err != nil {
		t.Fatalf("Failed to validate token: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("Expected user ID %d, got %d", userID, claims.UserID)
	}

	if claims.Username != username {
		t.Errorf("Expected username %s, got %s", username, claims.Username)
	}
}

// Test_REQ01_N_002_ValidateInvalidToken verifies invalid token is rejected
func Test_REQ01_N_002_ValidateInvalidToken(t *testing.T) {
	_, err := ValidateToken("invalid.token.here")
	if err == nil {
		t.Error("Expected error for invalid token")
	}
}

// Test_REQ01_E_001_TokenExpiry verifies expired tokens are rejected
func Test_REQ01_E_001_TokenExpiry(t *testing.T) {
	// Create an expired token manually
	claims := &Claims{
		UserID:   1,
		Username: "testuser",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)), // Expired 1 hour ago
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString(jwtSecret)

	_, err := ValidateToken(tokenString)
	if err == nil {
		t.Error("Expected error for expired token")
	}
}

// =============================================================================
// AuthMiddleware Tests
// =============================================================================

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

// Test_REQ01_P_003_AuthMiddlewareValidToken verifies middleware passes valid token
func Test_REQ01_P_003_AuthMiddlewareValidToken(t *testing.T) {
	router := setupTestRouter()

	router.GET("/protected", AuthMiddleware(), func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		username, _ := c.Get("username")
		c.JSON(http.StatusOK, gin.H{"user_id": userID, "username": username})
	})

	token, _ := GenerateToken(1, "testuser")

	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}

// Test_REQ01_N_003_AuthMiddlewareNoHeader verifies middleware rejects missing header
func Test_REQ01_N_003_AuthMiddlewareNoHeader(t *testing.T) {
	router := setupTestRouter()

	router.GET("/protected", AuthMiddleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req, _ := http.NewRequest("GET", "/protected", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// Test_REQ01_N_004_AuthMiddlewareInvalidFormat verifies middleware rejects invalid format
func Test_REQ01_N_004_AuthMiddlewareInvalidFormat(t *testing.T) {
	router := setupTestRouter()

	router.GET("/protected", AuthMiddleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "InvalidFormat token123")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// Test_REQ01_N_005_AuthMiddlewareInvalidToken verifies middleware rejects invalid token
func Test_REQ01_N_005_AuthMiddlewareInvalidToken(t *testing.T) {
	router := setupTestRouter()

	router.GET("/protected", AuthMiddleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// Test_REQ01_N_006_AuthMiddlewareMalformedHeader verifies middleware rejects malformed header
func Test_REQ01_N_006_AuthMiddlewareMalformedHeader(t *testing.T) {
	router := setupTestRouter()

	router.GET("/protected", AuthMiddleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "BearerNoSpace")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

// =============================================================================
// GetUserIDFromContext Tests
// =============================================================================

// Test_REQ01_P_004_GetUserIDFromContextSuccess verifies user ID extraction works
func Test_REQ01_P_004_GetUserIDFromContextSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("user_id", uint(123))

	userID, ok := GetUserIDFromContext(c)
	if !ok {
		t.Error("Expected ok to be true")
	}
	if userID != 123 {
		t.Errorf("Expected user ID 123, got %d", userID)
	}
}

// Test_REQ01_N_007_GetUserIDFromContextMissing verifies missing user ID returns false
func Test_REQ01_N_007_GetUserIDFromContextMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	_, ok := GetUserIDFromContext(c)
	if ok {
		t.Error("Expected ok to be false when user_id not set")
	}
}
