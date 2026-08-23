package database

import "testing"

// El pool debe quedar acotado: sin tope, la concurrencia agota el
// max_connections de PostgreSQL en lugar de esperar turno.
func TestConnectionPoolIsBounded(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	stats := db.conn.Stats()

	if stats.MaxOpenConnections != maxOpenConns {
		t.Fatalf(
			"expected at most %d open connections, got %d",
			maxOpenConns,
			stats.MaxOpenConnections,
		)
	}
}
