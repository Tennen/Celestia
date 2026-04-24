package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

const (
	agentLegacyStateDocumentKey = "agent/state"

	agentSettingsLLMDocumentKey       = "agent/settings/llm"
	agentSettingsWeComDocumentKey     = "agent/settings/wecom"
	agentSettingsTerminalDocumentKey  = "agent/settings/terminal"
	agentSettingsEvolutionDocumentKey = "agent/settings/evolution"
	agentSettingsSTTDocumentKey       = "agent/settings/stt"
	agentSettingsSearchDocumentKey    = "agent/settings/search"
	agentSettingsMemoryDocumentKey    = "agent/settings/memory"
	agentSettingsMD2ImgDocumentKey    = "agent/settings/md2img"

	agentSearchLogDocumentKey           = "agent/search/log"
	agentDirectInputDocumentKey         = "agent/direct-input"
	agentWeComMenuDocumentKey           = "agent/wecom/menu"
	agentWeComEventsDocumentKey         = "agent/wecom/events"
	agentWeComUsersDocumentKey          = "agent/wecom/users"
	agentConversationsDocumentKey       = "agent/conversations"
	agentMemoryRawDocumentKey           = "agent/memory/raw"
	agentMemorySummariesDocumentKey     = "agent/memory/summaries"
	agentMemoryWindowsDocumentKey       = "agent/memory/windows"
	agentWorkflowDefinitionsDocumentKey = "agent/workflow/definitions"
	agentWorkflowRunsDocumentKey        = "agent/workflow/runs"
	legacyAgentTopicProfilesDocumentKey = "agent/topic/profiles"
	legacyAgentTopicRunsDocumentKey     = "agent/topic/runs"
	agentWritingTopicsDocumentKey       = "agent/writing/topics"
	agentMarketPortfolioDocumentKey     = "agent/market/portfolio"
	agentMarketConfigDocumentKey        = "agent/market/config"
	agentMarketRunsDocumentKey          = "agent/market/runs"
	agentEvolutionGoalsDocumentKey      = "agent/evolution/goals"
)

type agentDocumentLoader struct {
	key  string
	load func(models.AgentDocument, *models.AgentSnapshot) error
}

type agentSettingsLLMDocument struct {
	RuntimeMode          string                    `json:"runtime_mode"`
	DefaultLLMProviderID string                    `json:"default_llm_provider_id"`
	LLMProviders         []models.AgentLLMProvider `json:"llm_providers"`
	UpdatedAt            time.Time                 `json:"updated_at"`
}

type agentSettingsSearchDocument struct {
	SearchEngines []models.AgentSearchProvider `json:"search_engines"`
	UpdatedAt     time.Time                    `json:"updated_at"`
}

type agentSearchLogDocument struct {
	RecentQueries []models.AgentSearchQueryLog `json:"recent_queries"`
	UpdatedAt     time.Time                    `json:"updated_at"`
}

type agentWeComMenuDocument struct {
	Config           models.AgentWeComMenuConfig `json:"config"`
	PublishPayload   map[string]any              `json:"publish_payload,omitempty"`
	ValidationErrors []string                    `json:"validation_errors,omitempty"`
	UpdatedAt        time.Time                   `json:"updated_at"`
}

type agentWeComEventsDocument struct {
	RecentEvents []models.AgentWeComEventRecord `json:"recent_events"`
	UpdatedAt    time.Time                      `json:"updated_at"`
}

type agentWeComUsersDocument struct {
	Users     []models.AgentPushUser `json:"users"`
	UpdatedAt time.Time              `json:"updated_at"`
}

type agentConversationsDocument struct {
	Conversations []models.AgentConversation `json:"conversations"`
	UpdatedAt     time.Time                  `json:"updated_at"`
}

type agentMemoryRawDocument struct {
	RawRecords []models.AgentRawMemoryRecord `json:"raw_records"`
	UpdatedAt  time.Time                     `json:"updated_at"`
}

