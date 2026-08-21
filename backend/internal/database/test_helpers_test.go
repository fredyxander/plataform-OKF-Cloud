package database

import (
	"os"
	"testing"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Fatal("DATABASE_URL is required")
	}

	db, err := New(dsn)
	if err != nil {
		t.Fatalf("connect to database: %v", err)
	}

	return db
}