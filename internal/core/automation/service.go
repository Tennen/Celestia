package automation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/chentianyu/celestia/internal/core/audit"
	"github.com/chentianyu/celestia/internal/core/eventbus"
	"github.com/chentianyu/celestia/internal/core/pluginmgr"
	"github.com/chentianyu/celestia/internal/core/policy"
	"github.com/chentianyu/celestia/internal/core/registry"
	"github.com/chentianyu/celestia/internal/core/state"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/storage"
	"github.com/google/uuid"
)

type Service struct {
	store     storage.Store
	bus       *eventbus.Bus
	registry  *registry.Service
	state     *state.Service
	policy    *policy.Service
	audit     *audit.Service
	pluginMgr *pluginmgr.Manager

	mu                sync.RWMutex
	automationIndex   map[string][]string
	automationCache   map[string]models.Automation
	automationDevices map[string]string

	subscriptionID int
}

func New(
	store storage.Store,
	bus *eventbus.Bus,
	registry *registry.Service,
	state *state.Service,
	policySvc *policy.Service,
	auditSvc *audit.Service,
	pluginMgr *pluginmgr.Manager,
) *Service {
	svc := &Service{
		store:             store,
		bus:               bus,
		registry:          registry,
		state:             state,
		policy:            policySvc,
		audit:             auditSvc,
		pluginMgr:         pluginMgr,
		automationIndex:   map[string][]string{},
		automationCache:   map[string]models.Automation{},
		automationDevices: map[string]string{},
	}
	svc.loadIndexOnStart()
	svc.start()
	return svc
}

func (s *Service) start() {
	id, ch := s.bus.Subscribe(128)
	s.subscriptionID = id
	go func() {
		for event := range ch {
			if event.Type != models.EventDeviceStateChanged {
				continue
			}
			go s.handleStateChange(event)
		}
	}()
}

func (s *Service) Close() {
	if s.bus != nil {
		s.bus.Unsubscribe(s.subscriptionID)
	}
}

func (s *Service) List(ctx context.Context) ([]models.Automation, error) {
	return s.store.ListAutomations(ctx)
}

func (s *Service) Get(ctx context.Context, id string) (models.Automation, bool, error) {
	return s.store.GetAutomation(ctx, strings.TrimSpace(id))
}

func (s *Service) Save(ctx context.Context, automation models.Automation) (models.Automation, error) {
	now := time.Now().UTC()
	automation.ID = strings.TrimSpace(automation.ID)
	if automation.ID == "" {
		automation.ID = uuid.NewString()
	}
	if existing, ok, err := s.store.GetAutomation(ctx, automation.ID); err != nil {
		return models.Automation{}, err
	} else if ok {
		automation.CreatedAt = existing.CreatedAt
		if automation.LastTriggeredAt == nil {
			automation.LastTriggeredAt = existing.LastTriggeredAt
		}
		if automation.LastRunStatus == "" {
			automation.LastRunStatus = existing.LastRunStatus
		}
		if automation.LastError == "" {
			automation.LastError = existing.LastError
		}
	}
	if automation.CreatedAt.IsZero() {
		automation.CreatedAt = now
	}
	automation.UpdatedAt = now
	normalized, err := s.normalizeAutomation(ctx, automation)
	if err != nil {
		return models.Automation{}, err
	}
	if err := s.store.UpsertAutomation(ctx, normalized); err != nil {
		return models.Automation{}, err
	}
	s.upsertIndexedAutomation(normalized)
	return normalized, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("automation id is required")
	}
	if err := s.store.DeleteAutomation(ctx, id); err != nil {
		return err
	}
	s.removeIndexedAutomation(id)
	return nil
}

func (s *Service) normalizeAutomation(ctx context.Context, automation models.Automation) (models.Automation, error) {
	automation.Name = strings.TrimSpace(automation.Name)
	if automation.Name == "" {
		return models.Automation{}, errors.New("automation name is required")
	}
	switch automation.ConditionLogic {
	case models.AutomationLogicAll, models.AutomationLogicAny:
	default:
		automation.ConditionLogic = models.AutomationLogicAll
	}
	if automation.LastRunStatus == "" {
		automation.LastRunStatus = models.AutomationRunStatusIdle
	}
	if len(automation.Conditions) == 0 {
		return models.Automation{}, errors.New("automation requires at least one condition")
	}
	eventConditionCount := 0
	for idx := range automation.Conditions {
		normalized, err := s.normalizeCondition(ctx, automation.Conditions[idx], idx)
		if err != nil {
			return models.Automation{}, err
		}
		if normalized.Type == models.AutomationConditionTypeStateChanged {
			eventConditionCount++
		}
		automation.Conditions[idx] = normalized
	}
	if eventConditionCount != 1 {
		return models.Automation{}, errors.New("automation requires exactly one state_changed condition")
	}
	if automation.TimeWindow != nil {
		automation.TimeWindow.Start = strings.TrimSpace(automation.TimeWindow.Start)
		automation.TimeWindow.End = strings.TrimSpace(automation.TimeWindow.End)
		if automation.TimeWindow.Start == "" && automation.TimeWindow.End == "" {
			automation.TimeWindow = nil
		} else if automation.TimeWindow.Start == "" || automation.TimeWindow.End == "" {
			return models.Automation{}, errors.New("time_window requires both start and end")
		} else {
			if _, err := parseClockHM(automation.TimeWindow.Start); err != nil {
				return models.Automation{}, fmt.Errorf("invalid time_window start: %w", err)
			}
			if _, err := parseClockHM(automation.TimeWindow.End); err != nil {
				return models.Automation{}, fmt.Errorf("invalid time_window end: %w", err)
			}
		}
	}
	if len(automation.Actions) == 0 {
		return models.Automation{}, errors.New("automation requires at least one action")
	}
	for idx := range automation.Actions {
		action := &automation.Actions[idx]
		action.DeviceID = strings.TrimSpace(action.DeviceID)
		action.Label = strings.TrimSpace(action.Label)
		action.Action = strings.TrimSpace(action.Action)
		if action.DeviceID == "" || action.Action == "" {
			return models.Automation{}, fmt.Errorf("action %d requires device_id and action", idx)
		}
		if _, ok, err := s.registry.Get(ctx, action.DeviceID); err != nil {
			return models.Automation{}, err
		} else if !ok {
			return models.Automation{}, fmt.Errorf("action device %q not found", action.DeviceID)
		}
		if action.Params == nil {
			action.Params = map[string]any{}
		}
	}
	return automation, nil
}

