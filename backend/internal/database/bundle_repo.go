package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/fredyxander/okf-platform/backend/internal/domain"
)

func (db *DB) CreateBundle(jobID, ownerID, minioPrefix string, isValid bool, conceptCount int) (*domain.Bundle, error) {
	b := &domain.Bundle{}
	var publishedAt *time.Time
	if isValid {
		now := time.Now()
		publishedAt = &now
	}
	err := db.conn.QueryRow(`
		INSERT INTO bundles (job_id, owner_id, storage_key, is_valid, concept_count, published_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, job_id, owner_id, storage_key, is_valid, concept_count, published_at, created_at`,
		jobID, ownerID, minioPrefix, isValid, conceptCount, publishedAt,
	).Scan(&b.ID, &b.JobID, &b.OwnerID, &b.StorageKey, &b.IsValid, &b.ConceptCount, &b.PublishedAt, &b.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create bundle: %w", err)
	}
	return b, nil
}

func (db *DB) GetBundleByJobID(jobID, ownerID string) (*domain.Bundle, error) {
	b := &domain.Bundle{}
	err := db.conn.QueryRow(`
		SELECT id, job_id, owner_id, storage_key, is_valid, concept_count, published_at, created_at
		FROM bundles WHERE job_id = $1 AND owner_id = $2`,
		jobID, ownerID,
	).Scan(&b.ID, &b.JobID, &b.OwnerID, &b.StorageKey, &b.IsValid, &b.ConceptCount, &b.PublishedAt, &b.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get bundle: %w", err)
	}
	return b, nil
}
