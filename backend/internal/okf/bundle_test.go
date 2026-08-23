package okf

import (
	"strings"
	"testing"

	"github.com/fredyxander/okf-platform/backend/internal/domain"
)

func bundleFile(t *testing.T, bundle *Bundle, name string) string {
	t.Helper()

	for _, file := range bundle.Files {
		if file.Name == name {
			return string(file.Content)
		}
	}

	t.Fatalf("bundle has no file %s", name)

	return ""
}

func buildFrom(t *testing.T, filename, format, content string) *Bundle {
	t.Helper()

	bundle, err := BuildBundle(convert(t, filename, format, content))
	if err != nil {
		t.Fatalf("BuildBundle returned error: %v", err)
	}

	return bundle
}

// Estructura mínima obligatoria, incluso para un documento breve.
func TestBuildBundleMinimumStructure(t *testing.T) {
	bundle := buildFrom(t, "test.md", "markdown", "# Introduction\n\nHello OKF")

	if bundle.ConceptCount != 1 {
		t.Fatalf("expected 1 concept, got %d", bundle.ConceptCount)
	}

	expectedNames := []string{"index.md", "log.md", "concept-01.md"}

	if len(bundle.Files) != len(expectedNames) {
		t.Fatalf("expected %d files, got %d", len(expectedNames), len(bundle.Files))
	}

	for i, expected := range expectedNames {
		if bundle.Files[i].Name != expected {
			t.Errorf("expected file %q, got %q", expected, bundle.Files[i].Name)
		}
	}
}

func TestBuildBundleWithoutConcepts(t *testing.T) {
	if _, err := BuildBundle(nil); err == nil {
		t.Fatal("expected error when there is no conversion")
	}

	if _, err := BuildBundle(&Conversion{}); err == nil {
		t.Fatal("expected error when bundle has no concepts")
	}
}

// index.md aporta navegación y los datos del bundle, y conserva el
// orden del documento de origen.
func TestBuildBundleIndexNavigationAndData(t *testing.T) {
	bundle := buildFrom(t, "manual.md", "markdown",
		"# Uno\n\na\n\n# Dos\n\nb\n\n# Tres\n\nc")

	index := bundleFile(t, bundle, "index.md")

	for _, expected := range []string{
		"- Source: manual.md",
		"- Format: markdown",
		"- Concepts: 3",
	} {
		if !strings.Contains(index, expected) {
			t.Errorf("index.md is missing %q:\n%s", expected, index)
		}
	}

	positions := []int{
		strings.Index(index, "[Uno](concept-01.md)"),
		strings.Index(index, "[Dos](concept-02.md)"),
		strings.Index(index, "[Tres](concept-03.md)"),
	}

	for i, position := range positions {
		if position < 0 {
			t.Fatalf("index.md does not link concept %d:\n%s", i+1, index)
		}

		if i > 0 && position < positions[i-1] {
			t.Errorf("index.md does not preserve the original order:\n%s", index)
		}
	}
}

// log.md registra el origen, las operaciones aplicadas y las unidades
// detectadas.
func TestBuildBundleLogTraceability(t *testing.T) {
	bundle := buildFrom(t, "manual.md", "markdown", "# Uno\n\na\n\n# Dos\n\nb")

	conversionLog := bundleFile(t, bundle, "log.md")

	for _, expected := range []string{
		"- File: manual.md",
		"- Format: markdown",
		"## Operations",
		"Segmented the document by Markdown heading level H1",
		"## Detected units",
		"1. Uno -> concept-01.md",
		"2. Dos -> concept-02.md",
		"Total units detected: 2",
	} {
		if !strings.Contains(conversionLog, expected) {
			t.Errorf("log.md is missing %q:\n%s", expected, conversionLog)
		}
	}
}

// Un título que contiene sintaxis de enlace no debe romper la
// resolución de los enlaces del índice.
func TestBuildBundleSanitizesTitlesWithLinks(t *testing.T) {
	bundle := buildFrom(t, "doc.md", "markdown",
		"# Ver [la fuente](https://example.org)\n\ncuerpo\n\n# Dos\n\nb")

	index := bundleFile(t, bundle, "index.md")

	if !strings.Contains(index, "[Ver la fuente](concept-01.md)") {
		t.Errorf("index.md label was not sanitized:\n%s", index)
	}

	result := ValidateBundle(bundle)

	if result.Status != domain.BundleValid {
		t.Fatalf("expected a valid bundle, got %s (errors=%v)",
			result.Status, result.Errors)
	}
}

// Una unidad sin título recibe una etiqueta utilizable en el índice.
func TestBuildBundleLabelsUntitledUnits(t *testing.T) {
	bundle, err := BuildBundle(&Conversion{
		Filename: "doc.md",
		Format:   "markdown",
		Concepts: []Concept{{Title: "", Content: "cuerpo"}},
	})
	if err != nil {
		t.Fatalf("BuildBundle returned error: %v", err)
	}

	index := bundleFile(t, bundle, "index.md")

	if !strings.Contains(index, "[Untitled unit](concept-01.md)") {
		t.Errorf("expected a fallback label:\n%s", index)
	}

	if result := ValidateBundle(bundle); result.Status != domain.BundleValid {
		t.Fatalf("expected a valid bundle, got %s (errors=%v)",
			result.Status, result.Errors)
	}
}

// El resultado de la validación se incorpora a log.md sin alterar la
// estructura ya validada.
func TestAppendValidationLog(t *testing.T) {
	bundle := buildFrom(t, "doc.md", "markdown", "# Uno\n\na\n\n# Dos\n\nb")

	before := ValidateBundle(bundle)

	AppendValidationLog(bundle, before)

	conversionLog := bundleFile(t, bundle, "log.md")

	for _, expected := range []string{
		"## Validation",
		"- Result: valid",
		"- Warnings: none.",
	} {
		if !strings.Contains(conversionLog, expected) {
			t.Errorf("log.md is missing %q:\n%s", expected, conversionLog)
		}
	}

	after := ValidateBundle(bundle)

	if after.Status != before.Status {
		t.Fatalf("appending the log changed the classification: %s -> %s",
			before.Status, after.Status)
	}

	if len(bundle.Files) != 4 {
		t.Fatalf("appending the log changed the file set: %d files",
			len(bundle.Files))
	}
}

func TestAppendValidationLogListsWarnings(t *testing.T) {
	bundle := buildFrom(t, "doc.md", "markdown", "# Uno\n# Dos\n\nb")

	result := ValidateBundle(bundle)

	if result.Status != domain.BundleValidWithWarnings {
		t.Fatalf("expected warnings, got %s", result.Status)
	}

	AppendValidationLog(bundle, result)

	conversionLog := bundleFile(t, bundle, "log.md")

	if !strings.Contains(conversionLog, "- Warnings:\n") {
		t.Errorf("log.md does not list the warnings:\n%s", conversionLog)
	}
}