type agentMemorySummariesDocument struct {
	Summaries []models.AgentSummaryMemoryRecord `json:"summaries"`
	UpdatedAt time.Time                         `json:"updated_at"`
}

type agentMemoryWindowsDocument struct {
	Windows   []models.AgentConversationWindow `json:"windows"`
	UpdatedAt time.Time                        `json:"updated_at"`
}

type agentWorkflowDefinitionsDocument struct {
	ActiveWorkflowID string                  `json:"active_workflow_id,omitempty"`
	ActiveProfileID  string                  `json:"active_profile_id,omitempty"`
	Workflows        []models.AgentWorkflow  `json:"workflows,omitempty"`
	Profiles         []legacyWorkflowProfile `json:"profiles,omitempty"`
	UpdatedAt        time.Time               `json:"updated_at"`
}

type agentWorkflowRunsDocument struct {
	Runs      []models.AgentWorkflowRun         `json:"runs"`
	SentLog   []models.AgentWorkflowSentLogItem `json:"sent_log"`
	UpdatedAt time.Time                         `json:"updated_at"`
}

type agentWritingTopicsDocument struct {
	Topics    []models.AgentWritingTopic `json:"topics"`
	UpdatedAt time.Time                  `json:"updated_at"`
}

type agentMarketPortfolioDocument struct {
	Portfolio models.AgentMarketPortfolio `json:"portfolio"`
	UpdatedAt time.Time                   `json:"updated_at"`
}

type agentMarketConfigDocument struct {
	Config    models.AgentMarketConfig `json:"config"`
	UpdatedAt time.Time                `json:"updated_at"`
}

type agentMarketRunsDocument struct {
	Runs      []models.AgentMarketRun `json:"runs"`
	UpdatedAt time.Time               `json:"updated_at"`
}

type agentEvolutionGoalsDocument struct {
	Goals     []models.AgentEvolutionGoal `json:"goals"`
	UpdatedAt time.Time                   `json:"updated_at"`
}

func (s *Service) loadSplitSnapshot(ctx context.Context) (models.AgentSnapshot, error) {
	snapshot := defaultSnapshot()
	clearSnapshotPersistenceTimes(&snapshot)
	foundSplitDocument := false
	var latestDocumentAt time.Time
	var latestSettingsAt time.Time

	for _, loader := range agentDocumentLoaders() {
		doc, ok, err := s.store.GetAgentDocument(ctx, loader.key)
		if err != nil {
			return models.AgentSnapshot{}, err
		}
		if !ok {
			continue
		}
		foundSplitDocument = true
		latestDocumentAt = maxTime(latestDocumentAt, doc.UpdatedAt)
		if strings.HasPrefix(loader.key, "agent/settings/") {
			latestSettingsAt = maxTime(latestSettingsAt, doc.UpdatedAt)
		}
		if err := loader.load(doc, &snapshot); err != nil {
			return models.AgentSnapshot{}, err
		}
	}

	if !foundSplitDocument {
		legacy, ok, err := s.loadLegacySnapshot(ctx)
		if err != nil {
			return models.AgentSnapshot{}, err
		}
		if ok {
			if err := s.saveSplitSnapshot(ctx, legacy); err != nil {
				return models.AgentSnapshot{}, err
			}
			if err := s.store.DeleteAgentDocument(ctx, agentLegacyStateDocumentKey); err != nil {
				return models.AgentSnapshot{}, err
			}
			return legacy, nil
		}
		snapshot = normalizeSnapshot(defaultSnapshot())
		if err := s.saveSplitSnapshot(ctx, snapshot); err != nil {
			return models.AgentSnapshot{}, err
		}
		return snapshot, nil
	}

	snapshot = normalizeSnapshot(snapshot)
	if !latestDocumentAt.IsZero() {
		snapshot.UpdatedAt = latestDocumentAt
	}
	if !latestSettingsAt.IsZero() {
		snapshot.Settings.UpdatedAt = latestSettingsAt
	}
	return snapshot, nil
}

