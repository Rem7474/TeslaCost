package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/config"
)

func TestAuthHandlerLogout(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:                  "test-secret",
		JWTAccessExpirationMinutes: 15,
		JWTRefreshExpirationDays:   30,
		CookieSecure:               false,
	}

	h := NewAuthHandler(nil, cfg, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{
		Name:  refreshTokenCookie,
		Value: "some-plain-token",
	})

	h.Logout(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	cookies := rec.Result().Cookies()
	var found bool
	for _, c := range cookies {
		if c.Name == refreshTokenCookie {
			found = true
			if c.MaxAge != -1 {
				t.Errorf("expected cookie MaxAge -1, got %d", c.MaxAge)
			}
			if c.Value != "" {
				t.Errorf("expected empty cookie value, got %s", c.Value)
			}
		}
	}
	if !found {
		t.Fatalf("expected Set-Cookie header clearing %s", refreshTokenCookie)
	}
}

func TestAuthHandlerRefreshMissingToken(t *testing.T) {
	cfg := &config.Config{
		JWTSecret:                  "test-secret",
		JWTAccessExpirationMinutes: 15,
		JWTRefreshExpirationDays:   30,
		CookieSecure:               false,
	}

	h := NewAuthHandler(nil, cfg, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")

	h.RefreshToken(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "Missing refresh token" {
		t.Errorf("expected 'Missing refresh token', got %q", resp["error"])
	}
}

func TestSetRefreshTokenCookie(t *testing.T) {
	cfg := &config.Config{
		CookieSecure: true,
	}
	h := NewAuthHandler(nil, cfg, nil)

	rec := httptest.NewRecorder()
	expiresAt := time.Now().Add(24 * time.Hour)
	h.setRefreshTokenCookie(rec, "secret-token-val", expiresAt)

	cookies := rec.Result().Cookies()
	var found bool
	for _, c := range cookies {
		if c.Name == refreshTokenCookie {
			found = true
			if c.Value != "secret-token-val" {
				t.Errorf("expected value secret-token-val, got %s", c.Value)
			}
			if !c.HttpOnly {
				t.Errorf("expected HttpOnly true")
			}
			if !c.Secure {
				t.Errorf("expected Secure true")
			}
			if c.Path != "/" {
				t.Errorf("expected Path '/', got %s", c.Path)
			}
		}
	}
	if !found {
		t.Fatalf("cookie not set")
	}
}

func TestCookieSecureRespectsConfig(t *testing.T) {
	cfg := &config.Config{CookieSecure: false}
	h := NewAuthHandler(nil, cfg, nil)

	rec := httptest.NewRecorder()
	h.setRefreshTokenCookie(rec, "tok", time.Now().Add(time.Hour))
	h.setAccessTokenCookie(rec, "tok", time.Now().Add(time.Hour))

	for _, c := range rec.Result().Cookies() {
		if c.Secure {
			t.Errorf("expected Secure=false for cookie %s when CookieSecure is false, got true", c.Name)
		}
	}
}

func TestSetAndClearAccessTokenCookie(t *testing.T) {
	cfg := &config.Config{CookieSecure: true}
	h := NewAuthHandler(nil, cfg, nil)

	rec := httptest.NewRecorder()
	h.setAccessTokenCookie(rec, "access-tok", time.Now().Add(15*time.Minute))

	var found bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == accessTokenCookie {
			found = true
			if c.Value != "access-tok" {
				t.Errorf("expected value access-tok, got %s", c.Value)
			}
			if !c.HttpOnly || !c.Secure {
				t.Errorf("expected HttpOnly and Secure true")
			}
		}
	}
	if !found {
		t.Fatalf("access token cookie not set")
	}

	rec2 := httptest.NewRecorder()
	h.clearAccessTokenCookie(rec2)
	found = false
	for _, c := range rec2.Result().Cookies() {
		if c.Name == accessTokenCookie {
			found = true
			if c.MaxAge != -1 || c.Value != "" {
				t.Errorf("expected cleared cookie, got MaxAge=%d Value=%q", c.MaxAge, c.Value)
			}
		}
	}
	if !found {
		t.Fatalf("expected Set-Cookie clearing the access token cookie")
	}
}
