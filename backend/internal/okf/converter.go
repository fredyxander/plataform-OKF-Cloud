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
				"Se leyó el documento como texto plano UTF-8.",
				"Se normalizaron los finales de línea.",
				"Se mantuvo el documento como una única unidad lógica: " +
					"el texto plano no tiene estructura detectable.",
			},
		}, nil

	case "markdown":
		concepts, level := segmentMarkdown(filename, text)

		operations := []string{
			"Se leyó el documento como Markdown UTF-8.",
			"Se normalizaron los finales de línea.",
			"Se ignoraron los encabezados dentro de bloques de código delimitados.",
		}

		if level == 0 {
			operations = append(
				operations,
				"Ningún nivel de encabezado dividía el documento: "+
					"se mantuvo como una única unidad lógica.",
			)
		} else {
			operations = append(
				operations,
				fmt.Sprintf(
					"Se segmentó el documento por el nivel de encabezado Markdown H%d, "+
						"el nivel más alto que realmente lo divide.",
					level,
				),
				"Se preservó el orden original de las unidades detectadas.",
				"Se mantuvo el encabezado de cada unidad dentro de su documento de concepto.",
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
