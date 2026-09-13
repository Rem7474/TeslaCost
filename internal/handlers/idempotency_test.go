package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
)

// dedicatedTestDatabase creates (if needed) a sibling database of the test database and returns its URL.
func dedicatedTestDatabase(t *testing.T, baseURL, suffix string) string {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(baseURL)
	if err != nil {
		t.Fatal(err)
	}
	name := cfg.ConnConfig.Database + "_" + suffix
	admin, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	var exists bool
	if err := admin.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)`, name).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if !exists {
		if _, err := admin.Exec(context.Background(), `CREATE DATABASE "`+name+`"`); err != nil {
			t.Fatal(err)
		}
	}
	u, err := neturl.Parse(baseURL)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	return u.String()
}

// Requires TEST_DATABASE_URL (the public schema of that database is dropped and recreated).
func TestIdempotencyReplaysSuccessfulMutations(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	// Dedicated database: integration tests of other packages may run in parallel on TEST_DATABASE_URL.
	pool, err := pgxpool.New(ctx, dedicatedTestDatabase(t, url, "handlers"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		t.Fatal(err)
	}
	if err := (&database.DB{Pool: pool}).Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	repo := database.NewRepository(pool)
	user, err := repo.CreateUser(ctx, "idem@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}

	calls := 0
	h := Idempotency(repo)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if strings.Contains(r.URL.Path, "fail") {
			writeError(w, http.StatusBadRequest, "invalid")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]int{"call": calls})
	}))
	send := func(method, path, key string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(`{}`))
		req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, user.ID))
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	first := send(http.MethodPost, "/api/vehicles/v/expenses", "k1")
	replay := send(http.MethodPost, "/api/vehicles/v/expenses", "k1")
	if calls != 1 || replay.Code != http.StatusCreated || replay.Body.String() != first.Body.String() || replay.Header().Get("Idempotent-Replay") != "true" {
		t.Fatalf("expected a replay of the first response, calls=%d code=%d body=%q", calls, replay.Code, replay.Body.String())
	}
	if rec := send(http.MethodPut, "/api/vehicles/v/other", "k1"); rec.Code != http.StatusUnprocessableEntity || calls != 1 {
		t.Fatalf("reusing a key for another request must be rejected, got %d", rec.Code)
	}
	send(http.MethodPost, "/api/vehicles/v/expenses", "")
	if calls != 2 {
		t.Fatal("requests without key are always processed")
	}
	// Failed requests are not stored: a corrected retry with the same key is processed.
	send(http.MethodPost, "/api/fail", "k2")
	send(http.MethodPost, "/api/fail", "k2")
	if calls != 4 {
		t.Fatalf("failed requests must not be replayed, calls=%d", calls)
	}
}
