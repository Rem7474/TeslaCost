package storage_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/teslacost/teslacost/internal/storage"
)

func TestFileStorageService(t *testing.T) {
	dir := t.TempDir()
	svc, err := storage.NewFileStorageService(dir)
	if err != nil {
		t.Fatalf("NewFileStorageService: %v", err)
	}

	vehicleID := "vehicle-uuid-123"
	docID := "doc-uuid-456"
	data := []byte("PDF content here")

	// Save
	path, err := svc.Save(vehicleID, docID, data)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	expected := vehicleID + "/" + docID
	if path != expected {
		t.Fatalf("expected path %q, got %q", expected, path)
	}
	if _, err := os.Stat(filepath.Join(dir, vehicleID, docID)); err != nil {
		t.Fatalf("file not on disk: %v", err)
	}

	// Read
	got, err := svc.Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != string(data) {
		t.Fatalf("data mismatch: got %q", got)
	}

	// Path traversal rejected
	if _, err := svc.Read("../../etc/passwd"); err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}

	// Delete
	if err := svc.Delete(path); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, vehicleID, docID)); !os.IsNotExist(statErr) {
		t.Fatal("file should be deleted")
	}

	// Delete missing file is idempotent
	if err := svc.Delete(path); err != nil {
		t.Fatalf("Delete missing file should be idempotent: %v", err)
	}
}