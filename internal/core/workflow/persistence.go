package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

const (
	workflowDefinitionsDocumentKey = "agent/workflow/definitions"
	workflowRunsDocumentKey        = "agent/workflow/runs"
	workflowSettingsLLMDocumentKey = "agent/settings/llm"
	workflowSettingsSearchKey      = "agent/settings/search"
)

type workflowDefinitionsDocument struct {
	ActiveWorkflowID string                 `json:"active_workflow_id,omitempty"`
	Workflows        []models.AgentWorkflow `json:"workflows,omitempty"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

type workflowRunsDocument struct {
	Runs         []models.AgentWorkflowRun         `json:"runs"`
	SentLog      []models.AgentWorkflowSentLogItem `json:"sent_log"`
	SourceStates []models.AgentWorkflowSourceState `json:"source_states,omitempty"`
	TimerStates  []models.AgentWorkflowTimerState  `json:"timer_states,omitempty"`
	UpdatedAt    time.Time                         `json:"updated_at"`
}

type workflowSettingsLLMDocument struct {
	RuntimeMode          string                    `json:"runtime_mode"`
	DefaultLLMProviderID string                    `json:"default_llm_provider_id"`
	LLMProviders         []models.AgentLLMProvider `json:"llm_providers"`
	UpdatedAt            time.Time                 `json:"updated_at"`
}

type workflowSettingsSearchDocument struct {
	SearchEngines []models.AgentSearchProvider `json:"search_engines"`
	UpdatedAt     time.Time                    `json:"updated_at"`
}

func (s *Service) load(ctx context.Context) (models.AgentSnapshot, error) {
	snapshot := models.AgentSnapshot{
		Settings: models.AgentSettings{
			LLMProviders:  []models.AgentLLMProvider{},
			SearchEngines: []models.AgentSearchProvider{},
		},
		Workflow: models.AgentWorkflowSnapshot{
			Workflows:    []models.AgentWorkflow{},
			Runs:         []models.AgentWorkflowRun{},
			SentLog:      []models.AgentWorkflowSentLogItem{},
			SourceStates: []models.AgentWorkflowSourceState{},
			TimerStates:  []models.AgentWorkflowTimerState{},
		},
	}
	if err := s.loadSettings(ctx, &snapshot); err != nil {
		return models.AgentSnapshot{}, err
	}
	if err := s.loadWorkflow(ctx, &snapshot); err != nil {
		return models.AgentSnapshot{}, err
	}
	snapshot.Workflow = normalizeLoadedWorkflowSnapshot(snapshot.Workflow)
	return snapshot, nil
}

func (s *Service) save(ctx context.Context, snapshot models.AgentSnapshot) error {
	workflowUpdatedAt := workflowFirstTime(snapshot.Workflow.UpdatedAt, snapshot.UpdatedAt, time.Now().UTC())
	definitions := workflowDefinitionsDocument{
		ActiveWorkflowID: snapshot.Workflow.ActiveWorkflowID,
		Workflows:        snapshot.Workflow.Workflows,
		UpdatedAt:        workflowUpdatedAt,
	}
	runs := workflowRunsDocument{
		Runs:         snapshot.Workflow.Runs,
		SentLog:      snapshot.Workflow.SentLog,
		SourceStates: snapshot.Workflow.SourceStates,
		TimerStates:  snapshot.Workflow.TimerStates,
		UpdatedAt:    workflowUpdatedAt,
	}
	if err := s.upsertJSON(ctx, workflowDefinitionsDocumentKey, "workflow.definitions", definitions, workflowUpdatedAt); err != nil {
		return err
	}
	return s.upsertJSON(ctx, workflowRunsDocumentKey, "workflow.runs", runs, workflowUpdatedAt)
}

func (s *Service) loadSettings(ctx context.Context, snapshot *models.AgentSnapshot) error {
	doc, ok, err := s.store.GetAgentDocument(ctx, workflowSettingsLLMDocumentKey)
	if err != nil {
		return err
	}
	if ok {
		var payload workflowSettingsLLMDocument
		if err := json.Unmarshal(doc.Payload, &payload); err != nil {
			return err
		}
		snapshot.Settings.RuntimeMode = payload.RuntimeMode
		snapshot.Settings.DefaultLLMProviderID = payload.DefaultLLMProviderID
		snapshot.Settings.LLMProviders = payload.LLMProviders
		snapshot.Settings.UpdatedAt = maxTime(snapshot.Settings.UpdatedAt, workflowFirstTime(payload.UpdatedAt, doc.UpdatedAt))
	}
	doc, ok, err = s.store.GetAgentDocument(ctx, workflowSettingsSearchKey)
	if err != nil {
		return err
	}
	if ok {
		var payload workflowSettingsSearchDocument
		if err := json.Unmarshal(doc.Payload, &payload); err != nil {
			return err
		}
		snapshot.Settings.SearchEngines = payload.SearchEngines
		snapshot.Settings.UpdatedAt = maxTime(snapshot.Settings.UpdatedAt, workflowFirstTime(payload.UpdatedAt, doc.UpdatedAt))
	}
	if snapshot.Settings.LLMProviders == nil {
		snapshot.Settings.LLMProviders = []models.AgentLLMProvider{}
	}
	if snapshot.Settings.SearchEngines == nil {
		snapshot.Settings.SearchEngines = []models.AgentSearchProvider{}
	}
	return nil
}

func (s *Service) loadWorkflow(ctx context.Context, snapshot *models.AgentSnapshot) error {
	doc, ok, err := s.store.GetAgentDocument(ctx, workflowDefinitionsDocumentKey)
	if err != nil {
		return err
	}
	if ok {
		var payload workflowDefinitionsDocument
		if err := json.Unmarshal(doc.Payload, &payload); err != nil {
			return err
		}
		snapshot.Workflow.ActiveWorkflowID = payload.ActiveWorkflowID
		snapshot.Workflow.Workflows = payload.Workflows
		snapshot.Workflow.UpdatedAt = maxTime(snapshot.Workflow.UpdatedAt, workflowFirstTime(payload.UpdatedAt, doc.UpdatedAt))
	}
	doc, ok, err = s.store.GetAgentDocument(ctx, workflowRunsDocumentKey)
	if err != nil {
		return err
	}
	if ok {
		var payload workflowRunsDocument
		if err := json.Unmarshal(doc.Payload, &payload); err != nil {
			return err
		}
		for idx := range payload.Runs {
			if payload.Runs[idx].StartedAt.IsZero() {
				payload.Runs[idx].StartedAt = payload.Runs[idx].CreatedAt
			}
		}
		snapshot.Workflow.Runs = payload.Runs
		snapshot.Workflow.SentLog = payload.SentLog
		snapshot.Workflow.SourceStates = payload.SourceStates
		snapshot.Workflow.TimerStates = payload.TimerStates
		snapshot.Workflow.UpdatedAt = maxTime(snapshot.Workflow.UpdatedAt, workflowFirstTime(payload.UpdatedAt, doc.UpdatedAt))
	}
	return nil
}

func normalizeLoadedWorkflowSnapshot(snapshot models.AgentWorkflowSnapshot) models.AgentWorkflowSnapshot {
	if snapshot.Workflows == nil {
		snapshot.Workflows = []models.AgentWorkflow{}
	}
	if snapshot.Runs == nil {
		snapshot.Runs = []models.AgentWorkflowRun{}
	}
	if snapshot.SentLog == nil {
		snapshot.SentLog = []models.AgentWorkflowSentLogItem{}
	}
	if snapshot.SourceStates == nil {
		snapshot.SourceStates = []models.AgentWorkflowSourceState{}
	}
	snapshot.SourceStates = pruneWorkflowSourceStates(snapshot.SourceStates, snapshot.Workflows)
	if snapshot.TimerStates == nil {
		snapshot.TimerStates = []models.AgentWorkflowTimerState{}
	}
	snapshot.TimerStates = pruneWorkflowTimerStates(snapshot.TimerStates, snapshot.Workflows)
	return snapshot
}

func (s *Service) upsertJSON(ctx context.Context, key string, domain string, payload any, updatedAt time.Time) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	updatedAt = workflowFirstTime(updatedAt, time.Now().UTC()).UTC()
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

func maxTime(left time.Time, right time.Time) time.Time {
	if left.IsZero() || right.After(left) {
		return right
	}
	return left
}
