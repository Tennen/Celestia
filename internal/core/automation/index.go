package automation

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

func (s *Service) loadIndexOnStart() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.reloadIndex(ctx); err != nil {
		log.Printf("automation: initial index load failed: %v", err)
	}
}

func (s *Service) reloadIndex(ctx context.Context) error {
	automations, err := s.store.ListAutomations(ctx)
	if err != nil {
		return err
	}

	index := make(map[string][]string, len(automations))
	cache := make(map[string]models.Automation, len(automations))
	devices := make(map[string]string, len(automations))
	for _, automation := range automations {
		if !automation.Enabled {
			continue
		}
		trigger, ok := stateChangedCondition(automation.Conditions)
		deviceID := strings.TrimSpace(trigger.DeviceID)
		if !ok || deviceID == "" {
			log.Printf("automation: skip index id=%s invalid state_changed configuration", automation.ID)
			continue
		}
		cache[automation.ID] = automation
		devices[automation.ID] = deviceID
		index[deviceID] = append(index[deviceID], automation.ID)
	}

	s.mu.Lock()
	s.automationIndex = index
	s.automationCache = cache
	s.automationDevices = devices
	s.mu.Unlock()
	return nil
}

func (s *Service) upsertIndexedAutomation(automation models.Automation) {
	trigger, ok := stateChangedCondition(automation.Conditions)
	deviceID := strings.TrimSpace(trigger.DeviceID)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeIndexedAutomationLocked(automation.ID)
	if !automation.Enabled || !ok || deviceID == "" {
		return
	}
	s.automationCache[automation.ID] = automation
	s.automationDevices[automation.ID] = deviceID
	s.automationIndex[deviceID] = append(s.automationIndex[deviceID], automation.ID)
}

func (s *Service) removeIndexedAutomation(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeIndexedAutomationLocked(id)
}

func (s *Service) removeIndexedAutomationLocked(id string) {
	deviceID := s.automationDevices[id]
	delete(s.automationCache, id)
	delete(s.automationDevices, id)
	if deviceID == "" {
		return
	}
	ids := s.automationIndex[deviceID]
	filtered := ids[:0]
	for _, candidate := range ids {
		if candidate != id {
			filtered = append(filtered, candidate)
		}
	}
	if len(filtered) == 0 {
		delete(s.automationIndex, deviceID)
		return
	}
	s.automationIndex[deviceID] = append([]string(nil), filtered...)
}

func (s *Service) indexedAutomationsForDevice(deviceID string) []models.Automation {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil
	}

	s.mu.RLock()
	ids := append([]string(nil), s.automationIndex[deviceID]...)
	automations := make([]models.Automation, 0, len(ids))
	for _, id := range ids {
		automation, ok := s.automationCache[id]
		if ok {
			automations = append(automations, automation)
		}
	}
	s.mu.RUnlock()
	return automations
}

func (s *Service) updateIndexedAutomation(automation models.Automation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.automationCache[automation.ID]; ok {
		s.automationCache[automation.ID] = automation
	}
}

func stateChangedCondition(conditions []models.AutomationCondition) (models.AutomationCondition, bool) {
	var (
		match models.AutomationCondition
		found bool
	)
	for _, condition := range conditions {
		if normalizeConditionType(condition) != models.AutomationConditionTypeStateChanged {
			continue
		}
		if found {
			return models.AutomationCondition{}, false
		}
		match = condition
		found = true
	}
	return match, found
}
