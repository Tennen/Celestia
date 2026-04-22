package gateway

import (
	"context"
	"net/http"

	coreagent "github.com/chentianyu/celestia/internal/core/agent"
	"github.com/chentianyu/celestia/internal/models"
)

func (s *RuntimeService) GetAgentSnapshot(ctx context.Context) (models.AgentSnapshot, error) {
	snapshot, err := s.runtime.Agent.Snapshot(ctx)
	if err != nil {
		return models.AgentSnapshot{}, statusError(http.StatusInternalServerError, err)
	}
	return snapshot, nil
}

func (s *RuntimeService) SaveAgentSettings(ctx context.Context, settings models.AgentSettings) (models.AgentSnapshot, error) {
	snapshot, err := s.runtime.Agent.SaveSettings(ctx, settings)
	return s.agentSnapshot(ctx, snapshot, err)
}

func (s *RuntimeService) SaveAgentDirectInput(ctx context.Context, config models.AgentDirectInputConfig) (models.AgentSnapshot, error) {
	snapshot, err := s.runtime.Agent.SaveDirectInput(ctx, config)
	return s.agentSnapshot(ctx, snapshot, err)
}

func (s *RuntimeService) SaveAgentPush(ctx context.Context, push models.AgentPushSnapshot) (models.AgentSnapshot, error) {
	snapshot, err := s.runtime.Agent.SavePush(ctx, push)
	return s.agentSnapshot(ctx, snapshot, err)
}

func (s *RuntimeService) SaveAgentWeComMenu(ctx context.Context, config models.AgentWeComMenuConfig) (models.AgentSnapshot, error) {
	snapshot, err := s.runtime.Agent.SaveWeComMenu(ctx, config)
	return s.agentSnapshot(ctx, snapshot, err)
}

func (s *RuntimeService) PublishAgentWeComMenu(ctx context.Context) (models.AgentWeComMenuSnapshot, error) {
	menu, err := s.runtime.Agent.PublishWeComMenu(ctx)
	if err != nil {
		return models.AgentWeComMenuSnapshot{}, statusError(http.StatusBadRequest, err)
	}
	return menu, nil
}

func (s *RuntimeService) SendAgentWeComMessage(ctx context.Context, req AgentWeComSendRequest) error {
	err := s.runtime.Agent.SendWeComMessage(ctx, coreagent.WeComSendRequest(req))
	if err != nil {
		return statusError(http.StatusBadRequest, err)
	}
	return nil
}

func (s *RuntimeService) SendAgentWeComImage(ctx context.Context, req AgentWeComImageRequest) error {
	err := s.runtime.Agent.SendWeComImage(ctx, coreagent.WeComImageRequest(req))
	if err != nil {
		return statusError(http.StatusBadRequest, err)
	}
	return nil
}

func (s *RuntimeService) RecordAgentWeComCallback(ctx context.Context, raw []byte) (models.AgentWeComEventRecord, error) {
	record, err := s.runtime.Agent.RecordWeComXML(ctx, raw)
	if err != nil {
		return models.AgentWeComEventRecord{}, statusError(http.StatusBadRequest, err)
	}
	return record, nil
}

func (s *RuntimeService) HandleAgentWeComIngress(ctx context.Context, raw []byte) (models.AgentWeComInboundResult, error) {
	result, err := s.runtime.Agent.HandleWeComXML(ctx, raw)
	if err != nil {
		return models.AgentWeComInboundResult{}, statusError(http.StatusBadRequest, err)
	}
	return result, nil
}

func (s *RuntimeService) RunAgentConversation(ctx context.Context, req models.AgentConversationRequest) (models.AgentConversation, error) {
	item, err := s.runtime.Agent.Converse(ctx, req)
	if err != nil && item.ID == "" {
		return models.AgentConversation{}, statusError(http.StatusBadRequest, err)
	}
	return item, nil
}

func (s *RuntimeService) SaveAgentTopic(ctx context.Context, topic models.AgentTopicSnapshot) (models.AgentSnapshot, error) {
	snapshot, err := s.runtime.Agent.SaveTopic(ctx, topic)
	return s.agentSnapshot(ctx, snapshot, err)
}

func (s *RuntimeService) RunAgentTopicSummary(ctx context.Context, profileID string) (models.AgentTopicRun, error) {
	run, err := s.runtime.Agent.RunTopicSummary(ctx, profileID)
	if err != nil {
		return models.AgentTopicRun{}, statusError(http.StatusBadRequest, err)
	}
	return run, nil
}

