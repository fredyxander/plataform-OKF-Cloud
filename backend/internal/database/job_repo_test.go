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
		"",
		"",
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

	// 7.1. El listado trae el documento de origen, para que una vista
	//      de lista no muestre solo UUIDs.
	listed := jobs[0]

	if listed.DocumentFilename != "job-test.txt" {
		t.Errorf(
			"expected the source filename, got %q",
			listed.DocumentFilename,
		)
	}

	if listed.DocumentFormat != "plaintext" {
		t.Errorf("expected the format, got %q", listed.DocumentFormat)
	}

	// 7.2. Sin bundle todavía, el campo llega nil y no falla el escaneo
	//      de las columnas nulas del LEFT JOIN.
	if listed.Bundle != nil {
		t.Errorf("expected no bundle yet, got %v", listed.Bundle)
	}

	// 7.3. Una vez publicado, el listado lo incluye con su clasificación.
	if _, err := db.CreateBundle(
		job.ID,
		user.ID,
		fmt.Sprintf("bundles/%s/bundle.zip", job.ID),
		2,
		domain.BundleValidation{
			Status:   domain.BundleValidWithWarnings,
			Warnings: []string{"el concepto concept-02.md no tiene contenido"},
			Errors:   []string{},
		},
	); err != nil {
		t.Fatalf("create bundle: %v", err)
	}

	jobs, err = db.ListJobsByOwner(user.ID)
	if err != nil {
		t.Fatalf("list jobs after bundle: %v", err)
	}

	listed = jobs[0]

	if listed.Bundle == nil {
		t.Fatal("expected the published bundle in the listing")
	}

	if listed.Bundle.ConceptCount != 2 {
		t.Errorf("expected 2 concepts, got %d", listed.Bundle.ConceptCount)
	}

	if listed.Bundle.Validation.Status != domain.BundleValidWithWarnings {
		t.Errorf(
			"expected %s, got %s",
			domain.BundleValidWithWarnings,
			listed.Bundle.Validation.Status,
		)
	}

	if len(listed.Bundle.Validation.Warnings) != 1 {
		t.Errorf(
			"expected the warnings in the listing, got %v",
			listed.Bundle.Validation.Warnings,
		)
	}

	// 8. Comprobar aislamiento.
	otherUser, err := db.CreateUser(
		fmt.Sprintf("job-other-%d@example.com", time.Now().UnixNano()),
		"fake-hash-for-test",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("create second user: %v", err)
	}

	_, err = db.GetJobByID(job.ID, otherUser.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for another owner, got %v", err)
	}

	// 8.1. Un propietario sin Jobs recibe una lista vacía, nunca nil.
	otherJobs, err := db.ListJobsByOwner(otherUser.ID)
	if err != nil {
		t.Fatalf("list jobs of second owner: %v", err)
	}

	if otherJobs == nil {
		t.Fatal("expected an empty slice, got nil")
	}

	if len(otherJobs) != 0 {
		t.Fatalf("expected no jobs for another owner, got %d", len(otherJobs))
	}
}

//Test para que un job queued no pueda ser tomado por dos workers a la vez, el segundo en tomarlo debe fallar.
func TestClaimJobForProcessingOnlyOnce(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// 1. Crear owner.
	user, err := db.CreateUser(
		fmt.Sprintf("claim-job-test-%d@example.com", time.Now().UnixNano()),
		"fake-hash-for-test",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// 2. Crear documento.
	document, err := db.CreateDocument(
		user.ID,
		"claim-job-test.txt",
		fmt.Sprintf("documents/%d/claim-job-test.txt", time.Now().UnixNano()),
		"plaintext",
		100,
	)
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	// 3. Crear Job. Debe comenzar en queued.
	job, err := db.CreateJob(
		document.ID,
		user.ID,
		fmt.Sprintf("claim-idempotency-%d", time.Now().UnixNano()),
	)
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	if job.Status != domain.JobStatusQueued {
		t.Fatalf(
			"expected initial status %s, got %s",
			domain.JobStatusQueued,
			job.Status,
		)
	}

	// 4. Primer claim: debe funcionar.
	staleBefore := time.Now().Add(-5 * time.Minute)

	claimedJob, err := db.ClaimJobForProcessing(
		job.ID,
		staleBefore,
	)
	if err != nil {
		t.Fatalf("first claim should succeed: %v", err)
	}

	if claimedJob.Status != domain.JobStatusProcessing {
		t.Fatalf(
			"expected claimed job status %s, got %s",
			domain.JobStatusProcessing,
			claimedJob.Status,
		)
	}

	// 5. Segundo claim: debe fallar porque el Job
	// ya no está en estado queued.
	_, err = db.ClaimJobForProcessing(
		job.ID,
		staleBefore,
	)
	if !errors.Is(err, ErrJobNotClaimable) {
		t.Fatalf(
			"second claim should fail with ErrJobNotClaimable, got %v",
			err,
		)
	}

	_, err = db.ClaimJobForProcessing(
		"00000000-0000-0000-0000-000000000000",
		time.Now().Add(-5*time.Minute),
	)

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf(
			"nonexistent job should fail with ErrNotFound, got %v",
			err,
		)
	}
}

