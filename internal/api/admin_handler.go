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
