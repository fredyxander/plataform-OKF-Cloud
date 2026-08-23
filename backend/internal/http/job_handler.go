package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

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

// JobDetailResponse es lo que consume el seguimiento de un Job.
//
// Terminal indica si el Job ya no cambiará: es el criterio de parada
// del cliente, para que no tenga que codificar por su cuenta qué
// estados son finales.
type JobDetailResponse struct {
	*domain.Job
	Terminal bool            `json:"terminal"`
	Bundle   *BundleResponse `json:"bundle"`
}

// JobListItemResponse es una entrada del listado general de Jobs.
//
// Incluye el nombre del documento porque una lista de UUIDs no permite
// al usuario reconocer qué subió, y el bundle para poder ofrecer la
// descarga sin pedir el detalle de cada Job.
type JobListItemResponse struct {
	ID           string           `json:"id"`
	Status       domain.JobStatus `json:"status"`
	Terminal     bool             `json:"terminal"`
	ErrorMessage *string          `json:"error_message,omitempty"`
	Document     DocumentSummary  `json:"document"`
	Bundle       *BundleResponse  `json:"bundle"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
}

type DocumentSummary struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Format   string `json:"format"`
}

// buildBundleResponse traduce la metadata del bundle al contrato HTTP.
//
// La URL de descarga solo se emite cuando el Job terminó correctamente
// y la validación permitió publicar el bundle: su ausencia es la señal
// de que no hay nada que descargar.
func buildBundleResponse(
	jobID string,
	status domain.JobStatus,
	bundle *domain.Bundle,
) *BundleResponse {
	if bundle == nil {
		return nil
	}

	response := &BundleResponse{
		ID:           bundle.ID,
		ConceptCount: bundle.ConceptCount,
		IsValid:      bundle.IsValid,
		Validation:   bundle.Validation,
	}

	if status == domain.JobStatusCompleted && bundle.IsValid {
		response.DownloadURL = fmt.Sprintf("/jobs/%s/bundle", jobID)
	}

	return response
}

func buildJobDetailResponse(
	job *domain.Job,
	bundle *domain.Bundle,
) JobDetailResponse {
	return JobDetailResponse{
		Job:      job,
		Terminal: job.Status.IsTerminal(),
		Bundle:   buildBundleResponse(job.ID, job.Status, bundle),
	}
}

func buildJobListResponse(
	items []*domain.JobListItem,
) []JobListItemResponse {
	// Nunca null: una lista vacía debe serializarse como [].
	response := make([]JobListItemResponse, 0, len(items))

	for _, item := range items {
		response = append(response, JobListItemResponse{
			ID:           item.ID,
			Status:       item.Status,
			Terminal:     item.Status.IsTerminal(),
			ErrorMessage: item.ErrorMessage,
			Document: DocumentSummary{
				ID:       item.DocumentID,
				Filename: item.DocumentFilename,
				Format:   item.DocumentFormat,
			},
			Bundle:    buildBundleResponse(item.ID, item.Status, item.Bundle),
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		})
	}

	return response
}

// List devuelve el listado general de Jobs del usuario autenticado.
//
// Es la navegación normal de la aplicación y existe con independencia
// de que el cliente esté siguiendo un Job concreto.
func (h *JobHandler) List(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	items, err := h.db.ListJobsByOwner(ownerID)
	if err != nil {
		log.Printf("could not list jobs of owner %s: %v", ownerID, err)
		http.Error(w, "could not list jobs", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(buildJobListResponse(items)); err != nil {
		return
	}
}

// loadJobDetail obtiene el Job del propietario junto con su bundle.
//
// Lo comparten la consulta puntual y el stream de eventos, para que
// ambos emitan exactamente la misma representación.
func (h *JobHandler) loadJobDetail(
	jobID string,
	ownerID string,
) (JobDetailResponse, error) {
	job, err := h.db.GetJobByID(jobID, ownerID)
	if err != nil {
		return JobDetailResponse{}, err
	}

	var bundle *domain.Bundle

	// El bundle se consulta cuando el Job ya terminó. También para un
	// Job fallido: si la validación lo rechazó, existe una fila que
	// explica por qué, aunque no sea descargable.
	if job.Status.IsTerminal() {
		bundle, err = h.db.GetBundleByJobID(job.ID, ownerID)

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

	return buildJobDetailResponse(job, bundle), nil
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

	response, err := h.loadJobDetail(jobID, ownerID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			http.Error(w, "job not found", http.StatusNotFound)
			return
		}

		http.Error(w, "could not get job", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(response); err != nil {
		return
	}
}
