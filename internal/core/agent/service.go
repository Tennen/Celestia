package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	agentcaps "github.com/chentianyu/celestia/internal/core/agent/capabilities"
	"github.com/chentianyu/celestia/internal/core/eventbus"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/storage"
	"github.com/google/uuid"
)

const stateDocumentKey = "agent/state"

type Service struct {
	store    storage.Store
	bus      *eventbus.Bus
	mu       sync.Mutex
	stop     chan struct{}
	stopOnce sync.Once
	done     chan struct{}
	wg       sync.WaitGroup
	started  bool
}

func New(store storage.Store, bus *eventbus.Bus) *Service {
	return &Service{
		store: store,
		bus:   bus,
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}
}

func (s *Service) Init(ctx context.Context) error {
	_, err := s.Snapshot(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return nil
	}
	s.started = true
	s.mu.Unlock()
	s.wg.Add(2)
	go func() {
		defer s.wg.Done()
		s.runScheduler()
	}()
	go func() {
		defer s.wg.Done()
		s.runWeComBridge()
	}()
	go func() {
		s.wg.Wait()
		close(s.done)
	}()
	return nil
}

func (s *Service) Close() {
	s.mu.Lock()
	started := s.started
	s.mu.Unlock()
	if !started {
		return
	}
	select {
	case <-s.done:
		return
	default:
	}
	s.stopOnce.Do(func() {
		close(s.stop)
	})
	<-s.done
}

func (s *Service) Snapshot(ctx context.Context) (models.AgentSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load(ctx)
}

func (s *Service) SaveSettings(ctx context.Context, settings models.AgentSettings) (models.AgentSnapshot, error) {
	return s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		now := time.Now().UTC()
		settings.UpdatedAt = now
		snapshot.Settings = normalizeSettings(settings)
		snapshot.UpdatedAt = now
		return nil
	})
}

func (s *Service) SaveDirectInput(ctx context.Context, config models.AgentDirectInputConfig) (models.AgentSnapshot, error) {
	return s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		now := time.Now().UTC()
		config.Version = 1
		config.UpdatedAt = now
		for idx := range config.Rules {
			config.Rules[idx] = normalizeDirectRule(config.Rules[idx])
		}
		snapshot.DirectInput = config
		snapshot.UpdatedAt = now
		return nil
	})
}

func (s *Service) SavePush(ctx context.Context, push models.AgentPushSnapshot) (models.AgentSnapshot, error) {
	return s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		now := time.Now().UTC()
		for idx := range push.Users {
			push.Users[idx].ID = firstNonEmpty(push.Users[idx].ID, uuid.NewString())
			push.Users[idx].UpdatedAt = now
		}
		for idx := range push.Tasks {
			push.Tasks[idx].ID = firstNonEmpty(push.Tasks[idx].ID, uuid.NewString())
			push.Tasks[idx].UpdatedAt = now
			if push.Tasks[idx].Enabled && push.Tasks[idx].NextRunAt == nil {
				next := now.Add(time.Duration(maxInt(push.Tasks[idx].IntervalM, 1)) * time.Minute)
				push.Tasks[idx].NextRunAt = &next
			}
		}
		push.UpdatedAt = now
		snapshot.Push = push
		snapshot.UpdatedAt = now
		return nil
	})
}

func (s *Service) update(ctx context.Context, mutate func(*models.AgentSnapshot) error) (models.AgentSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot, err := s.load(ctx)
	if err != nil {
		return models.AgentSnapshot{}, err
	}
	if err := mutate(&snapshot); err != nil {
		return models.AgentSnapshot{}, err
	}
	if err := s.save(ctx, snapshot); err != nil {
		return models.AgentSnapshot{}, err
	}
	_ = s.emit(ctx, models.EventAgentStateChanged, map[string]any{"updated_at": snapshot.UpdatedAt})
	return snapshot, nil
}

func (s *Service) load(ctx context.Context) (models.AgentSnapshot, error) {
	doc, ok, err := s.store.GetAgentDocument(ctx, stateDocumentKey)
	if err != nil {
		return models.AgentSnapshot{}, err
	}
	if !ok {
		snapshot := defaultSnapshot()
		return snapshot, s.save(ctx, snapshot)
	}
	var snapshot models.AgentSnapshot
	if err := json.Unmarshal(doc.Payload, &snapshot); err != nil {
		return models.AgentSnapshot{}, err
	}
	return normalizeSnapshot(snapshot), nil
}

func (s *Service) save(ctx context.Context, snapshot models.AgentSnapshot) error {
	payload, err := json.Marshal(normalizeSnapshot(snapshot))
	if err != nil {
		return err
	}
	return s.store.UpsertAgentDocument(ctx, models.AgentDocument{
		Key:       stateDocumentKey,
		Domain:    "agent",
		Payload:   payload,
		UpdatedAt: snapshot.UpdatedAt,
	})
}

