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
	cleanPath := path.Clean(r.URL.Path)
	if cleanPath == "/" || cleanPath == "." {
		cleanPath = "index.html"
	} else {
		cleanPath = strings.TrimPrefix(cleanPath, "/")
	}

	f, err := s.fsys.Open(cleanPath)
	if err != nil {
		// SPA fallback: return index.html for Vue client-side routes
		r.URL.Path = "/"
		http.FileServer(s.fsys).ServeHTTP(w, r)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if stat.IsDir() {
		r.URL.Path = "/"
		http.FileServer(s.fsys).ServeHTTP(w, r)
		return
	}

	http.FileServer(s.fsys).ServeHTTP(w, r)
}
