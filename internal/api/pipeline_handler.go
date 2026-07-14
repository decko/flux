package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// pipelineRunPage is the JSON response envelope for the pipeline run list endpoint.
type pipelineRunPage struct {
	Items []model.PipelineRun `json:"items"`
}

// handleListPipelineRuns handles GET /api/v1/pipeline-runs.
// Supports query params: project_id, ticket_id, status.
// Returns a JSON object with an "items" array.
func (s *Server) handleListPipelineRuns(w http.ResponseWriter, r *http.Request) {
	var filter repository.PipelineRunFilter

	if pid := r.URL.Query().Get("project_id"); pid != "" {
		filter.ProjectID = pid
	}
	if tid := r.URL.Query().Get("ticket_id"); tid != "" {
		filter.TicketID = tid
	}
	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = model.RunStatus(status)
	}

	runs, err := s.pipelineSvc.List(r.Context(), filter)
	if err != nil {
		code, msg := serviceError(err)
		if code == http.StatusInternalServerError {
			slog.Error("list pipeline runs", "error", err, "request_id", middleware.GetReqID(r.Context()))
		}
		writeJSONError(w, code, msg, middleware.GetReqID(r.Context()))
		return
	}

	if runs == nil {
		runs = []model.PipelineRun{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(pipelineRunPage{Items: runs})
}

// handleGetPipelineRun handles GET /api/v1/pipeline-runs/{id}.
// It retrieves a pipeline run by its ID and returns 200 OK with the run JSON,
// or 404 Not Found if no pipeline run with the given ID exists.
func (s *Server) handleGetPipelineRun(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	run, err := s.pipelineSvc.Get(r.Context(), id)
	if err != nil {
		code, msg := serviceError(err)
		if code == http.StatusInternalServerError {
			slog.Error("get pipeline run", "error", err, "request_id", middleware.GetReqID(r.Context()))
		}
		writeJSONError(w, code, msg, middleware.GetReqID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(run)
}

// handleCreatePipelineRun handles POST /api/v1/pipeline-runs.
// It decodes a PipelineRun from the JSON body, generates an ID and timestamp,
// defaults status to "pending" if empty, delegates to the pipeline run service,
// and returns 201 Created with the Location header set to the new resource's URL.
func (s *Server) handleCreatePipelineRun(w http.ResponseWriter, r *http.Request) {
	var run model.PipelineRun
	if err := json.NewDecoder(r.Body).Decode(&run); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON", middleware.GetReqID(r.Context()))
		return
	}

	run.ID = uuid.New().String()
	if run.Status == "" {
		run.Status = model.RunStatusPending
	} else if run.Status != model.RunStatusPending {
		// Reject valid-but-wrong-for-creation statuses (completed, failed, etc.)
		// to prevent bypassing the pending → trigger → running → terminal lifecycle.
		// Bogus strings (e.g., "bogus") fall through to service-layer validation.
		switch run.Status {
		case model.RunStatusRunning, model.RunStatusCompleted, model.RunStatusFailed, model.RunStatusCanceled:
			writeJSONError(w, http.StatusBadRequest, "status must be empty or 'pending'", middleware.GetReqID(r.Context()))
			return
		}
	}
	run.StartedAt = time.Now().UTC()

	if err := s.pipelineSvc.Create(r.Context(), run); err != nil {
		code, msg := serviceError(err)
		if code == http.StatusInternalServerError {
			slog.Error("create pipeline run", "error", err, "request_id", middleware.GetReqID(r.Context()))
		}
		writeJSONError(w, code, msg, middleware.GetReqID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/api/v1/pipeline-runs/"+run.ID)
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(run)
}

// handleTriggerPipelineRun handles POST /api/v1/pipeline-runs/{id}/trigger.
// It validates the run exists and is pending, then spawns a goroutine to execute
// the orchestrator asynchronously. Returns 202 Accepted immediately — the caller
// should poll GET /pipeline-runs/{id} to observe status transitions from pending
// to running to completed/failed.
func (s *Server) handleTriggerPipelineRun(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Fetch the run once — validates it exists and lets us check status.
	run, err := s.pipelineSvc.Get(r.Context(), id)
	if err != nil {
		code, msg := serviceError(err)
		writeJSONError(w, code, msg, middleware.GetReqID(r.Context()))
		return
	}
	if run.Status != model.RunStatusPending {
		writeJSONError(w, http.StatusConflict, "pipeline run is not in pending status", middleware.GetReqID(r.Context()))
		return
	}

	// Try using the ticket external ID (soda expects GitHub issue numbers).
	externalTicketID := ""
	if s.ticketSvc != nil {
		if tkt, tktErr := s.ticketSvc.Get(r.Context(), run.TicketID); tktErr == nil && tkt.ExternalID != "" {
			externalTicketID = tkt.ExternalID
		} else if tktErr != nil {
			slog.WarnContext(r.Context(), "ticket lookup failed, proceeding without external ID",
				"run_id", id, "ticket_id", run.TicketID, "error", tktErr)
		}
	}

	// Fire-and-forget: execute the orchestrator in a background goroutine
	// with a detached context so the request lifecycle doesn't kill soda.
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("trigger pipeline run panicked", "run_id", id, "panic", r)
				// Best-effort: mark run as failed so the frontend doesn't show
				// a perpetually-running spinner. If this also fails, the
				// reconciler (#281) will catch it.
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				run, err := s.pipelineSvc.Get(ctx, id)
				if err != nil {
					return
				}
				run.Status = model.RunStatusFailed
				now := time.Now().UTC()
				run.CompletedAt = &now
				_ = s.pipelineSvc.Update(ctx, run)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
		defer cancel()
		if err := s.pipelineSvc.TriggerWithTicketID(ctx, id, externalTicketID); err != nil {
			slog.WarnContext(ctx, "trigger pipeline run failed", "run_id", id, "error", err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
}

// handleCancelPipelineRun handles POST /api/v1/pipeline-runs/{id}/cancel.
// It delegates to the pipeline run service to notify the orchestrator and
// set the run status to canceled. Returns 200 OK on success.
func (s *Server) handleCancelPipelineRun(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := s.pipelineSvc.Cancel(r.Context(), id); err != nil {
		code, msg := serviceError(err)
		if code == http.StatusInternalServerError {
			slog.Error("cancel pipeline run", "error", err, "request_id", middleware.GetReqID(r.Context()))
		}
		writeJSONError(w, code, msg, middleware.GetReqID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "canceled"})
}
