package api

import (
	"net/http"
	"strings"

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

	if s.serveSPA {
		s.registerSPARoutes()
	}
}

// registerSPARoutes registers the embedded SPA file server and catch-all
// fallback for client-side routing. It handles three cases:
//  1. Exact file matches (e.g., /favicon.svg, /assets/main.js) — serve the file
//  2. Root path (/) — serve index.html
//  3. Non-existent paths (e.g., /projects, /tickets/123) — SPA fallback to index.html
//
// API routes under /api/v1 and /health are registered first and take precedence.
func (s *Server) registerSPARoutes() {
	fsys := spaFilesystem()
	fileServer := http.FileServer(fsys)

	s.router.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Normalise the path: strip leading slash and handle empty (root).
		cleanPath := strings.TrimPrefix(r.URL.Path, "/")
		if cleanPath == "" {
			cleanPath = "."
		}

		// If the file exists in the embedded FS, serve it directly.
		// Otherwise, fall back to index.html for client-side routing.
		if _, err := fsys.Open(cleanPath); err != nil {
			// SPA fallback: rewrite URL to root and serve index.html.
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}

		fileServer.ServeHTTP(w, r)
	}))
}