func (s *Service) saveSplitSnapshot(ctx context.Context, snapshot models.AgentSnapshot) error {
	snapshot = normalizeSnapshot(snapshot)
	settings := snapshot.Settings
	settingsUpdatedAt := firstTime(settings.UpdatedAt, snapshot.UpdatedAt)
	searchUpdatedAt := firstTime(snapshot.Search.UpdatedAt, snapshot.UpdatedAt)
	wecomMenuUpdatedAt := firstTime(snapshot.WeComMenu.Config.UpdatedAt, snapshot.UpdatedAt)
	memoryUpdatedAt := firstTime(snapshot.Memory.UpdatedAt, snapshot.UpdatedAt)
	workflowUpdatedAt := firstTime(snapshot.Workflow.UpdatedAt, snapshot.UpdatedAt)
	writingUpdatedAt := firstTime(snapshot.Writing.UpdatedAt, snapshot.UpdatedAt)
	marketUpdatedAt := firstTime(snapshot.Market.UpdatedAt, snapshot.UpdatedAt)
	evolutionUpdatedAt := firstTime(snapshot.Evolution.UpdatedAt, snapshot.UpdatedAt)

	writes := []struct {
		key       string
		domain    string
		payload   any
		updatedAt time.Time
	}{
		{
			key:    agentSettingsLLMDocumentKey,
			domain: "agent.settings.llm",
			payload: agentSettingsLLMDocument{
				RuntimeMode:          settings.RuntimeMode,
				DefaultLLMProviderID: settings.DefaultLLMProviderID,
				LLMProviders:         settings.LLMProviders,
				UpdatedAt:            settingsUpdatedAt,
			},
			updatedAt: settingsUpdatedAt,
		},
		{key: agentSettingsWeComDocumentKey, domain: "agent.settings.wecom", payload: withUpdatedAt(settings.WeCom, settingsUpdatedAt), updatedAt: settingsUpdatedAt},
		{key: agentSettingsTerminalDocumentKey, domain: "agent.settings.terminal", payload: withUpdatedAt(settings.Terminal, settingsUpdatedAt), updatedAt: settingsUpdatedAt},
		{key: agentSettingsEvolutionDocumentKey, domain: "agent.settings.evolution", payload: withUpdatedAt(settings.Evolution, settingsUpdatedAt), updatedAt: settingsUpdatedAt},
		{key: agentSettingsSTTDocumentKey, domain: "agent.settings.stt", payload: withUpdatedAt(settings.STT, settingsUpdatedAt), updatedAt: settingsUpdatedAt},
		{
			key:       agentSettingsSearchDocumentKey,
			domain:    "agent.settings.search",
			payload:   agentSettingsSearchDocument{SearchEngines: settings.SearchEngines, UpdatedAt: settingsUpdatedAt},
			updatedAt: settingsUpdatedAt,
		},
		{key: agentSettingsMemoryDocumentKey, domain: "agent.settings.memory", payload: withUpdatedAt(settings.Memory, settingsUpdatedAt), updatedAt: settingsUpdatedAt},
		{key: agentSettingsMD2ImgDocumentKey, domain: "agent.settings.md2img", payload: withUpdatedAt(settings.MD2Img, settingsUpdatedAt), updatedAt: settingsUpdatedAt},
		{key: agentSearchLogDocumentKey, domain: "agent.search.log", payload: agentSearchLogDocument{RecentQueries: snapshot.Search.RecentQueries, UpdatedAt: searchUpdatedAt}, updatedAt: searchUpdatedAt},
		{key: agentDirectInputDocumentKey, domain: "agent.input.direct", payload: snapshot.DirectInput, updatedAt: firstTime(snapshot.DirectInput.UpdatedAt, snapshot.UpdatedAt)},
		{
			key:    agentWeComMenuDocumentKey,
			domain: "agent.wecom.menu",
			payload: agentWeComMenuDocument{
				Config:           snapshot.WeComMenu.Config,
				PublishPayload:   snapshot.WeComMenu.PublishPayload,
				ValidationErrors: snapshot.WeComMenu.ValidationErrors,
				UpdatedAt:        wecomMenuUpdatedAt,
			},
			updatedAt: wecomMenuUpdatedAt,
		},
		{
			key:       agentWeComEventsDocumentKey,
			domain:    "agent.wecom.events",
			payload:   agentWeComEventsDocument{RecentEvents: snapshot.WeComMenu.RecentEvents, UpdatedAt: wecomMenuUpdatedAt},
			updatedAt: wecomMenuUpdatedAt,
		},
		{
			key:       agentWeComUsersDocumentKey,
			domain:    "agent.wecom.users",
			payload:   agentWeComUsersDocument{Users: snapshot.Push.Users, UpdatedAt: firstTime(snapshot.Push.UpdatedAt, snapshot.UpdatedAt)},
			updatedAt: firstTime(snapshot.Push.UpdatedAt, snapshot.UpdatedAt),
		},
		{
			key:       agentConversationsDocumentKey,
			domain:    "agent.conversations",
			payload:   agentConversationsDocument{Conversations: snapshot.Conversations, UpdatedAt: snapshot.UpdatedAt},
			updatedAt: snapshot.UpdatedAt,
		},
		{key: agentMemoryRawDocumentKey, domain: "agent.memory.raw", payload: agentMemoryRawDocument{RawRecords: snapshot.Memory.RawRecords, UpdatedAt: memoryUpdatedAt}, updatedAt: memoryUpdatedAt},
		{key: agentMemorySummariesDocumentKey, domain: "agent.memory.summaries", payload: agentMemorySummariesDocument{Summaries: snapshot.Memory.Summaries, UpdatedAt: memoryUpdatedAt}, updatedAt: memoryUpdatedAt},
		{key: agentMemoryWindowsDocumentKey, domain: "agent.memory.windows", payload: agentMemoryWindowsDocument{Windows: snapshot.Memory.Windows, UpdatedAt: memoryUpdatedAt}, updatedAt: memoryUpdatedAt},
		{key: agentWorkflowDefinitionsDocumentKey, domain: "agent.workflow.definitions", payload: agentWorkflowDefinitionsDocument{ActiveWorkflowID: snapshot.Workflow.ActiveWorkflowID, Workflows: snapshot.Workflow.Workflows, UpdatedAt: workflowUpdatedAt}, updatedAt: workflowUpdatedAt},
		{key: agentWorkflowRunsDocumentKey, domain: "agent.workflow.runs", payload: agentWorkflowRunsDocument{Runs: snapshot.Workflow.Runs, SentLog: snapshot.Workflow.SentLog, UpdatedAt: workflowUpdatedAt}, updatedAt: workflowUpdatedAt},
		{key: agentWritingTopicsDocumentKey, domain: "agent.writing.topics", payload: agentWritingTopicsDocument{Topics: snapshot.Writing.Topics, UpdatedAt: writingUpdatedAt}, updatedAt: writingUpdatedAt},
		{key: agentMarketPortfolioDocumentKey, domain: "agent.market.portfolio", payload: agentMarketPortfolioDocument{Portfolio: snapshot.Market.Portfolio, UpdatedAt: marketUpdatedAt}, updatedAt: marketUpdatedAt},
		{key: agentMarketConfigDocumentKey, domain: "agent.market.config", payload: agentMarketConfigDocument{Config: snapshot.Market.Config, UpdatedAt: marketUpdatedAt}, updatedAt: marketUpdatedAt},
		{key: agentMarketRunsDocumentKey, domain: "agent.market.runs", payload: agentMarketRunsDocument{Runs: snapshot.Market.Runs, UpdatedAt: marketUpdatedAt}, updatedAt: marketUpdatedAt},
		{key: agentEvolutionGoalsDocumentKey, domain: "agent.evolution.goals", payload: agentEvolutionGoalsDocument{Goals: snapshot.Evolution.Goals, UpdatedAt: evolutionUpdatedAt}, updatedAt: evolutionUpdatedAt},
	}

	for _, write := range writes {
		if err := s.upsertAgentJSON(ctx, write.key, write.domain, write.payload, write.updatedAt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) loadLegacySnapshot(ctx context.Context) (models.AgentSnapshot, bool, error) {
	doc, ok, err := s.store.GetAgentDocument(ctx, agentLegacyStateDocumentKey)
	if err != nil || !ok {
		return models.AgentSnapshot{}, ok, err
	}
	var snapshot models.AgentSnapshot
	if err := json.Unmarshal(doc.Payload, &snapshot); err != nil {
		return models.AgentSnapshot{}, false, err
	}
	return normalizeSnapshot(snapshot), true, nil
}

func (s *Service) upsertAgentJSON(ctx context.Context, key string, domain string, payload any, updatedAt time.Time) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	updatedAt = firstTime(updatedAt, time.Now().UTC()).UTC()
	existing, ok, err := s.store.GetAgentDocument(ctx, key)
	if err != nil {
		return err
	}
	if ok && existing.UpdatedAt.Equal(updatedAt) && bytes.Equal(bytes.TrimSpace(existing.Payload), raw) {
		return nil
	}
	return s.store.UpsertAgentDocument(ctx, models.AgentDocument{
		Key:       key,
		Domain:    domain,
		Payload:   raw,
		UpdatedAt: updatedAt,
	})
}

func agentDocumentLoaders() []agentDocumentLoader {
	return []agentDocumentLoader{
		{key: agentSettingsLLMDocumentKey, load: loadAgentSettingsLLMDocument},
		{key: agentSettingsWeComDocumentKey, load: loadWrappedAgentDocument[models.AgentWeComConfig](func(snapshot *models.AgentSnapshot, payload models.AgentWeComConfig, _ time.Time) {
			snapshot.Settings.WeCom = payload
		})},
		{key: agentSettingsTerminalDocumentKey, load: loadWrappedAgentDocument[models.AgentTerminalConfig](func(snapshot *models.AgentSnapshot, payload models.AgentTerminalConfig, _ time.Time) {
			snapshot.Settings.Terminal = payload
		})},
		{key: agentSettingsEvolutionDocumentKey, load: loadWrappedAgentDocument[models.AgentEvolutionConfig](func(snapshot *models.AgentSnapshot, payload models.AgentEvolutionConfig, _ time.Time) {
			snapshot.Settings.Evolution = payload
		})},
		{key: agentSettingsSTTDocumentKey, load: loadWrappedAgentDocument[models.AgentSpeechConfig](func(snapshot *models.AgentSnapshot, payload models.AgentSpeechConfig, _ time.Time) {
			snapshot.Settings.STT = payload
		})},
		{key: agentSettingsSearchDocumentKey, load: loadAgentSettingsSearchDocument},
		{key: agentSettingsMemoryDocumentKey, load: loadWrappedAgentDocument[models.AgentMemoryConfig](func(snapshot *models.AgentSnapshot, payload models.AgentMemoryConfig, _ time.Time) {
			snapshot.Settings.Memory = payload
		})},
		{key: agentSettingsMD2ImgDocumentKey, load: loadWrappedAgentDocument[models.AgentMD2ImgConfig](func(snapshot *models.AgentSnapshot, payload models.AgentMD2ImgConfig, _ time.Time) {
			snapshot.Settings.MD2Img = payload
		})},
		{key: agentSearchLogDocumentKey, load: loadAgentSearchLogDocument},
		{key: agentDirectInputDocumentKey, load: loadPlainAgentDocument[models.AgentDirectInputConfig](func(snapshot *models.AgentSnapshot, payload models.AgentDirectInputConfig, updatedAt time.Time) {
			if payload.UpdatedAt.IsZero() {
				payload.UpdatedAt = updatedAt
			}
			snapshot.DirectInput = payload
		})},
		{key: agentWeComMenuDocumentKey, load: loadAgentWeComMenuDocument},
		{key: agentWeComEventsDocumentKey, load: loadAgentWeComEventsDocument},
		{key: agentWeComUsersDocumentKey, load: loadAgentWeComUsersDocument},
		{key: agentConversationsDocumentKey, load: loadAgentConversationsDocument},
		{key: agentMemoryRawDocumentKey, load: loadAgentMemoryRawDocument},
		{key: agentMemorySummariesDocumentKey, load: loadAgentMemorySummariesDocument},
		{key: agentMemoryWindowsDocumentKey, load: loadAgentMemoryWindowsDocument},
		{key: agentWorkflowDefinitionsDocumentKey, load: loadAgentWorkflowDefinitionsDocument},
		{key: legacyAgentTopicProfilesDocumentKey, load: loadAgentWorkflowDefinitionsDocument},
		{key: agentWorkflowRunsDocumentKey, load: loadAgentWorkflowRunsDocument},
		{key: legacyAgentTopicRunsDocumentKey, load: loadAgentWorkflowRunsDocument},
		{key: agentWritingTopicsDocumentKey, load: loadAgentWritingTopicsDocument},
		{key: agentMarketPortfolioDocumentKey, load: loadAgentMarketPortfolioDocument},
		{key: agentMarketConfigDocumentKey, load: loadAgentMarketConfigDocument},
		{key: agentMarketRunsDocumentKey, load: loadAgentMarketRunsDocument},
		{key: agentEvolutionGoalsDocumentKey, load: loadAgentEvolutionGoalsDocument},
	}
}

func loadAgentSettingsLLMDocument(doc models.AgentDocument, snapshot *models.AgentSnapshot) error {
	var payload agentSettingsLLMDocument
	if err := decodeAgentDocument(doc, &payload); err != nil {
		return err
	}
	snapshot.Settings.RuntimeMode = payload.RuntimeMode
	snapshot.Settings.DefaultLLMProviderID = payload.DefaultLLMProviderID
	snapshot.Settings.LLMProviders = payload.LLMProviders
	return nil
}

func loadAgentSettingsSearchDocument(doc models.AgentDocument, snapshot *models.AgentSnapshot) error {
	var payload agentSettingsSearchDocument
	if err := decodeAgentDocument(doc, &payload); err != nil {
		return err
	}
	snapshot.Settings.SearchEngines = payload.SearchEngines
	return nil
}

func loadAgentWeComMenuDocument(doc models.AgentDocument, snapshot *models.AgentSnapshot) error {
	var payload agentWeComMenuDocument
	if err := decodeAgentDocument(doc, &payload); err != nil {
		return err
	}
	snapshot.WeComMenu.Config = payload.Config
	if snapshot.WeComMenu.Config.UpdatedAt.IsZero() {
		snapshot.WeComMenu.Config.UpdatedAt = firstTime(payload.UpdatedAt, doc.UpdatedAt)
	}
	snapshot.WeComMenu.PublishPayload = payload.PublishPayload
	snapshot.WeComMenu.ValidationErrors = payload.ValidationErrors
	return nil
}

func loadAgentWeComEventsDocument(doc models.AgentDocument, snapshot *models.AgentSnapshot) error {
	var payload agentWeComEventsDocument
	if err := decodeAgentDocument(doc, &payload); err != nil {
		return err
	}
	snapshot.WeComMenu.RecentEvents = payload.RecentEvents
	return nil
}

func loadAgentWeComUsersDocument(doc models.AgentDocument, snapshot *models.AgentSnapshot) error {
	var payload agentWeComUsersDocument
	if err := decodeAgentDocument(doc, &payload); err != nil {
		return err
	}
	snapshot.Push.Users = payload.Users
	snapshot.Push.UpdatedAt = firstTime(payload.UpdatedAt, doc.UpdatedAt)
	return nil
}
