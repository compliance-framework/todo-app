package handlers

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ContainerSolutions/todo-app/auth"
	"github.com/ContainerSolutions/todo-app/db"
	"github.com/ContainerSolutions/todo-app/models"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/oauth2"
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

type oidcLoginState struct {
	State string `json:"state"`
	Nonce string `json:"nonce"`
}

var randomReader = rand.Reader

const (
	oidcLoginStateTTL                      = 10 * time.Minute
	defaultOIDCCodeVerifierStoreMaxEntries = 1024
	maxOIDCEmailLength                     = 320
	maxOIDCUsernameLength                  = 255
)

type oidcCodeVerifierEntry struct {
	verifier  string
	expiresAt time.Time
}

var oidcCodeVerifierStore = struct {
	sync.Mutex
	entries map[string]oidcCodeVerifierEntry
}{
	entries: make(map[string]oidcCodeVerifierEntry),
}

var (
	errOIDCUserAlreadyLinked            = errors.New("user is already linked to a different OIDC identity")
	errOIDCEmailMatchAmbiguous          = errors.New("email match is ambiguous")
	errOIDCUsernameMatchAmbiguous       = errors.New("username match is ambiguous")
	errOIDCUsernameMatchNotPasswordUser = errors.New("username match is not an unlinked password user")
)

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
	codeVerifier, err := randomState()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start OIDC login"})
		return
	}
	nonce, err := randomState()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start OIDC login"})
		return
	}
	cookieValue, err := encodeOIDCLoginState(oidcLoginState{
		State: state,
		Nonce: nonce,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start OIDC login"})
		return
	}
	expiresAt := time.Now().Add(oidcLoginStateTTL)
	storeOIDCCodeVerifier(state, codeVerifier, expiresAt)

	http.SetCookie(c.Writer, &http.Cookie{ // #nosec G124 -- Secure is configurable for local HTTP development and defaults to true.
		Name:     "oidc_state",
		Value:    cookieValue,
		Path:     "/api/auth/oidc",
		MaxAge:   int(oidcLoginStateTTL / time.Second),
		Expires:  expiresAt,
		Secure:   oidcCookieSecure(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	c.Redirect(http.StatusFound, oauthConfig.AuthCodeURL(
		state,
		oauth2.SetAuthURLParam("code_challenge", pkceChallenge(codeVerifier)),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oidc.Nonce(nonce),
	))
}

// OIDCCallback handles the provider authorization-code callback.
func OIDCCallback(c *gin.Context) {
	stateCookie, err := c.Request.Cookie("oidc_state")
	loginState, stateErr := decodeOIDCLoginState(stateCookie)
	if err != nil || stateErr != nil || loginState.State == "" || loginState.Nonce == "" || loginState.State != c.Query("state") {
		if err == nil {
			clearOIDCStateCookie(c)
		}
		if stateErr == nil && loginState.State != "" {
			deleteOIDCCodeVerifier(loginState.State)
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OIDC state"})
		return
	}
	clearOIDCStateCookie(c)

	codeVerifier, ok := takeOIDCCodeVerifier(loginState.State)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OIDC state"})
		return
	}

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

	oauthToken, err := oauthConfig.Exchange(c.Request.Context(), code, oauth2.SetAuthURLParam("code_verifier", codeVerifier))
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
	if claims.Nonce != loginState.Nonce {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "OIDC nonce is invalid"})
		return
	}
	if claims.Subject == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "OIDC subject is missing"})
		return
	}

	user, err := upsertOIDCUser(oidcConfig.IssuerURL, claims)
	if err != nil {
		if errors.Is(err, errOIDCUserAlreadyLinked) || errors.Is(err, errOIDCEmailMatchAmbiguous) || errors.Is(err, errOIDCUsernameMatchAmbiguous) || errors.Is(err, errOIDCUsernameMatchNotPasswordUser) {
			c.JSON(http.StatusConflict, gin.H{"error": "OIDC account linking conflict"})
			return
		}
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
	err := findOIDCUser(issuer, claims.Subject, &user)
	if err == nil {
		verifiedEmail := verifiedOIDCEmail(claims)
		if verifiedEmail != "" && user.Email == nil {
			if err := db.GetDB().Model(&models.User{}).Where("id = ?", user.ID).Update("email", verifiedEmail).Error; err != nil {
				return models.User{}, err
			}
			if err := db.GetDB().First(&user, user.ID).Error; err != nil {
				return models.User{}, err
			}
		}
		return user, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.User{}, err
	}

	verifiedEmail := verifiedOIDCEmail(claims)
	if verifiedEmail != "" {
		err = db.GetDB().Where("email = ?", verifiedEmail).First(&user).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			user, err = findUniqueCaseInsensitiveEmailUser(verifiedEmail)
		}
		if err == nil {
			return attachOIDCIdentity(user, issuer, claims)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return models.User{}, err
		}

		if len(verifiedEmail) <= maxOIDCUsernameLength {
			err = db.GetDB().Where("username = ?", verifiedEmail).First(&user).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				user, err = findUniqueCaseInsensitiveUsernameUser(verifiedEmail)
			}
			if err == nil {
				if user.OIDCIssuer != nil || user.OIDCSubject != nil || (user.AuthProvider != "" && user.AuthProvider != "password") {
					return models.User{}, errOIDCUsernameMatchNotPasswordUser
				}
				return attachOIDCIdentity(user, issuer, claims)
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return models.User{}, err
			}
		}
	}

	passwordHash, err := oidcOnlyPasswordHash()
	if err != nil {
		return models.User{}, err
	}

	usernameClaims := claims
	usernameClaims.Email = verifiedEmail
	user = models.User{
		Username:     oidcUsername(issuer, usernameClaims),
		Password:     passwordHash,
		Email:        stringPointerOrNil(verifiedEmail),
		OIDCIssuer:   &issuer,
		OIDCSubject:  &claims.Subject,
		AuthProvider: "oidc",
	}
	if err := createOIDCUser(&user); err != nil {
		var existing models.User
		if lookupErr := findOIDCUser(issuer, claims.Subject, &existing); lookupErr == nil {
			return existing, nil
		}
		return models.User{}, err
	}
	return user, nil
}

