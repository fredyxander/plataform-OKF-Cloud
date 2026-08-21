package auth

import "testing"

func TestHashAndCheckPassword(t *testing.T) {
	password := "secure-password-123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	if hash == password {
		t.Fatal("password must not be stored in plain text")
	}

	if !CheckPassword(password, hash) {
		t.Fatal("expected correct password to match")
	}

	if CheckPassword("wrong-password", hash) {
		t.Fatal("expected incorrect password not to match")
	}
}