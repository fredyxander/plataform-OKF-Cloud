package httpapi

import (
	"encoding/json"
	"testing"

	"github.com/fredyxander/okf-platform/backend/internal/domain"
)

// Todos los contadores deben viajar siempre, incluso en cero: una vista
// que los muestre no debería distinguir entre "cero" y "ausente".
func TestStatsResponseAlwaysCarriesEveryCounter(t *testing.T) {
	encoded, err := json.Marshal(StatsResponse{
		Jobs: domain.JobStats{
			Completed: 12,
			Total:     12,
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]map[string]int

	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	jobs, ok := decoded["jobs"]
	if !ok {
		t.Fatalf("expected a jobs group, got %s", encoded)
	}

	for _, counter := range []string{
		"queued",
		"processing",
		"completed",
		"failed",
		"total",
	} {
		if _, ok := jobs[counter]; !ok {
			t.Errorf("missing counter %q in %s", counter, encoded)
		}
	}

	if jobs["completed"] != 12 {
		t.Errorf("expected 12 completed, got %d", jobs["completed"])
	}

	if jobs["queued"] != 0 {
		t.Errorf("expected 0 queued, got %d", jobs["queued"])
	}
}
