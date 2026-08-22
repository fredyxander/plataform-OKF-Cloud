package okf

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestPackageBundle(t *testing.T) {
	bundle := &Bundle{
		Files: []File{
			{
				Name:    "index.md",
				Content: []byte("# Index"),
			},
			{
				Name:    "log.md",
				Content: []byte("# Log"),
			},
			{
				Name:    "concept-01.md",
				Content: []byte("# Concept"),
			},
		},
		ConceptCount: 1,
	}

	data, err := PackageBundle(bundle)
	if err != nil {
		t.Fatalf("PackageBundle returned error: %v", err)
	}

	reader, err := zip.NewReader(
		bytes.NewReader(data),
		int64(len(data)),
	)
	if err != nil {
		t.Fatalf("could not open generated zip: %v", err)
	}

	if len(reader.File) != 3 {
		t.Fatalf(
			"expected 3 files in zip, got %d",
			len(reader.File),
		)
	}

	expected := []string{
		"index.md",
		"log.md",
		"concept-01.md",
	}

	for i, name := range expected {
		if reader.File[i].Name != name {
			t.Errorf(
				"expected file %q, got %q",
				name,
				reader.File[i].Name,
			)
		}
	}
}
