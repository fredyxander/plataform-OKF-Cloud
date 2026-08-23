package application

import (
	"errors"
	"strings"

	"github.com/fredyxander/okf-platform/backend/internal/auth"
	"github.com/fredyxander/okf-platform/backend/internal/database"
	"github.com/fredyxander/okf-platform/backend/internal/domain"
)

var (
	ErrEmailRequired      = errors.New("email is required")
	ErrPasswordTooShort   = errors.New("password must contain at least 8 characters")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

type UserRepository interface {
	CreateUser(email, passwordHash, nombre, apellido string) (*domain.User, error)
	GetUserByEmail(email string) (*domain.User, error)
}

type LoginResult struct {
	User  *domain.User
	Token string
}

type AuthService struct {
	users  UserRepository
	tokens *auth.TokenManager
}

func NewAuthService(
	users UserRepository,
	tokens *auth.TokenManager,
) *AuthService {
	return &AuthService{
		users:  users,
		tokens: tokens,
	}
}

func (s *AuthService) Register(email, password, nombre, apellido string) (*domain.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, ErrEmailRequired
	}
	if len(password) < 8 {
		return nil, ErrPasswordTooShort
	}
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}
	user, err := s.users.CreateUser(email, passwordHash, nombre, apellido)
	if err != nil {
		if errors.Is(err, database.ErrAlreadyExists) {
			return nil, ErrEmailAlreadyExists
		}
		return nil, err
	}
	return user, nil
}

func (s *AuthService) Login(
	email,
	password string,
) (*LoginResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	user, err := s.users.GetUserByEmail(email)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if !auth.CheckPassword(password, user.PasswordHash) {
		return nil, ErrInvalidCredentials
	}
	token, err := s.tokens.Generate(user.ID)
	if err != nil {
		return nil, err
	}
	return &LoginResult{
		User:  user,
		Token: token,
	}, nil
}
