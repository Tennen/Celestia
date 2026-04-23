package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *HTTPService) GetAgentSnapshot(ctx context.Context) (models.AgentSnapshot, error) {
	var out models.AgentSnapshot
	if err := s.get(ctx, "/api/v1/agent", nil, &out); err != nil {
		return models.AgentSnapshot{}, err
	}
	return out, nil
}

func (s *HTTPService) SaveAgentSettings(ctx context.Context, settings models.AgentSettings) (models.AgentSnapshot, error) {
	return s.putAgentSnapshot(ctx, "/api/v1/agent/settings", settings)
}

func (s *HTTPService) SaveAgentDirectInput(ctx context.Context, config models.AgentDirectInputConfig) (models.AgentSnapshot, error) {
	return s.putAgentSnapshot(ctx, "/api/v1/touchpoints/input-mappings", config)
}

func (s *HTTPService) SaveAgentPush(ctx context.Context, push models.AgentPushSnapshot) (models.AgentSnapshot, error) {
	return s.putAgentSnapshot(ctx, "/api/v1/touchpoints/wecom/users", push)
}

func (s *HTTPService) SaveAgentWeComMenu(ctx context.Context, config models.AgentWeComMenuConfig) (models.AgentSnapshot, error) {
	return s.putAgentSnapshot(ctx, "/api/v1/touchpoints/wecom/menu", config)
}

func (s *HTTPService) PublishAgentWeComMenu(ctx context.Context) (models.AgentWeComMenuSnapshot, error) {
	var out models.AgentWeComMenuSnapshot
	if err := s.request(ctx, http.MethodPost, "/api/v1/touchpoints/wecom/menu/publish", nil, nil, &out, ""); err != nil {
		return models.AgentWeComMenuSnapshot{}, err
	}
	return out, nil
}

func (s *HTTPService) SendAgentWeComMessage(ctx context.Context, req AgentWeComSendRequest) error {
	return s.request(ctx, http.MethodPost, "/api/v1/touchpoints/wecom/send", nil, req, nil, "")
}

func (s *HTTPService) SendAgentWeComImage(ctx context.Context, req AgentWeComImageRequest) error {
	return s.request(ctx, http.MethodPost, "/api/v1/touchpoints/wecom/image", nil, req, nil, "")
}

