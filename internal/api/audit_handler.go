package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
)

// handleAuditIntegrity handles GET /api/v1/audit/integrity.
// It verifies the hash chain of all audit events and returns whether the
// chain is intact. Admin role required.
func (s *Server) handleAuditIntegrity(w http.ResponseWriter, r *http.Request) {
	if s.auditSvc == nil {
		slog.Error("audit service not configured", "request_id", middleware.GetReqID(r.Context()))
		writeJSONError(w, http.StatusInternalServerError, "Internal Server Error", middleware.GetReqID(r.Context()))
		return
	}

	valid, firstBrokenAt, err := s.auditSvc.VerifyIntegrity(r.Context())
	if err != nil {
		slog.Error("verify audit integrity", "error", err, "request_id", middleware.GetReqID(r.Context()))
		writeJSONError(w, http.StatusInternalServerError, "Internal Server Error", middleware.GetReqID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"valid":           valid,
		"first_broken_at": firstBrokenAt,
	})
}
