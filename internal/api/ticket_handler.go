package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/decko/flux/internal/model"
	"github.com/decko/flux/internal/repository"
)

// ticketPage is the paginated JSON response envelope for the ticket list endpoint.
type ticketPage struct {
	Items []model.Ticket `json:"items"`
	Page  int            `json:"page"`
	Limit int            `json:"limit"`
	Total int            `json:"total"`
}

// handleListTickets handles GET /api/v1/tickets.
// Supports query params: project_id, status, labels (comma-separated), page, limit.
// Returns a paginated envelope with items, page, limit, and total count.
func (s *Server) handleListTickets(w http.ResponseWriter, r *http.Request) {
	var filter repository.TicketFilter

	if pid := r.URL.Query().Get("project_id"); pid != "" {
		filter.ProjectID = pid
	}
	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = model.TicketStatus(status)
	}
	if labels := r.URL.Query().Get("labels"); labels != "" {
		filter.Labels = strings.Split(labels, ",")
	}

	page, limit := 1, 20

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		p, err := strconv.Atoi(pageStr)
		if err != nil || p < 1 {
			writeJSONError(w, http.StatusBadRequest, "invalid page", middleware.GetReqID(r.Context()))
			return
		}
		page = p
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		l, err := strconv.Atoi(limitStr)
		if err != nil || l < 1 || l > 100 {
			writeJSONError(w, http.StatusBadRequest, "invalid limit", middleware.GetReqID(r.Context()))
			return
		}
		limit = l
	}

	tickets, err := s.ticketSvc.List(r.Context(), filter)
	if err != nil {
		code, msg := serviceError(err)
		if code == http.StatusInternalServerError {
			slog.Error("list tickets", "error", err, "request_id", middleware.GetReqID(r.Context()))
		}
		writeJSONError(w, code, msg, middleware.GetReqID(r.Context()))
		return
	}

	if tickets == nil {
		tickets = []model.Ticket{}
	}

	total := len(tickets)

	start := (page - 1) * limit
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	items := tickets[start:end]
	if items == nil {
		items = []model.Ticket{}
	}

	resp := ticketPage{
		Items: items,
		Page:  page,
		Limit: limit,
		Total: total,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleGetTicket handles GET /api/v1/tickets/{id}.
// It retrieves a ticket by its ID from the path parameter and returns
// 200 OK with the ticket JSON, or 404 Not Found if no ticket exists.
func (s *Server) handleGetTicket(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	ticket, err := s.ticketSvc.Get(r.Context(), id)
	if err != nil {
		code, msg := serviceError(err)
		if code == http.StatusInternalServerError {
			slog.Error("get ticket", "error", err, "request_id", middleware.GetReqID(r.Context()))
		}
		writeJSONError(w, code, msg, middleware.GetReqID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ticket)
}

// handleUpdateTicket handles PUT /api/v1/tickets/{id}.
// It decodes the updated Ticket from the JSON body, validates that the
// URL path ID matches the body ID, updates the timestamp, and delegates
// to the ticket service. Returns 200 OK on success, 400 on validation
// or ID mismatch, and 404 if the ticket does not exist.
func (s *Server) handleUpdateTicket(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var t model.Ticket
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON", middleware.GetReqID(r.Context()))
		return
	}

	if t.ID != id {
		writeJSONError(w, http.StatusBadRequest, "ID mismatch", middleware.GetReqID(r.Context()))
		return
	}

	t.UpdatedAt = time.Now().UTC()

	if err := s.ticketSvc.Update(r.Context(), t); err != nil {
		code, msg := serviceError(err)
		if code == http.StatusInternalServerError {
			slog.Error("update ticket", "error", err, "request_id", middleware.GetReqID(r.Context()))
		}
		writeJSONError(w, code, msg, middleware.GetReqID(r.Context()))
		return
	}

	// Re-fetch to return the persisted state with correct timestamps.
	persisted, err := s.ticketSvc.Get(r.Context(), id)
	if err != nil {
		slog.Error("re-fetch after update ticket", "error", err, "request_id", middleware.GetReqID(r.Context()))
		writeJSONError(w, http.StatusInternalServerError, "Internal Server Error", middleware.GetReqID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(persisted)
}
