package okf

import (
	"archive/zip"
	"bytes"
	"fmt"
)

func PackageBundle(bundle *Bundle) ([]byte, error) {
	if bundle == nil {
		return nil, fmt.Errorf("bundle is nil")
	}

	var buffer bytes.Buffer

	zipWriter := zip.NewWriter(&buffer)

	for _, file := range bundle.Files {
		writer, err := zipWriter.Create(file.Name)
		if err != nil {
			_ = zipWriter.Close()
			return nil, fmt.Errorf("create zip entry %s: %w", file.Name, err)
		}

		if _, err := writer.Write(file.Content); err != nil {
			_ = zipWriter.Close()
			return nil, fmt.Errorf("write zip entry %s: %w", file.Name, err)
		}
	}

	if err := zipWriter.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}

	return buffer.Bytes(), nil
}
