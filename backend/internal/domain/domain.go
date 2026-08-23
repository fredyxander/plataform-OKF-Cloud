package domain

import "time"

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Nombre       string    `json:"nombre"`
	Apellido     string    `json:"apellido"`
	CreatedAt    time.Time `json:"created_at"`
}

type Document struct {
	ID         string    `json:"id"`
	OwnerID    string    `json:"owner_id"`
	Filename   string    `json:"filename"`
	StorageKey string    `json:"storage_key"`
	Format     string    `json:"format"`
	SizeBytes  int64     `json:"size_bytes"`
	CreatedAt  time.Time `json:"created_at"`
}

type JobStatus string

const (
	JobStatusQueued     JobStatus = "queued"
	JobStatusProcessing JobStatus = "processing"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
)

// IsTerminal indica si el Job ya no volverá a cambiar de estado.
//
// Es el criterio de parada del seguimiento: un cliente consulta el Job
// mientras no sea terminal y deja de hacerlo en cuanto lo sea.
func (s JobStatus) IsTerminal() bool {
	return s == JobStatusCompleted || s == JobStatusFailed
}

type Job struct {
	ID             string    `json:"id"`
	DocumentID     string    `json:"document_id"`
	OwnerID        string    `json:"owner_id"`
	Status         JobStatus `json:"status"`
	IdempotencyKey string    `json:"-"`
	ErrorMessage   *string   `json:"error_message,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// JobStats resume el flujo de trabajos de un propietario.
//
// Todos los estados aparecen siempre, incluso con cero Jobs: una vista
// que muestre contadores no debería tener que distinguir entre "cero" y
// "ausente".
type JobStats struct {
	Queued     int `json:"queued"`
	Processing int `json:"processing"`
	Completed  int `json:"completed"`
	Failed     int `json:"failed"`
	Total      int `json:"total"`
}

// JobListItem es un Job con el contexto mínimo que necesita una vista
// de lista: qué documento lo originó y si ya produjo un bundle.
//
// Existe para que el listado se resuelva en una sola consulta, en lugar
// de obligar al cliente a pedir el detalle de cada Job por separado.
type JobListItem struct {
	Job

	DocumentFilename string
	DocumentFormat   string

	// Bundle es nil mientras el Job no haya producido uno.
	Bundle *Bundle
}

type Bundle struct {
	ID           string           `json:"id"`
	JobID        string           `json:"job_id"`
	OwnerID      string           `json:"owner_id"`
	StorageKey   string           `json:"storage_key"`
	IsValid      bool             `json:"is_valid"`
	ConceptCount int              `json:"concept_count"`
	Validation   BundleValidation `json:"validation"`
	PublishedAt  *time.Time       `json:"published_at,omitempty"`
	CreatedAt    time.Time        `json:"created_at"`
}
