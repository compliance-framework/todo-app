package auth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
)

func resetOIDCProviderCacheForTest() {
	oidcProviderCache.Lock()
	defer oidcProviderCache.Unlock()
	oidcProviderCache.entries = make(map[oidcProviderCacheKey]oidcProviderCacheEntry)
	oidcProviderCache.calls = make(map[oidcProviderCacheKey]*oidcProviderCacheCall)
}

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

	req, _ := http.NewRequestWithContext(t.Context(), "GET", "/protected", nil)
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

	req, _ := http.NewRequestWithContext(t.Context(), "GET", "/protected", nil)
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

	req, _ := http.NewRequestWithContext(t.Context(), "GET", "/protected", nil)
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

	req, _ := http.NewRequestWithContext(t.Context(), "GET", "/protected", nil)
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

	req, _ := http.NewRequestWithContext(t.Context(), "GET", "/protected", nil)
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

// Test_REQ01_N_008_GetUserIDFromContextInvalidType verifies invalid user ID type returns false
func Test_REQ01_N_008_GetUserIDFromContextInvalidType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("user_id", "abc")

	_, ok := GetUserIDFromContext(c)
	if ok {
		t.Error("Expected ok to be false when user_id not set")
	}
}

func Test_Auth_P_001_ConfigureJWTSecretFromEnv(t *testing.T) {
	originalSecret := jwtSecret
	defer func() { jwtSecret = originalSecret }()

	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("APP_ENV", "production")

	if err := ConfigureJWTSecretFromEnv(); err != nil {
		t.Fatalf("ConfigureJWTSecretFromEnv returned error: %v", err)
	}
	if string(jwtSecret) != "test-secret" {
		t.Errorf("Expected JWT secret from env, got %q", string(jwtSecret))
	}
}

func Test_Auth_P_002_ConfigureJWTSecretDevelopmentFallback(t *testing.T) {
	originalSecret := jwtSecret
	defer func() { jwtSecret = originalSecret }()

	t.Setenv("JWT_SECRET", "")
	t.Setenv("APP_ENV", "development")

	if err := ConfigureJWTSecretFromEnv(); err != nil {
		t.Fatalf("ConfigureJWTSecretFromEnv returned error: %v", err)
	}
	if string(jwtSecret) != devJWTSecret {
		t.Errorf("Expected development JWT secret, got %q", string(jwtSecret))
	}
}

func Test_Auth_N_001_ConfigureJWTSecretProductionMissing(t *testing.T) {
	originalSecret := jwtSecret
	defer func() { jwtSecret = originalSecret }()

	t.Setenv("JWT_SECRET", "")
	t.Setenv("APP_ENV", "production")
	t.Setenv("ENV", "")
	t.Setenv("GIN_MODE", "")

	if err := ConfigureJWTSecretFromEnv(); err == nil {
		t.Error("Expected error when JWT_SECRET is missing outside development")
	}
}

func Test_Auth_P_003_IsDevelopmentModeFallbacks(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("ENV", "local")
	t.Setenv("GIN_MODE", "")
	if !isDevelopmentMode() {
		t.Error("Expected ENV=local to be development mode")
	}

	t.Setenv("ENV", "")
	t.Setenv("GIN_MODE", "release")
	if isDevelopmentMode() {
		t.Error("Expected GIN_MODE=release to be non-development mode")
	}

	t.Setenv("APP_ENV", "")
	t.Setenv("ENV", "")
	t.Setenv("GIN_MODE", "")
	if !isDevelopmentMode() {
		t.Error("Expected empty env to default to development mode")
	}
}

