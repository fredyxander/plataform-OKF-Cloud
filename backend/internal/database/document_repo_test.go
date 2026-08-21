package database

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestCreateAndGetDocument(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Necesitamos primero un owner real porque documents.owner_id
	// tiene una FK hacia users.id.
	email := fmt.Sprintf("document-test-%d@example.com", time.Now().UnixNano())

	user, err := db.CreateUser(email, "fake-hash-for-test")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	document, err := db.CreateDocument(
		user.ID,
		"example.txt",
		fmt.Sprintf(
			"documents/test/%d/example.txt",
			time.Now().UnixNano(),
		),
		"plaintext",
		1234,
	)
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	if document.ID == "" {
		t.Fatal("expected generated document ID")
	}

	found, err := db.GetDocumentByID(document.ID, user.ID)
	if err != nil {
		t.Fatalf("get document: %v", err)
	}

	if found.ID != document.ID {
		t.Fatalf("expected document ID %s, got %s", document.ID, found.ID)
	}

	if found.OwnerID != user.ID {
		t.Fatalf("expected owner ID %s, got %s", user.ID, found.OwnerID)
	}

	// Verificar aislamiento por propietario.
	otherUser, err := db.CreateUser(
		fmt.Sprintf("other-user-%d@example.com", time.Now().UnixNano()),
		"fake-hash-for-test",
	)
	if err != nil {
		t.Fatalf("create second user: %v", err)
	}

	_, err = db.GetDocumentByID(document.ID, otherUser.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for another owner, got %v", err)
	}
}