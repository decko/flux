package api

import (
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

// handleGitHubInstallations returns the list of GitHub App installations.
// This is a stub that always returns 501 until the real handler is implemented.
func (s *Server) handleGitHubInstallations(w http.ResponseWriter, r *http.Request) {
	writeJSONError(w, http.StatusNotImplemented, "not implemented", middleware.GetReqID(r.Context()))
}

// handleGitHubInstallationRepositories returns the repositories for a GitHub
// App installation. This is a stub that always returns 501 until the real
// handler is implemented.
func (s *Server) handleGitHubInstallationRepositories(w http.ResponseWriter, r *http.Request) {
	writeJSONError(w, http.StatusNotImplemented, "not implemented", middleware.GetReqID(r.Context()))
}
