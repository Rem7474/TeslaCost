package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/teslacost/teslacost/internal/config"
)

func TestRequestIDHandlerAttachesRequestID(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(requestIDHandler{slog.NewJSONHandler(&buf, nil)})

	ctx := context.WithValue(context.Background(), chiMiddleware.RequestIDKey, "abc-123")
	logger.ErrorContext(ctx, "boom")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log line %q: %v", buf.String(), err)
	}
	if entry["request_id"] != "abc-123" {
		t.Fatalf("expected request_id=abc-123 in log entry, got %v", entry)
	}
}

func TestRequestIDHandlerOmitsFieldWithoutRequestID(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(requestIDHandler{slog.NewJSONHandler(&buf, nil)})

	logger.Info("no request context here")

	if strings.Contains(buf.String(), "request_id") {
		t.Fatalf("did not expect a request_id field, got: %s", buf.String())
	}
}

func TestInsecureDefaultsErrorBlocksProductionOnly(t *testing.T) {
	published := func(env string) *config.Config {
		return &config.Config{
			Environment:      env,
			JWTSecret:        "super_secret_jwt_signing_key_for_teslacost_app",
			AppEncryptionKey: "change-this-to-a-secure-32-byte-key-in-prod!",
			DatabaseURL:      "postgres://teslacost:teslacost_dev_secret@postgres:5432/teslacost",
		}
	}

	err := insecureDefaultsError(published("production"))
	if err == nil {
		t.Fatal("a production instance with the published secrets must not start")
	}
	for _, want := range []string{"JWT_SECRET", "APP_ENCRYPTION_KEY", "DB_PASSWORD", "openssl rand"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the message must name %q so the fix is obvious: %v", want, err)
		}
	}
	if err := insecureDefaultsError(published("PRODUCTION")); err == nil {
		t.Error("the environment name is case insensitive")
	}
	if err := insecureDefaultsError(published("development")); err != nil {
		t.Errorf("development keeps the defaults, got %v", err)
	}

	real := &config.Config{
		Environment:      "production",
		JWTSecret:        "0f3c9a1e7b2d4c58a6e1f0937b5d2c48e9a1b7c3d5f60718293a4b5c6d7e8f90",
		AppEncryptionKey: "b7e1c4a9d2f8036e5a4b1c7d9e0f2a3b4c5d6e7f8091a2b3c4d5e6f708192a3b",
		DatabaseURL:      "postgres://teslacost:Zx9-real-password@postgres:5432/teslacost",
	}
	if err := insecureDefaultsError(real); err != nil {
		t.Errorf("real secrets must start, got %v", err)
	}
}

func TestContentSecurityPolicyChoice(t *testing.T) {
	if got := contentSecurityPolicy(&config.Config{}); !strings.Contains(got, "default-src 'self'") {
		t.Errorf("default policy expected, got %q", got)
	}
	if got := contentSecurityPolicy(&config.Config{ContentSecurityPolicy: "OFF"}); got != "" {
		t.Errorf("off must send no policy, got %q", got)
	}
	if got := contentSecurityPolicy(&config.Config{ContentSecurityPolicy: "default-src 'none'"}); got != "default-src 'none'" {
		t.Errorf("a custom policy replaces the default, got %q", got)
	}
}
