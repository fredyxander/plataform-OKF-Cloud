package okf

//Validacion de:
// Bundle
// - index.md existe
// - log.md existen
// - >= 1 concepto
// - conceptos existen
// - index enlaza los conceptos
// - No hay nombres duplicados

import (
	"fmt"
	"strings"
)

func ValidateBundle(bundle *Bundle) error {
	if bundle == nil {
		return fmt.Errorf("bundle is nil")
	}

	if bundle.ConceptCount < 1 {
		return fmt.Errorf("bundle must contain at least one concept")
	}

	files := make(map[string]File)

	for _, file := range bundle.Files {
		if file.Name == "" {
			return fmt.Errorf("bundle contains file without name")
		}

		if _, exists := files[file.Name]; exists {
			return fmt.Errorf("duplicate file: %s", file.Name)
		}

		files[file.Name] = file
	}

	index, ok := files["index.md"]
	if !ok {
		return fmt.Errorf("bundle is missing index.md")
	}

	if _, ok := files["log.md"]; !ok {
		return fmt.Errorf("bundle is missing log.md")
	}

	// Comprobar que cada concepto generado existe
	// y está referenciado desde index.md.
	indexContent := string(index.Content)

	for i := 1; i <= bundle.ConceptCount; i++ {
		conceptFilename := fmt.Sprintf("concept-%02d.md", i)

		if _, ok := files[conceptFilename]; !ok {
			return fmt.Errorf(
				"bundle is missing concept file: %s",
				conceptFilename,
			)
		}

		if !strings.Contains(indexContent, "("+conceptFilename+")") {
			return fmt.Errorf(
				"index.md does not reference concept: %s",
				conceptFilename,
			)
		}
	}

	return nil
}
