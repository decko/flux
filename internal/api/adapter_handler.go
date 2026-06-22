package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/decko/flux/internal/domain"
)

func (s *Server) handleListAdapters(w http.ResponseWriter, r *http.Request) {
	if s.adapters == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode([]domain.AdapterInfo{})
		return
	}

	result := make([]domain.AdapterInfo, 0, len(s.adapters))
	for _, a := range s.adapters {
		result = append(result, a)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

func (s *Server) handleAdapterHealth(w http.ResponseWriter, r *http.Request) {
	if s.adapters == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "adapters not configured", middleware.GetReqID(r.Context()))
		return
	}
	adapterType := chi.URLParam(r, "type")
	info, ok := s.adapters[adapterType]
	if !ok {
		writeJSONError(w, http.StatusNotFound, "adapter not found", middleware.GetReqID(r.Context()))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(info)
}
