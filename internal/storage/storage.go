package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// FileStorageService manages document files on the local filesystem.
// Files are stored as <storageDir>/<vehicleID>/<docID> (UUID, no user-controlled path component).
// All access must go through authenticated API endpoints — the directory is never exposed via HTTP.
type FileStorageService struct {
	storageDir string
}

// NewFileStorageService creates a new FileStorageService and ensures the root storage directory exists.
func NewFileStorageService(storageDir string) (*FileStorageService, error) {
	if err := os.MkdirAll(storageDir, 0o750); err != nil {
		return nil, fmt.Errorf("storage: cannot create storage directory %q: %w", storageDir, err)
	}
	return &FileStorageService{storageDir: storageDir}, nil
}

// Save writes document bytes to <storageDir>/<vehicleID>/<docID>.
// vehicleID and docID are UUID strings from the database — no path traversal possible.
// Returns the relative storage path stored in the database (vehicleID/docID).
func (s *FileStorageService) Save(vehicleID, docID string, data []byte) (string, error) {
	dir := filepath.Join(s.storageDir, vehicleID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("storage: cannot create vehicle dir %q: %w", dir, err)
	}
	filePath := filepath.Join(dir, docID)
	if err := os.WriteFile(filePath, data, 0o640); err != nil {
		return "", fmt.Errorf("storage: cannot write file %q: %w", filePath, err)
	}
	// Return relative path: vehicleID/docID
	return vehicleID + "/" + docID, nil
}

// Read reads document bytes from the given relative storage path.
// The path is vehicleID/docID from the database — validated against vehicleID before calling.
func (s *FileStorageService) Read(storagePath string) ([]byte, error) {
	// Clean the path to prevent any traversal, then re-join with root.
	clean := filepath.Clean(storagePath)
	absPath := filepath.Join(s.storageDir, clean)

	// Safety guard: resulting path must start with storageDir.
	absRoot := filepath.Clean(s.storageDir)
	if len(absPath) <= len(absRoot) || absPath[:len(absRoot)] != absRoot {
		return nil, fmt.Errorf("storage: path traversal detected for %q", storagePath)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("storage: cannot read file %q: %w", absPath, err)
	}
	return data, nil
}

// Delete removes the document file. Returns nil if the file does not exist (idempotent).
func (s *FileStorageService) Delete(storagePath string) error {
	clean := filepath.Clean(storagePath)
	absPath := filepath.Join(s.storageDir, clean)

	// Safety guard: resulting path must start with storageDir.
	absRoot := filepath.Clean(s.storageDir)
	if len(absPath) <= len(absRoot) || absPath[:len(absRoot)] != absRoot {
		return fmt.Errorf("storage: path traversal detected for %q", storagePath)
	}

	if err := os.Remove(absPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("storage: cannot delete file %q: %w", absPath, err)
	}
	return nil
}