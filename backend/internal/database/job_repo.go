package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/fredyxander/okf-platform/backend/internal/domain"
)

func (db *DB) CreateJob(documentID, ownerID, idempotencyKey string) (*domain.Job, error) {
	j := &domain.Job{}
	err := db.conn.QueryRow(`
		INSERT INTO jobs (document_id, owner_id, idempotency_key)
		VALUES ($1, $2, $3)
		RETURNING id, document_id, owner_id, status, idempotency_key, error_message, created_at, updated_at`,
		documentID, ownerID, idempotencyKey,
	).Scan(&j.ID, &j.DocumentID, &j.OwnerID, &j.Status, &j.IdempotencyKey, &j.ErrorMessage, &j.CreatedAt, &j.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("create job: %w", err)
	}
	return j, nil
}

func (db *DB) GetJobByID(id, ownerID string) (*domain.Job, error) {
	j := &domain.Job{}
	err := db.conn.QueryRow(`
		SELECT id, document_id, owner_id, status, idempotency_key, error_message, created_at, updated_at
		FROM jobs WHERE id = $1 AND owner_id = $2`,
		id, ownerID,
	).Scan(&j.ID, &j.DocumentID, &j.OwnerID, &j.Status, &j.IdempotencyKey, &j.ErrorMessage, &j.CreatedAt, &j.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get job: %w", err)
	}
	return j, nil
}

func (db *DB) UpdateJobStatus(id string, status domain.JobStatus, errMsg *string) error {
	_, err := db.conn.Exec(`
		UPDATE jobs SET status = $1, error_message = $2
		WHERE id = $3`,
		status, errMsg, id,
	)
	if err != nil {
		return fmt.Errorf("update job status: %w", err)
	}
	return nil
}

func (db *DB) ListJobsByOwner(ownerID string) ([]*domain.Job, error) {
	rows, err := db.conn.Query(`
		SELECT 
		id, 
		document_id, 
		owner_id,
		status,
		idempotency_key, 
		error_message, 
		created_at, 
		updated_at
		FROM jobs WHERE owner_id = $1
		ORDER BY created_at DESC`,
		ownerID,
	)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*domain.Job
	for rows.Next() {
		j := &domain.Job{}
		if err := rows.Scan(&j.ID, &j.DocumentID, &j.OwnerID, &j.Status, &j.IdempotencyKey, &j.ErrorMessage, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}
