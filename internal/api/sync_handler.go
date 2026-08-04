package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/decko/flux/internal/domain"
	"github.com/decko/flux/pkg/authctx"
)

// syncService defines the interface for sync operations used by HTTP handlers.
type syncService interface {
	Status() domain.SyncStatus
	SyncNow(ctx context.Context) error
	SyncProject(ctx context.Context, projectID string) error
}

// projectSyncStatusResponse is the JSON body for a single project's sync status.
type projectSyncStatusResponse struct {
	ProjectID       string     `json:"project_id"`
	LastSyncAt      *time.Time `json:"last_sync_at"`
	LastSyncError   string     `json:"last_sync_error"`
	TicketsSynced   int        `json:"tickets_synced"`
	PRsSynced       int        `json:"prs_synced"`
	WebhooksHealthy bool       `json:"webhooks_healthy"`
}

// syncStatusResponse is the JSON body for GET /api/v1/sync/status.
type syncStatusResponse struct {
	LastSyncAt      *time.Time                           `json:"last_sync_at"`
	LastSyncError   string                               `json:"last_sync_error"`
	TicketsSynced   int                                  `json:"tickets_synced"`
	PRsSynced       int                                  `json:"prs_synced"`
	WebhooksHealthy bool                                 `json:"webhooks_healthy"`
	Projects        map[string]projectSyncStatusResponse `json:"projects"`
}

func (s *Server) handleSyncStatus(w http.ResponseWriter, r *http.Request) {
	if s.syncSvc == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "sync service not configured", middleware.GetReqID(r.Context()))
		return
	}

	status := s.syncSvc.Status()

	resp := syncStatusResponse{
		TicketsSynced:   status.TicketsSynced,
		PRsSynced:       status.PRsSynced,
		WebhooksHealthy: status.WebhooksHealthy,
		Projects:        make(map[string]projectSyncStatusResponse, len(status.Projects)),
	}
	// Only admins see error details (may contain upstream URLs, auth failures).
	isAdmin := authctx.Role(r.Context()) == "admin"
	if isAdmin {
		resp.LastSyncError = status.LastSyncError
	}
	if status.LastSyncAt != nil {
		resp.LastSyncAt = status.LastSyncAt
	}
	for projectID, ps := range status.Projects {
		p := projectSyncStatusResponse{
			ProjectID:       ps.ProjectID,
			LastSyncAt:      ps.LastSyncAt,
			TicketsSynced:   ps.TicketsSynced,
			PRsSynced:       ps.PRsSynced,
			WebhooksHealthy: ps.WebhooksHealthy,
		}
		if isAdmin {
			p.LastSyncError = ps.LastSyncError
		}
		resp.Projects[projectID] = p
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

// handleSyncProject triggers a sync for a single project.
// POST /api/v1/projects/{id}/sync (admin-only).
func (s *Server) handleSyncProject(w http.ResponseWriter, r *http.Request) {
	if s.syncSvc == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "sync service not available", middleware.GetReqID(r.Context()))
		return
	}
	projectID := chi.URLParam(r, "id")
	if projectID == "" {
		writeJSONError(w, http.StatusBadRequest, "project ID is required", middleware.GetReqID(r.Context()))
		return
	}

	if !s.syncMu.TryLock() {
		writeJSONError(w, http.StatusConflict, "sync already in progress", middleware.GetReqID(r.Context()))
		return
	}
	go func() {
		defer s.syncMu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := s.syncSvc.SyncProject(ctx, projectID); err != nil {
			slog.WarnContext(ctx, "project sync failed", "error", err, "project_id", projectID)
		}
	}()

	w.WriteHeader(http.StatusAccepted)
}
