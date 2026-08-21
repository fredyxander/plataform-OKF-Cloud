package database

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/fredyxander/okf-platform/backend/internal/domain"
)

func TestJobRepository(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// 1. Crear owner.
	user, err := db.CreateUser(
		fmt.Sprintf("job-test-%d@example.com", time.Now().UnixNano()),
		"fake-hash-for-test",
	)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// 2. Crear documento.
	document, err := db.CreateDocument(
		user.ID,
		"job-test.txt",
		fmt.Sprintf("documents/%d/job-test.txt", time.Now().UnixNano()),
		"plaintext",
		100,
	)
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	// 3. Crear job.
	job, err := db.CreateJob(
		document.ID,
		user.ID,
		fmt.Sprintf("idempotency-%d", time.Now().UnixNano()),
	)
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	if job.ID == "" {
		t.Fatal("expected generated job ID")
	}

	if job.Status != domain.JobStatusQueued {
		t.Fatalf("expected queued status, got %s", job.Status)
	}

	// 4. Recuperar job.
	found, err := db.GetJobByID(job.ID, user.ID)
	if err != nil {
		t.Fatalf("get job: %v", err)
	}

	if found.ID != job.ID {
		t.Fatalf("expected job ID %s, got %s", job.ID, found.ID)
	}

	// 5. Cambiar a processing.
	err = db.UpdateJobStatus(job.ID, domain.JobStatusProcessing, nil)
	if err != nil {
		t.Fatalf("update job to processing: %v", err)
	}

	processing, err := db.GetJobByID(job.ID, user.ID)
	if err != nil {
		t.Fatalf("get processing job: %v", err)
	}

	if processing.Status != domain.JobStatusProcessing {
		t.Fatalf("expected processing status, got %s", processing.Status)
	}

	// 6. Cambiar a completed.
	err = db.UpdateJobStatus(job.ID, domain.JobStatusCompleted, nil)
	if err != nil {
		t.Fatalf("update job to completed: %v", err)
	}

	completed, err := db.GetJobByID(job.ID, user.ID)
	if err != nil {
		t.Fatalf("get completed job: %v", err)
	}

	if completed.Status != domain.JobStatusCompleted {
		t.Fatalf("expected completed status, got %s", completed.Status)
	}

	// El trigger debería haber actualizado updated_at.
	if !completed.UpdatedAt.After(job.UpdatedAt) {
		t.Fatalf(
			"expected updated_at after %v, got %v",
			job.UpdatedAt,
			completed.UpdatedAt,
		)
	}

	// 7. Listar jobs del owner.
	jobs, err := db.ListJobsByOwner(user.ID)
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}

	if len(jobs) == 0 {
		t.Fatal("expected at least one job")
	}

	// 8. Comprobar aislamiento.
	otherUser, err := db.CreateUser(
		fmt.Sprintf("job-other-%d@example.com", time.Now().UnixNano()),
		"fake-hash-for-test",
	)
	if err != nil {
		t.Fatalf("create second user: %v", err)
	}

	_, err = db.GetJobByID(job.ID, otherUser.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for another owner, got %v", err)
	}
}