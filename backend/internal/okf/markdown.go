package okf

// Segmentación de Markdown en unidades lógicas.
//
// Reglas aplicadas:
//
//  1. Los encabezados dentro de bloques de código no son encabezados.
//     Sin esto, un comentario `# ...` dentro de ```bash partía el
//     documento y destrozaba el bloque de código.
//
//  2. El documento se divide por el nivel de encabezado más alto que
//     realmente lo divide en más de una unidad. Un documento cuyo H1 es
//     el título y cuyas secciones son H2 se segmenta por H2.
//
//  3. El texto anterior al primer encabezado nunca se descarta: forma
//     su propia unidad.
//
//  4. Cada unidad conserva su encabezado, de modo que cada documento de
//     concepto es Markdown autocontenido.

import (
	"regexp"
	"strings"
)

// maxHeadingLevel es el nivel más profundo por el que aceptamos
// segmentar. Más allá de H3 las divisiones dejan de corresponder a
// unidades lógicas del documento.
const maxHeadingLevel = 3

// headingPattern reconoce encabezados ATX admitiendo hasta tres
// espacios de sangría, como el propio Markdown.
var headingPattern = regexp.MustCompile(`^ {0,3}(#{1,6})\s+(.*)$`)

// closingHashes elimina la secuencia de cierre opcional de un
// encabezado ATX (`## Título ##`) sin tocar títulos como `C#`.
var closingHashes = regexp.MustCompile(`\s+#+\s*$`)

// mdLine es una línea del documento junto con el encabezado que
// representa. headingLevel == 0 significa que no es un encabezado.
type mdLine struct {
	text         string
	headingLevel int
	headingTitle string
}

// scanMarkdown recorre el documento una sola vez y marca qué líneas son
// encabezados reales, ignorando las que están dentro de un bloque de
// código delimitado por ``` o ~~~.
func scanMarkdown(text string) []mdLine {
	rawLines := strings.Split(text, "\n")
	lines := make([]mdLine, 0, len(rawLines))

	// fence guarda el delimitador que abrió el bloque de código actual,
	// o "" si estamos fuera de todo bloque.
	fence := ""

	for _, raw := range rawLines {
		trimmed := strings.TrimSpace(raw)

		if fence != "" {
			if strings.HasPrefix(trimmed, fence) {
				fence = ""
			}

			lines = append(lines, mdLine{text: raw})

			continue
		}

		if marker := fenceMarker(trimmed); marker != "" {
			fence = marker
			lines = append(lines, mdLine{text: raw})

			continue
		}

		match := headingPattern.FindStringSubmatch(raw)
		if match == nil {
			lines = append(lines, mdLine{text: raw})

			continue
		}

		title := strings.TrimSpace(match[2])
		title = strings.TrimSpace(closingHashes.ReplaceAllString(title, ""))

		lines = append(lines, mdLine{
			text:         raw,
			headingLevel: len(match[1]),
			headingTitle: title,
		})
	}

	return lines
}

func fenceMarker(trimmed string) string {
	switch {
	case strings.HasPrefix(trimmed, "```"):
		return "```"

	case strings.HasPrefix(trimmed, "~~~"):
		return "~~~"
	}

	return ""
}

// segmentMarkdown devuelve las unidades detectadas y el nivel de
// encabezado utilizado. Un nivel 0 indica que ningún encabezado dividía
// el documento y que se conservó como una única unidad.
func segmentMarkdown(filename, text string) ([]Concept, int) {
	lines := scanMarkdown(text)

	for level := 1; level <= maxHeadingLevel; level++ {
		concepts := splitByLevel(filename, lines, level)

		// Un nivel que produce una sola unidad no está segmentando
		// nada: seguimos bajando de nivel.
		if len(concepts) >= 2 {
			return concepts, level
		}
	}

	return []Concept{
		{
			Title:   documentTitle(filename, lines),
			Content: text,
		},
	}, 0
}

func splitByLevel(filename string, lines []mdLine, level int) []Concept {
	var starts []int

	for i, line := range lines {
		if line.headingLevel == level {
			starts = append(starts, i)
		}
	}

	if len(starts) == 0 {
		return nil
	}

	var concepts []Concept

	// Todo lo anterior al primer encabezado es una unidad propia. Si se
	// descartara, la conversión perdería contenido del original.
	if preamble := joinLines(lines[:starts[0]]); preamble != "" {
		concepts = append(concepts, Concept{
			Title:   documentTitle(filename, lines[:starts[0]]),
			Content: preamble,
		})
	}

	for i, start := range starts {
		end := len(lines)

		if i+1 < len(starts) {
			end = starts[i+1]
		}

		concepts = append(concepts, Concept{
			Title:   lines[start].headingTitle,
			Content: joinLines(lines[start:end]),
		})
	}

	return concepts
}

// documentTitle usa el primer encabezado disponible y, si no hay
// ninguno, el nombre del archivo original.
func documentTitle(filename string, lines []mdLine) string {
	for _, line := range lines {
		if line.headingLevel > 0 && line.headingTitle != "" {
			return line.headingTitle
		}
	}

	return filename
}

func joinLines(lines []mdLine) string {
	texts := make([]string, 0, len(lines))

	for _, line := range lines {
		texts = append(texts, line.text)
	}

	return strings.TrimSpace(strings.Join(texts, "\n"))
}