func TestClaimJobForProcessingRecoversStaleJob(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// 1. Crear owner.
	user, err := db.CreateUser(
		fmt.Sprintf("stale-job-test-%d@example.com", time.Now().UnixNano()),
		"fake-hash-for-test",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// 2. Crear documento.
	document, err := db.CreateDocument(
		user.ID,
		"stale-job-test.txt",
		fmt.Sprintf("documents/%d/stale-job-test.txt", time.Now().UnixNano()),
		"plaintext",
		100,
	)
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	// 3. Crear Job inicialmente queued.
	job, err := db.CreateJob(
		document.ID,
		user.ID,
		fmt.Sprintf("stale-idempotency-%d", time.Now().UnixNano()),
	)
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	// 4. Primer claim: queued -> processing.
	claimedJob, err := db.ClaimJobForProcessing(
		job.ID,
		time.Now().Add(-5*time.Minute),
	)
	if err != nil {
		t.Fatalf("first claim should succeed: %v", err)
	}

	if claimedJob.Status != domain.JobStatusProcessing {
		t.Fatalf(
			"expected processing status, got %s",
			claimedJob.Status,
		)
	}

	// 5. Intentar reclamarlo considerándolo stale solo si lleva
	// más de 5 minutos en processing.
	//
	// Acaba de ser actualizado, así que NO debería poder reclamarse.
	_, err = db.ClaimJobForProcessing(
		job.ID,
		time.Now().Add(-5*time.Minute),
	)

	if !errors.Is(err, ErrJobNotClaimable) {
		t.Fatalf(
			"recent processing job should not be claimable, got %v",
			err,
		)
	}

	// 6. Ahora usamos un staleBefore futuro.
	//
	// Esto simula que ha pasado suficiente tiempo:
	// updated_at < staleBefore será verdadero.
	recoveredJob, err := db.ClaimJobForProcessing(
		job.ID,
		time.Now().Add(5*time.Minute),
	)
	if err != nil {
		t.Fatalf("stale processing job should be claimable: %v", err)
	}

	if recoveredJob.Status != domain.JobStatusProcessing {
		t.Fatalf(
			"expected recovered job status %s, got %s",
			domain.JobStatusProcessing,
			recoveredJob.Status,
		)
	}
}

func TestRequeueJobFromProcessing(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	user, err := db.CreateUser(
		fmt.Sprintf("requeue-job-%d@example.com", time.Now().UnixNano()),
		"fake-hash-for-test",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	document, err := db.CreateDocument(
		user.ID,
		"requeue-test.txt",
		fmt.Sprintf("documents/%d/requeue-test.txt", time.Now().UnixNano()),
		"plaintext",
		100,
	)
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	job, err := db.CreateJob(
		document.ID,
		user.ID,
		fmt.Sprintf("requeue-%d", time.Now().UnixNano()),
	)
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	// queued -> processing
	_, err = db.ClaimJobForProcessing(
		job.ID,
		time.Now().Add(-5*time.Minute),
	)
	if err != nil {
		t.Fatalf("claim job: %v", err)
	}

	errorMessage := "temporary storage failure"

	// processing -> queued
	if err := db.RequeueJob(job.ID, &errorMessage); err != nil {
		t.Fatalf("requeue job: %v", err)
	}

	requeuedJob, err := db.GetJobByIDForProcessing(job.ID)
	if err != nil {
		t.Fatalf("get requeued job: %v", err)
	}

	if requeuedJob.Status != domain.JobStatusQueued {
		t.Fatalf(
			"expected status %s, got %s",
			domain.JobStatusQueued,
			requeuedJob.Status,
		)
	}

	if requeuedJob.ErrorMessage == nil {
		t.Fatal("expected error message to be persisted")
	}

	if *requeuedJob.ErrorMessage != errorMessage {
		t.Fatalf(
			"expected error message %q, got %q",
			errorMessage,
			*requeuedJob.ErrorMessage,
		)
	}
}

func TestRequeueJobRejectsNonProcessingJob(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	user, err := db.CreateUser(
		fmt.Sprintf("requeue-reject-%d@example.com", time.Now().UnixNano()),
		"fake-hash-for-test",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	document, err := db.CreateDocument(
		user.ID,
		"requeue-reject.txt",
		fmt.Sprintf("documents/%d/requeue-reject.txt", time.Now().UnixNano()),
		"plaintext",
		100,
	)
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	job, err := db.CreateJob(
		document.ID,
		user.ID,
		fmt.Sprintf("requeue-reject-%d", time.Now().UnixNano()),
	)
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	// El Job sigue QUEUED.
	errorMessage := "should not be requeued"

	err = db.RequeueJob(job.ID, &errorMessage)

	if !errors.Is(err, ErrJobNotClaimable) {
		t.Fatalf(
			"expected ErrJobNotClaimable, got %v",
			err,
		)
	}
}