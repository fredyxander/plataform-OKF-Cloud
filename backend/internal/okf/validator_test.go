package okf

import "testing"

func TestValidateBundle(t *testing.T) {
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

	if err := ValidateBundle(bundle); err != nil {
		t.Fatalf("expected valid bundle, got: %v", err)
	}
}

func TestValidateBundleMissingIndex(t *testing.T) {
	bundle := &Bundle{
		Files: []File{
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

	if err := ValidateBundle(bundle); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateBundleMissingLog(t *testing.T) {
	bundle := &Bundle{
		Files: []File{
			{
				Name:    "index.md",
				Content: []byte("- [Concept](concept-01.md)"),
			},
			{
				Name:    "concept-01.md",
				Content: []byte("# Concept"),
			},
		},
		ConceptCount: 1,
	}

	if err := ValidateBundle(bundle); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestValidateBundleBrokenIndexReference(t *testing.T) {
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

	if err := ValidateBundle(bundle); err == nil {
		t.Fatal("expected validation error")
	}
}
