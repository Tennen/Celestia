package httpapi

import (
	"encoding/json"
	"io"
	"net/http"

	gatewayapi "github.com/chentianyu/celestia/internal/api/gateway"
	"github.com/chentianyu/celestia/internal/models"
)

func (s *Server) handleAgentSnapshot(w http.ResponseWriter, r *http.Request) {
	snapshot, err := s.gateway.GetAgentSnapshot(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handleAgentSettings(w http.ResponseWriter, r *http.Request) {
	var payload models.AgentSettings
	if !decodeJSON(w, r, &payload) {
		return
	}
	snapshot, err := s.gateway.SaveAgentSettings(r.Context(), payload)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handleAgentDirectInput(w http.ResponseWriter, r *http.Request) {
	var payload models.AgentDirectInputConfig
	if !decodeJSON(w, r, &payload) {
		return
	}
	snapshot, err := s.gateway.SaveAgentDirectInput(r.Context(), payload)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handleAgentPush(w http.ResponseWriter, r *http.Request) {
	var payload models.AgentPushSnapshot
	if !decodeJSON(w, r, &payload) {
		return
	}
	snapshot, err := s.gateway.SaveAgentPush(r.Context(), payload)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handleAgentWeComMenu(w http.ResponseWriter, r *http.Request) {
	var payload models.AgentWeComMenuConfig
	if !decodeJSON(w, r, &payload) {
		return
	}
	snapshot, err := s.gateway.SaveAgentWeComMenu(r.Context(), payload)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handleAgentWeComMenuPublish(w http.ResponseWriter, r *http.Request) {
	menu, err := s.gateway.PublishAgentWeComMenu(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, menu)
}

func (s *Server) handleAgentWeComSend(w http.ResponseWriter, r *http.Request) {
	var payload gatewayapi.AgentWeComSendRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	if err := s.gateway.SendAgentWeComMessage(r.Context(), payload); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAgentWeComCallback(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	record, err := s.gateway.RecordAgentWeComCallback(r.Context(), raw)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, record)
}

func (s *Server) handleAgentConversation(w http.ResponseWriter, r *http.Request) {
	var payload models.AgentConversationRequest
	if !decodeJSON(w, r, &payload) {
		return
	}
	payload.Actor = actorFromRequestWithDefault(r, "agent")
	item, err := s.gateway.RunAgentConversation(r.Context(), payload)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return false
	}
	return true
}
