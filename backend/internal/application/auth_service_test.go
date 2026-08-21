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
	err          error
}

func (f *fakeUserRepository) CreateUser(
	email,
	passwordHash string,
) (*domain.User, error) {
	if f.err != nil {
		return nil, f.err
	}

	f.email = email
	f.passwordHash = passwordHash

	return &domain.User{
		ID:           "user-123",
		Email:        email,
		PasswordHash: passwordHash,
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
	)
	if err != nil {
		t.Fatalf("register user: %v", err)
	}

	if user.Email != "test@example.com" {
		t.Fatalf("unexpected email: %s", user.Email)
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
	)

	if !errors.Is(err, ErrEmailRequired) {
		t.Fatalf(
			"expected ErrEmailRequired, got %v",
			err,
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