package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"

	"github.com/fredyxander/okf-platform/backend/internal/domain"
)

// CreateBundle persiste la metadata del bundle junto con el resultado
// clasificado de su validación.
//
// Solo un bundle publicable recibe published_at: un bundle inválido se
// registra como evidencia de la validación, pero nunca queda publicado
// ni descargable.
func (db *DB) CreateBundle(
	jobID, ownerID, storageKey string,
	conceptCount int,
	validation domain.BundleValidation,
) (*domain.Bundle, error) {
	var publishedAt *time.Time

	isValid := validation.IsPublishable()

	if isValid {
		now := time.Now()
		publishedAt = &now
	}

	b := &domain.Bundle{}

	err := db.conn.QueryRow(`
		INSERT INTO bundles (
			job_id, owner_id, storage_key, is_valid, concept_count,
			validation_status, validation_warnings, validation_errors,
			published_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, job_id, owner_id, storage_key, is_valid, concept_count,
		          validation_status, validation_warnings, validation_errors,
		          published_at, created_at`,
		jobID, ownerID, storageKey, isValid, conceptCount,
		string(validation.Status),
		pq.Array(validation.Warnings),
		pq.Array(validation.Errors),
		publishedAt,
	).Scan(
		&b.ID, &b.JobID, &b.OwnerID, &b.StorageKey, &b.IsValid, &b.ConceptCount,
		&b.Validation.Status,
		pq.Array(&b.Validation.Warnings),
		pq.Array(&b.Validation.Errors),
		&b.PublishedAt, &b.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create bundle: %w", err)
	}

	return b, nil
}

func (db *DB) GetBundleByJobID(jobID, ownerID string) (*domain.Bundle, error) {
	b := &domain.Bundle{}

	err := db.conn.QueryRow(`
		SELECT id, job_id, owner_id, storage_key, is_valid, concept_count,
		       validation_status, validation_warnings, validation_errors,
		       published_at, created_at
		FROM bundles WHERE job_id = $1 AND owner_id = $2`,
		jobID, ownerID,
	).Scan(
		&b.ID, &b.JobID, &b.OwnerID, &b.StorageKey, &b.IsValid, &b.ConceptCount,
		&b.Validation.Status,
		pq.Array(&b.Validation.Warnings),
		pq.Array(&b.Validation.Errors),
		&b.PublishedAt, &b.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get bundle: %w", err)
	}

	return b, nil
}
