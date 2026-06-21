package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/decko/flux/internal/repository"
)

// serviceError maps domain service errors to HTTP status codes and messages.
// Validation errors (returned unwrapped by services) get 400.
// ErrNotFound gets 404. All other errors (wrapped repository errors) get 500.
func serviceError(err error) (int, string) {
	if errors.Is(err, repository.ErrNotFound) {
		return http.StatusNotFound, "Not Found"
	}
	// Validation errors are returned unwrapped by the service; repo errors are wrapped.
	// We use a heuristic to distinguish them: validation messages contain known phrases.
	// TODO: replace with typed validation error for clean distinction.
	msg := err.Error()
	if strings.Contains(msg, "is required") || strings.Contains(msg, "invalid ") {
		return http.StatusBadRequest, msg
	}
	return http.StatusInternalServerError, "Internal Server Error"
}
