package okf

import (
	"testing"

	"github.com/fredyxander/okf-platform/backend/internal/domain"
)

func buildTestBundle(t *testing.T) *Bundle {
	t.Helper()

	return buildFrom(t, "demo.md", "markdown", "# Uno\n\ncontenido uno")
}

// Sin variable de entorno no se altera nada: el pipeline normal no
// puede degradarse por accidente.
func TestApplyFaultDisabledByDefault(t *testing.T) {
	bundle := buildTestBundle(t)

	if applied := ApplyFault(bundle, ""); applied != "" {
		t.Fatalf("expected no fault, got %q", applied)
	}

	if len(bundle.Files) != 3 {
		t.Fatalf("expected untouched bundle, got %d files", len(bundle.Files))
	}
}

// El fallo inyectado debe producir exactamente el caso "bundle
// incompleto".
func TestApplyFaultDropIndexProducesInvalidBundle(t *testing.T) {
	bundle := buildTestBundle(t)

	if applied := ApplyFault(bundle, FaultDropIndex); applied == "" {
		t.Fatal("expected the fault to be applied")
	}

	result := ValidateBundle(bundle)

	if result.Status != domain.BundleInvalid {
		t.Fatalf("expected invalid bundle, got %s", result.Status)
	}
}

func TestApplyFaultDropLogProducesInvalidBundle(t *testing.T) {
	bundle := buildTestBundle(t)

	if applied := ApplyFault(bundle, FaultDropLog); applied == "" {
		t.Fatal("expected the fault to be applied")
	}

	result := ValidateBundle(bundle)

	if result.Status != domain.BundleInvalid {
		t.Fatalf("expected invalid bundle, got %s", result.Status)
	}
}

func TestApplyFaultEmptyConceptProducesWarnings(t *testing.T) {
	bundle := buildTestBundle(t)

	if applied := ApplyFault(bundle, FaultEmptyConcept); applied == "" {
		t.Fatal("expected the fault to be applied")
	}

	result := ValidateBundle(bundle)

	if result.Status != domain.BundleValidWithWarnings {
		t.Fatalf("expected valid with warnings, got %s", result.Status)
	}
}
