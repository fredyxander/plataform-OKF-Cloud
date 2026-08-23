package okf

import (
	"fmt"
	"strings"
)

// Concept es una unidad lógica detectada en el documento original.
type Concept struct {
	Title   string
	Content string
}

// Conversion es el resultado completo de convertir un documento.
//
// Además de las unidades detectadas transporta las operaciones
// aplicadas, que log.md necesita para dejar trazabilidad de la
// conversión.
type Conversion struct {
	Filename   string
	Format     string
	Concepts   []Concept
	Operations []string
}

// Convert traduce el documento original a unidades lógicas.
//
// La segmentación es determinista: el mismo documento produce siempre
// las mismas unidades en el mismo orden.
func Convert(filename, format string, content []byte) (*Conversion, error) {
	// Los finales de línea de Windows se normalizan antes de analizar
	// el documento para que la segmentación no dependa del sistema en
	// el que se escribió el archivo.
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	text = strings.TrimSpace(text)

	if text == "" {
		return nil, fmt.Errorf("document is empty")
	}

	switch format {
	case "plaintext":
		return &Conversion{
			Filename: filename,
			Format:   format,
			Concepts: []Concept{
				{
					Title:   filename,
					Content: text,
				},
			},
			Operations: []string{
				"Read the document as plain UTF-8 text.",
				"Normalized line endings.",
				"Kept the document as a single logical unit: " +
					"plain text has no detectable structure.",
			},
		}, nil

	case "markdown":
		concepts, level := segmentMarkdown(filename, text)

		operations := []string{
			"Read the document as UTF-8 Markdown.",
			"Normalized line endings.",
			"Ignored headings inside fenced code blocks.",
		}

		if level == 0 {
			operations = append(
				operations,
				"No heading level divided the document: "+
					"kept it as a single logical unit.",
			)
		} else {
			operations = append(
				operations,
				fmt.Sprintf(
					"Segmented the document by Markdown heading level H%d, "+
						"the highest level that actually divides it.",
					level,
				),
				"Preserved the original order of the detected units.",
				"Kept each unit heading inside its concept document.",
			)
		}

		return &Conversion{
			Filename:   filename,
			Format:     format,
			Concepts:   concepts,
			Operations: operations,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported document format: %s", format)
	}
}
