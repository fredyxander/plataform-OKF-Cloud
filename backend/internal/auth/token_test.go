package auth

import (
	"testing"
	"time"
)

func TestGenerateToken(t *testing.T) {
	manager := NewTokenManager(
		"test-secret",
		time.Hour,
	)

	token, err := manager.Generate("user-123")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	if token == "" {
		t.Fatal("expected generated token")
	}
}

func TestValidateToken(t *testing.T) {
	manager := NewTokenManager(
		"test-secret",
		time.Hour,
	)

	token, err := manager.Generate("user-123")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	claims, err := manager.Validate(token)
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}

	if claims.UserID != "user-123" {
		t.Fatalf(
			"expected user ID user-123, got %s",
			claims.UserID,
		)
	}
}

//firma incorrecta rechazada
func TestValidateTokenWithWrongSecret(t *testing.T) {
	generator := NewTokenManager(
		"secret-one",
		time.Hour,
	)

	validator := NewTokenManager(
		"secret-two",
		time.Hour,
	)

	token, err := generator.Generate("user-123")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	_, err = validator.Validate(token)
	if err == nil {
		t.Fatal("expected token validation to fail")
	}
}