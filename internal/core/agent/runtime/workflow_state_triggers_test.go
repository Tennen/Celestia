package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/chentianyu/celestia/internal/models"
)

type workflowTestDeviceRuntime struct {
	states   map[string]models.DeviceStateSnapshot
	commands []models.CommandRequest
}

func (r *workflowTestDeviceRuntime) DeviceState(_ context.Context, deviceID string) (models.DeviceStateSnapshot, bool, error) {
	state, ok := r.states[deviceID]
	return state, ok, nil
}

func (r *workflowTestDeviceRuntime) ExecuteDeviceCommand(_ context.Context, _ string, deviceID string, action string, params map[string]any) error {
	r.commands = append(r.commands, models.CommandRequest{DeviceID: deviceID, Action: action, Params: cloneWorkflowParams(params)})
	return nil
}

func TestDueWorkflowStateTriggerNodesMatchesStateChangeWithinWindow(t *testing.T) {
	workflow := workflowStateTriggerDefinition()
	event := models.Event{
		Type:     models.EventDeviceStateChanged,
		DeviceID: "device-1",
		TS:       time.Date(2026, 4, 28, 9, 30, 0, 0, time.UTC),
		Payload: map[string]any{
			"previous_state": map[string]any{"power": false},
			"state":          map[string]any{"power": true},
		},
	}
	due := dueWorkflowStateTriggerNodes(models.AgentWorkflowSnapshot{Workflows: []models.AgentWorkflow{workflow}}, event, event.TS)
	if got := due[workflow.ID]; len(got) != 1 || got[0] != "state-changed" {
		t.Fatalf("due state triggers = %#v, want state-changed", due)
	}
}

func TestDueWorkflowStateTriggerNodesSkipsStateChangeOutsideWindow(t *testing.T) {
	workflow := workflowStateTriggerDefinition()
	event := models.Event{
		Type:     models.EventDeviceStateChanged,
		DeviceID: "device-1",
		TS:       time.Date(2026, 4, 28, 20, 30, 0, 0, time.UTC),
		Payload: map[string]any{
			"previous_state": map[string]any{"power": false},
			"state":          map[string]any{"power": true},
		},
	}
	due := dueWorkflowStateTriggerNodes(models.AgentWorkflowSnapshot{Workflows: []models.AgentWorkflow{workflow}}, event, event.TS)
	if len(due) != 0 {
		t.Fatalf("due state triggers = %#v, want none", due)
	}
}

func TestDueWorkflowStateTriggerNodesSupportsWindowAttachedToTrigger(t *testing.T) {
	workflow := workflowStateTriggerWindowAttachedDefinition()
	event := models.Event{
		Type:     models.EventDeviceStateChanged,
		DeviceID: "device-1",
		TS:       time.Date(2026, 4, 28, 9, 30, 0, 0, time.UTC),
		Payload: map[string]any{
			"previous_state": map[string]any{"power": false},
			"state":          map[string]any{"power": true},
		},
	}
	due := dueWorkflowStateTriggerNodes(models.AgentWorkflowSnapshot{Workflows: []models.AgentWorkflow{workflow}}, event, event.TS)
	if got := due[workflow.ID]; len(got) != 1 || got[0] != "state-changed" {
		t.Fatalf("due state triggers = %#v, want state-changed", due)
	}

	event.TS = time.Date(2026, 4, 28, 20, 30, 0, 0, time.UTC)
	due = dueWorkflowStateTriggerNodes(models.AgentWorkflowSnapshot{Workflows: []models.AgentWorkflow{workflow}}, event, event.TS)
	if len(due) != 0 {
		t.Fatalf("due state triggers = %#v, want none outside trigger-attached window", due)
	}
}

func TestRunWorkflowExecutesDeviceCommandBehindStateIsGate(t *testing.T) {
	ctx := context.Background()
	svc, _ := newAgentPersistenceTestService(t)
	devices := &workflowTestDeviceRuntime{states: map[string]models.DeviceStateSnapshot{
		"device-1": {
			DeviceID: "device-1",
			PluginID: "xiaomi",
			TS:       time.Date(2026, 4, 28, 9, 0, 0, 0, time.UTC),
			State:    map[string]any{"power": true},
		},
	}}
	svc.SetWorkflowDeviceRuntime(devices)
	workflow := workflowStateIsCommandDefinition()
	if _, err := svc.SaveWorkflow(ctx, models.AgentWorkflowSnapshot{ActiveWorkflowID: workflow.ID, Workflows: []models.AgentWorkflow{workflow}}); err != nil {
		t.Fatalf("SaveWorkflow() error = %v", err)
	}
	run, err := svc.RunWorkflow(ctx, workflow.ID)
	if err != nil {
		t.Fatalf("RunWorkflow() error = %v", err)
	}
	if run.Status != "succeeded" {
		t.Fatalf("workflow run status = %q, want succeeded: %#v", run.Status, run.NodeResults)
	}
	if len(devices.commands) != 1 {
		t.Fatalf("commands = %d, want 1", len(devices.commands))
	}
	if got := devices.commands[0]; got.DeviceID != "device-1" || got.Action != "turn_on" {
		t.Fatalf("command = %#v, want device-1 turn_on", got)
	}
}

