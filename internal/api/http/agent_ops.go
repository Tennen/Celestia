package httpapi

import (
	"net/http"

	gatewayapi "github.com/chentianyu/celestia/internal/api/gateway"
	"github.com/chentianyu/celestia/internal/models"
)

func (s *Server) handleAgentEvolutionGoal(w http.ResponseWriter, r *http.Request) {
	var payload gatewayapi.AgentEvolutionGoalRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	goal, err := s.gateway.CreateAgentEvolutionGoal(r.Context(), payload)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, goal)
}

func (s *Server) handleAgentEvolutionRun(w http.ResponseWriter, r *http.Request) {
	goal, err := s.gateway.RunAgentEvolutionGoal(r.Context(), r.PathValue("id"))
	if err != nil {
		if goal.ID != "" {
			writeJSON(w, gatewayapi.StatusCode(err), goal)
			return
		}
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, goal)
}

func (s *Server) handleAgentTerminal(w http.ResponseWriter, r *http.Request) {
	var payload models.AgentTerminalRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	result, err := s.gateway.RunAgentTerminal(r.Context(), payload)
	if err != nil {
		if result.Command != "" {
			writeJSON(w, gatewayapi.StatusCode(err), result)
			return
		}
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
