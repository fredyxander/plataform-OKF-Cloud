package okf

import "testing"

func TestScanMarkdownDetectsHeadingsOutsideCodeBlocks(t *testing.T) {
	lines := scanMarkdown(
		"# Uno\n```\n# no cuenta\n```\n## Dos\n",
	)

	var levels []int

	for _, line := range lines {
		if line.headingLevel > 0 {
			levels = append(levels, line.headingLevel)
		}
	}

	if len(levels) != 2 || levels[0] != 1 || levels[1] != 2 {
		t.Fatalf("expected headings H1 and H2, got %v", levels)
	}
}

// segmentMarkdown elige el nivel más alto que realmente divide el
// documento e informa cuál usó.
func TestSegmentMarkdownChoosesTheDividingLevel(t *testing.T) {
	cases := []struct {
		name          string
		content       string
		expectedLevel int
		expectedUnits int
	}{
		{
			name:          "varios H1",
			content:       "# Uno\n\na\n\n# Dos\n\nb\n\n# Tres\n\nc",
			expectedLevel: 1,
			expectedUnits: 3,
		},
		{
			name:          "H1 como titulo y secciones H2",
			content:       "# Titulo\n\nintro\n\n## Uno\n\na\n\n## Dos\n\nb",
			expectedLevel: 2,
			expectedUnits: 3,
		},
		{
			name:          "solo H3",
			content:       "### Uno\n\na\n\n### Dos\n\nb",
			expectedLevel: 3,
			expectedUnits: 2,
		},
		{
			name:          "sin encabezados que dividan",
			content:       "# Solo uno\n\ncuerpo",
			expectedLevel: 0,
			expectedUnits: 1,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			concepts, level := segmentMarkdown("doc.md", c.content)

			if level != c.expectedLevel {
				t.Errorf("expected level %d, got %d", c.expectedLevel, level)
			}

			if len(concepts) != c.expectedUnits {
				t.Errorf("expected %d units, got %d",
					c.expectedUnits, len(concepts))
			}
		})
	}
}

// H4 en adelante no se considera una unidad lógica: el documento se
// conserva entero.
func TestSegmentMarkdownIgnoresDeepHeadings(t *testing.T) {
	concepts, level := segmentMarkdown(
		"doc.md",
		"#### Uno\n\na\n\n#### Dos\n\nb",
	)

	if level != 0 {
		t.Errorf("expected no segmentation, got level %d", level)
	}

	if len(concepts) != 1 {
		t.Errorf("expected a single unit, got %d", len(concepts))
	}
}
