package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/fredyxander/okf-platform/backend/internal/auth"
)

type contextKey string

const userIDContextKey contextKey = "userID"

func AuthMiddleware(
	tokens *auth.TokenManager,
	next http.HandlerFunc,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")

		if header == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		const prefix = "Bearer "

		if !strings.HasPrefix(header, prefix) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimSpace(
			strings.TrimPrefix(header, prefix),
		)

		if tokenString == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		claims, err := tokens.Validate(tokenString)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(
			r.Context(),
			userIDContextKey,
			claims.UserID,
		)

		next(w, r.WithContext(ctx))
	}
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDContextKey).(string)

	return userID, ok
}