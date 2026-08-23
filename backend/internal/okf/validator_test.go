package okf

import (
	"strings"
	"testing"

	"github.com/fredyxander/okf-platform/backend/internal/domain"
)

// containsSubstring facilita comprobar que un hallazgo concreto está
// presente sin depender del texto exacto completo.
func containsSubstring(values []string, substring string) bool {
	for _, value := range values {
		if strings.Contains(value, substring) {
			return true
		}
	}

	return false
}

// Documento breve: un único concepto debe clasificarse como VALID,
// sin errores y sin advertencias. La rúbrica lo exige explícitamente.
func TestValidateBundleShortDocumentIsValid(t *testing.T) {
	bundle := buildFrom(t, "notas.md", "markdown", "# Introduction\n\nHello OKF")

	result := ValidateBundle(bundle)

	if result.Status != domain.BundleValid {
		t.Fatalf(
			"expected %s, got %s (errors=%v warnings=%v)",
			domain.BundleValid,
			result.Status,
			result.Errors,
			result.Warnings,
		)
	}

	if !result.IsPublishable() {
		t.Fatal("expected a short document bundle to be publishable")
	}

	if result.Err() != nil {
		t.Fatalf("expected no error, got %v", result.Err())
	}
}

// Documento estructurado: varios conceptos enlazados en orden también
// deben clasificarse como VALID.
func TestValidateBundleStructuredDocumentIsValid(t *testing.T) {
	bundle := buildFrom(
		t,
		"manual.md",
		"markdown",
		"# Uno\n\nuno\n\n# Dos\n\ndos\n\n# Tres\n\ntres",
	)

	result := ValidateBundle(bundle)

	if result.Status != domain.BundleValid {
		t.Fatalf(
			"expected %s, got %s (errors=%v)",
			domain.BundleValid,
			result.Status,
			result.Errors,
		)
	}
}

func TestValidateBundleMissingIndexIsInvalid(t *testing.T) {
	bundle := &Bundle{
		Files: []File{
			{Name: "log.md", Content: []byte("# Log")},
			{Name: "concept-01.md", Content: []byte("# Concept")},
		},
		ConceptCount: 1,
	}

	result := ValidateBundle(bundle)

	if result.Status != domain.BundleInvalid {
		t.Fatalf("expected invalid bundle, got %s", result.Status)
	}

	if result.IsPublishable() {
		t.Fatal("an invalid bundle must never be publishable")
	}

	if !containsSubstring(result.Errors, "le falta index.md") {
		t.Fatalf("expected missing index.md error, got %v", result.Errors)
	}

	if result.Err() == nil {
		t.Fatal("expected an error for an invalid bundle")
	}
}

func TestValidateBundleMissingLogIsInvalid(t *testing.T) {
	bundle := &Bundle{
		Files: []File{
			{
				Name:    "index.md",
				Content: []byte("# Index\n\n- [Concept](concept-01.md)\n"),
			},
			{Name: "concept-01.md", Content: []byte("# Concept")},
		},
		ConceptCount: 1,
	}

	result := ValidateBundle(bundle)

	if result.Status != domain.BundleInvalid {
		t.Fatalf("expected invalid bundle, got %s", result.Status)
	}

	if !containsSubstring(result.Errors, "le falta log.md") {
		t.Fatalf("expected missing log.md error, got %v", result.Errors)
	}
}

// El índice existe pero no enlaza el concepto generado.
func TestValidateBundleUnlinkedConceptIsInvalid(t *testing.T) {
	bundle := &Bundle{
		Files: []File{
			{Name: "index.md", Content: []byte("# Index")},
			{Name: "log.md", Content: []byte("# Log")},
			{Name: "concept-01.md", Content: []byte("# Concept")},
		},
		ConceptCount: 1,
	}

	result := ValidateBundle(bundle)

	if result.Status != domain.BundleInvalid {
		t.Fatalf("expected invalid bundle, got %s", result.Status)
	}

	if !containsSubstring(result.Errors, "no referencia el concepto") {
		t.Fatalf("expected unlinked concept error, got %v", result.Errors)
	}
}

// Enlace del índice que no resuelve a ningún archivo del bundle.
func TestValidateBundleDanglingIndexLinkIsInvalid(t *testing.T) {
	bundle := &Bundle{
		Files: []File{
			{
				Name: "index.md",
				Content: []byte(
					"# Index\n\n" +
						"- [Uno](concept-01.md)\n" +
						"- [Dos](concept-02.md)\n",
				),
			},
			{Name: "log.md", Content: []byte("# Log")},
			{Name: "concept-01.md", Content: []byte("uno")},
		},
		ConceptCount: 1,
	}

	result := ValidateBundle(bundle)

	if result.Status != domain.BundleInvalid {
		t.Fatalf("expected invalid bundle, got %s", result.Status)
	}

	if !containsSubstring(result.Errors, "concept-02.md") {
		t.Fatalf("expected dangling link error, got %v", result.Errors)
	}
}

