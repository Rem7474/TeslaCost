package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/teslacost/teslacost/internal/auth"
)

func TestAuthenticateJWTAcceptsBearerHeader(t *testing.T) {
	secret := "test-secret"
	token, err := auth.GenerateAccessToken("user-1", "user@example.com", secret, 15)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	var gotUserID string
	handler := AuthenticateJWT(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = GetUserID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if gotUserID != "user-1" {
		t.Errorf("expected user-1, got %q", gotUserID)
	}
}

func TestAuthenticateJWTFallsBackToAccessTokenCookie(t *testing.T) {
	secret := "test-secret"
	token, err := auth.GenerateAccessToken("user-2", "cookie@example.com", secret, 15)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	var gotUserID string
	handler := AuthenticateJWT(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = GetUserID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: accessTokenCookieName, Value: token})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if gotUserID != "user-2" {
		t.Errorf("expected user-2, got %q", gotUserID)
	}
}

func TestAuthenticateJWTPrefersHeaderOverCookie(t *testing.T) {
	secret := "test-secret"
	headerToken, _ := auth.GenerateAccessToken("header-user", "h@example.com", secret, 15)
	cookieToken, _ := auth.GenerateAccessToken("cookie-user", "c@example.com", secret, 15)

	var gotUserID string
	handler := AuthenticateJWT(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID = GetUserID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+headerToken)
	req.AddCookie(&http.Cookie{Name: accessTokenCookieName, Value: cookieToken})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if gotUserID != "header-user" {
		t.Errorf("expected header token to take precedence, got %q", gotUserID)
	}
}

func TestAuthenticateJWTRejectsMissingCredentials(t *testing.T) {
	handler := AuthenticateJWT("test-secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called without credentials")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
