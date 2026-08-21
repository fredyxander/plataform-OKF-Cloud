package database

import (
	"errors"
	"fmt"
	"testing"
	"time"
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

	// 4. Crear bundle válido.
	bundle, err := db.CreateBundle(
		job.ID,
		user.ID,
		fmt.Sprintf("bundles/%s", job.ID),
		true,
		3,
	)
	if err != nil {
		t.Fatalf("create bundle: %v", err)
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

	// 7. El mismo job no puede tener dos bundles.
	_, err = db.CreateBundle(
		job.ID,
		user.ID,
		fmt.Sprintf("bundles/%s-duplicate", job.ID),
		true,
		3,
	)
	if err == nil {
		t.Fatal("expected error when creating duplicate bundle for same job")
	}
}