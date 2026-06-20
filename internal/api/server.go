package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server is the HTTP server for the flux API. It wraps a chi router
// with middleware and routes configured.
type Server struct {
	router     *chi.Mux
	corsOrigin string
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
func slogLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := middleware.GetReqID(r.Context())
		if reqID != "" {
			w.Header().Set("X-Request-Id", reqID)
		}

		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(lrw, r)

		slog.LogAttrs(r.Context(), slog.LevelInfo, "request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", lrw.statusCode),
			slog.Duration("duration", time.Since(start)),
			slog.String("request_id", reqID),
		)
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
