package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/decko/flux/internal/repository"
)

// handleAuditEvents handles GET /api/v1/audit-events.
// Admin-only. Supports query params: actor_id, resource_type, resource_id,
// action, since, until, limit, offset.
// Returns paginated audit events ordered by created_at descending.
func (s *Server) handleAuditEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var filter repository.AuditFilter

	if v := q.Get("actor_id"); v != "" {
		filter.ActorID = v
	}
	if v := q.Get("resource_type"); v != "" {
		filter.ResourceType = v
	}
	if v := q.Get("resource_id"); v != "" {
		filter.ResourceID = v
	}
	if v := q.Get("action"); v != "" {
		filter.Action = v
	}
	if v := q.Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.Since = t
		}
	}
	if v := q.Get("until"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			filter.Until = t
		}
	}
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			filter.Limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			filter.Offset = n
		}
	}

	events, err := s.auditSvc.List(r.Context(), filter)
	if err != nil {
		slog.Error("list audit events", "error", err, "request_id", middleware.GetReqID(r.Context()))
		writeJSONError(w, http.StatusInternalServerError, "Internal Server Error", middleware.GetReqID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(events)
}
