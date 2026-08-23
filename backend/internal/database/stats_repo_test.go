package database

import (
	"fmt"
	"testing"
	"time"

	"github.com/fredyxander/okf-platform/backend/internal/domain"
)

func TestJobStatsByOwner(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	user, err := db.CreateUser(
		fmt.Sprintf("stats-test-%d@example.com", time.Now().UnixNano()),
		"fake-hash-for-test",
	)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Un propietario sin Jobs: todos los contadores a cero, no ausentes.
	stats, err := db.JobStatsByOwner(user.ID)
	if err != nil {
		t.Fatalf("job stats: %v", err)
	}

	if stats.Total != 0 || stats.Queued != 0 || stats.Processing != 0 ||
		stats.Completed != 0 || stats.Failed != 0 {
		t.Fatalf("expected an empty summary, got %+v", stats)
	}

	document, err := db.CreateDocument(
		user.ID,
		"stats-test.txt",
		fmt.Sprintf("documents/%d/stats-test.txt", time.Now().UnixNano()),
		"plaintext",
		100,
	)
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	// Tres Jobs: uno queued, uno completed y uno failed.
	statuses := []domain.JobStatus{
		domain.JobStatusQueued,
		domain.JobStatusCompleted,
		domain.JobStatusFailed,
	}

	for i, status := range statuses {
		job, err := db.CreateJob(
			document.ID,
			user.ID,
			fmt.Sprintf("stats-idempotency-%d-%d", time.Now().UnixNano(), i),
		)
		if err != nil {
			t.Fatalf("create job: %v", err)
		}

		if status == domain.JobStatusQueued {
			continue
		}

		if err := db.UpdateJobStatus(job.ID, status, nil); err != nil {
			t.Fatalf("update job status: %v", err)
		}
	}

	stats, err = db.JobStatsByOwner(user.ID)
	if err != nil {
		t.Fatalf("job stats: %v", err)
	}

	if stats.Queued != 1 {
		t.Errorf("expected 1 queued, got %d", stats.Queued)
	}

	if stats.Completed != 1 {
		t.Errorf("expected 1 completed, got %d", stats.Completed)
	}

	if stats.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", stats.Failed)
	}

	// processing sigue sin Jobs y debe reportarse como cero.
	if stats.Processing != 0 {
		t.Errorf("expected 0 processing, got %d", stats.Processing)
	}

	if stats.Total != 3 {
		t.Errorf("expected 3 jobs in total, got %d", stats.Total)
	}

	// Las métricas no pueden agregar Jobs de otro propietario.
	otherUser, err := db.CreateUser(
		fmt.Sprintf("stats-other-%d@example.com", time.Now().UnixNano()),
		"fake-hash-for-test",
	)
	if err != nil {
		t.Fatalf("create second user: %v", err)
	}

	otherStats, err := db.JobStatsByOwner(otherUser.ID)
	if err != nil {
		t.Fatalf("job stats of second owner: %v", err)
	}

	if otherStats.Total != 0 {
		t.Fatalf(
			"another owner must not see these jobs, got %+v",
			otherStats,
		)
	}
}
