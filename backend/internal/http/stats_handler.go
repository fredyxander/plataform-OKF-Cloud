package httpapi

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/fredyxander/okf-platform/backend/internal/database"
	"github.com/fredyxander/okf-platform/backend/internal/domain"
)

type StatsHandler struct {
	db *database.DB
}

func NewStatsHandler(db *database.DB) *StatsHandler {
	return &StatsHandler{
		db: db,
	}
}

// StatsResponse agrupa las métricas por área para poder añadir otras
// más adelante sin romper el contrato existente.
type StatsResponse struct {
	Jobs domain.JobStats `json:"jobs"`
}

// Get devuelve el resumen del flujo de trabajos del usuario autenticado.
//
// Como toda operación de la API, está acotada al propietario: nunca
// agrega Jobs de otros usuarios.
func (h *StatsHandler) Get(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	stats, err := h.db.JobStatsByOwner(ownerID)
	if err != nil {
		log.Printf("could not get stats of owner %s: %v", ownerID, err)
		http.Error(w, "could not get stats", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(StatsResponse{Jobs: *stats}); err != nil {
		return
	}
}