func findUniqueCaseInsensitiveEmailUser(email string) (models.User, error) {
	var users []models.User
	if err := db.GetDB().Where("LOWER(email) = ?", email).Order("id").Limit(2).Find(&users).Error; err != nil {
		return models.User{}, err
	}
	if len(users) == 0 {
		return models.User{}, gorm.ErrRecordNotFound
	}
	if len(users) > 1 {
		return models.User{}, errOIDCEmailMatchAmbiguous
	}
	return users[0], nil
}

func findUniqueCaseInsensitiveUsernameUser(username string) (models.User, error) {
	var users []models.User
	if err := db.GetDB().Where("LOWER(username) = ?", username).Order("id").Limit(2).Find(&users).Error; err != nil {
		return models.User{}, err
	}
	if len(users) == 0 {
		return models.User{}, gorm.ErrRecordNotFound
	}
	if len(users) > 1 {
		return models.User{}, errOIDCUsernameMatchAmbiguous
	}
	return users[0], nil
}

func oidcOnlyPasswordHash() (string, error) {
	randomPassword := make([]byte, 32)
	if _, err := rand.Read(randomPassword); err != nil {
		return "", err
	}
	return auth.HashPassword(base64.RawURLEncoding.EncodeToString(randomPassword))
}

var createOIDCUser = func(user *models.User) error {
	return db.GetDB().Create(user).Error
}

func findOIDCUser(issuer, subject string, user *models.User) error {
	return db.GetDB().
		Where("oidc_issuer = ? AND oidc_subject = ?", issuer, subject).
		First(user).
		Error
}

func attachOIDCIdentity(user models.User, issuer string, claims auth.OIDCClaims) (models.User, error) {
	if user.OIDCIssuer != nil || user.OIDCSubject != nil {
		if user.OIDCIssuer == nil || user.OIDCSubject == nil || *user.OIDCIssuer != issuer || *user.OIDCSubject != claims.Subject {
			return models.User{}, errOIDCUserAlreadyLinked
		}
		return user, nil
	}

	updates := map[string]interface{}{
		"oidc_issuer":  issuer,
		"oidc_subject": claims.Subject,
	}
	if user.AuthProvider == "" {
		updates["auth_provider"] = "password"
	}
	if verifiedEmail := verifiedOIDCEmail(claims); verifiedEmail != "" {
		updates["email"] = verifiedEmail
	}

	result := db.GetDB().
		Model(&models.User{}).
		Where("id = ? AND oidc_issuer IS NULL AND oidc_subject IS NULL", user.ID).
		Updates(updates)
	if result.Error != nil {
		if isUniqueConstraintError(result.Error) {
			return models.User{}, errOIDCUserAlreadyLinked
		}
		return models.User{}, result.Error
	}

	var reloaded models.User
	if err := db.GetDB().First(&reloaded, user.ID).Error; err != nil {
		return models.User{}, err
	}
	if result.RowsAffected == 1 {
		return reloaded, nil
	}
	if reloaded.OIDCIssuer != nil && reloaded.OIDCSubject != nil && *reloaded.OIDCIssuer == issuer && *reloaded.OIDCSubject == claims.Subject {
		return reloaded, nil
	}
	return models.User{}, errOIDCUserAlreadyLinked
}

