package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ContainerSolutions/todo-app/auth"
	"github.com/ContainerSolutions/todo-app/db"
	"github.com/ContainerSolutions/todo-app/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RegisterRequest represents the registration request body
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=255"`
	Password string `json:"password" binding:"required,min=6"`
}

// LoginRequest represents the login request body
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse represents the login response
type LoginResponse struct {
	Token string      `json:"token"`
	User  models.User `json:"user"`
}

// AuthConfigResponse exposes configured authentication options.
type AuthConfigResponse struct {
	OIDCConfigured bool `json:"oidc_configured"`
}

var randomReader = rand.Reader

// Register handles user registration
// REQ01: Users should be able to LOGIN (registration is prerequisite)
func Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user already exists
	var existingUser models.User
	if err := db.GetDB().Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
		return
	}

	// Hash password
	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Create user
	user := models.User{
		Username:     req.Username,
		Password:     hashedPassword,
		AuthProvider: "password",
	}

	if err := db.GetDB().Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "User created successfully", "user": user})
}

// Login handles user authentication
// REQ01: Users should be able to LOGIN
func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find user
	var user models.User
	if err := db.GetDB().Where("username = ?", req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Check password
	if !auth.CheckPasswordHash(req.Password, user.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Generate token
	token, err := auth.GenerateToken(user.ID, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		Token: token,
		User:  user,
	})
}

// AuthConfig returns public auth configuration for clients.
func AuthConfig(c *gin.Context) {
	c.JSON(http.StatusOK, AuthConfigResponse{
		OIDCConfigured: auth.IsOIDCConfigured(),
	})
}

// OIDCLogin redirects the user to the configured OIDC provider.
func OIDCLogin(c *gin.Context) {
	oauthConfig, _, err := auth.OIDCConfigFromEnv().OAuth2Config(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "OIDC login is not configured"})
		return
	}

	state, err := randomState()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start OIDC login"})
		return
	}

	http.SetCookie(c.Writer, &http.Cookie{ // #nosec G124 -- Secure is configurable for local HTTP development and defaults to true.
		Name:     "oidc_state",
		Value:    state,
		Path:     "/api/auth/oidc",
		MaxAge:   600,
		Expires:  time.Now().Add(10 * time.Minute),
		Secure:   oidcCookieSecure(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	c.Redirect(http.StatusFound, oauthConfig.AuthCodeURL(state))
}

// OIDCCallback handles the provider authorization-code callback.
func OIDCCallback(c *gin.Context) {
	stateCookie, err := c.Request.Cookie("oidc_state")
	if err != nil || stateCookie.Value == "" || stateCookie.Value != c.Query("state") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OIDC state"})
		return
	}
	clearOIDCStateCookie(c)

	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OIDC authorization code is required"})
		return
	}

	oidcConfig := auth.OIDCConfigFromEnv()
	oauthConfig, verifier, err := oidcConfig.OAuth2Config(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "OIDC login is not configured"})
		return
	}

	oauthToken, err := oauthConfig.Exchange(c.Request.Context(), code)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "OIDC token exchange failed"})
		return
	}

	rawIDToken, ok := oauthToken.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "OIDC ID token is missing"})
		return
	}

	idToken, err := verifier.Verify(c.Request.Context(), rawIDToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "OIDC ID token verification failed"})
		return
	}

	var claims auth.OIDCClaims
	if err := idToken.Claims(&claims); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "OIDC claims are invalid"})
		return
	}
	if claims.Subject == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "OIDC subject is missing"})
		return
	}

	user, err := upsertOIDCUser(oidcConfig.IssuerURL, claims)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upsert OIDC user"})
		return
	}

	token, err := auth.GenerateToken(user.ID, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		Token: token,
		User:  user,
	})
}

func upsertOIDCUser(issuer string, claims auth.OIDCClaims) (models.User, error) {
	var user models.User
	err := db.GetDB().
		Where("oidc_issuer = ? AND oidc_subject = ?", issuer, claims.Subject).
		First(&user).
		Error
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.User{}, err
	}

	claims.Email = strings.TrimSpace(claims.Email)
	verifiedEmail := ""
	if claims.EmailVerified {
		verifiedEmail = claims.Email
	}
	if verifiedEmail != "" {
		err = db.GetDB().Where("email = ?", verifiedEmail).First(&user).Error
		if err == nil {
			return attachOIDCIdentity(user, issuer, claims)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return models.User{}, err
		}

		err = db.GetDB().Where("username = ?", verifiedEmail).First(&user).Error
		if err == nil {
			if user.OIDCIssuer != nil || user.OIDCSubject != nil || (user.AuthProvider != "" && user.AuthProvider != "password") {
				return models.User{}, errors.New("username match is not an unlinked password user")
			}
			return attachOIDCIdentity(user, issuer, claims)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return models.User{}, err
		}
	}

	usernameClaims := claims
	if !claims.EmailVerified {
		usernameClaims.Email = ""
	}
	user = models.User{
		Username:     oidcUsername(issuer, usernameClaims),
		Password:     "OIDC_LOGIN_ONLY", // #nosec G101 -- non-secret sentinel; OIDC users do not use password login.
		Email:        stringPointerOrNil(verifiedEmail),
		OIDCIssuer:   &issuer,
		OIDCSubject:  &claims.Subject,
		AuthProvider: "oidc",
	}
	if err := db.GetDB().Create(&user).Error; err != nil {
		return models.User{}, err
	}
	return user, nil
}

func attachOIDCIdentity(user models.User, issuer string, claims auth.OIDCClaims) (models.User, error) {
	if user.OIDCIssuer != nil || user.OIDCSubject != nil {
		if user.OIDCIssuer == nil || user.OIDCSubject == nil || *user.OIDCIssuer != issuer || *user.OIDCSubject != claims.Subject {
			return models.User{}, errors.New("user is already linked to a different OIDC identity")
		}
	}

	user.OIDCIssuer = &issuer
	user.OIDCSubject = &claims.Subject
	user.AuthProvider = "oidc"
	claims.Email = strings.TrimSpace(claims.Email)
	if claims.EmailVerified && claims.Email != "" {
		user.Email = &claims.Email
	}
	if err := db.GetDB().Save(&user).Error; err != nil {
		return models.User{}, err
	}
	return user, nil
}

func oidcUsername(issuer string, claims auth.OIDCClaims) string {
	email := strings.TrimSpace(claims.Email)
	if email != "" && len(email) <= 255 {
		return email
	}

	hash := sha256.Sum256([]byte(issuer + "\x00" + claims.Subject))
	return "oidc-" + hex.EncodeToString(hash[:])[:32]
}

func randomState() (string, error) {
	bytes := make([]byte, 32)
	if _, err := io.ReadFull(randomReader, bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func clearOIDCStateCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{ // #nosec G124 -- Secure is configurable for local HTTP development and defaults to true.
		Name:     "oidc_state",
		Value:    "",
		Path:     "/api/auth/oidc",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		Secure:   oidcCookieSecure(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func oidcCookieSecure() bool {
	value := os.Getenv("OIDC_COOKIE_SECURE")
	if value == "" {
		return true
	}
	enabled, err := strconv.ParseBool(value)
	if err != nil {
		return true
	}
	return enabled
}

func stringPointerOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
