package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"

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

// ListJobsByOwner devuelve los Jobs del propietario junto con el
// documento que los originó y el bundle que produjeron, si existe.
//
// Se resuelve en una sola consulta para que una vista de lista no tenga
// que pedir el detalle de cada Job por separado.
//
// El orden es descendente por fecha de creación, con el id como
// desempate: sin él, dos Jobs creados en el mismo instante podrían
// alternar de posición entre consultas y la lista parpadearía.
//
// Devuelve siempre un slice no nulo: un propietario sin Jobs produce
// una lista vacía, no null.
func (db *DB) ListJobsByOwner(ownerID string) ([]*domain.JobListItem, error) {
	rows, err := db.conn.Query(`
		SELECT j.id, j.document_id, j.owner_id, j.status,
		       j.idempotency_key, j.error_message,
		       j.created_at, j.updated_at,
		       d.filename, d.format,
		       b.id, b.storage_key, b.is_valid, b.concept_count,
		       b.validation_status, b.validation_warnings,
		       b.validation_errors, b.published_at, b.created_at
		FROM jobs j
		JOIN documents d ON d.id = j.document_id
		LEFT JOIN bundles b ON b.job_id = j.id
		WHERE j.owner_id = $1
		ORDER BY j.created_at DESC, j.id DESC`,
		ownerID,
	)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}

	defer rows.Close()

	jobs := make([]*domain.JobListItem, 0)

	for rows.Next() {
		item := &domain.JobListItem{}

		// Las columnas del bundle llegan nulas mientras el Job no haya
		// producido uno, así que se leen en tipos que admiten NULL.
		var (
			bundleID        sql.NullString
			storageKey      sql.NullString
			isValid         sql.NullBool
			conceptCount    sql.NullInt64
			validationState sql.NullString
			warnings        []string
			validationErrs  []string
			publishedAt     *time.Time
			bundleCreatedAt sql.NullTime
		)

		if err := rows.Scan(
			&item.ID, &item.DocumentID, &item.OwnerID, &item.Status,
			&item.IdempotencyKey, &item.ErrorMessage,
			&item.CreatedAt, &item.UpdatedAt,
			&item.DocumentFilename, &item.DocumentFormat,
			&bundleID, &storageKey, &isValid, &conceptCount,
			&validationState, pq.Array(&warnings),
			pq.Array(&validationErrs), &publishedAt, &bundleCreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}

		if bundleID.Valid {
			item.Bundle = &domain.Bundle{
				ID:           bundleID.String,
				JobID:        item.ID,
				OwnerID:      item.OwnerID,
				StorageKey:   storageKey.String,
				IsValid:      isValid.Bool,
				ConceptCount: int(conceptCount.Int64),
				Validation: domain.BundleValidation{
					Status:   domain.BundleValidationStatus(validationState.String),
					Warnings: warnings,
					Errors:   validationErrs,
				},
				PublishedAt: publishedAt,
				CreatedAt:   bundleCreatedAt.Time,
			}
		}

		jobs = append(jobs, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}

	return jobs, nil
}

func (db *DB) GetJobByIDForProcessing(id string) (*domain.Job, error) {
	j := &domain.Job{}

	err := db.conn.QueryRow(`
		SELECT id, document_id, owner_id, status, idempotency_key,
		       error_message, created_at, updated_at
		FROM jobs
		WHERE id = $1`,
		id,
	).Scan(
		&j.ID,
		&j.DocumentID,
		&j.OwnerID,
		&j.Status,
		&j.IdempotencyKey,
		&j.ErrorMessage,
		&j.CreatedAt,
		&j.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("get job for processing: %w", err)
	}

	return j, nil
}

func (db *DB) ClaimJobForProcessing(
	id string,
	staleBefore time.Time,
) (*domain.Job, error) {
	j := &domain.Job{}

	err := db.conn.QueryRow(`
		UPDATE jobs
		SET status = $1,
		    error_message = NULL
		WHERE id = $2
		  AND (
		        status = $3
		        OR (
		            status = $1
		            AND updated_at < $4
		        )
		      )
		RETURNING id, document_id, owner_id, status, idempotency_key,
		          error_message, created_at, updated_at`,
		domain.JobStatusProcessing,
		id,
		domain.JobStatusQueued,
		staleBefore,
	).Scan(
		&j.ID,
		&j.DocumentID,
		&j.OwnerID,
		&j.Status,
		&j.IdempotencyKey,
		&j.ErrorMessage,
		&j.CreatedAt,
		&j.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		var exists bool

		checkErr := db.conn.QueryRow(`
			SELECT EXISTS (
				SELECT 1
				FROM jobs
				WHERE id = $1
			)`,
			id,
		).Scan(&exists)

		if checkErr != nil {
			return nil, fmt.Errorf(
				"check job existence after failed claim: %w",
				checkErr,
			)
		}

		if !exists {
			return nil, ErrNotFound
		}

		return nil, ErrJobNotClaimable
	}

	if err != nil {
		return nil, fmt.Errorf("claim job for processing: %w", err)
	}

	return j, nil
}

func (db *DB) RequeueJob(id string, errorMessage *string) error {
	result, err := db.conn.Exec(`
		UPDATE jobs
		SET status = $1,
		    error_message = $2
		WHERE id = $3
		  AND status = $4`,
		domain.JobStatusQueued,
		errorMessage,
		id,
		domain.JobStatusProcessing,
	)
	if err != nil {
		return fmt.Errorf("requeue job: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get requeue job rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrJobNotClaimable
	}

	return nil
}