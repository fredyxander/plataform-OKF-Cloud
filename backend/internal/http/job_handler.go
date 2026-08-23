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

// BundleResponse describe el bundle asociado a un Job.
//
// Validation transporta la clasificación completa
// (valid / valid_with_warnings / invalid) para que el cliente pueda
// distinguir un bundle publicado sin observaciones de uno publicado
// con advertencias o rechazado.
//
// DownloadURL solo se envía cuando el bundle es realmente descargable.
type BundleResponse struct {
	ID           string                  `json:"id"`
	ConceptCount int                     `json:"concept_count"`
	IsValid      bool                    `json:"is_valid"`
	Validation   domain.BundleValidation `json:"validation"`
	DownloadURL  string                  `json:"download_url,omitempty"`
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

	if bundle == nil {
		return response
	}

	response.Bundle = &BundleResponse{
		ID:           bundle.ID,
		ConceptCount: bundle.ConceptCount,
		IsValid:      bundle.IsValid,
		Validation:   bundle.Validation,
	}

	// La descarga solo se ofrece cuando el Job terminó correctamente y
	// la validación permitió publicar el bundle.
	if job.Status == domain.JobStatusCompleted && bundle.IsValid {
		response.Bundle.DownloadURL = fmt.Sprintf(
			"/jobs/%s/bundle",
			job.ID,
		)
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

	// 3. Buscamos el Bundle cuando el Job ya terminó.
	//
	//    También se consulta para un Job fallido: si la validación
	//    rechazó el bundle, existe una fila que explica por qué, aunque
	//    no sea descargable.
	if job.Status == domain.JobStatusCompleted ||
		job.Status == domain.JobStatusFailed {
		bundle, err = h.db.GetBundleByJobID(
			job.ID,
			ownerID,
		)

		if err != nil {
			if !errors.Is(err, database.ErrNotFound) {
				log.Printf(
					"could not load bundle of job %s: %v",
					job.ID,
					err,
				)
			}

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
