package workflow

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/chentianyu/celestia/internal/core/eventbus"
	corellm "github.com/chentianyu/celestia/internal/core/llm"
	coresearch "github.com/chentianyu/celestia/internal/core/search"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/storage"
)

type Service struct {
	store           storage.Store
	bus             *eventbus.Bus
	workflowOutput  OutputRuntime
	workflowInput   InputRuntime
	workflowDevices WorkflowDeviceRuntime
	mu              sync.Mutex
	startOnce       sync.Once
	eventOnce       sync.Once
	workerOnce      sync.Once
	stop            chan struct{}
	stopOnce        sync.Once
	workflowJobs    chan workflowScheduledRun
	eventSubID      int
	eventSubscribed bool
}

func New(store storage.Store, bus *eventbus.Bus) *Service {
	return &Service{
		store:        store,
		bus:          bus,
		stop:         make(chan struct{}),
		workflowJobs: make(chan workflowScheduledRun, 256),
	}
}

func (s *Service) Init(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if _, err := s.Snapshot(ctx); err != nil {
		return err
	}
	s.startWorkflowSchedulerWorker()
	s.startWorkflowEventTriggers()
	s.startOnce.Do(func() {
		go s.runWorkflowTimeScheduler()
	})
	return nil
}

func (s *Service) Close() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stop)
	})
	if s.bus != nil && s.eventSubscribed {
		s.bus.Unsubscribe(s.eventSubID)
	}
}

func (s *Service) Snapshot(ctx context.Context) (models.AgentSnapshot, error) {
	if s == nil || s.store == nil {
		return models.AgentSnapshot{}, errors.New("workflow store is not configured")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load(ctx)
}

func (s *Service) update(ctx context.Context, mutate func(*models.AgentSnapshot) error) (models.AgentSnapshot, error) {
	if s == nil || s.store == nil {
		return models.AgentSnapshot{}, errors.New("workflow store is not configured")
	}
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
	return snapshot, nil
}

func (s *Service) RunSearch(ctx context.Context, req models.AgentSearchRequest) (models.AgentSearchResult, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return models.AgentSearchResult{}, err
	}
	run, err := coresearch.Run(ctx, snapshot.Settings.SearchEngines, req)
	return run.Result, err
}

func (s *Service) GenerateTextWithProvider(ctx context.Context, providerID string, prompt string) (string, error) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		return "", err
	}
	return corellm.GenerateTextWithProvider(ctx, snapshot.Settings, providerID, prompt)
}

func (s *Service) HandleTimeTick(now time.Time) {
	s.handleWorkflowTimeTick(now)
}
