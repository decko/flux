package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// prPage is the JSON response envelope for the PR list endpoint.
type prPage struct {
	Items []model.PullRequest `json:"items"`
}

// handleListPRs handles GET /api/v1/pull-requests.
// Supports query params: project_id, status.
// Returns a JSON object with an "items" array.
func (s *Server) handleListPRs(w http.ResponseWriter, r *http.Request) {
	var filter repository.PullRequestFilter

	if pid := r.URL.Query().Get("project_id"); pid != "" {
		filter.ProjectID = pid
	}
	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = model.PRStatus(status)
	}

	prs, err := s.prSvc.List(r.Context(), filter)
	if err != nil {
		code, msg := serviceError(err)
		if code == http.StatusInternalServerError {
			slog.Error("list pull requests", "error", err, "request_id", middleware.GetReqID(r.Context()))
		}
		writeJSONError(w, code, msg, middleware.GetReqID(r.Context()))
		return
	}

	if prs == nil {
		prs = []model.PullRequest{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(prPage{Items: prs})
}

// handleGetPR handles GET /api/v1/pull-requests/{id}.
// It retrieves a pull request by its ID from the path parameter and returns
// 200 OK with the pull request JSON, or 404 Not Found if none exists.
func (s *Server) handleGetPR(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	pr, err := s.prSvc.Get(r.Context(), id)
	if err != nil {
		code, msg := serviceError(err)
		if code == http.StatusInternalServerError {
			slog.Error("get pull request", "error", err, "request_id", middleware.GetReqID(r.Context()))
		}
		writeJSONError(w, code, msg, middleware.GetReqID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(pr)
}

// handleUpdatePR handles PUT /api/v1/pull-requests/{id}.
// It decodes the updated PullRequest from the JSON body, validates that the
// URL path ID matches the body ID, updates the timestamp, and delegates
// to the pull request service. Returns 200 OK on success, 400 on validation
// or ID mismatch, and 404 if the pull request does not exist.
func (s *Server) handleUpdatePR(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var pr model.PullRequest
	if err := json.NewDecoder(r.Body).Decode(&pr); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON", middleware.GetReqID(r.Context()))
		return
	}

	if pr.ID != id {
		writeJSONError(w, http.StatusBadRequest, "ID mismatch", middleware.GetReqID(r.Context()))
		return
	}

	pr.UpdatedAt = time.Now().UTC()

	if err := s.prSvc.Update(r.Context(), pr); err != nil {
		code, msg := serviceError(err)
		if code == http.StatusInternalServerError {
			slog.Error("update pull request", "error", err, "request_id", middleware.GetReqID(r.Context()))
		}
		writeJSONError(w, code, msg, middleware.GetReqID(r.Context()))
		return
	}

	// Re-fetch to return the persisted state with correct timestamps.
	persisted, err := s.prSvc.Get(r.Context(), id)
	if err != nil {
		slog.Error("re-fetch after update pull request", "error", err, "request_id", middleware.GetReqID(r.Context()))
		writeJSONError(w, http.StatusInternalServerError, "Internal Server Error", middleware.GetReqID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(persisted)
}
