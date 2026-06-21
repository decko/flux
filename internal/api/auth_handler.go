package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/decko/flux/internal/repository"
)

// authRequest represents a login or registration request body.
type authRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// handleRegister handles POST /api/v1/auth/register.
// It decodes {email, password, role} from the JSON body, calls the auth
// service to register the user, and returns 201 Created with the user data
// (without the password hash). Returns 400 for validation errors and 409
// for duplicate email.
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON", middleware.GetReqID(r.Context()))
		return
	}

	user, err := s.authSvc.Register(r.Context(), req.Email, req.Password, req.Role)
	if err != nil {
		if errors.Is(err, repository.ErrDuplicateEmail) {
			writeJSONError(w, http.StatusConflict, "email already exists", middleware.GetReqID(r.Context()))
			return
		}
		// Validation errors and other errors get 400.
		writeJSONError(w, http.StatusBadRequest, err.Error(), middleware.GetReqID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/api/v1/auth/users/"+user.ID)
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(user)
}

// handleLogin handles POST /api/v1/auth/login.
// It decodes {email, password} from the JSON body, verifies credentials
// via the auth service, and returns 200 OK with a JWT token.
// Returns 401 for invalid credentials.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON", middleware.GetReqID(r.Context()))
		return
	}

	token, err := s.authSvc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeJSONError(w, http.StatusUnauthorized, "invalid credentials", middleware.GetReqID(r.Context()))
			return
		}
		slog.Error("login error", "error", err, "request_id", middleware.GetReqID(r.Context()))
		writeJSONError(w, http.StatusUnauthorized, "invalid credentials", middleware.GetReqID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
}

// handleRefresh handles POST /api/v1/auth/refresh.
// It reads the Bearer token from the Authorization header, validates it,
// and returns a new JWT token with extended expiry.
// Returns 401 for missing or invalid tokens.
func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		writeJSONError(w, http.StatusUnauthorized, "missing authorization header", middleware.GetReqID(r.Context()))
		return
	}

	// Extract Bearer token.
	var tokenStr string
	if err := scan(authHeader, "Bearer ", &tokenStr); err != nil {
		writeJSONError(w, http.StatusUnauthorized, "invalid authorization header format", middleware.GetReqID(r.Context()))
		return
	}

	newToken, err := s.authSvc.RefreshToken(r.Context(), tokenStr)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "invalid or expired token", middleware.GetReqID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"token": newToken})
}

// scan checks if s starts with prefix and, if so, stores the remainder in *dst.
// Returns an error if the prefix does not match.
func scan(s, prefix string, dst *string) error {
	if len(s) < len(prefix) || s[:len(prefix)] != prefix {
		return errors.New("prefix mismatch")
	}
	*dst = s[len(prefix):]
	return nil
}
