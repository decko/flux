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

	"github.com/decko/flux/internal/adapter/github"
	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// handleCreateProject handles POST /api/v1/projects.
// It decodes a Project from the JSON body, generates an ID and timestamps,
// delegates to the project service, and returns 201 Created with the
// Location header set to the new resource's URL.
func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var p model.Project
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON", middleware.GetReqID(r.Context()))
		return
	}

	p.ID = uuid.New().String()
	now := time.Now().UTC()
	p.CreatedAt = now
	p.UpdatedAt = now

	if err := s.projectSvc.Create(r.Context(), p); err != nil {
		code, msg := serviceError(err)
		if code == http.StatusInternalServerError {
			slog.Error("create project", "error", err, "request_id", middleware.GetReqID(r.Context()))
		}
		writeJSONError(w, code, msg, middleware.GetReqID(r.Context()))
		return
	}

	// Fire-and-forget webhook registration — don't block the response.
	if s.webhookCreator != nil {
		go s.webhookCreator.CreateForProject(r.Context(), p)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/api/v1/projects/"+p.ID)
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(p)
}

// handleGetProject handles GET /api/v1/projects/{id}.
// It retrieves a project by its ID from the path parameter and returns
// 200 OK with the project JSON, or 404 Not Found if no project exists.
func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	p, err := s.projectSvc.Get(r.Context(), id)
	if err != nil {
		code, msg := serviceError(err)
		if code == http.StatusInternalServerError {
			slog.Error("get project", "error", err, "request_id", middleware.GetReqID(r.Context()))
		}
		writeJSONError(w, code, msg, middleware.GetReqID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(p)
}

// handleListProjects handles GET /api/v1/projects.
// It returns all projects as a JSON array. Returns an empty array when
// no projects exist.
func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.projectSvc.List(r.Context(), repository.ProjectFilter{})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Internal Server Error", middleware.GetReqID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(projects)
}

// handleUpdateProject handles PUT /api/v1/projects/{id}.
// It decodes the updated Project from the JSON body, validates that the
// URL path ID matches the body ID, updates the timestamp, and delegates
// to the project service. Returns 200 OK on success, 400 on validation
// or ID mismatch, and 404 if the project does not exist.
func (s *Server) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var p model.Project
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON", middleware.GetReqID(r.Context()))
		return
	}

	if p.ID != id {
		writeJSONError(w, http.StatusBadRequest, "ID mismatch", middleware.GetReqID(r.Context()))
		return
	}

	p.UpdatedAt = time.Now().UTC()

	if err := s.projectSvc.Update(r.Context(), p); err != nil {
		code, msg := serviceError(err)
		if code == http.StatusInternalServerError {
			slog.Error("update project", "error", err, "request_id", middleware.GetReqID(r.Context()))
		}
		writeJSONError(w, code, msg, middleware.GetReqID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(p)
}

// handleDeleteProject handles DELETE /api/v1/projects/{id}.
// It deletes a project by ID and returns 204 No Content on success,
// or 404 Not Found if no project with the given ID exists. If the project
// has a registered webhook, it fire-and-forgets the GitHub webhook deletion
// in a goroutine (does not block the response).
func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Fetch the project first to check for webhook.
	project, err := s.projectSvc.Get(r.Context(), id)
	if err != nil {
		code, msg := serviceError(err)
		if code == http.StatusInternalServerError {
			slog.Error("get project for delete", "error", err, "request_id", middleware.GetReqID(r.Context()))
		}
		writeJSONError(w, code, msg, middleware.GetReqID(r.Context()))
		return
	}

	if err := s.projectSvc.Delete(r.Context(), id); err != nil {
		code, msg := serviceError(err)
		if code == http.StatusInternalServerError {
			slog.Error("delete project", "error", err, "request_id", middleware.GetReqID(r.Context()))
		}
		writeJSONError(w, code, msg, middleware.GetReqID(r.Context()))
		return
	}

	// Fire-and-forget: delete the GitHub webhook if one exists.
	if project.WebhookID != nil && *project.WebhookID > 0 && s.appAuth != nil {
		go func(ctx context.Context, p model.Project) {
			// Derive owner/repo from adapter config.
			owner, repo := "", ""
			for _, a := range p.Adapters {
				if a.Type == "github" {
					owner = a.Config["owner"]
					repo = a.Config["repo"]
					break
				}
			}
			if owner == "" || repo == "" {
				slog.Warn("cannot delete webhook: missing owner/repo",
					"project_id", p.ID, "webhook_id", p.WebhookID)
				return
			}

			if err := github.DeleteWebhook(ctx, s.appAuth, p.InstallationID, owner, repo, *p.WebhookID); err != nil {
				slog.Warn("failed to delete webhook (fire-and-forget)",
					"project_id", p.ID, "webhook_id", p.WebhookID, "error", err)
			} else {
				slog.Info("webhook deleted", "project_id", p.ID, "webhook_id", p.WebhookID)
			}

			// Clean up the webhook secret.
			if s.webhookSecretRepo != nil {
				if err := s.webhookSecretRepo.Delete(ctx, p.ID); err != nil {
					slog.Warn("failed to delete webhook secret",
						"project_id", p.ID, "error", err)
				}
			}
		}(r.Context(), project)
	}

	w.WriteHeader(http.StatusNoContent)
}