func (s *RuntimeService) SaveAgentWritingTopic(ctx context.Context, req AgentWritingTopicRequest) (models.AgentWritingTopic, error) {
	topic, err := s.runtime.Agent.SaveWritingTopic(ctx, coreagent.WritingTopicRequest(req))
	if err != nil {
		return models.AgentWritingTopic{}, statusError(http.StatusBadRequest, err)
	}
	return topic, nil
}

func (s *RuntimeService) AddAgentWritingMaterial(ctx context.Context, topicID string, req AgentWritingMaterialRequest) (models.AgentWritingTopic, error) {
	topic, err := s.runtime.Agent.AddWritingMaterial(ctx, topicID, coreagent.WritingMaterialRequest(req))
	if err != nil {
		return models.AgentWritingTopic{}, statusError(http.StatusBadRequest, err)
	}
	return topic, nil
}

func (s *RuntimeService) SummarizeAgentWritingTopic(ctx context.Context, topicID string) (models.AgentWritingTopic, error) {
	topic, err := s.runtime.Agent.SummarizeWritingTopic(ctx, topicID)
	if err != nil {
		return models.AgentWritingTopic{}, statusError(http.StatusBadRequest, err)
	}
	return topic, nil
}

func (s *RuntimeService) SaveAgentMarketPortfolio(ctx context.Context, portfolio models.AgentMarketPortfolio) (models.AgentSnapshot, error) {
	snapshot, err := s.runtime.Agent.SaveMarketPortfolio(ctx, portfolio)
	return s.agentSnapshot(ctx, snapshot, err)
}

func (s *RuntimeService) RunAgentMarketAnalysis(ctx context.Context, req AgentMarketRunRequest) (models.AgentMarketRun, error) {
	run, err := s.runtime.Agent.RunMarketAnalysis(ctx, coreagent.MarketRunRequest(req))
	if err != nil {
		return models.AgentMarketRun{}, statusError(http.StatusBadRequest, err)
	}
	return run, nil
}

func (s *RuntimeService) CreateAgentEvolutionGoal(ctx context.Context, req AgentEvolutionGoalRequest) (models.AgentEvolutionGoal, error) {
	goal, err := s.runtime.Agent.CreateEvolutionGoal(ctx, coreagent.EvolutionGoalRequest(req))
	if err != nil {
		return models.AgentEvolutionGoal{}, statusError(http.StatusBadRequest, err)
	}
	return goal, nil
}

func (s *RuntimeService) RunAgentEvolutionGoal(ctx context.Context, goalID string) (models.AgentEvolutionGoal, error) {
	goal, err := s.runtime.Agent.RunEvolutionGoal(ctx, goalID)
	if err != nil {
		return goal, statusError(http.StatusBadRequest, err)
	}
	return goal, nil
}

func (s *RuntimeService) RunAgentTerminal(ctx context.Context, req models.AgentTerminalRequest) (models.AgentTerminalResult, error) {
	result, err := s.runtime.Agent.RunTerminal(ctx, req)
	if err != nil && result.Command == "" {
		return result, statusError(http.StatusBadRequest, err)
	}
	return result, nil
}

func (s *RuntimeService) RunAgentSearch(ctx context.Context, req models.AgentSearchRequest) (models.AgentSearchResult, error) {
	result, err := s.runtime.Agent.RunSearch(ctx, req)
	if err != nil {
		return models.AgentSearchResult{}, statusError(http.StatusBadRequest, err)
	}
	return result, nil
}

func (s *RuntimeService) TranscribeAgentSpeech(ctx context.Context, req models.AgentSpeechRequest) (models.AgentSpeechResult, error) {
	result, err := s.runtime.Agent.Transcribe(ctx, req)
	if err != nil {
		return models.AgentSpeechResult{}, statusError(http.StatusBadRequest, err)
	}
	return result, nil
}

func (s *RuntimeService) RunAgentCodex(ctx context.Context, req models.AgentCodexRequest) (models.AgentCodexResult, error) {
	result, err := s.runtime.Agent.RunCodex(ctx, req)
	if err != nil && result.TaskID == "" {
		return result, statusError(http.StatusBadRequest, err)
	}
	return result, nil
}

func (s *RuntimeService) RunAgentMarkdownRender(ctx context.Context, req models.AgentMarkdownRenderRequest) (models.AgentMarkdownRenderResult, error) {
	result, err := s.runtime.Agent.RunMarkdownRender(ctx, req)
	if err != nil {
		return result, statusError(http.StatusBadRequest, err)
	}
	return result, nil
}

func (s *RuntimeService) agentSnapshot(_ context.Context, snapshot models.AgentSnapshot, err error) (models.AgentSnapshot, error) {
	if err != nil {
		return models.AgentSnapshot{}, statusError(http.StatusBadRequest, err)
	}
	return snapshot, nil
}
