package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/fredyxander/okf-platform/backend/internal/database"
	"github.com/fredyxander/okf-platform/backend/internal/domain"
)

type JobHandler struct {
	db *database.DB
}

func NewJobHandler(db *database.DB) *JobHandler {
	return &JobHandler{
		db: db,
	}
}

type BundleResponse struct {
	ID           string `json:"id"`
	ConceptCount int    `json:"concept_count"`
	IsValid      bool   `json:"is_valid"`
	DownloadURL  string `json:"download_url"`
}

type JobDetailResponse struct {
	*domain.Job
	Bundle *BundleResponse `json:"bundle"`
}

func buildJobDetailResponse(
	job *domain.Job,
	bundle *domain.Bundle,
) JobDetailResponse {
	response := JobDetailResponse{
		Job:    job,
		Bundle: nil,
	}

	if job.Status == domain.JobStatusCompleted && bundle != nil {
		response.Bundle = &BundleResponse{
			ID:           bundle.ID,
			ConceptCount: bundle.ConceptCount,
			IsValid:      bundle.IsValid,
			DownloadURL: fmt.Sprintf(
				"/jobs/%s/bundle",
				job.ID,
			),
		}
	}

	return response
}

func (h *JobHandler) Get(w http.ResponseWriter, r *http.Request) {
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

	// 1. Obtener el Job y verificar que pertenece al usuario.
	job, err := h.db.GetJobByID(jobID, ownerID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			http.Error(w, "job not found", http.StatusNotFound)
			return
		}

		http.Error(w, "could not get job", http.StatusInternalServerError)
		return
	}

	// 2. Por defecto no hay Bundle asociado a la respuesta.
	var bundle *domain.Bundle

	// 3. Solo buscamos el Bundle si el Job ya terminó.
	if job.Status == domain.JobStatusCompleted {
		bundle, err = h.db.GetBundleByJobID(
			job.ID,
			ownerID,
		)

		if err != nil {
			log.Printf(
				"completed job %s has no bundle: %v",
				job.ID,
				err,
			)

			bundle = nil
		}
	}

	// 4. Construir la respuesta HTTP.
	response := buildJobDetailResponse(
		job,
		bundle,
	)

	// 5. Enviar JSON.
	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(response); err != nil {
		return
	}
}
