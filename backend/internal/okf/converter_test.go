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

	if concepts[0].Content != "# Introduction\n\nHello OKF" {
		t.Errorf("unexpected content: %q", concepts[0].Content)
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
