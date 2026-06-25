package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// handleGitHubInstallations returns the list of GitHub App installations.
// Returns 503 Service Unavailable if the GitHub App is not configured.
func (s *Server) handleGitHubInstallations(w http.ResponseWriter, r *http.Request) {
	if s.appAuth == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "GitHub App not configured", middleware.GetReqID(r.Context()))
		return
	}

	installations, err := s.appAuth.ListInstallations(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error(), middleware.GetReqID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(installations)
}

// handleGitHubInstallationRepositories returns the repositories for a GitHub
// App installation. The installation ID is taken from the URL path parameter {id}.
// Returns 400 if the ID is not a valid positive integer.
// Returns 503 Service Unavailable if the GitHub App is not configured.
func (s *Server) handleGitHubInstallationRepositories(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		writeJSONError(w, http.StatusBadRequest, "invalid installation ID: must be a positive integer", middleware.GetReqID(r.Context()))
		return
	}

	if s.appAuth == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "GitHub App not configured", middleware.GetReqID(r.Context()))
		return
	}

	repos, err := s.appAuth.ListInstallationRepositories(r.Context(), idStr)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error(), middleware.GetReqID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(repos)
}
