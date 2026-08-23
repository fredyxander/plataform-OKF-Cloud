package httpapi

import (
	"encoding/json"
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

	response := buildJobDetailResponse(job, nil, bundle)

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

// El detalle lleva el documento de origen para que la vista pueda
// mostrar qué se subió y no un UUID.
func TestBuildJobDetailCarriesDocument(t *testing.T) {
	job := &domain.Job{
		ID:         "job-123",
		DocumentID: "doc-123",
		Status:     domain.JobStatusProcessing,
	}

	document := &domain.Document{
		ID:       "doc-123",
		Filename: "manual.md",
		Format:   "markdown",
	}

	response := buildJobDetailResponse(job, document, nil)

	if response.Document.Filename != "manual.md" {
		t.Errorf(
			"unexpected filename: %s",
			response.Document.Filename,
		)
	}

	if response.Document.Format != "markdown" {
		t.Errorf("unexpected format: %s", response.Document.Format)
	}

	if response.Document.ID != "doc-123" {
		t.Errorf("unexpected document id: %s", response.Document.ID)
	}
}

// Si el documento no se pudo resolver, el detalle sigue siendo válido y
// conserva al menos el identificador que el Job ya conocía.
func TestBuildJobDetailWithoutDocumentKeepsID(t *testing.T) {
	job := &domain.Job{
		ID:         "job-123",
		DocumentID: "doc-123",
		Status:     domain.JobStatusProcessing,
	}

	response := buildJobDetailResponse(job, nil, nil)

	if response.Document.ID != "doc-123" {
		t.Errorf("unexpected document id: %s", response.Document.ID)
	}

	if response.Document.Filename != "" {
		t.Errorf(
			"expected empty filename, got %s",
			response.Document.Filename,
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

	response := buildJobDetailResponse(job, nil, bundle)

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

	response := buildJobDetailResponse(job, nil, bundle)

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

	response := buildJobDetailResponse(job, nil, nil)

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

	response := buildJobDetailResponse(job, nil, nil)

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

// El seguimiento se detiene en los estados terminales: el cliente no
// debe tener que codificar por su cuenta cuáles lo son.
func TestJobDetailTerminalFlag(t *testing.T) {
	cases := []struct {
		status   domain.JobStatus
		terminal bool
	}{
		{domain.JobStatusQueued, false},
		{domain.JobStatusProcessing, false},
		{domain.JobStatusCompleted, true},
		{domain.JobStatusFailed, true},
	}

	for _, c := range cases {
		job := &domain.Job{ID: "job-1", Status: c.status}

		response := buildJobDetailResponse(job, nil, nil)

		if response.Terminal != c.terminal {
			t.Errorf(
				"status %s: expected terminal=%v, got %v",
				c.status,
				c.terminal,
				response.Terminal,
			)
		}
	}
}

// Una lista vacía debe serializarse como [] y nunca como null: un
// cliente que recorra la respuesta fallaría con null.
func TestJobListEmptySerializesAsArray(t *testing.T) {
	encoded, err := json.Marshal(buildJobListResponse(nil))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if string(encoded) != "[]" {
		t.Fatalf("expected [], got %s", encoded)
	}
}

// El listado debe bastar para pintar la vista de Jobs sin pedir el
// detalle de cada uno.
func TestJobListCarriesDocumentAndBundle(t *testing.T) {
	items := []*domain.JobListItem{
		{
			Job: domain.Job{
				ID:         "job-completed",
				DocumentID: "doc-1",
				Status:     domain.JobStatusCompleted,
			},
			DocumentFilename: "manual.md",
			DocumentFormat:   "markdown",
			Bundle: &domain.Bundle{
				ID:           "bundle-1",
				IsValid:      true,
				ConceptCount: 4,
				Validation: domain.BundleValidation{
					Status: domain.BundleValid,
				},
			},
		},
		{
			Job: domain.Job{
				ID:         "job-processing",
				DocumentID: "doc-2",
				Status:     domain.JobStatusProcessing,
			},
			DocumentFilename: "notas.txt",
			DocumentFormat:   "plaintext",
		},
	}

	response := buildJobListResponse(items)

	if len(response) != 2 {
		t.Fatalf("expected 2 items, got %d", len(response))
	}

	completed := response[0]

	if completed.Document.Filename != "manual.md" {
		t.Errorf("expected the filename, got %q", completed.Document.Filename)
	}

	if !completed.Terminal {
		t.Error("a completed job must be terminal")
	}

	if completed.Bundle == nil {
		t.Fatal("expected the bundle of a completed job")
	}

	if completed.Bundle.DownloadURL != "/jobs/job-completed/bundle" {
		t.Errorf("unexpected download URL: %q", completed.Bundle.DownloadURL)
	}

	processing := response[1]

	if processing.Terminal {
		t.Error("a processing job must not be terminal")
	}

	if processing.Bundle != nil {
		t.Error("a processing job has no bundle yet")
	}
}

// Un Job fallido cuyo bundle fue rechazado se lista con su validación
// pero sin ofrecer descarga.
func TestJobListRejectedBundleHasNoDownload(t *testing.T) {
	message := "bundle validation failed: bundle is missing index.md"

	items := []*domain.JobListItem{
		{
			Job: domain.Job{
				ID:           "job-failed",
				DocumentID:   "doc-3",
				Status:       domain.JobStatusFailed,
				ErrorMessage: &message,
			},
			DocumentFilename: "roto.md",
			DocumentFormat:   "markdown",
			Bundle: &domain.Bundle{
				ID:      "bundle-3",
				IsValid: false,
				Validation: domain.BundleValidation{
					Status: domain.BundleInvalid,
					Errors: []string{"bundle is missing index.md"},
				},
			},
		},
	}

	item := buildJobListResponse(items)[0]

	if !item.Terminal {
		t.Error("a failed job must be terminal")
	}

	if item.ErrorMessage == nil {
		t.Fatal("expected the error message in the list")
	}

	if item.Bundle == nil {
		t.Fatal("expected the rejected bundle to be reported")
	}

	if item.Bundle.DownloadURL != "" {
		t.Errorf("a rejected bundle must not be downloadable, got %q",
			item.Bundle.DownloadURL)
	}

	if item.Bundle.Validation.Status != domain.BundleInvalid {
		t.Errorf("unexpected validation status: %s", item.Bundle.Validation.Status)
	}
}
