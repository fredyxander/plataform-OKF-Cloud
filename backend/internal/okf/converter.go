package okf

import (
	"fmt"
	"strings"
)

type Concept struct {
	Title   string
	Content string
}

func Convert(filename, format string, content []byte) ([]Concept, error) {
	text := strings.TrimSpace(string(content))

	if text == "" {
		return nil, fmt.Errorf("document is empty")
	}

	switch format {
	case "plaintext":
		return []Concept{
			{
				Title:   filename,
				Content: text,
			},
		}, nil

	case "markdown":
		concepts := splitMarkdownByH1(text)

		// Si no existen encabezados H1, tratamos todo el documento
		// como un único concepto.
		if len(concepts) == 0 {
			return []Concept{
				{
					Title:   filename,
					Content: text,
				},
			}, nil
		}

		return concepts, nil

	default:
		return nil, fmt.Errorf("unsupported document format: %s", format)
	}
}
