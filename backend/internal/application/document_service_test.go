package application

// DocumentService.CreateDocument()
// - MinIO: guarda archivo
// - PostgreSQL: guarda metadata
// Luego storage_key coincide

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/fredyxander/okf-platform/backend/internal/database"
	"github.com/fredyxander/okf-platform/backend/internal/domain"
	"github.com/fredyxander/okf-platform/backend/internal/storage"
)

type failingDocumentRepository struct {
	storageKey string
}

func (r *failingDocumentRepository) CreateDocument(
	ownerID string,
	filename string,
	storageKey string,
	format string,
	sizeBytes int64,
) (*domain.Document, error) {
	r.storageKey = storageKey
	return nil, errors.New("simulated database error")
}

func (r *failingDocumentRepository) GetDocumentByID(
	id string,
	ownerID string,
) (*domain.Document, error) {
	return nil, errors.New("not implemented")
}

func TestDocumentServiceCreateDocument(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Fatal("DATABASE_URL is required")
	}

	db, err := database.New(dsn)
	if err != nil {
		t.Fatalf("connect to database: %v", err)
	}
	defer db.Close()

	endpoint := os.Getenv("MINIO_ENDPOINT")
	accessKey := os.Getenv("MINIO_ACCESS_KEY")
	secretKey := os.Getenv("MINIO_SECRET_KEY")
	bucket := os.Getenv("MINIO_BUCKET")

	if endpoint == "" || accessKey == "" || secretKey == "" || bucket == "" {
		t.Fatal("MinIO environment variables are required")
	}

	useSSL, err := strconv.ParseBool(os.Getenv("MINIO_USE_SSL"))
	if err != nil {
		t.Fatalf("invalid MINIO_USE_SSL: %v", err)
	}

	minioStorage, err := storage.NewMinIO(
		endpoint,
		accessKey,
		secretKey,
		useSSL,
		bucket,
	)
	if err != nil {
		t.Fatalf("create MinIO client: %v", err)
	}

	ctx := context.Background()

	if err := minioStorage.EnsureBucket(ctx); err != nil {
		t.Fatalf("ensure bucket: %v", err)
	}

	user, err := db.CreateUser(
		fmt.Sprintf("document-service-%d@example.com", time.Now().UnixNano()),
		"fake-hash",
	)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	service := NewDocumentService(db, minioStorage)

	content := []byte("Hello from DocumentService integration test")

	document, err := service.CreateDocument(
		ctx,
		user.ID,
		"test.txt",
		"plaintext",
		"text/plain",
		int64(len(content)),
		bytes.NewReader(content),
	)
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	if document.ID == "" {
		t.Fatal("expected document ID")
	}

	if document.StorageKey == "" {
		t.Fatal("expected storage key")
	}

	found, err := db.GetDocumentByID(document.ID, user.ID)
	if err != nil {
		t.Fatalf("get document from database: %v", err)
	}

	if found.StorageKey != document.StorageKey {
		t.Fatalf(
			"expected storage key %s, got %s",
			document.StorageKey,
			found.StorageKey,
		)
	}

	object, err := minioStorage.GetObject(ctx, document.StorageKey)
	if err != nil {
		t.Fatalf("get object from MinIO: %v", err)
	}
	defer object.Close()

	data, err := io.ReadAll(object)
	if err != nil {
		t.Fatalf("read object: %v", err)
	}

	if int64(len(data)) != int64(len(content)) {
		t.Fatalf(
			"expected object size %d, got %d",
			len(content),
			len(data),
		)
	}
}

func TestDocumentServiceDeletesObjectWhenRepositoryFails(t *testing.T) {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	accessKey := os.Getenv("MINIO_ACCESS_KEY")
	secretKey := os.Getenv("MINIO_SECRET_KEY")
	bucket := os.Getenv("MINIO_BUCKET")

	if endpoint == "" || accessKey == "" || secretKey == "" || bucket == "" {
		t.Fatal("MinIO environment variables are required")
	}

	useSSL, err := strconv.ParseBool(os.Getenv("MINIO_USE_SSL"))
	if err != nil {
		t.Fatalf("invalid MINIO_USE_SSL: %v", err)
	}

	minioStorage, err := storage.NewMinIO(
		endpoint,
		accessKey,
		secretKey,
		useSSL,
		bucket,
	)
	if err != nil {
		t.Fatalf("create MinIO client: %v", err)
	}

	ctx := context.Background()

	if err := minioStorage.EnsureBucket(ctx); err != nil {
		t.Fatalf("ensure bucket: %v", err)
	}

	repository := &failingDocumentRepository{}

	service := NewDocumentService(
		repository,
		minioStorage,
	)

	content := []byte("this object should be deleted")

	_, err = service.CreateDocument(
		ctx,
		"test-owner",
		"failed.txt",
		"plaintext",
		"text/plain",
		int64(len(content)),
		bytes.NewReader(content),
	)

	if err == nil {
		t.Fatal("expected CreateDocument to fail")
	}

	if repository.storageKey == "" {
		t.Fatal("expected repository to receive storage key")
	}

	object, err := minioStorage.GetObject(ctx, repository.storageKey)
	if err != nil {
		t.Fatalf("get object: %v", err)
	}
	defer object.Close()

	_, err = io.ReadAll(object)
	if err == nil {
		t.Fatal("expected object to have been deleted from MinIO")
	}
}
