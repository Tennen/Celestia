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

func (s *Server) handleAgentSearchRun(w http.ResponseWriter, r *http.Request) {
	var payload models.AgentSearchRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	result, err := s.gateway.RunAgentSearch(r.Context(), payload)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAgentSTT(w http.ResponseWriter, r *http.Request) {
	var payload models.AgentSpeechRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	result, err := s.gateway.TranscribeAgentSpeech(r.Context(), payload)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAgentCodexRun(w http.ResponseWriter, r *http.Request) {
	var payload models.AgentCodexRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	result, err := s.gateway.RunAgentCodex(r.Context(), payload)
	if err != nil {
		if result.TaskID != "" {
			writeJSON(w, gatewayapi.StatusCode(err), result)
			return
		}
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAgentMarkdownRender(w http.ResponseWriter, r *http.Request) {
	var payload models.AgentMarkdownRenderRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	result, err := s.gateway.RunAgentMarkdownRender(r.Context(), payload)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAgentTools(w http.ResponseWriter, r *http.Request) {
	items, err := s.gateway.ListAgentTools(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) handleAgentTool(w http.ResponseWriter, r *http.Request) {
	item, err := s.gateway.DescribeAgentTool(r.Context(), r.PathValue("name"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleAgentToolRun(w http.ResponseWriter, r *http.Request) {
	var payload models.AgentToolRunRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	result, err := s.gateway.RunAgentTool(r.Context(), r.PathValue("name"), payload)
	if err != nil {
		if result.Tool != "" {
			writeJSON(w, gatewayapi.StatusCode(err), result)
			return
		}
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
