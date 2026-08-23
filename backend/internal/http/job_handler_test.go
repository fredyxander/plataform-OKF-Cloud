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
		Validation: domain.BundleValidation{
			Status:   domain.BundleValid,
			Warnings: []string{},
			Errors:   []string{},
		},
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

	if response.Bundle.Validation.Status != domain.BundleValid {
		t.Errorf(
			"unexpected validation status: %s",
			response.Bundle.Validation.Status,
		)
	}
}

// Un bundle publicado con advertencias sigue siendo descargable y las
// advertencias viajan en el detalle del Job.
func TestBuildJobDetailCompletedWithWarnings(t *testing.T) {
	job := &domain.Job{
		ID:     "job-456",
		Status: domain.JobStatusCompleted,
	}

	bundle := &domain.Bundle{
		ID:           "bundle-456",
		JobID:        "job-456",
		IsValid:      true,
		ConceptCount: 2,
		Validation: domain.BundleValidation{
			Status:   domain.BundleValidWithWarnings,
			Warnings: []string{"concept concept-02.md has no content"},
			Errors:   []string{},
		},
	}

	response := buildJobDetailResponse(job, bundle)

	if response.Bundle == nil {
		t.Fatal("expected bundle for completed job")
	}

	if response.Bundle.Validation.Status != domain.BundleValidWithWarnings {
		t.Errorf(
			"expected %s, got %s",
			domain.BundleValidWithWarnings,
			response.Bundle.Validation.Status,
		)
	}

	if len(response.Bundle.Validation.Warnings) != 1 {
		t.Errorf(
			"expected the warnings to be exposed, got %v",
			response.Bundle.Validation.Warnings,
		)
	}

	if response.Bundle.DownloadURL != "/jobs/job-456/bundle" {
		t.Errorf(
			"a bundle with warnings must remain downloadable, got %q",
			response.Bundle.DownloadURL,
		)
	}
}

// Bundle incompleto: el Job falla, la clasificación queda visible y no
// se ofrece ninguna URL de descarga.
func TestBuildJobDetailInvalidBundleIsNotDownloadable(t *testing.T) {
	errorMessage := "bundle validation failed: bundle is missing index.md"

	job := &domain.Job{
		ID:           "job-789",
		Status:       domain.JobStatusFailed,
		ErrorMessage: &errorMessage,
	}

	bundle := &domain.Bundle{
		ID:           "bundle-789",
		JobID:        "job-789",
		IsValid:      false,
		ConceptCount: 1,
		Validation: domain.BundleValidation{
			Status:   domain.BundleInvalid,
			Warnings: []string{},
			Errors:   []string{"bundle is missing index.md"},
		},
	}

	response := buildJobDetailResponse(job, bundle)

	if response.Bundle == nil {
		t.Fatal("expected the rejected bundle to be reported")
	}

	if response.Bundle.Validation.Status != domain.BundleInvalid {
		t.Errorf(
			"expected %s, got %s",
			domain.BundleInvalid,
			response.Bundle.Validation.Status,
		)
	}

	if response.Bundle.DownloadURL != "" {
		t.Errorf(
			"an invalid bundle must not expose a download URL, got %q",
			response.Bundle.DownloadURL,
		)
	}

	if len(response.Bundle.Validation.Errors) != 1 {
		t.Errorf(
			"expected the validation errors to be exposed, got %v",
			response.Bundle.Validation.Errors,
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
