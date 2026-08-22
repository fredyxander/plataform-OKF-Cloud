package okf

import "strings"

func splitMarkdownByH1(content string) []Concept {
	lines := strings.Split(content, "\n")

	var concepts []Concept
	var title string
	var body strings.Builder

	flush := func() {
		if title == "" {
			return
		}

		concepts = append(concepts, Concept{
			Title:   title,
			Content: strings.TrimSpace(body.String()),
		})

		body.Reset()
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "# ") {
			flush()

			title = strings.TrimSpace(
				strings.TrimPrefix(line, "# "),
			)

			continue
		}

		if title != "" {
			body.WriteString(line)
			body.WriteString("\n")
		}
	}

	flush()

	return concepts
}
