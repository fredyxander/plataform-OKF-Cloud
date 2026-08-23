package okf

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/fredyxander/okf-platform/backend/internal/domain"
)

type File struct {
	Name    string
	Content []byte
}

type Bundle struct {
	Files        []File
	ConceptCount int
}

// linkInTitle reconoce la sintaxis de enlace Markdown dentro de un
// título de sección.
var linkInTitle = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)

// labelNoise elimina los caracteres que romperían el enlace del índice.
var labelNoise = strings.NewReplacer("[", "", "]", "", "(", "", ")", "")

// indexLabel convierte el título de una unidad en una etiqueta segura
// para un enlace Markdown.
//
// Sin esto, un título legítimo como `# Ver [la fuente](https://x.org)`
// generaba un enlace anidado en index.md, la resolución de enlaces del
// índice fallaba y el bundle se clasificaba como inválido.
func indexLabel(title string) string {
	label := linkInTitle.ReplaceAllString(title, "$1")
	label = labelNoise.Replace(label)
	label = strings.Join(strings.Fields(label), " ")

	if label == "" {
		return "Untitled unit"
	}

	return label
}

// BuildBundle genera la estructura completa del bundle: index.md con la
// navegación y los datos del bundle, log.md con la trazabilidad de la
// conversión y un documento de concepto por unidad detectada.
func BuildBundle(conversion *Conversion) (*Bundle, error) {
	if conversion == nil || len(conversion.Concepts) == 0 {
		return nil, fmt.Errorf("bundle requires at least one concept")
	}

	conceptFiles := make([]File, 0, len(conversion.Concepts))

	var (
		navigation strings.Builder
		units      strings.Builder
	)

	for i, concept := range conversion.Concepts {
		position := i + 1
		conceptFilename := ConceptFilename(position)
		label := indexLabel(concept.Title)

		navigation.WriteString(fmt.Sprintf(
			"%d. [%s](%s)\n",
			position,
			label,
			conceptFilename,
		))

		units.WriteString(fmt.Sprintf(
			"%d. %s -> %s\n",
			position,
			label,
			conceptFilename,
		))

		conceptFiles = append(conceptFiles, File{
			Name:    conceptFilename,
			Content: []byte(concept.Content),
		})
	}

	index := fmt.Sprintf(
		"# Index\n\n"+
			"- Source: %s\n"+
			"- Format: %s\n"+
			"- Concepts: %d\n\n"+
			"## Concepts\n\n%s",
		conversion.Filename,
		conversion.Format,
		len(conversion.Concepts),
		navigation.String(),
	)

	conversionLog := fmt.Sprintf(
		"# Conversion Log\n\n"+
			"## Source\n\n"+
			"- File: %s\n"+
			"- Format: %s\n\n"+
			"## Operations\n\n%s\n"+
			"## Detected units\n\n%s\n"+
			"Total units detected: %d\n",
		conversion.Filename,
		conversion.Format,
		operationList(conversion.Operations),
		units.String(),
		len(conversion.Concepts),
	)

	files := make([]File, 0, len(conceptFiles)+2)

	files = append(files, File{
		Name:    "index.md",
		Content: []byte(index),
	})

	files = append(files, File{
		Name:    "log.md",
		Content: []byte(conversionLog),
	})

	files = append(files, conceptFiles...)

	return &Bundle{
		Files:        files,
		ConceptCount: len(conversion.Concepts),
	}, nil
}

func operationList(operations []string) string {
	if len(operations) == 0 {
		return "- No transformation was recorded.\n"
	}

	var list strings.Builder

	for _, operation := range operations {
		list.WriteString("- " + operation + "\n")
	}

	return list.String()
}

// AppendValidationLog añade a log.md el resultado de la validación.
//
// Se ejecuta después de validar porque el resultado no existe antes.
// Solo agrega texto a un archivo que ya estaba presente, de modo que no
// puede alterar la estructura ni los enlaces que la validación
// comprobó.
func AppendValidationLog(bundle *Bundle, validation domain.BundleValidation) {
	if bundle == nil {
		return
	}

	var section strings.Builder

	section.WriteString("\n## Validation\n\n")
	section.WriteString(fmt.Sprintf("- Result: %s\n", validation.Status))
	section.WriteString("- Checks passed: minimum structure (index.md, " +
		"log.md and at least one concept) and resolution of every index link.\n")

	if len(validation.Warnings) == 0 {
		section.WriteString("- Warnings: none.\n")
	} else {
		section.WriteString("- Warnings:\n")

		for _, warning := range validation.Warnings {
			section.WriteString("  - " + warning + "\n")
		}
	}

	for i, file := range bundle.Files {
		if file.Name != "log.md" {
			continue
		}

		bundle.Files[i].Content = append(
			file.Content,
			[]byte(section.String())...,
		)

		return
	}
}