func TestValidateBundleDuplicateFileIsInvalid(t *testing.T) {
	bundle := &Bundle{
		Files: []File{
			{
				Name:    "index.md",
				Content: []byte("# Index\n\n- [Uno](concept-01.md)\n"),
			},
			{Name: "log.md", Content: []byte("# Log")},
			{Name: "concept-01.md", Content: []byte("uno")},
			{Name: "concept-01.md", Content: []byte("uno otra vez")},
		},
		ConceptCount: 1,
	}

	result := ValidateBundle(bundle)

	if result.Status != domain.BundleInvalid {
		t.Fatalf("expected invalid bundle, got %s", result.Status)
	}

	if !containsSubstring(result.Errors, "archivo duplicado") {
		t.Fatalf("expected duplicate file error, got %v", result.Errors)
	}
}

// La estructura mínima está completa, pero un concepto quedó vacío:
// el bundle es publicable y la observación queda registrada.
func TestValidateBundleEmptyConceptIsValidWithWarnings(t *testing.T) {
	bundle := &Bundle{
		Files: []File{
			{
				Name:    "index.md",
				Content: []byte("# Index\n\n- [Uno](concept-01.md)\n"),
			},
			{Name: "log.md", Content: []byte("# Log")},
			{Name: "concept-01.md", Content: []byte("   \n")},
		},
		ConceptCount: 1,
	}

	result := ValidateBundle(bundle)

	if result.Status != domain.BundleValidWithWarnings {
		t.Fatalf(
			"expected %s, got %s (errors=%v)",
			domain.BundleValidWithWarnings,
			result.Status,
			result.Errors,
		)
	}

	if !result.IsPublishable() {
		t.Fatal("a bundle with warnings must remain publishable")
	}

	if !containsSubstring(result.Warnings, "no tiene contenido") {
		t.Fatalf("expected empty concept warning, got %v", result.Warnings)
	}
}

// Un log.md vacío no impide publicar, pero deja el bundle sin
// trazabilidad de la conversión.
func TestValidateBundleEmptyLogIsValidWithWarnings(t *testing.T) {
	bundle := &Bundle{
		Files: []File{
			{
				Name:    "index.md",
				Content: []byte("# Index\n\n- [Uno](concept-01.md)\n"),
			},
			{Name: "log.md", Content: []byte("")},
			{Name: "concept-01.md", Content: []byte("uno")},
		},
		ConceptCount: 1,
	}

	result := ValidateBundle(bundle)

	if result.Status != domain.BundleValidWithWarnings {
		t.Fatalf("expected valid with warnings, got %s", result.Status)
	}

	if !containsSubstring(result.Warnings, "log.md está vacío") {
		t.Fatalf("expected empty log warning, got %v", result.Warnings)
	}
}

// Un archivo que nadie referencia no invalida el bundle.
func TestValidateBundleUnreferencedFileIsValidWithWarnings(t *testing.T) {
	bundle := &Bundle{
		Files: []File{
			{
				Name:    "index.md",
				Content: []byte("# Index\n\n- [Uno](concept-01.md)\n"),
			},
			{Name: "log.md", Content: []byte("# Log")},
			{Name: "concept-01.md", Content: []byte("uno")},
			{Name: "sobrante.md", Content: []byte("archivo suelto")},
		},
		ConceptCount: 1,
	}

	result := ValidateBundle(bundle)

	if result.Status != domain.BundleValidWithWarnings {
		t.Fatalf("expected valid with warnings, got %s", result.Status)
	}

	if !containsSubstring(result.Warnings, "sobrante.md") {
		t.Fatalf("expected unreferenced file warning, got %v", result.Warnings)
	}
}

// Los enlaces externos del índice no deben tratarse como archivos
// faltantes del bundle.
func TestValidateBundleIgnoresExternalLinks(t *testing.T) {
	bundle := &Bundle{
		Files: []File{
			{
				Name: "index.md",
				Content: []byte(
					"# Index\n\n" +
						"- [Uno](concept-01.md)\n" +
						"- [Fuente](https://example.org/doc)\n",
				),
			},
			{Name: "log.md", Content: []byte("# Log")},
			{Name: "concept-01.md", Content: []byte("uno")},
		},
		ConceptCount: 1,
	}

	result := ValidateBundle(bundle)

	if result.Status != domain.BundleValid {
		t.Fatalf(
			"expected valid bundle, got %s (errors=%v warnings=%v)",
			result.Status,
			result.Errors,
			result.Warnings,
		)
	}
}

// La validación acumula todos los hallazgos en una sola pasada.
func TestValidateBundleReportsEveryError(t *testing.T) {
	bundle := &Bundle{
		Files:        []File{},
		ConceptCount: 1,
	}

	result := ValidateBundle(bundle)

	if result.Status != domain.BundleInvalid {
		t.Fatalf("expected invalid bundle, got %s", result.Status)
	}

	if len(result.Errors) < 3 {
		t.Fatalf(
			"expected missing index, log and concept errors, got %v",
			result.Errors,
		)
	}
}
