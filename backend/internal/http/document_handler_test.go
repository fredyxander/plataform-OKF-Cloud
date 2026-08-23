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

	return NewDocumentHandler(service, nil)
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

// documento >10MB, size establecido por nosotros
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

// El navegador no siempre declara un tipo útil: en Windows un .md suele
// llegar sin Content-Type, que en multipart se traduce a
// application/octet-stream. La extensión resuelve esos casos.
func TestDetectFormat(t *testing.T) {
	cases := []struct {
		name        string
		contentType string
		filename    string
		format      string
		ok          bool
	}{
		{"markdown declarado", "text/markdown", "a.md", "markdown", true},
		{"texto declarado", "text/plain", "a.txt", "plaintext", true},
		{"con charset", "text/plain; charset=utf-8", "a.txt", "plaintext", true},
		{"markdown sin tipo", "", "a.md", "markdown", true},
		{"markdown como binario", "application/octet-stream", "a.md", "markdown", true},
		{"texto como binario", "application/octet-stream", "a.txt", "plaintext", true},
		{"extension en mayusculas", "application/octet-stream", "A.MD", "markdown", true},
		{"binario sin extension conocida", "application/octet-stream", "a.pdf", "", false},
		{"sin tipo ni extension", "", "a", "", false},
		{"tipo no admitido", "application/pdf", "a.md", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			format, ok := detectFormat(tc.contentType, tc.filename)

			if ok != tc.ok {
				t.Fatalf("expected ok=%v, got %v", tc.ok, ok)
			}

			if format != tc.format {
				t.Fatalf(
					"expected format %q, got %q",
					tc.format,
					format,
				)
			}
		})
	}
}
