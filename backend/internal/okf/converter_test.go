package okf

import (
	"strings"
	"testing"
)

func convert(t *testing.T, filename, format, content string) *Conversion {
	t.Helper()

	conversion, err := Convert(filename, format, []byte(content))
	if err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}

	return conversion
}

func titles(conversion *Conversion) []string {
	result := make([]string, 0, len(conversion.Concepts))

	for _, concept := range conversion.Concepts {
		result = append(result, concept.Title)
	}

	return result
}

func assertTitles(t *testing.T, conversion *Conversion, expected ...string) {
	t.Helper()

	got := titles(conversion)

	if len(got) != len(expected) {
		t.Fatalf("expected %d concepts %v, got %d: %v",
			len(expected), expected, len(got), got)
	}

	for i, title := range expected {
		if got[i] != title {
			t.Errorf("concept %d: expected title %q, got %q",
				i+1, title, got[i])
		}
	}
}

// Documento breve sin divisiones: una sola unidad lógica.
func TestConvertShortMarkdownDocument(t *testing.T) {
	conversion := convert(t, "test.md", "markdown", "# Introduction\n\nHello OKF")

	assertTitles(t, conversion, "Introduction")

	// El encabezado viaja dentro del concepto: cada documento de
	// concepto debe ser Markdown autocontenido.
	if conversion.Concepts[0].Content != "# Introduction\n\nHello OKF" {
		t.Errorf("unexpected content: %q", conversion.Concepts[0].Content)
	}
}

func TestConvertPlaintext(t *testing.T) {
	conversion := convert(t, "test.txt", "plaintext", "Hello OKF")

	assertTitles(t, conversion, "test.txt")

	if conversion.Concepts[0].Content != "Hello OKF" {
		t.Errorf("unexpected content: %q", conversion.Concepts[0].Content)
	}
}

func TestConvertEmptyDocument(t *testing.T) {
	if _, err := Convert("test.md", "markdown", []byte("   ")); err == nil {
		t.Fatal("expected error for empty document")
	}
}

