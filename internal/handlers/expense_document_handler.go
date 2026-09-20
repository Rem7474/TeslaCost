package handlers

import (
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
)

// UploadDocument uploads a new invoice or document (PDF or image) for a vehicle.
// The binary is written to the filesystem volume; only metadata and the storage path are persisted in PostgreSQL.
func (h *ExpenseHandler) UploadDocument(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	const maxUploadSize = 15 << 20 // 15 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		writeError(w, http.StatusBadRequest, "Fichier trop volumineux (max 15 Mo) ou formulaire invalide")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Fichier manquant (champ 'file' requis)")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Erreur lors de la lecture du fichier")
		return
	}
	if len(data) == 0 {
		writeError(w, http.StatusBadRequest, "Le fichier est vide")
		return
	}

	// The type comes from the content, never from what the client declared or the extension: a file named
	// receipt.pdf that is really HTML would otherwise be stored and served under a document type.
	mimeType, ok := documentMimeType(data)
	if !ok {
		writeError(w, http.StatusBadRequest, "Format de fichier non supporté. Formats acceptés : PDF, PNG, JPEG, WEBP")
		return
	}

	filename := filepath.Base(header.Filename)
	if filename == "" || filename == "." {
		filename = "document"
	}
	filename = strings.ReplaceAll(filename, "\n", "")
	filename = strings.ReplaceAll(filename, "\r", "")
	if len(filename) > 200 {
		filename = filename[:200]
	}

	var descPtr *string
	if desc := strings.TrimSpace(r.FormValue("description")); desc != "" {
		descPtr = &desc
	}

	// Save metadata first to obtain the DB-generated UUID (used as the filename on disk).
	doc := &models.ExpenseDocument{
		UserID:      userID,
		VehicleID:   vehicleID,
		Filename:    filename,
		MimeType:    mimeType,
		FileSize:    int64(len(data)),
		Description: descPtr,
	}

	if err := h.repo.SaveExpenseDocument(r.Context(), doc); err != nil {
		writeRepoError(w, r, err, "Failed to save document")
		return
	}

	// Write file to volume using vehicleID/docID as path (no user-controlled components).
	storagePath, err := h.storageService.Save(vehicleID, doc.ID, data)
	if err != nil {
		// Roll back DB record on storage failure.
		_ = h.repo.DeleteExpenseDocument(r.Context(), doc.ID, vehicleID, userID)
		writeError(w, http.StatusInternalServerError, "Erreur lors de l'écriture du fichier sur le volume")
		return
	}

	// Persist storage path (second update via raw query is expensive; we do a lightweight PATCH).
	if err := h.repo.UpdateDocumentStoragePath(r.Context(), doc.ID, storagePath); err != nil {
		_ = h.storageService.Delete(storagePath)
		_ = h.repo.DeleteExpenseDocument(r.Context(), doc.ID, vehicleID, userID)
		writeError(w, http.StatusInternalServerError, "Erreur lors de la mise à jour du chemin de stockage")
		return
	}
	doc.StoragePath = &storagePath

	headerResp := models.ExpenseDocumentHeader{
		ID:                  doc.ID,
		VehicleID:           doc.VehicleID,
		Filename:            doc.Filename,
		MimeType:            doc.MimeType,
		FileSize:            doc.FileSize,
		Description:         doc.Description,
		LinkedExpensesCount: 0,
		CreatedAt:           doc.CreatedAt,
	}

	writeJSON(w, http.StatusCreated, headerResp)
}

// ListDocuments lists all documents/invoices for a vehicle without returning large binary bodies.
func (h *ExpenseHandler) ListDocuments(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleViewer); v == nil {
		return
	}

	docs, err := h.repo.ListExpenseDocuments(r.Context(), vehicleID, userID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to list documents")
		return
	}
	if docs == nil {
		docs = []models.ExpenseDocumentHeader{}
	}

	writeJSON(w, http.StatusOK, docs)
}

// DownloadDocument streams the document binary to the client for display or download.
// The file is read from the filesystem volume; access is gated by DB ownership check.
// documentMimeType identifies an accepted document (PDF, PNG, JPEG, WEBP) from its first bytes.
func documentMimeType(data []byte) (string, bool) {
	switch detected := http.DetectContentType(data); detected {
	case "application/pdf", "image/jpeg", "image/png", "image/webp":
		return detected, true
	}
	return "", false
}

func (h *ExpenseHandler) DownloadDocument(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")
	docID := chi.URLParam(r, "docId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleViewer); v == nil {
		return
	}

	doc, err := h.repo.GetExpenseDocumentByID(r.Context(), docID, vehicleID, userID)
	if err != nil {
		writeRepoError(w, r, err, "Document not found")
		return
	}

	var fileData []byte
	if doc.StoragePath == nil || *doc.StoragePath == "" {
		if len(doc.Data) > 0 {
			// Legacy document still stored in database — write to volume on-the-fly and persist storage path
			storagePath, err := h.storageService.Save(vehicleID, doc.ID, doc.Data)
			if err == nil {
				_ = h.repo.UpdateDocumentStoragePath(r.Context(), doc.ID, storagePath)
				doc.StoragePath = &storagePath
			}
			fileData = doc.Data
		} else {
			writeError(w, http.StatusNotFound, "Fichier introuvable sur le volume de stockage")
			return
		}
	} else {
		data, err := h.storageService.Read(*doc.StoragePath)
		if err != nil {
			// If missing from volume but still present in database, heal and recover on-the-fly
			if len(doc.Data) > 0 {
				_, saveErr := h.storageService.Save(vehicleID, doc.ID, doc.Data)
				if saveErr == nil {
					fileData = doc.Data
				} else {
					writeError(w, http.StatusInternalServerError, "Erreur lors de la lecture du fichier")
					return
				}
			} else {
				writeError(w, http.StatusInternalServerError, "Erreur lors de la lecture du fichier")
				return
			}
		} else {
			fileData = data
		}
	}

	w.Header().Set("Content-Type", doc.MimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(fileData)), 10))
	disposition := mime.FormatMediaType("inline", map[string]string{"filename": doc.Filename})
	if disposition == "" {
		disposition = "inline"
	}
	w.Header().Set("Content-Disposition", disposition)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(fileData)
}

// DeleteDocument deletes a document. Linked expenses automatically have document_id set to NULL.
// After the DB record is removed the corresponding file on the volume is also deleted.
func (h *ExpenseHandler) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")
	docID := chi.URLParam(r, "docId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	// Fetch first to get the storage path before deletion.
	doc, err := h.repo.GetExpenseDocumentByID(r.Context(), docID, vehicleID, userID)
	if err != nil {
		writeRepoError(w, r, err, "Document not found")
		return
	}

	if err := h.repo.DeleteExpenseDocument(r.Context(), docID, vehicleID, userID); err != nil {
		writeRepoError(w, r, err, "Failed to delete document")
		return
	}

	// Best-effort: delete file from volume (non-fatal if file is missing).
	if doc.StoragePath != nil && *doc.StoragePath != "" {
		_ = h.storageService.Delete(*doc.StoragePath)
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Document deleted successfully"})
}
