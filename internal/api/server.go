package api

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/decko/flux/internal/adapter/github"
	"github.com/decko/flux/internal/domain"
)

// Server is the HTTP server for the flux API. It wraps a chi router
// with middleware and routes configured.
type Server struct {
	router      *chi.Mux
	corsOrigin  string
	serveSPA    bool
	jwtSecret   []byte
	projectSvc  *domain.ProjectService
	ticketSvc   *domain.TicketService
	prSvc       *domain.PullRequestService
	pipelineSvc *domain.PipelineRunService
	auditSvc    *domain.AuditService
	authSvc     *domain.AuthService
	syncSvc     syncService
	syncMu      sync.Mutex
	adapters    map[string]domain.AdapterInfo
	appAuth     *github.AppAuth
}

// ServerOption configures a Server.
type ServerOption func(*Server)

// WithCORSOrigin sets the allowed CORS origin.
// Defaults to "*" (allow all) for development.
// TODO(#14): restrict CORS origin to SPA origin once configuration loading is in place.
func WithCORSOrigin(origin string) ServerOption {
	return func(s *Server) {
		s.corsOrigin = origin
	}
}

// WithSPA enables serving the embedded SPA frontend at the root path.
// When enabled, static files from the frontend build are served and
// non-API routes fall back to index.html for client-side routing.
func WithSPA() ServerOption {
	return func(s *Server) {
		s.serveSPA = true
	}
}

// WithJWTSecret sets the JWT signing secret used by the AuthMiddleware
// for token validation on protected routes.
func WithJWTSecret(secret []byte) ServerOption {
	return func(s *Server) {
		s.jwtSecret = secret
	}
}

// WithProjectService injects the project service for project CRUD endpoints.
func WithProjectService(svc *domain.ProjectService) ServerOption {
	return func(s *Server) {
		s.projectSvc = svc
	}
}

// WithTicketService injects the ticket service for ticket endpoints.
func WithTicketService(svc *domain.TicketService) ServerOption {
	return func(s *Server) {
		s.ticketSvc = svc
	}
}

// WithPRService injects the pull request service for pull request endpoints.
func WithPRService(svc *domain.PullRequestService) ServerOption {
	return func(s *Server) {
		s.prSvc = svc
	}
}

// WithPipelineService injects the pipeline run service for pipeline run endpoints.
func WithPipelineService(svc *domain.PipelineRunService) ServerOption {
	return func(s *Server) {
		s.pipelineSvc = svc
	}
}

// WithAuditService injects the audit service for audit event listing endpoints.
func WithAuditService(svc *domain.AuditService) ServerOption {
	return func(s *Server) {
		s.auditSvc = svc
	}
}

// WithAuthService injects the auth service for authentication endpoints.
func WithAuthService(svc *domain.AuthService) ServerOption {
	return func(s *Server) {
		s.authSvc = svc
	}
}

// WithSyncService injects the sync service for sync management endpoints.
func WithSyncService(svc syncService) ServerOption {
	return func(s *Server) {
		s.syncSvc = svc
	}
}

// WithAdapters injects configured adapter metadata for listing and health checks.
func WithAdapters(adapters map[string]domain.AdapterInfo) ServerOption {
	return func(s *Server) {
		s.adapters = adapters
	}
}

// WithAppAuth injects the GitHub App authentication handler for GitHub
// discovery endpoints (list installations, installation repositories).
func WithAppAuth(auth *github.AppAuth) ServerOption {
	return func(s *Server) {
		s.appAuth = auth
	}
}

// NewServer creates a new Server with all middleware and routes registered.
// Middleware order: ErrorHandler (outermost, catches panics in all downstream middleware) →
// RequestID → Logger → CORS
func NewServer(opts ...ServerOption) *Server {
	s := &Server{
		router:     chi.NewRouter(),
		corsOrigin: "*",
	}

	// Apply options before middleware registration so they affect
	// middleware configuration (e.g., CORS origin).
	for _, opt := range opts {
		opt(s)
	}

	// ErrorHandlerMiddleware must be outermost so it catches panics
	// from all downstream middleware (RequestID, slogLogger, CORS) and handlers.
	s.router.Use(ErrorHandlerMiddleware)
	s.router.Use(middleware.RequestID)
	s.router.Use(slogLogger)
	s.router.Use(CORSMiddleware(s.corsOrigin))

	// Override chi's default 404/405 handlers with JSON responses.
	s.router.NotFound(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSONError(w, http.StatusNotFound, "Not Found", middleware.GetReqID(r.Context()))
	}))
	s.router.MethodNotAllowed(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method Not Allowed", middleware.GetReqID(r.Context()))
	}))

	s.registerRoutes()

	return s
}

// ServeHTTP implements the http.Handler interface.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// slogLogger is a middleware that logs each request using structured logging.
// It sets the X-Request-Id response header and records the status code.
// The log call uses defer to ensure it runs even when downstream handlers panic.
func slogLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := middleware.GetReqID(r.Context())
		if reqID != "" {
			w.Header().Set("X-Request-Id", reqID)
		}

		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		defer func() {
			slog.LogAttrs(r.Context(), slog.LevelInfo, "request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", lrw.statusCode),
				slog.Duration("duration", time.Since(start)),
				slog.String("request_id", reqID),
			)
		}()
		next.ServeHTTP(lrw, r)
	})
}

// loggingResponseWriter wraps http.ResponseWriter to capture the status code.
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code and delegates to the wrapped ResponseWriter.
func (w *loggingResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}
