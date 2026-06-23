package api

import (
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
// It delegates to the pipeline run service to notify the orchestrator and
// set the run status to running. Returns 202 Accepted on success.
func (s *Server) handleTriggerPipelineRun(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Try using the ticket external ID (soda expects GitHub issue numbers).
	if s.ticketSvc != nil {
		if run, err := s.pipelineSvc.Get(r.Context(), id); err == nil {
			if tkt, tktErr := s.ticketSvc.Get(r.Context(), run.TicketID); tktErr == nil && tkt.ExternalID != "" {
				err = s.pipelineSvc.TriggerWithTicketID(r.Context(), id, tkt.ExternalID)
				if err == nil {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusAccepted)
					_ = json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
					return
				}
				code, msg := serviceError(err)
				if code == http.StatusInternalServerError {
					slog.Error("trigger pipeline run", "error", err, "request_id", middleware.GetReqID(r.Context()))
				}
				writeJSONError(w, code, msg, middleware.GetReqID(r.Context()))
				return
			}
		}
	}

	if err := s.pipelineSvc.Trigger(r.Context(), id); err != nil {
		code, msg := serviceError(err)
		if code == http.StatusInternalServerError {
			slog.Error("trigger pipeline run", "error", err, "request_id", middleware.GetReqID(r.Context()))
		}
		writeJSONError(w, code, msg, middleware.GetReqID(r.Context()))
		return
	}

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