func Test_Auth_P_004_OIDCConfigFromEnv(t *testing.T) {
	t.Setenv("OIDC_ISSUER_URL", "https://issuer.example.com")
	t.Setenv("OIDC_CLIENT_ID", "client-id")
	t.Setenv("OIDC_CLIENT_SECRET", "client-secret")
	t.Setenv("OIDC_REDIRECT_URL", "https://app.example.com/callback")

	cfg := OIDCConfigFromEnv()
	if !cfg.Configured() {
		t.Error("Expected OIDC config to be configured")
	}
	if !IsOIDCConfigured() {
		t.Error("Expected IsOIDCConfigured to return true")
	}
}

func Test_Auth_N_002_OIDCConfigOAuth2ConfigUnconfigured(t *testing.T) {
	_, _, err := (OIDCConfig{}).OAuth2Config(t.Context())
	if err == nil {
		t.Error("Expected error for unconfigured OIDC config")
	}
}

func Test_Auth_P_005_OIDCConfigOAuth2ConfigSuccess(t *testing.T) {
	resetOIDCProviderCacheForTest()
	t.Cleanup(resetOIDCProviderCacheForTest)

	issuer := "https://issuer.example.com"
	client := &http.Client{Transport: authRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/.well-known/openid-configuration" {
			return authJSONResponse(http.StatusNotFound, "{}"), nil
		}
		return authJSONResponse(http.StatusOK, `{
			"issuer":"`+issuer+`",
			"authorization_endpoint":"`+issuer+`/auth",
			"token_endpoint":"`+issuer+`/token",
			"jwks_uri":"`+issuer+`/keys",
			"id_token_signing_alg_values_supported":["RS256"]
		}`), nil
	})}
	ctx := oidc.ClientContext(t.Context(), client)
	ctx = context.WithValue(ctx, oauth2.HTTPClient, client)

	oauthConfig, verifier, err := (OIDCConfig{
		IssuerURL:    issuer,
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "https://app.example.com/callback",
	}).OAuth2Config(ctx)
	if err != nil {
		t.Fatalf("OAuth2Config returned error: %v", err)
	}
	if oauthConfig.Endpoint.AuthURL != issuer+"/auth" || oauthConfig.Endpoint.TokenURL != issuer+"/token" {
		t.Fatalf("Unexpected OAuth2 endpoints: %+v", oauthConfig.Endpoint)
	}
	if verifier == nil {
		t.Error("Expected ID token verifier")
	}
}

func Test_Auth_P_006_OIDCConfigOAuth2ConfigConcurrentDiscoverySingleflight(t *testing.T) {
	resetOIDCProviderCacheForTest()
	t.Cleanup(resetOIDCProviderCacheForTest)

	issuer := "https://issuer.example.com"
	var discoveryRequests atomic.Int32
	client := &http.Client{Transport: authRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/.well-known/openid-configuration" {
			return authJSONResponse(http.StatusNotFound, "{}"), nil
		}
		discoveryRequests.Add(1)
		time.Sleep(50 * time.Millisecond)
		return authJSONResponse(http.StatusOK, `{
			"issuer":"`+issuer+`",
			"authorization_endpoint":"`+issuer+`/auth",
			"token_endpoint":"`+issuer+`/token",
			"jwks_uri":"`+issuer+`/keys",
			"id_token_signing_alg_values_supported":["RS256"]
		}`), nil
	})}
	ctx := oidc.ClientContext(t.Context(), client)
	ctx = context.WithValue(ctx, oauth2.HTTPClient, client)

	cfg := OIDCConfig{
		IssuerURL:    issuer,
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RedirectURL:  "https://app.example.com/callback",
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			oauthConfig, verifier, err := cfg.OAuth2Config(ctx)
			if err != nil {
				errs <- err
				return
			}
			if oauthConfig.Endpoint.AuthURL != issuer+"/auth" || verifier == nil {
				errs <- errors.New("unexpected OAuth2Config result")
			}
		}()
	}

	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("OAuth2Config returned error: %v", err)
		}
	}
	if got := discoveryRequests.Load(); got != 1 {
		t.Fatalf("Expected one provider discovery request, got %d", got)
	}
}

type authRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn authRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func authJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
