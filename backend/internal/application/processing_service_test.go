package application

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/fredyxander/okf-platform/backend/internal/domain"
)

type fakeDocumentCreator struct {
	document *domain.Document
	err      error
}

func (f *fakeDocumentCreator) CreateDocument(
	ctx context.Context,
	ownerID string,
	filename string,
	format string,
	contentType string,
	size int64,
	reader io.Reader,
) (*domain.Document, error) {
	return f.document, f.err
}

type fakeJobRepository struct {
	job *domain.Job
	err error

	statusUpdated bool
	updatedJobID  string
	updatedStatus domain.JobStatus
	updatedErrMsg *string
}

func (f *fakeJobRepository) CreateJob(
	documentID string,
	ownerID string,
	idempotencyKey string,
) (*domain.Job, error) {
	return f.job, f.err
}

func (f *fakeJobRepository) UpdateJobStatus(
	id string,
	status domain.JobStatus,
	errMsg *string,
) error {
	f.statusUpdated = true
	f.updatedJobID = id
	f.updatedStatus = status
	f.updatedErrMsg = errMsg

	return nil
}

type fakeJobPublisher struct {
	published bool
	job       domain.JobMessage
	err       error
}

func (f *fakeJobPublisher) PublishJob(
	ctx context.Context,
	job domain.JobMessage,
) error {
	f.published = true
	f.job = job

	return f.err
}

func TestCreateProcessingJobSuccess(t *testing.T) {
	documentCreator := &fakeDocumentCreator{
		document: &domain.Document{
			ID:      "document-123",
			OwnerID: "owner-123",
		},
	}

	jobRepository := &fakeJobRepository{
		job: &domain.Job{
			ID:         "job-123",
			DocumentID: "document-123",
			OwnerID:    "owner-123",
			Status:     domain.JobStatusQueued,
		},
	}

	jobPublisher := &fakeJobPublisher{}

	service := NewProcessingService(
		documentCreator,
		jobRepository,
		jobPublisher,
	)

	result, err := service.CreateProcessingJob(
		context.Background(),
		"owner-123",
		"test.md",
		"markdown",
		"text/markdown",
		5,
		strings.NewReader("hello"),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Document.ID != "document-123" {
		t.Errorf(
			"expected document-123, got %s",
			result.Document.ID,
		)
	}

	if result.Job.ID != "job-123" {
		t.Errorf(
			"expected job-123, got %s",
			result.Job.ID,
		)
	}

	if !jobPublisher.published {
		t.Fatal("expected job to be published")
	}

	if jobPublisher.job.JobID != "job-123" {
		t.Errorf(
			"expected published job job-123, got %s",
			jobPublisher.job.JobID,
		)
	}
}

// si crear el job falla, no se publica nada en la cola
func TestCreateProcessingJobCreateJobFails(t *testing.T) {
	documentCreator := &fakeDocumentCreator{
		document: &domain.Document{
			ID:      "document-123",
			OwnerID: "owner-123",
		},
	}

	jobRepository := &fakeJobRepository{
		err: fmt.Errorf("database error"),
	}

	jobPublisher := &fakeJobPublisher{}

	service := NewProcessingService(
		documentCreator,
		jobRepository,
		jobPublisher,
	)

	_, err := service.CreateProcessingJob(
		context.Background(),
		"owner-123",
		"test.md",
		"markdown",
		"text/markdown",
		5,
		strings.NewReader("hello"),
	)

	if err == nil {
		t.Fatal("expected error")
	}

	if jobPublisher.published {
		t.Fatal("job should not be published when CreateJob fails")
	}
}

// si el doc y el Job se crean, pero falla la publicación en la cola, el Job debe quedar en failed y retorna error
func TestCreateProcessingJobPublishFails(t *testing.T) {
	documentCreator := &fakeDocumentCreator{
		document: &domain.Document{
			ID:      "document-123",
			OwnerID: "owner-123",
		},
	}

	jobRepository := &fakeJobRepository{
		job: &domain.Job{
			ID:         "job-123",
			DocumentID: "document-123",
			OwnerID:    "owner-123",
			Status:     domain.JobStatusQueued,
		},
	}

	jobPublisher := &fakeJobPublisher{
		err: fmt.Errorf("rabbitmq unavailable"),
	}

	service := NewProcessingService(
		documentCreator,
		jobRepository,
		jobPublisher,
	)

	_, err := service.CreateProcessingJob(
		context.Background(),
		"owner-123",
		"test.md",
		"markdown",
		"text/markdown",
		5,
		strings.NewReader("hello"),
	)

	if err == nil {
		t.Fatal("expected error")
	}

	if !jobPublisher.published {
		t.Fatal("expected publish to be attempted")
	}

	if !jobRepository.statusUpdated {
		t.Fatal("expected job status to be updated")
	}

	if jobRepository.updatedJobID != "job-123" {
		t.Errorf(
			"expected job-123, got %s",
			jobRepository.updatedJobID,
		)
	}

	if jobRepository.updatedStatus != domain.JobStatusFailed {
		t.Errorf(
			"expected failed status, got %s",
			jobRepository.updatedStatus,
		)
	}

	if jobRepository.updatedErrMsg == nil {
		t.Fatal("expected error message")
	}
}
