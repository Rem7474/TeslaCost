package handlers

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// SPAServer serves static files from an embedded fs.FS with client-side routing fallback to index.html.
type SPAServer struct {
	fsys http.FileSystem
}

func NewSPAServer(fileSystem fs.FS) *SPAServer {
	return &SPAServer{fsys: http.FS(fileSystem)}
}

func (s *SPAServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"Endpoint introuvable ou service indisponible"}`))
		return
	}

	cleanPath := path.Clean(r.URL.Path)
	if cleanPath == "/" || cleanPath == "." || cleanPath == "/index.html" {
		s.serveIndex(w, r)
		return
	}
	cleanPath = strings.TrimPrefix(cleanPath, "/")

	f, err := s.fsys.Open(cleanPath)
	if err != nil {
		// A missing file (e.g. a hashed asset of a previous build) must be a real 404: answering with
		// index.html would be stored by browsers and service workers as the CSS/JS content.
		if isStaticFileRequest(cleanPath) {
			w.Header().Set("Cache-Control", "no-store")
			http.NotFound(w, r)
			return
		}
		// SPA fallback: return index.html for Vue client-side routes
		s.serveIndex(w, r)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if stat.IsDir() {
		s.serveIndex(w, r)
		return
	}

	switch {
	case strings.HasPrefix(cleanPath, "assets/"):
		// Build assets have content-hashed names: safe to cache forever.
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	default:
		// sw.js, manifest, icons: always revalidate so a new deployment is picked up.
		w.Header().Set("Cache-Control", "no-cache")
	}
	http.FileServer(s.fsys).ServeHTTP(w, r)
}

// serveIndex serves index.html without caching: it references the hashed assets of the current build.
func (s *SPAServer) serveIndex(w http.ResponseWriter, r *http.Request) {
	f, err := s.fsys.Open("index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, "index.html", stat.ModTime(), f)
}

// isStaticFileRequest reports whether a path targets a file (build asset or any path with an extension)
// rather than a client-side route.
func isStaticFileRequest(p string) bool {
	return strings.HasPrefix(p, "assets/") || path.Ext(p) != ""
}
