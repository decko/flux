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

// handleListTriggerRules handles GET /api/v1/projects/{id}/trigger-rules.
// It fetches the project to verify it exists (404 if not), then lists all
// trigger rules for that project. Any authenticated user can access.
func (s *Server) handleListTriggerRules(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	_, err := s.projectSvc.Get(r.Context(), projectID)
	if err != nil {
		code, msg := serviceError(err)
		if code == http.StatusInternalServerError {
			slog.Error("get project for trigger rules", "error", err, "request_id", middleware.GetReqID(r.Context()))
		}
		writeJSONError(w, code, msg, middleware.GetReqID(r.Context()))
		return
	}

	rules, err := s.triggerRuleRepo.ListByProject(r.Context(), projectID)
	if err != nil {
		slog.Error("list trigger rules", "error", err, "request_id", middleware.GetReqID(r.Context()))
		writeJSONError(w, http.StatusInternalServerError, "Internal Server Error", middleware.GetReqID(r.Context()))
		return
	}

	if rules == nil {
		rules = []model.TriggerRule{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(rules)
}

// handleCreateTriggerRule handles POST /api/v1/projects/{id}/trigger-rules.
// It decodes a TriggerRule from the JSON body, validates label and pipeline
// are non-empty, validates the pipeline name exists in the project, sets ID
// and timestamps, persists via the repository, and returns 201 Created with
// a Location header. Admin-only.
func (s *Server) handleCreateTriggerRule(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	project, err := s.projectSvc.Get(r.Context(), projectID)
	if err != nil {
		code, msg := serviceError(err)
		if code == http.StatusInternalServerError {
			slog.Error("get project for create trigger rule", "error", err, "request_id", middleware.GetReqID(r.Context()))
		}
		writeJSONError(w, code, msg, middleware.GetReqID(r.Context()))
		return
	}

	var rule model.TriggerRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON", middleware.GetReqID(r.Context()))
		return
	}

	if rule.Label == "" {
		writeJSONError(w, http.StatusBadRequest, "label is required", middleware.GetReqID(r.Context()))
		return
	}
	if rule.Pipeline == "" {
		writeJSONError(w, http.StatusBadRequest, "pipeline is required", middleware.GetReqID(r.Context()))
		return
	}

	// Validate that the pipeline name exists in the project's pipeline configs.
	pipelineValid := false
	for _, p := range project.Pipelines {
		if p.Name == rule.Pipeline {
			pipelineValid = true
			break
		}
	}
	if !pipelineValid {
		writeJSONError(w, http.StatusBadRequest, "invalid pipeline: pipeline not found in project configuration", middleware.GetReqID(r.Context()))
		return
	}

	rule.ID = uuid.New().String()
	rule.ProjectID = projectID
	now := time.Now().UTC()
	rule.CreatedAt = now
	rule.UpdatedAt = now

	if err := s.triggerRuleRepo.Create(r.Context(), rule); err != nil {
		slog.Error("create trigger rule", "error", err, "request_id", middleware.GetReqID(r.Context()))
		writeJSONError(w, http.StatusInternalServerError, "Internal Server Error", middleware.GetReqID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/api/v1/projects/"+projectID+"/trigger-rules/"+rule.ID)
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(rule)
}

// handleUpdateTriggerRule handles PUT /api/v1/projects/{id}/trigger-rules/{ruleId}.
// It decodes a TriggerRule from the JSON body, sets the ID from the URL param,
// updates the timestamp, and delegates to the repository. Admin-only.
func (s *Server) handleUpdateTriggerRule(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	ruleID := chi.URLParam(r, "ruleId")

	var rule model.TriggerRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON", middleware.GetReqID(r.Context()))
		return
	}

	rule.ID = ruleID
	rule.ProjectID = projectID
	rule.UpdatedAt = time.Now().UTC()

	if err := s.triggerRuleRepo.Update(r.Context(), rule); err != nil {
		if err == repository.ErrNotFound {
			writeJSONError(w, http.StatusNotFound, "Not Found", middleware.GetReqID(r.Context()))
			return
		}
		slog.Error("update trigger rule", "error", err, "request_id", middleware.GetReqID(r.Context()))
		writeJSONError(w, http.StatusInternalServerError, "Internal Server Error", middleware.GetReqID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(rule)
}

// handleDeleteTriggerRule handles DELETE /api/v1/projects/{id}/trigger-rules/{ruleId}.
// It deletes the rule by ID and returns 204 No Content on success. Admin-only.
func (s *Server) handleDeleteTriggerRule(w http.ResponseWriter, r *http.Request) {
	ruleID := chi.URLParam(r, "ruleId")

	if err := s.triggerRuleRepo.Delete(r.Context(), ruleID); err != nil {
		if err == repository.ErrNotFound {
			writeJSONError(w, http.StatusNotFound, "Not Found", middleware.GetReqID(r.Context()))
			return
		}
		slog.Error("delete trigger rule", "error", err, "request_id", middleware.GetReqID(r.Context()))
		writeJSONError(w, http.StatusInternalServerError, "Internal Server Error", middleware.GetReqID(r.Context()))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
