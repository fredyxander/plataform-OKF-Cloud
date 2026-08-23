package application

import (
	"errors"
	"testing"
	"time"

	"github.com/fredyxander/okf-platform/backend/internal/auth"
	"github.com/fredyxander/okf-platform/backend/internal/database"
	"github.com/fredyxander/okf-platform/backend/internal/domain"
)

type fakeUserRepository struct {
	email        string
	passwordHash string
	nombre       string
	apellido     string
	err          error
}

func (f *fakeUserRepository) CreateUser(
	email,
	passwordHash,
	nombre,
	apellido string,
) (*domain.User, error) {
	if f.err != nil {
		return nil, f.err
	}

	f.email = email
	f.passwordHash = passwordHash
	f.nombre = nombre
	f.apellido = apellido

	return &domain.User{
		ID:           "user-123",
		Email:        email,
		PasswordHash: passwordHash,
		Nombre:       nombre,
		Apellido:     apellido,
	}, nil
}

func (f *fakeUserRepository) GetUserByEmail(
	email string,
) (*domain.User, error) {
	if f.err != nil {
		return nil, f.err
	}

	return &domain.User{
		ID:           "user-123",
		Email:        email,
		PasswordHash: f.passwordHash,
	}, nil
}

// Register
func TestRegisterUser(t *testing.T) {
	repo := &fakeUserRepository{}
	// TokenManager usado por AuthService durante el test
	tokens := auth.NewTokenManager(
		"test-secret",
		time.Hour,
	)
	service := NewAuthService(repo, tokens)

	password := "secure-password-123"

	user, err := service.Register(
		"  TEST@Example.com ",
		password,
		"  Pepe  ",
		"  Perez  ",
	)
	if err != nil {
		t.Fatalf("register user: %v", err)
	}

	if user.Email != "test@example.com" {
		t.Fatalf("unexpected email: %s", user.Email)
	}

	// El nombre llega desde un formulario, así que se normaliza igual
	// que el email antes de persistirlo.
	if user.Nombre != "Pepe" || user.Apellido != "Perez" {
		t.Fatalf(
			"expected trimmed name, got %q %q",
			user.Nombre,
			user.Apellido,
		)
	}

	if repo.passwordHash == password {
		t.Fatal("password was stored in plain text")
	}

	if !auth.CheckPassword(password, repo.passwordHash) {
		t.Fatal("stored password hash does not match password")
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	repo := &fakeUserRepository{
		err: database.ErrAlreadyExists,
	}
	tokens := auth.NewTokenManager(
		"test-secret",
		time.Hour,
	)
	service := NewAuthService(repo, tokens)

	_, err := service.Register(
		"test@example.com",
		"secure-password-123",
		"Pepe",
		"Perez",
	)

	if !errors.Is(err, ErrEmailAlreadyExists) {
		t.Fatalf(
			"expected ErrEmailAlreadyExists, got %v",
			err,
		)
	}
}

func TestRegisterWithoutEmail(t *testing.T) {
	repo := &fakeUserRepository{}
	tokens := auth.NewTokenManager(
		"test-secret",
		time.Hour,
	)
	service := NewAuthService(repo, tokens)

	_, err := service.Register(
		"   ",
		"secure-password-123",
		"Pepe",
		"Perez",
	)

	if !errors.Is(err, ErrEmailRequired) {
		t.Fatalf(
			"expected ErrEmailRequired, got %v",
			err,
		)
	}
}

// El enunciado solo exige credenciales: nombre y apellido son datos de
// perfil y su ausencia no puede impedir el registro.
func TestRegisterWithoutNameIsAllowed(t *testing.T) {
	repo := &fakeUserRepository{}
	tokens := auth.NewTokenManager(
		"test-secret",
		time.Hour,
	)
	service := NewAuthService(repo, tokens)

	user, err := service.Register(
		"test@example.com",
		"secure-password-123",
		"",
		"   ",
	)
	if err != nil {
		t.Fatalf("register user without name: %v", err)
	}

	if user.Nombre != "" || user.Apellido != "" {
		t.Fatalf(
			"expected empty name, got %q %q",
			user.Nombre,
			user.Apellido,
		)
	}
}

func TestRegisterWithShortPassword(t *testing.T) {
	repo := &fakeUserRepository{}
	tokens := auth.NewTokenManager(
		"test-secret",
		time.Hour,
	)
	service := NewAuthService(repo, tokens)

	_, err := service.Register(
		"test@example.com",
		"1234567",
		"Pepe",
		"Perez",
	)

	if !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf(
			"expected ErrPasswordTooShort, got %v",
			err,
		)
	}
}

//login
func TestLoginSuccess(t *testing.T) {
	password := "secure-password-123"

	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	repo := &fakeUserRepository{
		passwordHash: hash,
	}
	tokens := auth.NewTokenManager(
		"test-secret",
		time.Hour,
	)
	service := NewAuthService(repo, tokens)

	result, err := service.Login(
		"TEST@Example.com",
		password,
	)
	if err != nil {
		t.Fatalf("login user: %v", err)
	}

	if result.User.ID != "user-123" {
		t.Fatalf(
			"unexpected user ID: %s",
			result.User.ID,
		)
	}

	if result.User.Email != "test@example.com" {
		t.Fatalf(
			"unexpected email: %s",
			result.User.Email,
		)
	}

	if result.Token == "" {
		t.Fatal("expected login token")
	}

	claims, err := tokens.Validate(result.Token)
	if err != nil {
		t.Fatalf("validate login token: %v", err)
	}

	if claims.UserID != result.User.ID {
		t.Fatal("token user ID does not match authenticated user")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	hash, err := auth.HashPassword("correct-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	repo := &fakeUserRepository{
		passwordHash: hash,
	}
	tokens := auth.NewTokenManager(
		"test-secret",
		time.Hour,
	)

	service := NewAuthService(repo, tokens)

	_, err = service.Login(
		"test@example.com",
		"wrong-password",
	)

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf(
			"expected ErrInvalidCredentials, got %v",
			err,
		)
	}
}

func TestLoginUserNotFound(t *testing.T) {
	repo := &fakeUserRepository{
		err: database.ErrNotFound,
	}
	tokens := auth.NewTokenManager(
		"test-secret",
		time.Hour,
	)
	service := NewAuthService(repo, tokens)

	_, err := service.Login(
		"missing@example.com",
		"some-password",
	)

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf(
			"expected ErrInvalidCredentials, got %v",
			err,
		)
	}
}