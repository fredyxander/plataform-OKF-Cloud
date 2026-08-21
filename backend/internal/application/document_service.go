package application

//Servicio para Coordinar agregar documento en minIO y metadata en postgreSQL
// Sin acoplarse directamente a las implementaciones.

import (
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"

	"github.com/fredyxander/okf-platform/backend/internal/domain"
)

type DocumentRepository interface {
	CreateDocument(
		ownerID string,
		filename string,
		storageKey string,
		format string,
		sizeBytes int64,
	) (*domain.Document, error)

	GetDocumentByID(
		id string,
		ownerID string,
	) (*domain.Document, error)
}

type ObjectStorage interface {
	PutObject(
		ctx context.Context,
		objectKey string,
		reader io.Reader,
		size int64,
		contentType string,
	) error

	DeleteObject(
		ctx context.Context,
		objectKey string,
	) error

	GetObject(
		ctx context.Context,
		objectKey string,
	) (io.ReadCloser, error)
}

type DocumentService struct {
	repository DocumentRepository //CreateDocument()
	storage    ObjectStorage      //PutObject()
}

func NewDocumentService(
	repository DocumentRepository,
	storage ObjectStorage,
) *DocumentService {
	return &DocumentService{
		repository: repository,
		storage:    storage,
	}
}

func (s *DocumentService) CreateDocument(
	ctx context.Context,
	ownerID string,
	filename string,
	format string,
	contentType string,
	size int64,
	reader io.Reader,
) (*domain.Document, error) {

	documentID := uuid.NewString()

	storageKey := fmt.Sprintf(
		"documents/%s/%s/%s",
		ownerID,
		documentID,
		filename,
	)

	if err := s.storage.PutObject(
		ctx,
		storageKey,
		reader,
		size,
		contentType,
	); err != nil {
		return nil, fmt.Errorf("store document: %w", err)
	}

	document, err := s.repository.CreateDocument(
		ownerID,
		filename,
		storageKey,
		format,
		size,
	)
	if err != nil {
		_ = s.storage.DeleteObject(ctx, storageKey)

		return nil, fmt.Errorf("persist document metadata: %w", err)
	}

	return document, nil
}

func (s *DocumentService) GetDocument(
	ctx context.Context,
	documentID string,
	ownerID string,
) (*domain.Document, io.ReadCloser, error) {

	document, err := s.repository.GetDocumentByID(
		documentID,
		ownerID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("get document metadata: %w", err)
	}

	object, err := s.storage.GetObject(
		ctx,
		document.StorageKey,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("get document object: %w", err)
	}

	return document, object, nil
}
