package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// Límites del pool de conexiones.
//
// database/sql no limita las conexiones abiertas por defecto, así que
// bajo concurrencia la API puede superar el max_connections de
// PostgreSQL (100 por defecto) y empezar a fallar con "too many
// clients already". Con este tope, el exceso de peticiones espera turno
// en lugar de agotar el servidor.
//
// 25 por proceso deja margen para la API y varios workers sobre la
// configuración por defecto de PostgreSQL.
const (
	maxOpenConns    = 25
	maxIdleConns    = 5
	connMaxLifetime = 5 * time.Minute
)

type DB struct {
	conn *sql.DB
}

func New(dsn string) (*DB, error) {
	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	conn.SetMaxOpenConns(maxOpenConns)
	conn.SetMaxIdleConns(maxIdleConns)

	// Reciclar conexiones evita quedarse con sesiones que el servidor
	// o un intermediario ya dieron por muertas.
	conn.SetConnMaxLifetime(connMaxLifetime)

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return &DB{conn: conn}, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}