func (s *HTTPService) RecordAgentWeComCallback(ctx context.Context, raw []byte) (models.AgentWeComEventRecord, error) {
	var out models.AgentWeComEventRecord
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/api/v1/touchpoints/wecom/callback", bytes.NewReader(raw))
	if err != nil {
		return models.AgentWeComEventRecord{}, statusError(http.StatusInternalServerError, err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return models.AgentWeComEventRecord{}, statusError(http.StatusBadGateway, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return models.AgentWeComEventRecord{}, s.decodeError(resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return models.AgentWeComEventRecord{}, err
	}
	return out, nil
}

func (s *HTTPService) HandleAgentWeComIngress(ctx context.Context, raw []byte) (models.AgentWeComInboundResult, error) {
	var out models.AgentWeComInboundResult
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/api/v1/touchpoints/wecom/ingress", bytes.NewReader(raw))
	if err != nil {
		return models.AgentWeComInboundResult{}, statusError(http.StatusInternalServerError, err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return models.AgentWeComInboundResult{}, statusError(http.StatusBadGateway, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return models.AgentWeComInboundResult{}, s.decodeError(resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return models.AgentWeComInboundResult{}, err
	}
	return out, nil
}

func (s *HTTPService) RunAgentConversation(ctx context.Context, req models.AgentConversationRequest) (models.AgentConversation, error) {
	var out models.AgentConversation
	if err := s.request(ctx, http.MethodPost, "/api/v1/agent/conversation", nil, req, &out, req.Actor); err != nil {
		return models.AgentConversation{}, err
	}
	return out, nil
}

func (s *HTTPService) ListAgentCapabilities(ctx context.Context) ([]models.AgentCapabilityInfo, error) {
	var out []models.AgentCapabilityInfo
	if err := s.get(ctx, "/api/v1/agent/capabilities", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *HTTPService) DescribeAgentCapability(ctx context.Context, name string) (models.AgentCapabilityInfo, error) {
	var out models.AgentCapabilityInfo
	path := fmt.Sprintf("/api/v1/agent/capabilities/%s", url.PathEscape(name))
	if err := s.get(ctx, path, nil, &out); err != nil {
		return models.AgentCapabilityInfo{}, err
	}
	return out, nil
}

func (s *HTTPService) RunAgentCapability(
	ctx context.Context,
	name string,
	req models.AgentCapabilityRunRequest,
) (models.AgentCapabilityRunResult, error) {
	var out models.AgentCapabilityRunResult
	path := fmt.Sprintf("/api/v1/agent/capabilities/%s/run", url.PathEscape(name))
	if err := s.request(ctx, http.MethodPost, path, nil, req, &out, ""); err != nil {
		return out, err
	}
	return out, nil
}

func (s *HTTPService) SaveAgentTopic(ctx context.Context, topic models.AgentTopicSnapshot) (models.AgentSnapshot, error) {
	return s.putAgentSnapshot(ctx, "/api/v1/agent/topic", topic)
}

func (s *HTTPService) RunAgentTopicSummary(ctx context.Context, profileID string) (models.AgentTopicRun, error) {
	var out models.AgentTopicRun
	payload := map[string]any{"profile_id": profileID}
	if err := s.request(ctx, http.MethodPost, "/api/v1/agent/topic/run", nil, payload, &out, ""); err != nil {
		return models.AgentTopicRun{}, err
	}
	return out, nil
}

func (s *HTTPService) SaveAgentWritingTopic(ctx context.Context, req AgentWritingTopicRequest) (models.AgentWritingTopic, error) {
	var out models.AgentWritingTopic
	if err := s.request(ctx, http.MethodPost, "/api/v1/agent/writing/topics", nil, req, &out, ""); err != nil {
		return models.AgentWritingTopic{}, err
	}
	return out, nil
}

func (s *HTTPService) AddAgentWritingMaterial(ctx context.Context, topicID string, req AgentWritingMaterialRequest) (models.AgentWritingTopic, error) {
	var out models.AgentWritingTopic
	path := fmt.Sprintf("/api/v1/agent/writing/topics/%s/materials", url.PathEscape(topicID))
	if err := s.request(ctx, http.MethodPost, path, nil, req, &out, ""); err != nil {
		return models.AgentWritingTopic{}, err
	}
	return out, nil
}

func (s *HTTPService) SummarizeAgentWritingTopic(ctx context.Context, topicID string) (models.AgentWritingTopic, error) {
	var out models.AgentWritingTopic
	path := fmt.Sprintf("/api/v1/agent/writing/topics/%s/summarize", url.PathEscape(topicID))
	if err := s.request(ctx, http.MethodPost, path, nil, nil, &out, ""); err != nil {
		return models.AgentWritingTopic{}, err
	}
	return out, nil
}

func (s *HTTPService) SaveAgentMarketPortfolio(ctx context.Context, portfolio models.AgentMarketPortfolio) (models.AgentSnapshot, error) {
	return s.putAgentSnapshot(ctx, "/api/v1/agent/market/portfolio", portfolio)
}

func (s *HTTPService) ImportAgentMarketPortfolioCodes(ctx context.Context, req models.AgentMarketImportCodesRequest) (models.AgentMarketImportCodesResponse, error) {
	var out models.AgentMarketImportCodesResponse
	if err := s.request(ctx, http.MethodPost, "/api/v1/agent/market/portfolio/import-codes", nil, req, &out, ""); err != nil {
		return models.AgentMarketImportCodesResponse{}, err
	}
	return out, nil
}

func (s *HTTPService) RunAgentMarketAnalysis(ctx context.Context, req AgentMarketRunRequest) (models.AgentMarketRun, error) {
	var out models.AgentMarketRun
	if err := s.request(ctx, http.MethodPost, "/api/v1/agent/market/run", nil, req, &out, ""); err != nil {
		return models.AgentMarketRun{}, err
	}
	return out, nil
}

func (s *HTTPService) CreateAgentEvolutionGoal(ctx context.Context, req AgentEvolutionGoalRequest) (models.AgentEvolutionGoal, error) {
	var out models.AgentEvolutionGoal
	if err := s.request(ctx, http.MethodPost, "/api/v1/agent/evolution/goals", nil, req, &out, ""); err != nil {
		return models.AgentEvolutionGoal{}, err
	}
	return out, nil
}

func (s *HTTPService) RunAgentEvolutionGoal(ctx context.Context, goalID string) (models.AgentEvolutionGoal, error) {
	var out models.AgentEvolutionGoal
	path := fmt.Sprintf("/api/v1/agent/evolution/goals/%s/run", url.PathEscape(goalID))
	if err := s.request(ctx, http.MethodPost, path, nil, nil, &out, ""); err != nil {
		return out, err
	}
	return out, nil
}

func (s *HTTPService) RunAgentTerminal(ctx context.Context, req models.AgentTerminalRequest) (models.AgentTerminalResult, error) {
	var out models.AgentTerminalResult
	if err := s.request(ctx, http.MethodPost, "/api/v1/agent/terminal", nil, req, &out, ""); err != nil {
		return out, err
	}
	return out, nil
}

func (s *HTTPService) RunAgentSearch(ctx context.Context, req models.AgentSearchRequest) (models.AgentSearchResult, error) {
	var out models.AgentSearchResult
	if err := s.request(ctx, http.MethodPost, "/api/v1/agent/search/run", nil, req, &out, ""); err != nil {
		return models.AgentSearchResult{}, err
	}
	return out, nil
}

func (s *HTTPService) TranscribeAgentSpeech(ctx context.Context, req models.AgentSpeechRequest) (models.AgentSpeechResult, error) {
	var out models.AgentSpeechResult
	if err := s.request(ctx, http.MethodPost, "/api/v1/touchpoints/voice/transcribe", nil, req, &out, ""); err != nil {
		return models.AgentSpeechResult{}, err
	}
	return out, nil
}

func (s *HTTPService) RunAgentCodex(ctx context.Context, req models.AgentCodexRequest) (models.AgentCodexResult, error) {
	var out models.AgentCodexResult
	if err := s.request(ctx, http.MethodPost, "/api/v1/agent/codex/run", nil, req, &out, ""); err != nil {
		return out, err
	}
	return out, nil
}

func (s *HTTPService) RunAgentMarkdownRender(ctx context.Context, req models.AgentMarkdownRenderRequest) (models.AgentMarkdownRenderResult, error) {
	var out models.AgentMarkdownRenderResult
	if err := s.request(ctx, http.MethodPost, "/api/v1/agent/md2img/render", nil, req, &out, ""); err != nil {
		return models.AgentMarkdownRenderResult{}, err
	}
	return out, nil
}

func (s *HTTPService) putAgentSnapshot(ctx context.Context, path string, body any) (models.AgentSnapshot, error) {
	var out models.AgentSnapshot
	if err := s.request(ctx, http.MethodPut, path, nil, body, &out, ""); err != nil {
		return models.AgentSnapshot{}, err
	}
	return out, nil
}
