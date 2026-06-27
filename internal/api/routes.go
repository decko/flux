package api

import (
	"io/fs"
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

	// Public webhook endpoint — outside auth middleware because GitHub
	// signs with HMAC, not JWTs.
	s.router.Post("/api/v1/webhooks/github", s.handleGitHubWebhook)

	s.router.Route("/api/v1", func(r chi.Router) {
		// Public auth routes.
		r.Post("/auth/register", s.handleRegister)
		r.Post("/auth/login", s.handleLogin)
		r.Post("/auth/refresh", s.handleRefresh)

		// Protected routes — require valid JWT token.
		r.Group(func(r chi.Router) {
			r.Use(AuthMiddleware(s.jwtSecret))

			r.Post("/projects", s.handleCreateProject)
			r.Get("/projects", s.handleListProjects)
			r.Get("/projects/{id}", s.handleGetProject)
			r.Put("/projects/{id}", s.handleUpdateProject)

			r.Get("/tickets", s.handleListTickets)
			r.Get("/tickets/{id}", s.handleGetTicket)
			r.Put("/tickets/{id}", s.handleUpdateTicket)

			r.Get("/pull-requests", s.handleListPRs)
			r.Get("/pull-requests/{id}", s.handleGetPR)
			r.Put("/pull-requests/{id}", s.handleUpdatePR)

			r.Get("/pipeline-runs", s.handleListPipelineRuns)
			r.Post("/pipeline-runs", s.handleCreatePipelineRun)
			r.Get("/pipeline-runs/{id}", s.handleGetPipelineRun)

			r.Get("/sync/status", s.handleSyncStatus)
			r.Get("/adapters", s.handleListAdapters)
			r.Get("/adapters/{type}/health", s.handleAdapterHealth)
			r.Get("/projects/{id}/trigger-rules", s.handleListTriggerRules)
			r.Get("/github/installations", s.handleGitHubInstallations)
			r.Get("/github/installations/{id}/repositories", s.handleGitHubInstallationRepositories)

			// Admin-only routes.
			r.Group(func(r chi.Router) {
				r.Use(RequireRole("admin"))
				r.Get("/audit-events", s.handleAuditEvents)
				r.Get("/audit/integrity", s.handleAuditIntegrity)
				r.Delete("/projects/{id}", s.handleDeleteProject)
				r.Post("/projects/{id}/webhook/rotate-secret", s.handleRotateWebhookSecret)
				r.Post("/projects/{id}/trigger-rules", s.handleCreateTriggerRule)
				r.Put("/projects/{id}/trigger-rules/{ruleId}", s.handleUpdateTriggerRule)
				r.Delete("/projects/{id}/trigger-rules/{ruleId}", s.handleDeleteTriggerRule)
				r.Post("/pipeline-runs/{id}/trigger", s.handleTriggerPipelineRun)
				r.Post("/pipeline-runs/{id}/cancel", s.handleCancelPipelineRun)
				r.Post("/sync/trigger", s.handleSyncTrigger)
			})

			// Admin user management routes — under /admin prefix.
			r.Route("/admin", func(r chi.Router) {
				r.Use(RequireRole("admin"))
				r.Get("/users", s.handleListUsers)
				r.Post("/users", s.handleCreateUser)
				r.Put("/users/{id}/role", s.handleUpdateUserRole)
				r.Put("/users/{id}/password", s.handleResetPassword)
				r.Delete("/users/{id}", s.handleDeleteUser)
			})
		})
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
	subFS := spaFS()
	fileServer := http.FileServer(fsys)

	s.router.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Normalise the path: strip leading slash and handle empty (root).
		cleanPath := strings.TrimPrefix(r.URL.Path, "/")
		if cleanPath == "" {
			cleanPath = "."
		}

		// If the file exists in the embedded FS, serve it directly.
		// Otherwise, fall back to index.html for client-side routing.
		if _, err := fs.Stat(subFS, cleanPath); err != nil {
			// SPA fallback: rewrite URL to root and serve index.html.
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}

		fileServer.ServeHTTP(w, r)
	}))
}