func (s *Service) normalizeCondition(ctx context.Context, condition models.AutomationCondition, idx int) (models.AutomationCondition, error) {
	condition.DeviceID = strings.TrimSpace(condition.DeviceID)
	condition.StateKey = strings.TrimSpace(condition.StateKey)
	if condition.DeviceID == "" || condition.StateKey == "" {
		return models.AutomationCondition{}, fmt.Errorf("condition %d requires device_id and state_key", idx)
	}
	if _, ok, err := s.registry.Get(ctx, condition.DeviceID); err != nil {
		return models.AutomationCondition{}, err
	} else if !ok {
		return models.AutomationCondition{}, fmt.Errorf("condition device %q not found", condition.DeviceID)
	}
	condition.Type = normalizeConditionType(condition)
	switch condition.Type {
	case models.AutomationConditionTypeStateChanged:
		from := derefMatch(condition.From)
		to := derefMatch(condition.To)
		from = normalizeMatch(from, true)
		to = normalizeMatch(to, false)
		if err := validateMatch(from, true); err != nil {
			return models.AutomationCondition{}, fmt.Errorf("condition %d has invalid from matcher: %w", idx, err)
		}
		if err := validateMatch(to, false); err != nil {
			return models.AutomationCondition{}, fmt.Errorf("condition %d has invalid to matcher: %w", idx, err)
		}
		condition.From = &from
		condition.To = &to
		condition.Match = nil
	case models.AutomationConditionTypeCurrentState:
		match := normalizeMatch(derefMatch(condition.Match), false)
		if err := validateMatch(match, false); err != nil {
			return models.AutomationCondition{}, fmt.Errorf("condition %d has invalid matcher: %w", idx, err)
		}
		condition.Match = &match
		condition.From = nil
		condition.To = nil
	default:
		return models.AutomationCondition{}, fmt.Errorf("condition %d has unsupported type %q", idx, condition.Type)
	}
	return condition, nil
}

func normalizeConditionType(condition models.AutomationCondition) models.AutomationConditionType {
	switch condition.Type {
	case models.AutomationConditionTypeStateChanged, models.AutomationConditionTypeCurrentState:
		return condition.Type
	}
	if condition.From != nil || condition.To != nil {
		return models.AutomationConditionTypeStateChanged
	}
	return models.AutomationConditionTypeCurrentState
}

func derefMatch(match *models.AutomationStateMatch) models.AutomationStateMatch {
	if match == nil {
		return models.AutomationStateMatch{}
	}
	return *match
}

func normalizeMatch(match models.AutomationStateMatch, allowAny bool) models.AutomationStateMatch {
	switch match.Operator {
	case models.AutomationMatchAny,
		models.AutomationMatchEquals,
		models.AutomationMatchNotEquals,
		models.AutomationMatchIn,
		models.AutomationMatchNotIn,
		models.AutomationMatchExists,
		models.AutomationMatchMissing:
		match.Value = normalizeMatchValue(match.Operator, match.Value)
		return match
	}
	if match.Value != nil {
		match.Operator = models.AutomationMatchEquals
		match.Value = normalizeMatchValue(match.Operator, match.Value)
		return match
	}
	if allowAny {
		match.Operator = models.AutomationMatchAny
		return match
	}
	match.Operator = models.AutomationMatchExists
	return match
}

func validateMatch(match models.AutomationStateMatch, allowAny bool) error {
	switch match.Operator {
	case models.AutomationMatchAny:
		if !allowAny {
			return errors.New(`operator "any" is only allowed for transition from`)
		}
	case models.AutomationMatchEquals, models.AutomationMatchNotEquals:
		if match.Value == nil {
			return errors.New("value is required")
		}
	case models.AutomationMatchIn, models.AutomationMatchNotIn:
		items := matchValueList(match.Value)
		if len(items) == 0 {
			return errors.New("non-empty value list is required")
		}
	case models.AutomationMatchExists, models.AutomationMatchMissing:
	default:
		return fmt.Errorf("unsupported operator %q", match.Operator)
	}
	return nil
}

func normalizeMatchValue(operator models.AutomationMatchOperator, value any) any {
	switch operator {
	case models.AutomationMatchIn, models.AutomationMatchNotIn:
		items := matchValueList(value)
		if len(items) == 0 {
			return nil
		}
		return items
	default:
		return value
	}
}

func matchValueList(value any) []any {
	switch typed := value.(type) {
	case nil:
		return nil
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out
	case []string:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out
	case []int:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out
	case []float64:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out
	case []bool:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out
	default:
		return []any{typed}
	}
}
