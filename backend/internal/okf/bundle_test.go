package okf

import "testing"

func TestBundleRepresentation(t *testing.T) {
	bundle := Bundle{
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

	if len(bundle.Files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(bundle.Files))
	}

	if bundle.ConceptCount != 1 {
		t.Fatalf(
			"expected 1 concept, got %d",
			bundle.ConceptCount,
		)
	}
}

func TestBuildBundle(t *testing.T) {
	concepts := []Concept{
		{
			Title:   "Introduction",
			Content: "# Introduction\n\nHello OKF",
		},
	}

	bundle, err := BuildBundle(
		"test.md",
		"markdown",
		concepts,
	)
	if err != nil {
		t.Fatalf("BuildBundle returned error: %v", err)
	}

	if bundle.ConceptCount != 1 {
		t.Fatalf(
			"expected 1 concept, got %d",
			bundle.ConceptCount,
		)
	}

	if len(bundle.Files) != 3 {
		t.Fatalf(
			"expected 3 files, got %d",
			len(bundle.Files),
		)
	}

	expectedNames := []string{
		"index.md",
		"log.md",
		"concept-01.md",
	}

	for i, expected := range expectedNames {
		if bundle.Files[i].Name != expected {
			t.Errorf(
				"expected file %q, got %q",
				expected,
				bundle.Files[i].Name,
			)
		}
	}
}

func TestBuildBundleWithoutConcepts(t *testing.T) {
	_, err := BuildBundle(
		"test.md",
		"markdown",
		nil,
	)

	if err == nil {
		t.Fatal("expected error when bundle has no concepts")
	}
}
