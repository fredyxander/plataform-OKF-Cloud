package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestPutObject(t *testing.T) {
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

	storage, err := NewMinIO(
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

	if err := storage.EnsureBucket(ctx); err != nil {
		t.Fatalf("ensure bucket: %v", err)
	}

	content := []byte("Hello from OKF MinIO integration test")

	objectKey := "tests/" + strconv.FormatInt(time.Now().UnixNano(), 10) + "/test.txt"

	err = storage.PutObject(
		ctx,
		objectKey,
		bytes.NewReader(content),
		int64(len(content)),
		"text/plain",
	)
	if err != nil {
		t.Fatalf("put object: %v", err)
	}

	t.Logf("object uploaded successfully: %s", objectKey)
}

func TestGetObject(t *testing.T) {
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

	storage, err := NewMinIO(
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

	if err := storage.EnsureBucket(ctx); err != nil {
		t.Fatalf("ensure bucket: %v", err)
	}

	expected := []byte("Hello from GetObject test")

	objectKey := "tests/" + strconv.FormatInt(time.Now().UnixNano(), 10) + "/get.txt"

	if err := storage.PutObject(
		ctx,
		objectKey,
		bytes.NewReader(expected),
		int64(len(expected)),
		"text/plain",
	); err != nil {
		t.Fatalf("put object: %v", err)
	}

	object, err := storage.GetObject(ctx, objectKey)
	if err != nil {
		t.Fatalf("get object: %v", err)
	}
	defer object.Close()

	actual, err := io.ReadAll(object)
	if err != nil {
		t.Fatalf("read object: %v", err)
	}

	if !bytes.Equal(actual, expected) {
		t.Fatalf("expected %q, got %q", expected, actual)
	}
}