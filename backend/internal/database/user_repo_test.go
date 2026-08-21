package database

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
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

	email := fmt.Sprintf(
		"test-user-%d@example.com",
		time.Now().UnixNano(),
	)
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

func TestCreateUserDuplicateEmail(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Fatal("DATABASE_URL is required")
	}

	db, err := New(dsn)
	if err != nil {
		t.Fatalf("connect to database: %v", err)
	}
	defer db.Close()

	email := fmt.Sprintf(
		"duplicate-user-%d@example.com",
		time.Now().UnixNano(),
	)

	_, err = db.CreateUser(email, "fake-hash-1")
	if err != nil {
		t.Fatalf("create first user: %v", err)
	}

	_, err = db.CreateUser(email, "fake-hash-2")

	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf(
			"expected ErrAlreadyExists, got %v",
			err,
		)
	}
}