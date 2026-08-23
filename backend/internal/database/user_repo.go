package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/lib/pq"

	"github.com/fredyxander/okf-platform/backend/internal/domain"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
)

func (db *DB) CreateUser(email, passwordHash, nombre, apellido string) (*domain.User, error) {
	u := &domain.User{}
	err := db.conn.QueryRow(`
		INSERT INTO users (email, password_hash, nombre, apellido)
		VALUES ($1, $2, $3, $4)
		RETURNING id, email, password_hash, nombre, apellido, created_at`,
		email, passwordHash, nombre, apellido,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Nombre, &u.Apellido, &u.CreatedAt)
	if err != nil {
		var pqErr *pq.Error

		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return nil, ErrAlreadyExists
		}

		return nil, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

func (db *DB) GetUserByEmail(email string) (*domain.User, error) {
	u := &domain.User{}
	err := db.conn.QueryRow(`
		SELECT id, email, password_hash, nombre, apellido, created_at
		FROM users WHERE email = $1`,
		email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Nombre, &u.Apellido, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return u, nil
}

func (db *DB) GetUserByID(id string) (*domain.User, error) {
	u := &domain.User{}
	err := db.conn.QueryRow(`
		SELECT id, email, password_hash, nombre, apellido, created_at
		FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Nombre, &u.Apellido, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}
