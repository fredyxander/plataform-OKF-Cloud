package okf

import "testing"

func TestSplitMarkdownByH1(t *testing.T) {
	content := `# Introducción

Contenido de introducción.

# Arquitectura

Contenido de arquitectura.

# Conclusiones

Contenido final.`

	concepts := splitMarkdownByH1(content)

	if len(concepts) != 3 {
		t.Fatalf("expected 3 concepts, got %d", len(concepts))
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
