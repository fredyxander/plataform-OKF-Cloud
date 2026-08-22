package okf

import "testing"

func TestConvertMarkdown(t *testing.T) {
	content := []byte("# Introduction\n\nHello OKF")

	concepts, err := Convert("test.md", "markdown", content)
	if err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}

	if len(concepts) != 1 {
		t.Fatalf("expected 1 concept, got %d", len(concepts))
	}

	if concepts[0].Title != "Introduction" {
		t.Errorf(
			"expected title %q, got %q",
			"Introduction",
			concepts[0].Title,
		)
	}

	if concepts[0].Content != "Hello OKF" {
		t.Errorf(
			"expected content %q, got %q",
			"Hello OKF",
			concepts[0].Content,
		)
	}
}

func TestConvertPlaintext(t *testing.T) {
	content := []byte("Hello OKF")

	concepts, err := Convert("test.txt", "plaintext", content)
	if err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}

	if len(concepts) != 1 {
		t.Fatalf("expected 1 concept, got %d", len(concepts))
	}

	if concepts[0].Content != "Hello OKF" {
		t.Errorf("unexpected content: %q", concepts[0].Content)
	}
}

func TestConvertEmptyDocument(t *testing.T) {
	_, err := Convert("test.md", "markdown", []byte("   "))

	if err == nil {
		t.Fatal("expected error for empty document")
	}
}

func TestConvertUnsupportedFormat(t *testing.T) {
	_, err := Convert("test.pdf", "pdf", []byte("content"))

	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestConvertMarkdownWithSections(t *testing.T) {
	content := []byte(`# Introducción

Contenido de introducción.

# Arquitectura

Contenido de arquitectura.

# Conclusiones

Contenido final.`)

	concepts, err := Convert(
		"upload-test.md",
		"markdown",
		content,
	)
	if err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}

	if len(concepts) != 3 {
		t.Fatalf(
			"expected 3 concepts, got %d",
			len(concepts),
		)
	}

	expectedTitles := []string{
		"Introducción",
		"Arquitectura",
		"Conclusiones",
	}

	for i, expected := range expectedTitles {
		if concepts[i].Title != expected {
			t.Errorf(
				"expected title %q, got %q",
				expected,
				concepts[i].Title,
			)
		}
	}
}
