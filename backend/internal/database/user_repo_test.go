package database

import (
	"os"
	"testing"
)

func TestCreateAndGetUser(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Fatal("DATABASE_URL is required")
	}

	db, err := New(dsn)
	if err != nil {
		t.Fatalf("connect to database: %v", err)
	}
	defer db.Close()

	email := "test-user@example.com"
	passwordHash := "fake-hash-for-test"

	user, err := db.CreateUser(email, passwordHash)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if user.ID == "" {
		t.Fatal("expected generated user ID")
	}

	found, err := db.GetUserByEmail(email)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}

	if found.ID != user.ID {
		t.Fatalf("expected user ID %s, got %s", user.ID, found.ID)
	}

	if found.Email != email {
		t.Fatalf("expected email %s, got %s", email, found.Email)
	}
}