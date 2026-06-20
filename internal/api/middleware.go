package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

// CORSMiddleware returns middleware that adds CORS headers to every response.
// The origin parameter controls the Access-Control-Allow-Origin header value.
// Standard methods (GET, POST, PUT, DELETE, OPTIONS) and headers
// (Content-Type, Authorization) are always allowed.
// Returns 204 No Content for preflight OPTIONS requests.
func CORSMiddleware(origin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ErrorHandlerMiddleware catches panics and converts them to JSON error responses.
func ErrorHandlerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				writeJSONError(w, http.StatusInternalServerError, "Internal Server Error", middleware.GetReqID(r.Context()))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// writeJSONError writes a JSON error response with the given status code, message, and request ID.
func writeJSONError(w http.ResponseWriter, status int, message, reqID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":      message,
		"request_id": reqID,
	})
}
