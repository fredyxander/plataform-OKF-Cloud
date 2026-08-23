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
