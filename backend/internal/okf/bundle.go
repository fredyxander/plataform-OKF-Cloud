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
		return "Unidad sin título"
	}

	return label
}

// BuildBundle genera la estructura completa del bundle: index.md con la
// navegación y los datos del bundle, log.md con la trazabilidad de la
// conversión y un documento de concepto por unidad detectada.
func BuildBundle(conversion *Conversion) (*Bundle, error) {
	if conversion == nil || len(conversion.Concepts) == 0 {
		return nil, fmt.Errorf("el bundle requiere al menos un concepto")
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
		"# Índice\n\n"+
			"- Documento de origen: %s\n"+
			"- Formato: %s\n"+
			"- Conceptos: %d\n\n"+
			"## Conceptos\n\n%s",
		conversion.Filename,
		conversion.Format,
		len(conversion.Concepts),
		navigation.String(),
	)

	conversionLog := fmt.Sprintf(
		"# Registro de conversión\n\n"+
			"## Documento de origen\n\n"+
			"- Archivo: %s\n"+
			"- Formato: %s\n\n"+
			"## Operaciones\n\n%s\n"+
			"## Unidades detectadas\n\n%s\n"+
			"Total de unidades detectadas: %d\n",
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
		return "- No se registró ninguna transformación.\n"
	}

	var list strings.Builder

	for _, operation := range operations {
		list.WriteString("- " + operation + "\n")
	}

	return list.String()
}

// validationLabel traduce la clasificación al texto de log.md, que es un
// documento para leer, no para consumir. El valor literal del contrato se
// escribe junto al texto, de modo que el registro siga siendo rastreable
// frente a lo que devuelve la API.
func validationLabel(status domain.BundleValidationStatus) string {
	switch status {
	case domain.BundleValid:
		return "válido"

	case domain.BundleValidWithWarnings:
		return "válido con advertencias"

	case domain.BundleInvalid:
		return "inválido"
	}

	return string(status)
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

	section.WriteString("\n## Validación\n\n")
	section.WriteString(fmt.Sprintf("- Resultado: %s (%s)\n", validationLabel(validation.Status), validation.Status))
	section.WriteString("- Comprobaciones superadas: estructura mínima (index.md, " +
		"log.md y al menos un concepto) y resolución de todos los enlaces del índice.\n")

	if len(validation.Warnings) == 0 {
		section.WriteString("- Advertencias: ninguna.\n")
	} else {
		section.WriteString("- Advertencias:\n")

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
