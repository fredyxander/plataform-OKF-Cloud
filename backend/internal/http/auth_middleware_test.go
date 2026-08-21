package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fredyxander/okf-platform/backend/internal/auth"
)

func TestAuthMiddlewareValidToken(t *testing.T) {
	tokens := auth.NewTokenManager("test-secret", time.Hour)

	token, err := tokens.Generate("user-123")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	next := func(w http.ResponseWriter, r *http.Request) {
		userID, ok := UserIDFromContext(r.Context())
		if !ok {
			t.Fatal("expected user ID in context")
		}

		if userID != "user-123" {
			t.Fatalf("unexpected user ID: %s", userID)
		}

		w.WriteHeader(http.StatusOK)
	}

	handler := AuthMiddleware(tokens, next)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAuthMiddlewareMissingToken(t *testing.T) {
	tokens := auth.NewTokenManager("test-secret", time.Hour)

	next := func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}

	handler := AuthMiddleware(tokens, next)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthMiddlewareInvalidToken(t *testing.T) {
	tokens := auth.NewTokenManager("test-secret", time.Hour)

	next := func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}

	handler := AuthMiddleware(tokens, next)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")

	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}