package handlers

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// SPAServer serves static files from an embedded fs.FS with client-side routing fallback to index.html.
type SPAServer struct {
	fileSystem http.FileSystem
}

func NewSPAServer(fileSystem fs.FS) *SPAServer {
	return &SPAServer{fileSystem: http.FS(fileSystem)}
}

func (s *SPAServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(filepath.Clean(r.URL.Path), "/")

	// Try opening the requested file
	f, err := s.fileSystem.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Fallback to index.html for Vue Router SPA routes
			r.URL.Path = "/"
			http.FileServer(s.fileSystem).ServeHTTP(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if stat.IsDir() {
		// Try index.html in that dir or root
		indexPath := filepath.Join(path, "index.html")
		if _, err := s.fileSystem.Open(indexPath); err != nil {
			r.URL.Path = "/"
		}
	}

	http.FileServer(s.fileSystem).ServeHTTP(w, r)
}