func TestRunWorkflowSerialStateIsGatesRequireAllPredicates(t *testing.T) {
	ctx := context.Background()
	svc, _ := newAgentPersistenceTestService(t)
	devices := &workflowTestDeviceRuntime{states: map[string]models.DeviceStateSnapshot{
		"device-1": {
			DeviceID: "device-1",
			PluginID: "xiaomi",
			TS:       time.Date(2026, 4, 28, 9, 0, 0, 0, time.UTC),
			State:    map[string]any{"power": false, "mode": "auto"},
		},
	}}
	svc.SetWorkflowDeviceRuntime(devices)
	workflow := workflowSerialStateGatesDefinition()
	if _, err := svc.SaveWorkflow(ctx, models.AgentWorkflowSnapshot{ActiveWorkflowID: workflow.ID, Workflows: []models.AgentWorkflow{workflow}}); err != nil {
		t.Fatalf("SaveWorkflow() error = %v", err)
	}
	run, err := svc.RunWorkflow(ctx, workflow.ID)
	if err != nil {
		t.Fatalf("RunWorkflow() error = %v", err)
	}
	if run.Status != "succeeded" {
		t.Fatalf("workflow run status = %q, want succeeded: %#v", run.Status, run.NodeResults)
	}
	if len(devices.commands) != 0 {
		t.Fatalf("commands = %d, want 0 when first gate blocks", len(devices.commands))
	}

	devices.states["device-1"] = models.DeviceStateSnapshot{
		DeviceID: "device-1",
		PluginID: "xiaomi",
		TS:       time.Date(2026, 4, 28, 9, 1, 0, 0, time.UTC),
		State:    map[string]any{"power": true, "mode": "auto"},
	}
	run, err = svc.RunWorkflow(ctx, workflow.ID)
	if err != nil {
		t.Fatalf("RunWorkflow() error = %v", err)
	}
	if run.Status != "succeeded" {
		t.Fatalf("workflow run status = %q, want succeeded: %#v", run.Status, run.NodeResults)
	}
	if len(devices.commands) != 1 {
		t.Fatalf("commands = %d, want 1 when both gates match", len(devices.commands))
	}
}

func workflowStateTriggerDefinition() models.AgentWorkflow {
	return models.AgentWorkflow{
		ID:   "workflow-state-trigger",
		Name: "State Trigger",
		Nodes: []models.AgentWorkflowNode{{
			ID:    "state-changed",
			Type:  workflowNodeTypeDeviceStateChanged,
			Label: "State Changed",
			Data: map[string]any{
				"device_id": "device-1",
				"state_key": "power",
				"from":      map[string]any{"operator": "equals", "value": false},
				"to":        map[string]any{"operator": "equals", "value": true},
			},
		}, {
			ID:    "window",
			Type:  workflowNodeTypeTimeWindow,
			Label: "Time Window",
			Data: map[string]any{
				"start":    "08:00",
				"end":      "18:00",
				"timezone": "UTC",
			},
		}, {
			ID:    "command",
			Type:  workflowNodeTypeDeviceCommand,
			Label: "Command",
			Data: map[string]any{
				"device_id": "device-1",
				"action":    "turn_on",
			},
		}},
		Edges: []models.AgentWorkflowEdge{{
			ID:     "edge-state-command",
			Source: "state-changed",
			Target: "command",
		}, {
			ID:     "edge-window-command",
			Source: "window",
			Target: "command",
		}},
	}
}

func workflowStateTriggerWindowAttachedDefinition() models.AgentWorkflow {
	workflow := workflowStateTriggerDefinition()
	workflow.ID = "workflow-state-trigger-window-attached"
	workflow.Edges = []models.AgentWorkflowEdge{{
		ID:           "edge-window-state",
		Source:       "window",
		SourceHandle: "gate",
		Target:       "state-changed",
		TargetHandle: "window",
	}, {
		ID:           "edge-state-command",
		Source:       "state-changed",
		SourceHandle: "trigger",
		Target:       "command",
		TargetHandle: "trigger",
	}}
	return workflow
}

func workflowSerialStateGatesDefinition() models.AgentWorkflow {
	return models.AgentWorkflow{
		ID:   "workflow-serial-state-gates",
		Name: "Serial State Gates",
		Nodes: []models.AgentWorkflowNode{{
			ID:    "power-is",
			Type:  workflowNodeTypeDeviceStateIs,
			Label: "Power Is",
			Data: map[string]any{
				"device_id": "device-1",
				"state_key": "power",
				"match":     map[string]any{"operator": "equals", "value": true},
			},
		}, {
			ID:    "mode-is",
			Type:  workflowNodeTypeDeviceStateIs,
			Label: "Mode Is",
			Data: map[string]any{
				"device_id": "device-1",
				"state_key": "mode",
				"match":     map[string]any{"operator": "equals", "value": "auto"},
			},
		}, {
			ID:    "command",
			Type:  workflowNodeTypeDeviceCommand,
			Label: "Command",
			Data: map[string]any{
				"device_id": "device-1",
				"action":    "turn_on",
			},
		}},
		Edges: []models.AgentWorkflowEdge{{
			ID:     "edge-power-mode",
			Source: "power-is",
			Target: "mode-is",
		}, {
			ID:     "edge-mode-command",
			Source: "mode-is",
			Target: "command",
		}},
	}
}

func workflowStateIsCommandDefinition() models.AgentWorkflow {
	return models.AgentWorkflow{
		ID:   "workflow-state-is-command",
		Name: "State Is Command",
		Nodes: []models.AgentWorkflowNode{{
			ID:    "state-is",
			Type:  workflowNodeTypeDeviceStateIs,
			Label: "State Is",
			Data: map[string]any{
				"device_id": "device-1",
				"state_key": "power",
				"match":     map[string]any{"operator": "equals", "value": true},
			},
		}, {
			ID:    "command",
			Type:  workflowNodeTypeDeviceCommand,
			Label: "Command",
			Data: map[string]any{
				"device_id": "device-1",
				"action":    "turn_on",
			},
		}},
		Edges: []models.AgentWorkflowEdge{{
			ID:     "edge-state-command",
			Source: "state-is",
			Target: "command",
		}},
	}
}