func (s *Service) emit(ctx context.Context, eventType models.EventType, payload map[string]any) error {
	event := models.Event{
		ID:      uuid.NewString(),
		Type:    eventType,
		TS:      time.Now().UTC(),
		Payload: payload,
	}
	if err := s.store.AppendEvent(ctx, event); err != nil {
		return err
	}
	if s.bus != nil {
		s.bus.Publish(event)
	}
	return nil
}

func defaultSnapshot() models.AgentSnapshot {
	now := time.Now().UTC()
	return models.AgentSnapshot{
		Settings: normalizeSettings(models.AgentSettings{
			RuntimeMode: "classic",
			Terminal: models.AgentTerminalConfig{
				TimeoutMS: 30000,
			},
			Evolution: models.AgentEvolutionConfig{
				TimeoutMS: 600000,
			},
			UpdatedAt: now,
		}),
		Capabilities: agentcaps.List(),
		DirectInput: models.AgentDirectInputConfig{
			Version:   1,
			Rules:     []models.AgentDirectInputRule{},
			UpdatedAt: now,
		},
		WeComMenu: models.AgentWeComMenuSnapshot{
			Config: models.AgentWeComMenuConfig{
				Version:   1,
				Buttons:   []models.AgentWeComButton{},
				UpdatedAt: now,
			},
			RecentEvents: []models.AgentWeComEventRecord{},
		},
		Push: models.AgentPushSnapshot{
			Users:     []models.AgentPushUser{},
			Tasks:     []models.AgentPushTask{},
			UpdatedAt: now,
		},
		Conversations: []models.AgentConversation{},
		Memory: models.AgentMemorySnapshot{
			RawRecords: []models.AgentRawMemoryRecord{},
			Summaries:  []models.AgentSummaryMemoryRecord{},
			Windows:    []models.AgentConversationWindow{},
			UpdatedAt:  now,
		},
		TopicSummary: models.AgentTopicSnapshot{
			Profiles:  []models.AgentTopicProfile{},
			Runs:      []models.AgentTopicRun{},
			UpdatedAt: now,
		},
		Writing: models.AgentWritingSnapshot{
			Topics:    []models.AgentWritingTopic{},
			UpdatedAt: now,
		},
		Market: models.AgentMarketSnapshot{
			Portfolio: models.AgentMarketPortfolio{Funds: []models.AgentMarketHolding{}},
			Runs:      []models.AgentMarketRun{},
			UpdatedAt: now,
		},
		Evolution: models.AgentEvolutionSnapshot{
			Goals:     []models.AgentEvolutionGoal{},
			UpdatedAt: now,
		},
		UpdatedAt: now,
	}
}

func normalizeSnapshot(snapshot models.AgentSnapshot) models.AgentSnapshot {
	if snapshot.UpdatedAt.IsZero() {
		snapshot.UpdatedAt = time.Now().UTC()
	}
	snapshot.Settings = normalizeSettings(snapshot.Settings)
	snapshot.Capabilities = agentcaps.List()
	if snapshot.Conversations == nil {
		snapshot.Conversations = []models.AgentConversation{}
	}
	if snapshot.DirectInput.Version == 0 {
		snapshot.DirectInput.Version = 1
	}
	if snapshot.WeComMenu.Config.Version == 0 {
		snapshot.WeComMenu.Config.Version = 1
	}
	if snapshot.DirectInput.Rules == nil {
		snapshot.DirectInput.Rules = []models.AgentDirectInputRule{}
	}
	if snapshot.WeComMenu.Config.Buttons == nil {
		snapshot.WeComMenu.Config.Buttons = []models.AgentWeComButton{}
	}
	if snapshot.WeComMenu.RecentEvents == nil {
		snapshot.WeComMenu.RecentEvents = []models.AgentWeComEventRecord{}
	}
	if snapshot.WeComMenu.ValidationErrors == nil {
		snapshot.WeComMenu.ValidationErrors = []string{}
	}
	if snapshot.Push.Users == nil {
		snapshot.Push.Users = []models.AgentPushUser{}
	}
	if snapshot.Push.Tasks == nil {
		snapshot.Push.Tasks = []models.AgentPushTask{}
	}
	if snapshot.Memory.RawRecords == nil {
		snapshot.Memory.RawRecords = []models.AgentRawMemoryRecord{}
	}
	if snapshot.Memory.Summaries == nil {
		snapshot.Memory.Summaries = []models.AgentSummaryMemoryRecord{}
	}
	if snapshot.Memory.Windows == nil {
		snapshot.Memory.Windows = []models.AgentConversationWindow{}
	}
	if snapshot.TopicSummary.Profiles == nil {
		snapshot.TopicSummary.Profiles = []models.AgentTopicProfile{}
	}
	if snapshot.TopicSummary.Runs == nil {
		snapshot.TopicSummary.Runs = []models.AgentTopicRun{}
	}
	if snapshot.TopicSummary.SentLog == nil {
		snapshot.TopicSummary.SentLog = []models.AgentTopicSentLogItem{}
	}
	if snapshot.Writing.Topics == nil {
		snapshot.Writing.Topics = []models.AgentWritingTopic{}
	}
	if snapshot.Market.Portfolio.Funds == nil {
		snapshot.Market.Portfolio.Funds = []models.AgentMarketHolding{}
	}
	if snapshot.Market.Runs == nil {
		snapshot.Market.Runs = []models.AgentMarketRun{}
	}
	if snapshot.Evolution.Goals == nil {
		snapshot.Evolution.Goals = []models.AgentEvolutionGoal{}
	}
	return snapshot
}

