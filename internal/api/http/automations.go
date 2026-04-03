package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Server) handleAutomations(w http.ResponseWriter, r *http.Request) {
	items, err := s.gateway.ListAutomations(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleCreateAutomation(w http.ResponseWriter, r *http.Request) {
	var automation models.Automation
	if err := json.NewDecoder(r.Body).Decode(&automation); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	item, err := s.gateway.SaveAutomation(r.Context(), automation)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleUpdateAutomation(w http.ResponseWriter, r *http.Request) {
	var automation models.Automation
	if err := json.NewDecoder(r.Body).Decode(&automation); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	automation.ID = r.PathValue("id")
	item, err := s.gateway.SaveAutomation(r.Context(), automation)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleDeleteAutomation(w http.ResponseWriter, r *http.Request) {
	if err := s.gateway.DeleteAutomation(r.Context(), r.PathValue("id")); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
