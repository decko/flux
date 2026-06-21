package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// registerRoutes registers all API routes on the server's router.
func (s *Server) registerRoutes() {
	s.router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	s.router.Route("/api/v1", func(r chi.Router) {
		r.Post("/projects", s.handleCreateProject)
		r.Get("/projects", s.handleListProjects)
		r.Get("/projects/{id}", s.handleGetProject)
		r.Put("/projects/{id}", s.handleUpdateProject)
		r.Delete("/projects/{id}", s.handleDeleteProject)

		r.Get("/tickets", s.handleListTickets)
		r.Get("/tickets/{id}", s.handleGetTicket)
		r.Put("/tickets/{id}", s.handleUpdateTicket)

		r.Get("/pull-requests", s.handleListPRs)
		r.Get("/pull-requests/{id}", s.handleGetPR)
		r.Put("/pull-requests/{id}", s.handleUpdatePR)

		r.Get("/pipeline-runs", s.handleListPipelineRuns)
		r.Post("/pipeline-runs", s.handleCreatePipelineRun)
		r.Get("/pipeline-runs/{id}", s.handleGetPipelineRun)
	})
}
