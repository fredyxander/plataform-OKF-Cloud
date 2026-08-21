package httpapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fredyxander/okf-platform/backend/internal/application"
	"github.com/fredyxander/okf-platform/backend/internal/domain"
)

type fakeDocumentRepository struct{}

func (f *fakeDocumentRepository) CreateDocument(
	ownerID string,
	filename string,
	storageKey string,
	format string,
	sizeBytes int64,
) (*domain.Document, error) {
	return &domain.Document{
		ID:         "doc-id",
		OwnerID:    ownerID,
		Filename:   filename,
		StorageKey: storageKey,
		Format:     format,
		SizeBytes:  sizeBytes,
	}, nil
}

func (f *fakeDocumentRepository) GetDocumentByID(
	id string,
	ownerID string,
) (*domain.Document, error) {
	return nil, errors.New("not implemented")
}

type fakeObjectStorage struct{}

func (f *fakeObjectStorage) PutObject(
	ctx context.Context,
	objectKey string,
	reader io.Reader,
	size int64,
	contentType string,
) error {
	return nil
}

func (f *fakeObjectStorage) DeleteObject(
	ctx context.Context,
	objectKey string,
) error {
	return nil
}

func (f *fakeObjectStorage) GetObject(
	ctx context.Context,
	objectKey string,
) (io.ReadCloser, error) {
	return nil, errors.New("not implemented")
}

func newTestDocumentHandler() *DocumentHandler {
	service := application.NewDocumentService(
		&fakeDocumentRepository{},
		&fakeObjectStorage{},
	)

	return NewDocumentHandler(service)
}

func withUserID(req *http.Request, userID string) *http.Request {
	ctx := context.WithValue(
		req.Context(),
		userIDContextKey,
		userID,
	)

	return req.WithContext(ctx)
}

// cargar documento sin owner
func TestUploadWithoutOwner(t *testing.T) {
	handler := newTestDocumentHandler()

	req := httptest.NewRequest(
		http.MethodPost,
		"/documents",
		nil,
	)

	rec := httptest.NewRecorder()

	handler.Upload(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

// cargar documento sin archivo
func TestUploadWithoutFile(t *testing.T) {
	handler := newTestDocumentHandler()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/documents",
		&body,
	)

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = withUserID(req, "owner-id")

	rec := httptest.NewRecorder()

	handler.Upload(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// cargar documento con formato invalido
func TestUploadUnsupportedType(t *testing.T) {
	handler := newTestDocumentHandler()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", "test.pdf")
	if err != nil {
		t.Fatal(err)
	}

	_, _ = part.Write([]byte("fake pdf"))
	_ = writer.Close()

	req := httptest.NewRequest(
		http.MethodPost,
		"/documents",
		&body,
	)

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = withUserID(req, "owner-id")

	rec := httptest.NewRecorder()

	handler.Upload(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d", rec.Code)
	}
}

//documento >10MB, size establecido por nosotros
func TestUploadFileTooLarge(t *testing.T) {
	handler := newTestDocumentHandler()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", "large.txt")
	if err != nil {
		t.Fatal(err)
	}

	largeContent := make([]byte, 11*1024*1024)

	if _, err := part.Write(largeContent); err != nil {
		t.Fatal(err)
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/documents",
		&body,
	)

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = withUserID(req, "owner-id")

	rec := httptest.NewRecorder()

	handler.Upload(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", rec.Code)
	}
}