package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"
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
