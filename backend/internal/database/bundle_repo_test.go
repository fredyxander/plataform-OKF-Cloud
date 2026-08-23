package database

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/fredyxander/okf-platform/backend/internal/domain"
)

func TestBundleRepository(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// 1. Crear usuario.
	user, err := db.CreateUser(
		fmt.Sprintf("bundle-test-%d@example.com", time.Now().UnixNano()),
		"fake-hash-for-test",
	)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// 2. Crear documento.
	document, err := db.CreateDocument(
		user.ID,
		"bundle-test.txt",
		fmt.Sprintf("documents/%d/bundle-test.txt", time.Now().UnixNano()),
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

	// 4. Crear bundle válido con advertencias.
	//
	//    Un bundle con advertencias sigue siendo publicable, por lo que
	//    debe quedar con is_valid = true y published_at.
	bundle, err := db.CreateBundle(
		job.ID,
		user.ID,
		fmt.Sprintf("bundles/%s", job.ID),
		3,
		domain.BundleValidation{
			Status:   domain.BundleValidWithWarnings,
			Warnings: []string{"concept concept-02.md has no content"},
			Errors:   []string{},
		},
	)
	if err != nil {
		t.Fatalf("create bundle: %v", err)
	}

	if bundle.Validation.Status != domain.BundleValidWithWarnings {
		t.Fatalf(
			"expected %s, got %s",
			domain.BundleValidWithWarnings,
			bundle.Validation.Status,
		)
	}

	if len(bundle.Validation.Warnings) != 1 {
		t.Fatalf(
			"expected one persisted warning, got %v",
			bundle.Validation.Warnings,
		)
	}

	if bundle.ID == "" {
		t.Fatal("expected generated bundle ID")
	}

	if !bundle.IsValid {
		t.Fatal("expected valid bundle")
	}

	if bundle.PublishedAt == nil {
		t.Fatal("expected published_at for valid bundle")
	}

	// 5. Recuperarlo.
	found, err := db.GetBundleByJobID(job.ID, user.ID)
	if err != nil {
		t.Fatalf("get bundle: %v", err)
	}

	if found.ID != bundle.ID {
		t.Fatalf("expected bundle ID %s, got %s", bundle.ID, found.ID)
	}

	// 6. Comprobar aislamiento.
	otherUser, err := db.CreateUser(
		fmt.Sprintf("bundle-other-%d@example.com", time.Now().UnixNano()),
		"fake-hash-for-test",
	)
	if err != nil {
		t.Fatalf("create second user: %v", err)
	}

	_, err = db.GetBundleByJobID(job.ID, otherUser.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for another owner, got %v", err)
	}

	// 6.1. La clasificación sobrevive al round-trip.
	if found.Validation.Status != domain.BundleValidWithWarnings {
		t.Fatalf(
			"expected persisted validation status, got %s",
			found.Validation.Status,
		)
	}

	if len(found.Validation.Warnings) != 1 {
		t.Fatalf(
			"expected persisted warnings, got %v",
			found.Validation.Warnings,
		)
	}

	// 7. El mismo job no puede tener dos bundles.
	_, err = db.CreateBundle(
		job.ID,
		user.ID,
		fmt.Sprintf("bundles/%s-duplicate", job.ID),
		3,
		domain.BundleValidation{
			Status:   domain.BundleValid,
			Warnings: []string{},
			Errors:   []string{},
		},
	)
	if err == nil {
		t.Fatal("expected error when creating duplicate bundle for same job")
	}
}

// Un bundle rechazado por la validación se registra como evidencia,
// pero nunca queda publicado ni marcado como válido.
func TestCreateInvalidBundleIsNotPublished(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	user, err := db.CreateUser(
		fmt.Sprintf("bundle-invalid-%d@example.com", time.Now().UnixNano()),
		"fake-hash-for-test",
	)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	document, err := db.CreateDocument(
		user.ID,
		"invalid-bundle.md",
		fmt.Sprintf("documents/%d/invalid-bundle.md", time.Now().UnixNano()),
		"markdown",
		100,
	)
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	job, err := db.CreateJob(
		document.ID,
		user.ID,
		fmt.Sprintf("idempotency-invalid-%d", time.Now().UnixNano()),
	)
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	bundle, err := db.CreateBundle(
		job.ID,
		user.ID,
		fmt.Sprintf("bundles/%s", job.ID),
		1,
		domain.BundleValidation{
			Status:   domain.BundleInvalid,
			Warnings: []string{},
			Errors:   []string{"bundle is missing index.md"},
		},
	)
	if err != nil {
		t.Fatalf("create invalid bundle: %v", err)
	}

	if bundle.IsValid {
		t.Fatal("an invalid bundle must not be marked as valid")
	}

	if bundle.PublishedAt != nil {
		t.Fatal("an invalid bundle must not be published")
	}

	if len(bundle.Validation.Errors) != 1 {
		t.Fatalf(
			"expected the validation errors to be persisted, got %v",
			bundle.Validation.Errors,
		)
	}
}