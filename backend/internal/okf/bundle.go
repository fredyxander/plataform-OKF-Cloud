package okf

import (
	"fmt"
	"strings"
)

type File struct {
	Name    string
	Content []byte
}

type Bundle struct {
	Files        []File
	ConceptCount int
}

func BuildBundle(
	filename string,
	format string,
	concepts []Concept,
) (*Bundle, error) {

	if len(concepts) == 0 {
		return nil, fmt.Errorf("bundle requires at least one concept")
	}

	files := make([]File, 0, len(concepts)+2)

	// Generar archivos de conceptos e index.
	var index strings.Builder
	index.WriteString("# Index\n\n")

	for i, concept := range concepts {
		conceptFilename := fmt.Sprintf("concept-%02d.md", i+1)

		index.WriteString(fmt.Sprintf(
			"- [%s](%s)\n",
			concept.Title,
			conceptFilename,
		))

		files = append(files, File{
			Name:    conceptFilename,
			Content: []byte(concept.Content),
		})
	}

	// Generar log de conversión.
	logContent := fmt.Sprintf(
		"# Conversion Log\n\n"+
			"- Source: %s\n"+
			"- Format: %s\n"+
			"- Concepts generated: %d\n",
		filename,
		format,
		len(concepts),
	)

	// Agregamos los archivos obligatorios.
	files = append(
		[]File{
			{
				Name:    "index.md",
				Content: []byte(index.String()),
			},
			{
				Name:    "log.md",
				Content: []byte(logContent),
			},
		},
		files...,
	)

	return &Bundle{
		Files:        files,
		ConceptCount: len(concepts),
	}, nil
}
