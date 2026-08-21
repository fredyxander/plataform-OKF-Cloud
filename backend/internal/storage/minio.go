package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIO struct {
	client *minio.Client
	bucket string
}

func NewMinIO(
	endpoint string,
	accessKey string,
	secretKey string,
	useSSL bool,
	bucket string,
) (*MinIO, error) {

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create MinIO client: %w", err)
	}

	return &MinIO{
		client: client,
		bucket: bucket,
	}, nil
}

func (m *MinIO) EnsureBucket(ctx context.Context) error {
	exists, err := m.client.BucketExists(ctx, m.bucket)
	if err != nil {
		return fmt.Errorf("check MinIO bucket %q: %w", m.bucket, err)
	}

	if exists {
		return nil
	}

	err = m.client.MakeBucket(ctx, m.bucket, minio.MakeBucketOptions{})
	if err != nil {
		return fmt.Errorf("create MinIO bucket %q: %w", m.bucket, err)
	}

	return nil
}

func (m *MinIO) PutObject(
	ctx context.Context,
	objectKey string, //identificador del objeto dentro del bucket para almacenar luego en documents.storage_key,
	//asi postgress guarda la metadata ubicacion del archivo, y minIO el archivo.
	reader io.Reader,
	size int64,
	contentType string,
) error {
	_, err := m.client.PutObject(
		ctx,
		m.bucket,
		objectKey,
		reader,
		size,
		minio.PutObjectOptions{
			ContentType: contentType,
		},
	)
	if err != nil {
		return fmt.Errorf("put MinIO object %q: %w", objectKey, err)
	}

	return nil
}

func (m *MinIO) GetObject(
	ctx context.Context,
	objectKey string,
) (*minio.Object, error) {
	object, err := m.client.GetObject(
		ctx,
		m.bucket,
		objectKey,
		minio.GetObjectOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("get MinIO object %q: %w", objectKey, err)
	}

	return object, nil
}

// metodo para eliminar el documento, si la persistencia de metadata en postgres falla y no dejar objetos huerfanos.
func (m *MinIO) DeleteObject(
	ctx context.Context,
	objectKey string,
) error {
	err := m.client.RemoveObject(
		ctx,
		m.bucket,
		objectKey,
		minio.RemoveObjectOptions{},
	)
	if err != nil {
		return fmt.Errorf("delete MinIO object %q: %w", objectKey, err)
	}

	return nil
}