func isUniqueConstraintError(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}
	errText := err.Error()
	return strings.Contains(errText, "UNIQUE constraint failed") ||
		strings.Contains(errText, "duplicate key value violates unique constraint")
}

func oidcUsername(issuer string, claims auth.OIDCClaims) string {
	email := strings.TrimSpace(claims.Email)
	if email != "" && len(email) <= maxOIDCUsernameLength {
		return email
	}

	hash := sha256.Sum256([]byte(issuer + "\x00" + claims.Subject))
	return "oidc-" + hex.EncodeToString(hash[:])[:32]
}

func verifiedOIDCEmail(claims auth.OIDCClaims) string {
	if !claims.EmailVerified {
		return ""
	}
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	if email == "" || len(email) > maxOIDCEmailLength {
		return ""
	}
	return email
}

func randomState() (string, error) {
	bytes := make([]byte, 32)
	if _, err := io.ReadFull(randomReader, bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func storeOIDCCodeVerifier(state, verifier string, expiresAt time.Time) {
	oidcCodeVerifierStore.Lock()
	defer oidcCodeVerifierStore.Unlock()

	now := time.Now()
	for key, entry := range oidcCodeVerifierStore.entries {
		if !entry.expiresAt.After(now) {
			delete(oidcCodeVerifierStore.entries, key)
		}
	}
	if maxEntries := oidcCodeVerifierStoreMaxEntries(); maxEntries > 0 && len(oidcCodeVerifierStore.entries) >= maxEntries {
		evictOldestOIDCCodeVerifier()
	}
	oidcCodeVerifierStore.entries[state] = oidcCodeVerifierEntry{
		verifier:  verifier,
		expiresAt: expiresAt,
	}
}

func oidcCodeVerifierStoreMaxEntries() int {
	value := strings.TrimSpace(os.Getenv("OIDC_CODE_VERIFIER_STORE_MAX_ENTRIES"))
	if value == "" {
		return defaultOIDCCodeVerifierStoreMaxEntries
	}
	entries, err := strconv.Atoi(value)
	if err != nil || entries < 1 {
		return defaultOIDCCodeVerifierStoreMaxEntries
	}
	return entries
}

func evictOldestOIDCCodeVerifier() {
	var oldestState string
	var oldestExpiresAt time.Time
	for state, entry := range oidcCodeVerifierStore.entries {
		if oldestState == "" || entry.expiresAt.Before(oldestExpiresAt) {
			oldestState = state
			oldestExpiresAt = entry.expiresAt
		}
	}
	if oldestState != "" {
		delete(oidcCodeVerifierStore.entries, oldestState)
	}
}

func takeOIDCCodeVerifier(state string) (string, bool) {
	oidcCodeVerifierStore.Lock()
	defer oidcCodeVerifierStore.Unlock()

	now := time.Now()
	entry, ok := oidcCodeVerifierStore.entries[state]
	delete(oidcCodeVerifierStore.entries, state)
	if !ok || !entry.expiresAt.After(now) {
		return "", false
	}
	return entry.verifier, true
}

func deleteOIDCCodeVerifier(state string) {
	oidcCodeVerifierStore.Lock()
	defer oidcCodeVerifierStore.Unlock()

	delete(oidcCodeVerifierStore.entries, state)
}

func encodeOIDCLoginState(state oidcLoginState) (string, error) {
	data, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(data)
	signature, err := signOIDCLoginStatePayload(payload)
	if err != nil {
		return "", err
	}
	return payload + "." + signature, nil
}

func decodeOIDCLoginState(cookie *http.Cookie) (oidcLoginState, error) {
	if cookie == nil || cookie.Value == "" {
		return oidcLoginState{}, errors.New("missing OIDC state")
	}
	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return oidcLoginState{}, errors.New("invalid OIDC state format")
	}
	if err := verifyOIDCLoginStatePayload(parts[0], parts[1]); err != nil {
		return oidcLoginState{}, err
	}
	data, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return oidcLoginState{}, err
	}
	var state oidcLoginState
	if err := json.Unmarshal(data, &state); err != nil {
		return oidcLoginState{}, err
	}
	return state, nil
}

func signOIDCLoginStatePayload(payload string) (string, error) {
	secret, err := auth.CookieSigningSecretFromEnv("OIDC_STATE_SECRET")
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func verifyOIDCLoginStatePayload(payload, signature string) error {
	expected, err := signOIDCLoginStatePayload(payload)
	if err != nil {
		return err
	}
	actual, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return err
	}
	expectedBytes, err := base64.RawURLEncoding.DecodeString(expected)
	if err != nil {
		return err
	}
	if !hmac.Equal(actual, expectedBytes) {
		return errors.New("invalid OIDC state signature")
	}
	return nil
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
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
