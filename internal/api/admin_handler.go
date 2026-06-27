package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/decko/flux/internal/repository"
	"github.com/decko/flux/pkg/authctx"
)

// createUserRequest represents a create user request body.
type createUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// resetPasswordRequest represents a reset password request body.
type resetPasswordRequest struct {
	Password string `json:"password"`
}

// updateRoleRequest represents a role update request body.
type updateRoleRequest struct {
	Role string `json:"role"`
}

// handleListUsers handles GET /api/v1/admin/users.
// Returns all users as a JSON array. Requires admin role.
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	if s.userSvc == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "user service not available", middleware.GetReqID(r.Context()))
		return
	}

	users, err := s.userSvc.ListUsers(r.Context())
	if err != nil {
		slog.Error("list users", "error", err, "request_id", middleware.GetReqID(r.Context()))
		writeJSONError(w, http.StatusInternalServerError, "Internal Server Error", middleware.GetReqID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(users)
}

// handleUpdateUserRole handles PUT /api/v1/admin/users/{id}/role.
// Body: {"role": "admin"} or {"role": "user"}.
// Returns 400 for invalid role, self-demotion, or last admin demotion.
func (s *Server) handleUpdateUserRole(w http.ResponseWriter, r *http.Request) {
	if s.userSvc == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "user service not available", middleware.GetReqID(r.Context()))
		return
	}

	var req updateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON", middleware.GetReqID(r.Context()))
		return
	}

	actorID := authctx.UserID(r.Context())
	targetID := chi.URLParam(r, "id")

	err := s.userSvc.UpdateRole(r.Context(), actorID, targetID, req.Role)
	if err != nil {
		status, msg := serviceError(err)
		writeJSONError(w, status, msg, middleware.GetReqID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

// handleDeleteUser handles DELETE /api/v1/admin/users/{id}.
// Returns 204 No Content on success.
func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	if s.userSvc == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "user service not available", middleware.GetReqID(r.Context()))
		return
	}

	actorID := authctx.UserID(r.Context())
	targetID := chi.URLParam(r, "id")

	err := s.userSvc.DeleteUser(r.Context(), actorID, targetID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeJSONError(w, http.StatusNotFound, "Not Found", middleware.GetReqID(r.Context()))
			return
		}
		status, msg := serviceError(err)
		writeJSONError(w, status, msg, middleware.GetReqID(r.Context()))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleCreateUser handles POST /api/v1/admin/users.
// Body: {"email","password","role"}. Returns 201 Created with the new user.
func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if s.userSvc == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "user service not available", middleware.GetReqID(r.Context()))
		return
	}
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON", middleware.GetReqID(r.Context()))
		return
	}
	actorID := authctx.UserID(r.Context())
	user, err := s.userSvc.CreateUser(r.Context(), actorID, req.Email, req.Password, req.Role)
	if err != nil {
		status, msg := serviceError(err)
		writeJSONError(w, status, msg, middleware.GetReqID(r.Context()))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(user)
}

// handleResetPassword handles PUT /api/v1/admin/users/{id}/password.
// Body: {"password"}. Returns 200 OK with the updated user on success.
// handleRotateWebhookSecret handles POST /api/v1/projects/{id}/webhook/rotate-secret.
// It generates a new webhook secret, updates the GitHub webhook, stores the new
// secret, and records an audit event. Requires admin role.
// Returns 200 with {"status":"rotated"} on success.
func (s *Server) handleRotateWebhookSecret(w http.ResponseWriter, r *http.Request) {
	if s.projectSvc == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "project service not available", middleware.GetReqID(r.Context()))
		return
	}

	projectID := chi.URLParam(r, "id")
	if projectID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing project ID", middleware.GetReqID(r.Context()))
		return
	}

	if err := s.projectSvc.RotateWebhookSecret(r.Context(), projectID); err != nil {
		code, msg := serviceError(err)
		if code == http.StatusInternalServerError {
			slog.Error("rotate webhook secret", "error", err, "request_id", middleware.GetReqID(r.Context()))
		}
		writeJSONError(w, code, msg, middleware.GetReqID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "rotated"})
}

func (s *Server) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	if s.userSvc == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "user service not available", middleware.GetReqID(r.Context()))
		return
	}
	var req resetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON", middleware.GetReqID(r.Context()))
		return
	}
	actorID := authctx.UserID(r.Context())
	targetID := chi.URLParam(r, "id")
	user, err := s.userSvc.ResetPassword(r.Context(), actorID, targetID, req.Password)
	if err != nil {
		status, msg := serviceError(err)
		writeJSONError(w, status, msg, middleware.GetReqID(r.Context()))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(user)
}
