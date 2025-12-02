package auth

import (
	"testing"
	"time"

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
