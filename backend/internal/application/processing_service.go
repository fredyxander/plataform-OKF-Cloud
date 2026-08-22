package application

import (
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"

	"github.com/fredyxander/okf-platform/backend/internal/domain"
)

type JobRepository interface {
	CreateJob(
		documentID string,
		ownerID string,
		idempotencyKey string,
	) (*domain.Job, error)

	UpdateJobStatus(
		id string,
		status domain.JobStatus,
		errMsg *string,
	) error
}

type JobPublisher interface {
	PublishJob(
		ctx context.Context,
		job domain.JobMessage,
	) error
}

type ProcessingResult struct {
	Document *domain.Document
	Job      *domain.Job
}

type DocumentCreator interface {
	CreateDocument(
		ctx context.Context,
		ownerID string,
		filename string,
		format string,
		contentType string,
		size int64,
		reader io.Reader,
	) (*domain.Document, error)
}

type ProcessingService struct {
	documentService DocumentCreator
	jobRepository   JobRepository
	jobPublisher    JobPublisher
}

func NewProcessingService(
	documentService DocumentCreator,
	jobRepository JobRepository,
	jobPublisher JobPublisher,
) *ProcessingService {
	return &ProcessingService{
		documentService: documentService,
		jobRepository:   jobRepository,
		jobPublisher:    jobPublisher,
	}
}

func (s *ProcessingService) CreateProcessingJob(
	ctx context.Context,
	ownerID string,
	filename string,
	format string,
	contentType string,
	size int64,
	reader io.Reader,
) (*ProcessingResult, error) {

	document, err := s.documentService.CreateDocument(
		ctx,
		ownerID,
		filename,
		format,
		contentType,
		size,
		reader,
	)
	if err != nil {
		return nil, fmt.Errorf("create document: %w", err)
	}

	job, err := s.jobRepository.CreateJob(
		document.ID,
		ownerID,
		uuid.NewString(),
	)
	if err != nil {
		return nil, fmt.Errorf("create job: %w", err)
	}

	message := domain.JobMessage{
		JobID: job.ID,
	}

	if err := s.jobPublisher.PublishJob(ctx, message); err != nil {
		errMsg := fmt.Sprintf(
			"could not publish job: %v",
			err,
		)

		if updateErr := s.jobRepository.UpdateJobStatus(
			job.ID,
			domain.JobStatusFailed,
			&errMsg,
		); updateErr != nil {
			return nil, fmt.Errorf(
				"publish job: %w; additionally could not mark job failed: %v",
				err,
				updateErr,
			)
		}

		return nil, fmt.Errorf("publish job: %w", err)
	}

	return &ProcessingResult{
		Document: document,
		Job:      job,
	}, nil
}
