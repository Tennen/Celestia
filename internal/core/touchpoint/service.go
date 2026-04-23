package touchpoint

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

type SnapshotStore interface {
	Snapshot(context.Context) (models.AgentSnapshot, error)
	UpdateSnapshot(context.Context, func(*models.AgentSnapshot) error) (models.AgentSnapshot, error)
}

type AgentRunner interface {
	Converse(context.Context, models.AgentConversationRequest) (models.AgentConversation, error)
}

type InputRunner interface {
	HandleInput(context.Context, models.ProjectInputRequest) (models.ProjectInputResult, error)
}

type VoiceProvider interface {
	Transcribe(context.Context, models.AgentSpeechRequest) (models.AgentSpeechResult, error)
}

type Service struct {
	state    SnapshotStore
	agent    AgentRunner
	input    InputRunner
	voice    VoiceProvider
	mu       sync.Mutex
	stop     chan struct{}
	stopOnce sync.Once
	done     chan struct{}
	wg       sync.WaitGroup
	started  bool
}

func New(state SnapshotStore, agent AgentRunner) *Service {
	return &Service{
		state: state,
		agent: agent,
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}
}

func (s *Service) SetInputRunner(input InputRunner) {
	s.mu.Lock()
	s.input = input
	s.mu.Unlock()
}

func (s *Service) SetVoiceProvider(provider VoiceProvider) {
	s.mu.Lock()
	s.voice = provider
	s.mu.Unlock()
}

func (s *Service) Init(ctx context.Context) error {
	if _, err := s.Snapshot(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return nil
	}
	s.started = true
	s.mu.Unlock()
	s.wg.Add(1)
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
	return s.state.Snapshot(ctx)
}

func (s *Service) update(ctx context.Context, mutate func(*models.AgentSnapshot) error) (models.AgentSnapshot, error) {
	return s.state.UpdateSnapshot(ctx, mutate)
}

func (s *Service) SaveWeComUsers(ctx context.Context, users models.AgentPushSnapshot) (models.AgentSnapshot, error) {
	return s.update(ctx, func(snapshot *models.AgentSnapshot) error {
		now := time.Now().UTC()
		seenWeComUsers := map[string]struct{}{}
		for idx := range users.Users {
			users.Users[idx].ID = firstNonEmpty(users.Users[idx].ID, uuid.NewString())
			users.Users[idx].Name = strings.TrimSpace(users.Users[idx].Name)
			users.Users[idx].WeComUser = strings.TrimSpace(users.Users[idx].WeComUser)
			if users.Users[idx].WeComUser == "" {
				return errors.New("wecom user is required")
			}
			normalizedWeComUser := strings.ToLower(users.Users[idx].WeComUser)
			if _, ok := seenWeComUsers[normalizedWeComUser]; ok {
				return errors.New("wecom user must be unique")
			}
			seenWeComUsers[normalizedWeComUser] = struct{}{}
			if users.Users[idx].Name == "" {
				users.Users[idx].Name = users.Users[idx].WeComUser
			}
			users.Users[idx].UpdatedAt = now
		}
		users.UpdatedAt = now
		snapshot.Push = users
		snapshot.UpdatedAt = now
		return nil
	})
}

func (s *Service) Transcribe(ctx context.Context, req models.AgentSpeechRequest) (models.AgentSpeechResult, error) {
	s.mu.Lock()
	provider := s.voice
	s.mu.Unlock()
	if provider == nil {
		return models.AgentSpeechResult{}, errors.New("STT provider is not configured")
	}
	return provider.Transcribe(ctx, req)
}

func requireText(value string, field string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New(field + " is required")
	}
	return nil
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
