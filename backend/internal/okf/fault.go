package okf

import "fmt"

// Inyección de fallo controlada.
//
// La rúbrica exige demostrar sobre el sistema desplegado que un bundle
// incompleto no se publica. El pipeline siempre genera la estructura
// mínima, así que sin este mecanismo esa condición solo podría probarse
// con tests unitarios y no en el video.
//
// Está desactivada por defecto: solo se aplica si el worker recibe la
// variable de entorno OKF_FAULT_INJECTION. No debe usarse fuera de una
// demostración.
const (
	FaultDropIndex    = "drop-index"
	FaultDropLog      = "drop-log"
	FaultEmptyConcept = "empty-concept"
)

// ApplyFault degrada el bundle según el fallo solicitado y describe lo
// que hizo. Devuelve una descripción vacía si no se aplicó nada.
func ApplyFault(bundle *Bundle, fault string) string {
	if bundle == nil || fault == "" {
		return ""
	}

	switch fault {
	case FaultDropIndex:
		if removeFile(bundle, "index.md") {
			return "removed index.md"
		}

	case FaultDropLog:
		if removeFile(bundle, "log.md") {
			return "removed log.md"
		}

	case FaultEmptyConcept:
		name := ConceptFilename(1)

		for i := range bundle.Files {
			if bundle.Files[i].Name == name {
				bundle.Files[i].Content = nil

				return fmt.Sprintf("emptied %s", name)
			}
		}
	}

	return ""
}

func removeFile(bundle *Bundle, name string) bool {
	for i, file := range bundle.Files {
		if file.Name != name {
			continue
		}

		bundle.Files = append(bundle.Files[:i], bundle.Files[i+1:]...)

		return true
	}

	return false
}
