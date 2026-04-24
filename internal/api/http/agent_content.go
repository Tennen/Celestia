package httpapi

import (
	"net/http"
	"strings"

	gatewayapi "github.com/chentianyu/celestia/internal/api/gateway"
	"github.com/chentianyu/celestia/internal/models"
)

func (s *Server) handleAgentTopicSave(w http.ResponseWriter, r *http.Request) {
	var payload models.AgentTopicSnapshot
	if !decodeJSON(w, r, &payload) {
		return
	}
	snapshot, err := s.gateway.SaveAgentTopic(r.Context(), payload)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handleAgentTopicRun(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		WorkflowID string `json:"workflow_id"`
		ProfileID  string `json:"profile_id"`
	}
	if !decodeJSON(w, r, &payload) {
		return
	}
	workflowID := strings.TrimSpace(payload.WorkflowID)
	if workflowID == "" {
		workflowID = strings.TrimSpace(payload.ProfileID)
	}
	run, err := s.gateway.RunAgentTopicSummary(r.Context(), workflowID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *Server) handleAgentWritingTopic(w http.ResponseWriter, r *http.Request) {
	var payload gatewayapi.AgentWritingTopicRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	topic, err := s.gateway.SaveAgentWritingTopic(r.Context(), payload)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, topic)
}

func (s *Server) handleAgentWritingMaterial(w http.ResponseWriter, r *http.Request) {
	var payload gatewayapi.AgentWritingMaterialRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	topic, err := s.gateway.AddAgentWritingMaterial(r.Context(), r.PathValue("id"), payload)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, topic)
}

func (s *Server) handleAgentWritingSummarize(w http.ResponseWriter, r *http.Request) {
	topic, err := s.gateway.SummarizeAgentWritingTopic(r.Context(), r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, topic)
}

func (s *Server) handleAgentMarketPortfolio(w http.ResponseWriter, r *http.Request) {
	var payload models.AgentMarketPortfolio
	if !decodeJSON(w, r, &payload) {
		return
	}
	snapshot, err := s.gateway.SaveAgentMarketPortfolio(r.Context(), payload)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handleAgentMarketImportCodes(w http.ResponseWriter, r *http.Request) {
	var payload models.AgentMarketImportCodesRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	result, err := s.gateway.ImportAgentMarketPortfolioCodes(r.Context(), payload)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAgentMarketRun(w http.ResponseWriter, r *http.Request) {
	var payload gatewayapi.AgentMarketRunRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	run, err := s.gateway.RunAgentMarketAnalysis(r.Context(), payload)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}
