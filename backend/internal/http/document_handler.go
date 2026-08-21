package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	"github.com/fredyxander/okf-platform/backend/internal/application"
)

const maxUploadSize int64 = 10 * 1024 * 1024 // 10 MB

type DocumentHandler struct {
	service *application.DocumentService
}

func NewDocumentHandler(service *application.DocumentService) *DocumentHandler {
	return &DocumentHandler{
		service: service,
	}
}

func (h *DocumentHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ownerID := r.Header.Get("X-Test-Owner-ID")
	if ownerID == "" {
		http.Error(w, "X-Test-Owner-ID is required", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize) //limite de tamaño

	file, header, err := r.FormFile("file")

	if err != nil {
		var maxBytesError *http.MaxBytesError

		if errors.As(err, &maxBytesError) {
			http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
			return
		}

		http.Error(w, "file is required", http.StatusBadRequest)
		return
	}

	defer file.Close()

	filename := filepath.Base(header.Filename)

	if filename == "." || filename == "" {
		http.Error(w, "invalid filename", http.StatusBadRequest)
		return
	}

	contentType := header.Header.Get("Content-Type")

	var format string

	switch contentType {
	case "text/plain":
		format = "plaintext"

	case "text/markdown":
		format = "markdown"

	default:
		http.Error(w, "unsupported file type", http.StatusUnsupportedMediaType)
		return
	}

	document, err := h.service.CreateDocument(
		r.Context(),
		ownerID,
		filename,
		format,
		contentType,
		header.Size,
		file,
	)
	if err != nil {
		http.Error(w, "could not create document", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	if err := json.NewEncoder(w).Encode(document); err != nil {
		return
	}
}

func (h *DocumentHandler) Download(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ownerID := r.Header.Get("X-Test-Owner-ID")
	if ownerID == "" {
		http.Error(w, "X-Test-Owner-ID is required", http.StatusBadRequest)
		return
	}

	documentID := r.PathValue("id")
	if documentID == "" {
		http.Error(w, "document id is required", http.StatusBadRequest)
		return
	}

	document, object, err := h.service.GetDocument(
		r.Context(),
		documentID,
		ownerID,
	)
	if err != nil {
		http.Error(w, "document not found", http.StatusNotFound)
		return
	}
	defer object.Close()

	w.Header().Set(
		"Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"`, document.Filename),
	)

	w.Header().Set("Content-Type", "application/octet-stream")

	if _, err := io.Copy(w, object); err != nil {
		return
	}
}
