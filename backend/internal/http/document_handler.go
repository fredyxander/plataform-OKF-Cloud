package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/fredyxander/okf-platform/backend/internal/application"
)

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

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	document, err := h.service.CreateDocument(
		r.Context(),
		ownerID,
		header.Filename,
		"plaintext",
		header.Header.Get("Content-Type"),
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
