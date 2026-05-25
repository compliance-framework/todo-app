package auth

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const devJWTSecret = "todo-app-dev-secret" // #nosec G101 -- local development fallback only; production requires JWT_SECRET.

var jwtSecret = []byte(devJWTSecret)

// Function variables for testing (allows mocking)
var (
	HashPasswordFunc  = hashPasswordImpl
	GenerateTokenFunc = generateTokenImpl
)

// Claims represents the JWT claims
type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// hashPasswordImpl is the actual implementation of password hashing
func hashPasswordImpl(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// HashPassword hashes a password using bcrypt (uses HashPasswordFunc for testability)
func HashPassword(password string) (string, error) {
	return HashPasswordFunc(password)
}

// CheckPasswordHash compares a password with a hash
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// generateTokenImpl is the actual implementation of token generation
func generateTokenImpl(userID uint, username string) (string, error) {
	claims := &Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// GenerateToken generates a JWT token for a user (uses GenerateTokenFunc for testability)
func GenerateToken(userID uint, username string) (string, error) {
	return GenerateTokenFunc(userID, username)
}

// ConfigureJWTSecretFromEnv reads the JWT signing secret from JWT_SECRET.
func ConfigureJWTSecretFromEnv() error {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret != "" {
		jwtSecret = []byte(secret)
		return nil
	}
	if isDevelopmentMode() {
		jwtSecret = []byte(devJWTSecret)
		return nil
	}
	return errors.New("JWT_SECRET is required outside development mode")
}

// CookieSigningSecretFromEnv reads a cookie signing secret with a JWT_SECRET fallback.
func CookieSigningSecretFromEnv(primaryKey string) ([]byte, error) {
	secret := strings.TrimSpace(os.Getenv(primaryKey))
	if secret != "" {
		return []byte(secret), nil
	}
	secret = strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret != "" {
		return []byte(secret), nil
	}
	if isDevelopmentMode() {
		return []byte(devJWTSecret), nil
	}
	return nil, errors.New(primaryKey + " or JWT_SECRET is required outside development mode")
}

// ValidateToken validates a JWT token and returns the claims
func ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	return claims, nil
}

// AuthMiddleware is a Gin middleware that validates JWT tokens
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		claims, err := ValidateToken(parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Set user info in context
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Next()
	}
}

// GetUserIDFromContext extracts the user ID from the Gin context
func GetUserIDFromContext(c *gin.Context) (uint, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}
	uintUser, ok := userID.(uint)
	if !ok {
		return 0, false
	}
	return uintUser, true
}

func isDevelopmentMode() bool {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	if env == "" {
		env = strings.ToLower(strings.TrimSpace(os.Getenv("ENV")))
	}
	if env == "" {
		env = strings.ToLower(strings.TrimSpace(os.Getenv("GIN_MODE")))
	}

	switch env {
	case "debug", "dev", "development", "local", "test":
		return true
	default:
		return false
	}
}
