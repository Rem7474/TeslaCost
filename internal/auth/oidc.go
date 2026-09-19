package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/teslacost/teslacost/internal/config"
)

// ErrEmailNotAllowed is returned when the OIDC email is not in the allowed-emails whitelist.
var ErrEmailNotAllowed = errors.New("email address is not authorized for this instance")

// ErrEmailNotVerified is returned when the identity provider states that it did not verify the email address.
// Accounts are matched on that address, so accepting an unverified one would let anyone who can register at the
// provider with someone else's address take over that person's account.
var ErrEmailNotVerified = errors.New("email address is not verified by the identity provider")

// UserInfo holds the claims extracted from a verified OIDC ID token.
type UserInfo struct {
	Subject     string // IdP-issued subject ("sub" claim) — stable unique identifier
	Email       string
	DisplayName string // "name" claim from IdP, may be empty
}

// OIDCService wraps the OIDC provider, verifier and OAuth2 config.
// It is initialised once at startup via OIDC Discovery.
type OIDCService struct {
	provider      *gooidc.Provider
	verifier      *gooidc.IDTokenVerifier
	oauth2Config  oauth2.Config
	allowedEmails map[string]bool // nil = no filter (all emails allowed)
}

// NewOIDCService initialises the OIDC service using the provider's Discovery endpoint.
// This performs a network call to <issuerURL>/.well-known/openid-configuration.
func NewOIDCService(ctx context.Context, cfg *config.Config) (*OIDCService, error) {
	provider, err := gooidc.NewProvider(ctx, cfg.OIDCIssuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc: failed to discover provider at %q: %w", cfg.OIDCIssuerURL, err)
	}

	verifier := provider.Verifier(&gooidc.Config{ClientID: cfg.OIDCClientID})

	oauth2Cfg := oauth2.Config{
		ClientID:     cfg.OIDCClientID,
		ClientSecret: cfg.OIDCClientSecret,
		RedirectURL:  cfg.OIDCRedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       cfg.OIDCScopes,
	}

	var allowedEmails map[string]bool
	if len(cfg.OIDCAllowedEmails) > 0 {
		allowedEmails = make(map[string]bool, len(cfg.OIDCAllowedEmails))
		for _, e := range cfg.OIDCAllowedEmails {
			allowedEmails[strings.ToLower(strings.TrimSpace(e))] = true
		}
	}

	return &OIDCService{
		provider:      provider,
		verifier:      verifier,
		oauth2Config:  oauth2Cfg,
		allowedEmails: allowedEmails,
	}, nil
}

// GenerateStateToken returns a cryptographically random URL-safe base64 string (32 bytes entropy).
// Used as the OAuth2 "state" parameter and stored in an HttpOnly cookie for CSRF protection.
func GenerateStateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("oidc: failed to generate state token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GenerateNonce returns a random nonce value (plain) suitable for the "nonce" cookie,
// and its SHA-256 hash for embedding in the authorization request (per OIDC spec).
func GenerateNonce() (plain, hashed string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("oidc: failed to generate nonce: %w", err)
	}
	plain = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(plain))
	hashed = base64.RawURLEncoding.EncodeToString(sum[:])
	return plain, hashed, nil
}

// GenerateCodeVerifier returns a PKCE code verifier (RFC 7636). It stays in an HttpOnly cookie; only its SHA-256
// challenge travels through the browser to the identity provider, so an intercepted authorization code cannot be
// redeemed by anyone who does not hold the cookie.
func GenerateCodeVerifier() string {
	return oauth2.GenerateVerifier()
}

// LoginURL builds the authorization redirect URL.
// state is the anti-CSRF token; nonceHashed is the SHA-256 of the nonce stored in the cookie; verifier is the
// PKCE code verifier, sent as an S256 challenge.
func (s *OIDCService) LoginURL(state, nonceHashed, verifier string) string {
	return s.oauth2Config.AuthCodeURL(state,
		oauth2.SetAuthURLParam("nonce", nonceHashed),
		oauth2.S256ChallengeOption(verifier),
	)
}

// ExchangeCode exchanges the authorization code for an ID token and extracts user claims.
// noncePlain is the raw nonce from the HttpOnly cookie — it is hashed and compared to the
// nonce embedded in the ID token by the IdP (per OIDC spec).
func (s *OIDCService) ExchangeCode(ctx context.Context, code, noncePlain, verifier string) (*UserInfo, error) {
	token, err := s.oauth2Config.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("oidc: code exchange failed: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("oidc: no id_token in token response")
	}

	idToken, err := s.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("oidc: id_token verification failed: %w", err)
	}

	// Verify nonce: hash the plain value from the cookie and compare to the claim in the token.
	sum := sha256.Sum256([]byte(noncePlain))
	expectedNonce := base64.RawURLEncoding.EncodeToString(sum[:])
	if idToken.Nonce != expectedNonce {
		return nil, fmt.Errorf("oidc: nonce mismatch")
	}

	var claims struct {
		Email         string `json:"email"`
		EmailVerified *bool  `json:"email_verified"`
		Name          string `json:"name"`
		Subject       string `json:"sub"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("oidc: failed to extract claims: %w", err)
	}

	if claims.Email == "" {
		return nil, fmt.Errorf("oidc: id_token missing email claim (ensure 'email' scope is requested)")
	}

	// A provider that does not send the claim is trusted as before; one that says the address is unverified is not.
	if claims.EmailVerified != nil && !*claims.EmailVerified {
		return nil, ErrEmailNotVerified
	}

	// Check allowed-emails whitelist (addresses are not case sensitive).
	if s.allowedEmails != nil && !s.allowedEmails[strings.ToLower(strings.TrimSpace(claims.Email))] {
		return nil, ErrEmailNotAllowed
	}

	return &UserInfo{
		Subject:     idToken.Subject,
		Email:       claims.Email,
		DisplayName: claims.Name,
	}, nil
}