func TestConvertUnsupportedFormat(t *testing.T) {
	if _, err := Convert("test.pdf", "pdf", []byte("content")); err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

// Documento estructurado: un concepto por unidad, en el orden original.
func TestConvertMarkdownWithSections(t *testing.T) {
	conversion := convert(t, "manual.md", "markdown",
		"# Introducción\n\nuno\n\n# Arquitectura\n\ndos\n\n# Conclusiones\n\ntres")

	assertTitles(t, conversion, "Introducción", "Arquitectura", "Conclusiones")

	for i, concept := range conversion.Concepts {
		if !strings.HasPrefix(concept.Content, "# ") {
			t.Errorf("concept %d does not keep its heading: %q",
				i+1, concept.Content)
		}
	}
}

// El texto anterior al primer encabezado no puede perderse.
func TestConvertPreservesPreamble(t *testing.T) {
	conversion := convert(t, "doc.md", "markdown",
		"Resumen introductorio importante.\n\n# Uno\n\ncuerpo uno\n\n# Dos\n\ncuerpo dos")

	assertTitles(t, conversion, "doc.md", "Uno", "Dos")

	if !strings.Contains(conversion.Concepts[0].Content, "Resumen introductorio") {
		t.Errorf("preamble was lost: %q", conversion.Concepts[0].Content)
	}
}

// Un comentario dentro de un bloque de código no es un encabezado.
func TestConvertIgnoresHeadingsInsideCodeBlocks(t *testing.T) {
	conversion := convert(t, "doc.md", "markdown",
		"# Uno\n\n```bash\n# esto es un comentario\necho hola\n```\n\nfin")

	assertTitles(t, conversion, "Uno")

	if !strings.Contains(conversion.Concepts[0].Content, "echo hola") {
		t.Errorf("code block was broken: %q", conversion.Concepts[0].Content)
	}
}

func TestConvertIgnoresHeadingsInsideTildeCodeBlocks(t *testing.T) {
	conversion := convert(t, "doc.md", "markdown",
		"# Uno\n\n~~~\n# no es un encabezado\n~~~\n\n# Dos\n\ncuerpo")

	assertTitles(t, conversion, "Uno", "Dos")
}

// Estructura habitual: H1 como título del documento y H2 como
// secciones. Debe segmentarse por H2, no quedarse en una sola unidad.
func TestConvertSegmentsByH2WhenH1IsTheDocumentTitle(t *testing.T) {
	conversion := convert(t, "doc.md", "markdown",
		"# Manual\n\nintro del manual\n\n## Uno\n\ncuerpo uno\n\n## Dos\n\ncuerpo dos")

	assertTitles(t, conversion, "Manual", "Uno", "Dos")

	if !strings.Contains(conversion.Concepts[0].Content, "intro del manual") {
		t.Errorf("document preamble was lost: %q", conversion.Concepts[0].Content)
	}
}

// Documento que solo usa H2 como nivel superior.
func TestConvertSegmentsByH2WhenThereIsNoH1(t *testing.T) {
	conversion := convert(t, "doc.md", "markdown",
		"## Uno\n\ncuerpo uno\n\n## Dos\n\ncuerpo dos")

	assertTitles(t, conversion, "Uno", "Dos")
}

// Un documento sin ningún encabezado que divida se conserva entero.
func TestConvertWithoutHeadingsKeepsSingleUnit(t *testing.T) {
	conversion := convert(t, "doc.md", "markdown",
		"Solo un párrafo.\n\nY otro párrafo.")

	assertTitles(t, conversion, "doc.md")

	if !strings.Contains(conversion.Concepts[0].Content, "Y otro párrafo") {
		t.Errorf("content was lost: %q", conversion.Concepts[0].Content)
	}
}

// Los finales de línea de Windows no deben afectar la segmentación.
func TestConvertNormalizesWindowsLineEndings(t *testing.T) {
	conversion := convert(t, "doc.md", "markdown",
		"# Uno\r\n\r\ncuerpo uno\r\n\r\n# Dos\r\n\r\ncuerpo dos\r\n")

	assertTitles(t, conversion, "Uno", "Dos")

	for _, concept := range conversion.Concepts {
		if strings.Contains(concept.Content, "\r") {
			t.Errorf("carriage returns survived: %q", concept.Content)
		}
	}
}

// Encabezados con sangría y con secuencia de cierre ATX.
func TestConvertHandlesIndentedAndClosedHeadings(t *testing.T) {
	conversion := convert(t, "doc.md", "markdown",
		"# Uno\n\ncuerpo\n\n   # Dos ##\n\ncuerpo dos")

	assertTitles(t, conversion, "Uno", "Dos")
}

// Un encabezado sin texto no debe hacer desaparecer su contenido.
func TestConvertKeepsContentOfHeadingWithoutTitle(t *testing.T) {
	conversion := convert(t, "doc.md", "markdown",
		"# Uno\n\ncuerpo uno\n\n# \n\ncuerpo perdido")

	if len(conversion.Concepts) != 2 {
		t.Fatalf("expected 2 concepts, got %v", titles(conversion))
	}

	if !strings.Contains(conversion.Concepts[1].Content, "cuerpo perdido") {
		t.Errorf("content after an empty heading was lost: %q",
			conversion.Concepts[1].Content)
	}
}

// Las operaciones aplicadas se registran para poder trazarlas en log.md.
func TestConvertRecordsOperations(t *testing.T) {
	conversion := convert(t, "doc.md", "markdown",
		"# Uno\n\ncuerpo\n\n# Dos\n\ncuerpo")

	if len(conversion.Operations) == 0 {
		t.Fatal("expected the conversion to record its operations")
	}

	joined := strings.Join(conversion.Operations, "\n")

	if !strings.Contains(joined, "H1") {
		t.Errorf("expected the segmentation level to be recorded: %v",
			conversion.Operations)
	}
}
