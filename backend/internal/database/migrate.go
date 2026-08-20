package database

import (
	"fmt"
	"io/fs"
	"log"
	"sort"

	"github.com/fredyxander/okf-platform/backend/migrations"
)

// migrationLockID identifica el advisory lock que serializa las
// migraciones. Si hubiera varias instancias del API arrancando al
// mismo tiempo, solo una aplica los cambios y las demás esperan.
const migrationLockID = 4823901

// Migrate aplica los archivos .sql embebidos que aún no se hayan
// ejecutado, en orden alfabético (001_, 002_, ...).
//
// Cada migración corre dentro de una transacción junto con el registro
// en schema_migrations: si el SQL falla, no queda marcada como aplicada.
func (db *DB) Migrate() error {
	if _, err := db.conn.Exec(
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT        PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
	); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	if _, err := db.conn.Exec(
		"SELECT pg_advisory_lock($1)",
		migrationLockID,
	); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}

	defer func() {
		if _, err := db.conn.Exec(
			"SELECT pg_advisory_unlock($1)",
			migrationLockID,
		); err != nil {
			log.Printf("could not release migration lock: %v", err)
		}
	}()

	applied, err := db.appliedMigrations()
	if err != nil {
		return err
	}

	files, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	sort.Strings(files)

	for _, name := range files {
		if applied[name] {
			continue
		}

		if err := db.applyMigration(name); err != nil {
			return err
		}

		log.Printf("migration applied: %s", name)
	}

	return nil
}

// appliedMigrations devuelve las versiones ya registradas.
func (db *DB) appliedMigrations() (map[string]bool, error) {
	rows, err := db.conn.Query("SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}

	defer rows.Close()

	applied := make(map[string]bool)

	for rows.Next() {
		var version string

		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan schema_migrations: %w", err)
		}

		applied[version] = true
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}

	return applied, nil
}

// applyMigration ejecuta un archivo .sql y lo marca como aplicado.
func (db *DB) applyMigration(name string) error {
	statements, err := migrations.FS.ReadFile(name)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", name, err)
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", name, err)
	}

	defer tx.Rollback()

	if _, err := tx.Exec(string(statements)); err != nil {
		return fmt.Errorf("apply migration %s: %w", name, err)
	}

	if _, err := tx.Exec(
		"INSERT INTO schema_migrations (version) VALUES ($1)",
		name,
	); err != nil {
		return fmt.Errorf("record migration %s: %w", name, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", name, err)
	}

	return nil
}
