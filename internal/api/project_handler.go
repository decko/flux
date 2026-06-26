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
// or 404 Not Found if no project with the given ID exists.
func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := s.projectSvc.Delete(r.Context(), id); err != nil {
		code, msg := serviceError(err)
		if code == http.StatusInternalServerError {
			slog.Error("delete project", "error", err, "request_id", middleware.GetReqID(r.Context()))
		}
		writeJSONError(w, code, msg, middleware.GetReqID(r.Context()))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
