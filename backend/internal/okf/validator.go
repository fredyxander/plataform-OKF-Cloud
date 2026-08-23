package okf

// Validación del bundle antes de publicarlo.
//
// Se comprueban dos cosas distintas y con consecuencias distintas:
//
// 1. Estructura mínima obligatoria y resolución de los enlaces del
//    índice. Su incumplimiento produce INVALID: el bundle no se
//    publica ni se habilita su descarga.
//
// 2. Observaciones de calidad que no impiden usar el bundle. Producen
//    VALID_WITH_WARNINGS: el bundle se publica y las advertencias
//    quedan registradas junto con su metadata.
//
// Un documento breve sin divisiones debe clasificarse como VALID: la
// rúbrica exige explícitamente que un único concepto no genere ni
// fallos ni advertencias.

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/fredyxander/okf-platform/backend/internal/domain"
)

// markdownLink captura los enlaces Markdown `[etiqueta](destino)` del
// índice para poder comprobar que todos resuelven a un archivo real.
var markdownLink = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)

// ValidateBundle recorre el bundle completo y devuelve el resultado
// clasificado. No se detiene en el primer problema: acumula todos los
// errores y advertencias para que el usuario reciba un diagnóstico
// completo en una sola pasada.
func ValidateBundle(bundle *Bundle) domain.BundleValidation {
	var (
		errs     []string
		warnings []string
	)

	if bundle == nil {
		return classify([]string{"bundle is nil"}, nil)
	}

	// 1. Indexar los archivos del bundle detectando nombres vacíos
	//    o duplicados.
	files := make(map[string]File, len(bundle.Files))

	for _, file := range bundle.Files {
		if file.Name == "" {
			errs = append(errs, "bundle contains a file without name")
			continue
		}

		if _, exists := files[file.Name]; exists {
			errs = append(errs, fmt.Sprintf(
				"duplicate file in bundle: %s",
				file.Name,
			))

			continue
		}

		files[file.Name] = file
	}

	// 2. Estructura mínima obligatoria.
	if bundle.ConceptCount < 1 {
		errs = append(errs, "bundle must contain at least one concept")
	}

	index, hasIndex := files["index.md"]
	if !hasIndex {
		errs = append(errs, "bundle is missing index.md")
	} else if strings.TrimSpace(string(index.Content)) == "" {
		errs = append(errs, "index.md is empty")
	}

	logFile, hasLog := files["log.md"]
	if !hasLog {
		errs = append(errs, "bundle is missing log.md")
	} else if strings.TrimSpace(string(logFile.Content)) == "" {
		warnings = append(
			warnings,
			"log.md is empty: the conversion has no traceability",
		)
	}

	indexContent := ""
	if hasIndex {
		indexContent = string(index.Content)
	}

	// 3. Enlaces declarados por el índice.
	//
	//    Se recogen aquí para poder comprobar tanto que resuelven a un
	//    archivo existente como que cada concepto está enlazado.
	linked := make(map[string]bool)

	for _, match := range markdownLink.FindAllStringSubmatch(indexContent, -1) {
		label := strings.TrimSpace(match[1])
		target := strings.TrimSpace(match[2])

		// Los enlaces externos y las anclas internas no apuntan a un
		// archivo del bundle, por lo que no se resuelven aquí.
		if isExternalLink(target) {
			continue
		}

		target = strings.SplitN(target, "#", 2)[0]
		if target == "" {
			continue
		}

		linked[target] = true

		if _, exists := files[target]; !exists {
			errs = append(errs, fmt.Sprintf(
				"index.md references a file that is not in the bundle: %s",
				target,
			))
		}

		if label == "" {
			warnings = append(warnings, fmt.Sprintf(
				"index.md links %s without a title",
				target,
			))
		}
	}

	// 4. Cada concepto declarado debe existir y estar enlazado.
	concepts := make(map[string]bool, bundle.ConceptCount)

	for i := 1; i <= bundle.ConceptCount; i++ {
		name := ConceptFilename(i)
		concepts[name] = true

		concept, exists := files[name]
		if !exists {
			errs = append(errs, fmt.Sprintf(
				"bundle is missing concept file: %s",
				name,
			))

			continue
		}

		if hasIndex && !linked[name] {
			errs = append(errs, fmt.Sprintf(
				"index.md does not reference concept: %s",
				name,
			))
		}

		if conceptBody(string(concept.Content)) == "" {
			warnings = append(warnings, fmt.Sprintf(
				"concept %s has no content",
				name,
			))
		}
	}

	// 5. Archivos presentes que nadie referencia.
	//
	//    No invalidan el bundle, pero indican que la conversión generó
	//    material que el índice no expone.
	for _, file := range bundle.Files {
		if file.Name == "" ||
			file.Name == "index.md" ||
			file.Name == "log.md" ||
			concepts[file.Name] ||
			linked[file.Name] {
			continue
		}

		warnings = append(warnings, fmt.Sprintf(
			"bundle contains a file not referenced from index.md: %s",
			file.Name,
		))
	}

	return classify(errs, warnings)
}

// ConceptFilename devuelve el nombre canónico del concepto n-ésimo.
// Lo comparten el constructor y el validador del bundle para que la
// convención exista en un único lugar.
func ConceptFilename(position int) string {
	return fmt.Sprintf("concept-%02d.md", position)
}

// conceptBody descarta el encabezado inicial de un concepto.
//
// Cada unidad conserva su encabezado, así que sin esto una sección que
// solo tiene título parecería tener contenido.
func conceptBody(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}

	parts := strings.SplitN(trimmed, "\n", 2)

	if !headingPattern.MatchString(parts[0]) {
		return trimmed
	}

	if len(parts) == 1 {
		return ""
	}

	return strings.TrimSpace(parts[1])
}

func isExternalLink(target string) bool {
	lower := strings.ToLower(target)

	return strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "mailto:") ||
		strings.HasPrefix(target, "#")
}

// classify traduce los hallazgos al estado exigido por la rúbrica.
func classify(errs, warnings []string) domain.BundleValidation {
	if errs == nil {
		errs = []string{}
	}

	if warnings == nil {
		warnings = []string{}
	}

	status := domain.BundleValid

	switch {
	case len(errs) > 0:
		status = domain.BundleInvalid
	case len(warnings) > 0:
		status = domain.BundleValidWithWarnings
	}

	return domain.BundleValidation{
		Status:   status,
		Warnings: warnings,
		Errors:   errs,
	}
}
