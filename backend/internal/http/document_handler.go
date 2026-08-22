package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"

	"github.com/fredyxander/okf-platform/backend/internal/application"
)

const maxUploadSize int64 = 10 * 1024 * 1024 // 10 MB

type DocumentHandler struct {
	documentService   *application.DocumentService
	processingService *application.ProcessingService
}

func NewDocumentHandler(
	documentService *application.DocumentService,
	processingService *application.ProcessingService,
) *DocumentHandler {
	return &DocumentHandler{
		documentService:   documentService,
		processingService: processingService,
	}
}

func (h *DocumentHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ownerID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
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

	result, err := h.processingService.CreateProcessingJob(
		r.Context(),
		ownerID,
		filename,
		format,
		contentType,
		header.Size,
		file,
	)
	if err != nil {
		log.Printf("could not create processing job: %v", err)

		http.Error(
			w,
			"could not create processing job",
			http.StatusInternalServerError,
		)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)

	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"document": result.Document,
		"jobId":    result.Job.ID,
		"status":   result.Job.Status,
	}); err != nil {
		return
	}
}

func (h *DocumentHandler) Download(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ownerID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	documentID := r.PathValue("id")
	if documentID == "" {
		http.Error(w, "document id is required", http.StatusBadRequest)
		return
	}

	document, object, err := h.documentService.GetDocument(
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
