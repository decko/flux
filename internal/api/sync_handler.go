package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/decko/flux/internal/domain"
)

// syncService defines the interface for sync operations used by HTTP handlers.
type syncService interface {
	Status() domain.SyncStatus
	SyncNow(ctx context.Context) error
}

// syncStatusResponse is the JSON body for GET /api/v1/sync/status.
type syncStatusResponse struct {
	LastSyncAt      *time.Time `json:"last_sync_at"`
	LastSyncError   string     `json:"last_sync_error"`
	TicketsSynced   int        `json:"tickets_synced"`
	PRsSynced       int        `json:"prs_synced"`
	WebhooksHealthy bool       `json:"webhooks_healthy"`
}

func (s *Server) handleSyncStatus(w http.ResponseWriter, r *http.Request) {
	if s.syncSvc == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "sync service not configured", middleware.GetReqID(r.Context()))
		return
	}

	status := s.syncSvc.Status()

	resp := syncStatusResponse{
		LastSyncError:   status.LastSyncError,
		TicketsSynced:   status.TicketsSynced,
		PRsSynced:       status.PRsSynced,
		WebhooksHealthy: status.WebhooksHealthy,
	}
	if status.LastSyncAt != nil {
		resp.LastSyncAt = status.LastSyncAt
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleSyncTrigger(w http.ResponseWriter, r *http.Request) {
	if s.syncSvc == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "sync service not configured", middleware.GetReqID(r.Context()))
		return
	}

	if !s.syncMu.TryLock() {
		writeJSONError(w, http.StatusConflict, "sync already in progress", middleware.GetReqID(r.Context()))
		return
	}

	// Fire-and-forget sync. Runs in background to avoid blocking the request.
	go func() {
		defer s.syncMu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := s.syncSvc.SyncNow(ctx); err != nil {
			slog.WarnContext(ctx, "sync failed", "error", err)
		}
	}()

	w.WriteHeader(http.StatusAccepted)
}
