package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/fredyxander/okf-platform/backend/internal/domain"
)

var ErrNotFound = errors.New("not found")

func (db *DB) CreateUser(email, passwordHash string) (*domain.User, error) {
	u := &domain.User{}
	err := db.conn.QueryRow(`
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id, email, password_hash, created_at`,
		email, passwordHash,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

func (db *DB) GetUserByEmail(email string) (*domain.User, error) {
	u := &domain.User{}
	err := db.conn.QueryRow(`
		SELECT id, email, password_hash, created_at
		FROM users WHERE email = $1`,
		email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
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
		SELECT id, email, password_hash, created_at
		FROM users WHERE id = $1`,
		id,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}
