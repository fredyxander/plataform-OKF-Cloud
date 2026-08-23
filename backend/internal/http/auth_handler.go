package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/fredyxander/okf-platform/backend/internal/application"
)

type AuthHandler struct {
	service *application.AuthService
}

func NewAuthHandler(service *application.AuthService) *AuthHandler {
	return &AuthHandler{
		service: service,
	}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Nombre   string `json:"nombre"`
		Apellido string `json:"apellido"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	user, err := h.service.Register(req.Email, req.Password, req.Nombre, req.Apellido)
	if err != nil {
		switch {
		case errors.Is(err, application.ErrEmailRequired):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, application.ErrPasswordTooShort):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, application.ErrEmailAlreadyExists):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			http.Error(w, "could not register user", http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	result, err := h.service.Login(req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, application.ErrInvalidCredentials):
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
		default:
			http.Error(w, "could not login", http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token": result.Token,
		"user":  result.User,
	})
}
