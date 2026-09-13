package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSPAServer(t *testing.T) {
	spa := NewSPAServer(fstest.MapFS{
		"index.html":           {Data: []byte("<html>app</html>")},
		"assets/index-abc.css": {Data: []byte("body{}")},
		"sw.js":                {Data: []byte("self")},
	})

	tests := []struct {
		path        string
		status      int
		contentType string
		body        string
		cache       string
	}{
		{"/", http.StatusOK, "text/html", "app", "no-cache"},
		{"/index.html", http.StatusOK, "text/html", "app", "no-cache"},
		{"/drives", http.StatusOK, "text/html", "app", "no-cache"},
		{"/assets/index-abc.css", http.StatusOK, "text/css", "body{}", "immutable"},
		{"/sw.js", http.StatusOK, "javascript", "self", "no-cache"},
		// A stale asset of a previous build must not be answered with index.html
		{"/assets/index-old.css", http.StatusNotFound, "text/plain", "404", "no-store"},
		{"/assets/index-old.js", http.StatusNotFound, "text/plain", "404", "no-store"},
		{"/favicon-missing.png", http.StatusNotFound, "text/plain", "404", "no-store"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			spa.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d", rec.Code, tt.status)
			}
			if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, tt.contentType) {
				t.Errorf("content-type = %q, want %q", ct, tt.contentType)
			}
			if !strings.Contains(rec.Body.String(), tt.body) {
				t.Errorf("body = %q, want %q", rec.Body.String(), tt.body)
			}
			if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, tt.cache) {
				t.Errorf("cache-control = %q, want %q", cc, tt.cache)
			}
		})
	}
}
