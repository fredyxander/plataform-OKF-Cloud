package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/fredyxander/okf-platform/backend/internal/domain"
)

func (db *DB) CreateDocument(ownerID, filename, minioKey, format string, sizeBytes int64) (*domain.Document, error) {
	d := &domain.Document{}
	err := db.conn.QueryRow(`
		INSERT INTO documents (owner_id, filename, storage_key, format, size_bytes)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, owner_id, filename, storage_key, format, size_bytes, created_at`,
		ownerID, filename, minioKey, format, sizeBytes,
	).Scan(&d.ID, &d.OwnerID, &d.Filename, &d.StorageKey, &d.Format, &d.SizeBytes, &d.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create document: %w", err)
	}
	return d, nil
}

func (db *DB) GetDocumentByID(id, ownerID string) (*domain.Document, error) {
	d := &domain.Document{}
	err := db.conn.QueryRow(`
		SELECT id, owner_id, filename, storage_key, format, size_bytes, created_at
		FROM documents WHERE id = $1 AND owner_id = $2`,
		id, ownerID,
	).Scan(&d.ID, &d.OwnerID, &d.Filename, &d.StorageKey, &d.Format, &d.SizeBytes, &d.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}
	return d, nil
}
