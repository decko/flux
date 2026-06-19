package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r := newRouter()

	addr := ":" + port
	log.Printf("flux listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}

// newRouter creates and configures the chi router with middleware and all routes.
// It sets up logging, recovery, and request ID middleware, then registers
// the /health endpoint and the /api/v1 route group.
func newRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "ok")
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/projects", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, "[]")
		})
	})

	return r
}
