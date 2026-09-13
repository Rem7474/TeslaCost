package handlers

import (
	"bytes"
	"log"
	"net/http"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
)

// maxIdempotentBody bounds the stored response size.
const maxIdempotentBody = 64 << 10

type recordingWriter struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (w *recordingWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *recordingWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if w.body.Len()+len(b) <= maxIdempotentBody {
		w.body.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

// Idempotency replays the stored response of a mutation already processed with the same Idempotency-Key,
// so that requests queued offline and resent after a lost response are applied only once.
// It must run after authentication.
func Idempotency(repo *database.Repository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("Idempotency-Key")
			if key == "" || len(key) > 100 || r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			userID := middleware.GetUserID(r.Context())

			stored, err := repo.GetIdempotentResponse(r.Context(), userID, key)
			if err != nil {
				log.Printf("[idempotency] lookup failed: %v", err)
				writeError(w, http.StatusInternalServerError, "Idempotency check failed")
				return
			}
			if stored != nil {
				if stored.Method != r.Method || stored.Path != r.URL.Path {
					writeError(w, http.StatusUnprocessableEntity, "Idempotency-Key déjà utilisée pour une autre requête")
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Idempotent-Replay", "true")
				w.WriteHeader(stored.StatusCode)
				_, _ = w.Write(stored.Body)
				return
			}

			rec := &recordingWriter{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			if rec.status >= 200 && rec.status < 300 {
				if err := repo.SaveIdempotentResponse(r.Context(), userID, key, database.StoredResponse{
					Method: r.Method, Path: r.URL.Path, StatusCode: rec.status, Body: rec.body.Bytes(),
				}); err != nil {
					log.Printf("[idempotency] store failed: %v", err)
				}
			}
		})
	}
}
