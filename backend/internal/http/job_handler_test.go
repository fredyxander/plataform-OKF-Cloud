package httpapi

import (
	"testing"

	"github.com/fredyxander/okf-platform/backend/internal/domain"
)

func TestBuildJobDetailCompleted(t *testing.T) {
	job := &domain.Job{
		ID:     "job-123",
		Status: domain.JobStatusCompleted,
	}

	bundle := &domain.Bundle{
		ID:           "bundle-123",
		JobID:        "job-123",
		IsValid:      true,
		ConceptCount: 3,
	}

	response := buildJobDetailResponse(job, bundle)

	if response.Bundle == nil {
		t.Fatal("expected bundle for completed job")
	}

	if response.Bundle.ID != "bundle-123" {
		t.Errorf("unexpected bundle id: %s", response.Bundle.ID)
	}

	if response.Bundle.ConceptCount != 3 {
		t.Errorf(
			"expected 3 concepts, got %d",
			response.Bundle.ConceptCount,
		)
	}

	if response.Bundle.DownloadURL != "/jobs/job-123/bundle" {
		t.Errorf(
			"unexpected download URL: %s",
			response.Bundle.DownloadURL,
		)
	}
}

func TestBuildJobDetailProcessing(t *testing.T) {
	job := &domain.Job{
		ID:     "job-123",
		Status: domain.JobStatusProcessing,
	}

	response := buildJobDetailResponse(job, nil)

	if response.Bundle != nil {
		t.Fatal("expected nil bundle for processing job")
	}
}

func TestBuildJobDetailFailed(t *testing.T) {
	errorMessage := "document is empty"

	job := &domain.Job{
		ID:           "job-123",
		Status:       domain.JobStatusFailed,
		ErrorMessage: &errorMessage,
	}

	response := buildJobDetailResponse(job, nil)

	if response.Bundle != nil {
		t.Fatal("expected nil bundle for failed job")
	}

	if response.ErrorMessage == nil {
		t.Fatal("expected error message for failed job")
	}

	if *response.ErrorMessage != "document is empty" {
		t.Errorf(
			"unexpected error message: %s",
			*response.ErrorMessage,
		)
	}
}
