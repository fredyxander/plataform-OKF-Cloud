package httpapi

import (
	"fmt"
	"io"
	"net/http"

	"github.com/fredyxander/okf-platform/backend/internal/database"
	"github.com/fredyxander/okf-platform/backend/internal/storage"
)

type BundleHandler struct {
	db      *database.DB
	storage *storage.MinIO
}

func NewBundleHandler(
	db *database.DB,
	storage *storage.MinIO,
) *BundleHandler {
	return &BundleHandler{
		db:      db,
		storage: storage,
	}
}

func (h *BundleHandler) Download(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ownerID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	jobID := r.PathValue("id")
	if jobID == "" {
		http.Error(w, "job id is required", http.StatusBadRequest)
		return
	}

	bundle, err := h.db.GetBundleByJobID(jobID, ownerID)
	if err != nil {
		http.Error(w, "bundle not found", http.StatusNotFound)
		return
	}

	// Un bundle rechazado por la validación queda registrado, pero su
	// descarga nunca se habilita.
	if !bundle.IsValid {
		http.Error(
			w,
			"bundle was rejected by validation and is not available",
			http.StatusConflict,
		)

		return
	}

	object, err := h.storage.GetObject(
		r.Context(),
		bundle.StorageKey,
	)
	if err != nil {
		http.Error(w, "could not retrieve bundle", http.StatusInternalServerError)
		return
	}
	defer object.Close()

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set(
		"Content-Disposition",
		fmt.Sprintf(
			`attachment; filename="bundle-%s.zip"`,
			jobID,
		),
	)

	if _, err := io.Copy(w, object); err != nil {
		return
	}
}
