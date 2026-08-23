package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/fredyxander/okf-platform/backend/internal/application"
)

const maxUploadSize int64 = 10 * 1024 * 1024 // 10 MB

// detectFormat decide el formato de un documento cargado.
//
// El Content-Type manda cuando dice algo útil, pero no se puede depender
// solo de él: lo elige el navegador a partir del registro del sistema y
// para .md suele venir vacío, que en multipart llega como
// application/octet-stream. Rechazar por eso convertiría una diferencia
// entre sistemas operativos en un error del usuario.
//
// Por eso, cuando el tipo declarado no aporta nada, se recurre a la
// extensión. El contenido sigue siendo no confiable: esto solo elige el
// conversor, y el conversor valida lo que recibe.
func detectFormat(contentType, filename string) (string, bool) {
	// El Content-Type puede traer parámetros: "text/plain; charset=utf-8".
	if mediaType, _, err := mime.ParseMediaType(contentType); err == nil {
		contentType = mediaType
	}

	switch contentType {
	case "text/plain":
		return "plaintext", true

	case "text/markdown", "text/x-markdown":
		return "markdown", true

	case "", "application/octet-stream":
		// Sin tipo declarado: lo decide la extensión.

	default:
		return "", false
	}

	switch strings.ToLower(filepath.Ext(filename)) {
	case ".md", ".markdown":
		return "markdown", true

	case ".txt":
		return "plaintext", true
	}

	return "", false
}

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

	format, ok := detectFormat(contentType, filename)
	if !ok {
		http.Error(
			w,
			"unsupported file type: only .txt and .md are accepted",
			http.StatusUnsupportedMediaType,
		)

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