func normalizeSettings(settings models.AgentSettings) models.AgentSettings {
	memoryWasEmpty := settings.Memory == (models.AgentMemoryConfig{})
	md2imgWasEmpty := settings.MD2Img == (models.AgentMD2ImgConfig{})
	settings.RuntimeMode = firstNonEmpty(settings.RuntimeMode, "classic")
	if settings.LLMProviders == nil {
		settings.LLMProviders = []models.AgentLLMProvider{}
	}
	if settings.SearchEngines == nil {
		settings.SearchEngines = []models.AgentSearchProvider{}
	}
	if settings.Evolution.TestCommands == nil {
		settings.Evolution.TestCommands = []models.AgentEvolutionTestCommand{}
	}
	if memoryWasEmpty {
		settings.Memory.Enabled = true
	}
	if settings.Memory.CompactEveryRounds <= 0 {
		settings.Memory.CompactEveryRounds = 4
	}
	if settings.Memory.CompactMaxBatchSize <= 0 {
		settings.Memory.CompactMaxBatchSize = 8
	}
	if settings.Memory.SummaryTopK <= 0 {
		settings.Memory.SummaryTopK = 4
	}
	if settings.Memory.RawRefLimit <= 0 {
		settings.Memory.RawRefLimit = 8
	}
	if settings.Memory.RawRecordLimit <= 0 {
		settings.Memory.RawRecordLimit = 3
	}
	if settings.Memory.WindowTimeoutSeconds <= 0 {
		settings.Memory.WindowTimeoutSeconds = 180
	}
	if settings.Memory.WindowMaxTurns <= 0 {
		settings.Memory.WindowMaxTurns = 6
	}
	if md2imgWasEmpty {
		settings.MD2Img.Enabled = true
	}
	settings.MD2Img.Mode = firstNonEmpty(settings.MD2Img.Mode, "long-image")
	settings.MD2Img.Command = firstNonEmpty(settings.MD2Img.Command, "node internal/core/agent/md2img/render.mjs")
	settings.MD2Img.OutputDir = firstNonEmpty(settings.MD2Img.OutputDir, "data/agent/md2img")
	if settings.MD2Img.TimeoutMS <= 0 {
		settings.MD2Img.TimeoutMS = 60000
	}
	if settings.Terminal.TimeoutMS <= 0 {
		settings.Terminal.TimeoutMS = 30000
	}
	if settings.Evolution.TimeoutMS <= 0 {
		settings.Evolution.TimeoutMS = 600000
	}
	if settings.Evolution.MaxFixAttempts <= 0 {
		settings.Evolution.MaxFixAttempts = 2
	}
	if strings.TrimSpace(settings.WeCom.BaseURL) == "" {
		settings.WeCom.BaseURL = "https://qyapi.weixin.qq.com"
	}
	if strings.TrimSpace(settings.WeCom.AudioDir) == "" {
		settings.WeCom.AudioDir = "data/agent/wecom-audio"
	}
	if settings.WeCom.TextMaxBytes <= 0 {
		settings.WeCom.TextMaxBytes = 1800
	}
	return settings
}

func normalizeDirectRule(rule models.AgentDirectInputRule) models.AgentDirectInputRule {
	rule.ID = firstNonEmpty(rule.ID, uuid.NewString())
	if rule.MatchMode != "fuzzy" {
		rule.MatchMode = "exact"
	}
	return rule
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func maxInt(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func truncateList[T any](items []T, limit int) []T {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}

func requireText(value string, field string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New(field + " is required")
	}
	return nil
}
