package auth

import (
	"context"
	"errors"
	"os"
	"sync"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OIDCConfig contains provider-generic authorization-code login settings.
type OIDCConfig struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// OIDCClaims contains the identity claims the app needs from the ID token.
type OIDCClaims struct {
	Subject           string `json:"sub"`
	Email             string `json:"email"`
	EmailVerified     bool   `json:"email_verified"`
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
}

type oidcProviderCacheKey struct {
	issuerURL string
	clientID  string
}

type oidcProviderCacheEntry struct {
	endpoint oauth2.Endpoint
	verifier *oidc.IDTokenVerifier
}

type oidcProviderCacheCall struct {
	done  chan struct{}
	entry oidcProviderCacheEntry
	err   error
}

var oidcProviderCache = struct {
	sync.Mutex
	entries map[oidcProviderCacheKey]oidcProviderCacheEntry
	calls   map[oidcProviderCacheKey]*oidcProviderCacheCall
}{
	entries: make(map[oidcProviderCacheKey]oidcProviderCacheEntry),
	calls:   make(map[oidcProviderCacheKey]*oidcProviderCacheCall),
}

// OIDCConfigFromEnv reads OIDC settings from environment variables.
func OIDCConfigFromEnv() OIDCConfig {
	return OIDCConfig{
		IssuerURL:    os.Getenv("OIDC_ISSUER_URL"),
		ClientID:     os.Getenv("OIDC_CLIENT_ID"),
		ClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("OIDC_REDIRECT_URL"),
	}
}

// IsOIDCConfigured reports whether all required OIDC settings are present.
func IsOIDCConfigured() bool {
	return OIDCConfigFromEnv().Configured()
}

// Configured reports whether all required OIDC settings are present.
func (cfg OIDCConfig) Configured() bool {
	return cfg.IssuerURL != "" && cfg.ClientID != "" && cfg.ClientSecret != "" && cfg.RedirectURL != ""
}

// OAuth2Config discovers provider metadata and returns OAuth2 and ID token verifier config.
func (cfg OIDCConfig) OAuth2Config(ctx context.Context) (*oauth2.Config, *oidc.IDTokenVerifier, error) {
	if !cfg.Configured() {
		return nil, nil, errors.New("OIDC is not configured")
	}

	cacheKey := oidcProviderCacheKey{issuerURL: cfg.IssuerURL, clientID: cfg.ClientID}
	oidcProviderCache.Lock()
	entry, ok := oidcProviderCache.entries[cacheKey]
	if !ok {
		if call, exists := oidcProviderCache.calls[cacheKey]; exists {
			oidcProviderCache.Unlock()
			select {
			case <-call.done:
				if call.err != nil {
					return nil, nil, call.err
				}
				entry = call.entry
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			}
		} else {
			call := &oidcProviderCacheCall{done: make(chan struct{})}
			oidcProviderCache.calls[cacheKey] = call
			oidcProviderCache.Unlock()

			provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
			if err != nil {
				call.err = err
			} else {
				call.entry = oidcProviderCacheEntry{
					endpoint: provider.Endpoint(),
					verifier: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
				}
			}

			oidcProviderCache.Lock()
			if call.err == nil {
				oidcProviderCache.entries[cacheKey] = call.entry
			}
			delete(oidcProviderCache.calls, cacheKey)
			close(call.done)
			oidcProviderCache.Unlock()

			if call.err != nil {
				return nil, nil, call.err
			}
			entry = call.entry
		}
	} else {
		oidcProviderCache.Unlock()
	}

	oauthConfig := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint:     entry.endpoint,
		Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
	}

	return oauthConfig, entry.verifier, nil
}
