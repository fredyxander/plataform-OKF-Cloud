package database

import (
	"fmt"

	"github.com/fredyxander/okf-platform/backend/internal/domain"
)

// JobStatsByOwner cuenta los Jobs del propietario agrupados por estado.
//
// La consulta solo devuelve los estados presentes, así que el resultado
// se construye sobre una estructura con todos los contadores a cero: un
// estado sin Jobs debe reportarse como 0, no omitirse.
func (db *DB) JobStatsByOwner(ownerID string) (*domain.JobStats, error) {
	rows, err := db.conn.Query(`
		SELECT status, count(*)
		FROM jobs
		WHERE owner_id = $1
		GROUP BY status`,
		ownerID,
	)
	if err != nil {
		return nil, fmt.Errorf("job stats: %w", err)
	}

	defer rows.Close()

	stats := &domain.JobStats{}

	for rows.Next() {
		var (
			status domain.JobStatus
			count  int
		)

		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan job stats: %w", err)
		}

		switch status {
		case domain.JobStatusQueued:
			stats.Queued = count

		case domain.JobStatusProcessing:
			stats.Processing = count

		case domain.JobStatusCompleted:
			stats.Completed = count

		case domain.JobStatusFailed:
			stats.Failed = count
		}

		// El total suma lo que hay en la tabla, incluso si apareciera
		// un estado que esta versión todavía no conoce.
		stats.Total += count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("job stats: %w", err)
	}

	return stats, nil
}
