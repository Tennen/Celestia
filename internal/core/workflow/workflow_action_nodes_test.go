package workflow

import (
	"context"
	"testing"

	"github.com/chentianyu/celestia/internal/models"
)

func TestRunWorkflowDeviceCommandSubstitutesInputParam(t *testing.T) {
	ctx := context.Background()
	svc, _ := newWorkflowPersistenceTestService(t)
	devices := &workflowTestDeviceRuntime{states: map[string]models.DeviceStateSnapshot{}}
	svc.SetWorkflowDeviceRuntime(devices)
	workflow := models.AgentWorkflow{
		ID:   "workflow-device-command-input",
		Name: "Device Command Input",
		Nodes: []models.AgentWorkflowNode{{
			ID:    "text-main",
			Type:  workflowNodeTypeText,
			Label: "Text",
			Data: map[string]any{
				"text": "今天有雨，记得关窗。",
			},
		}, {
			ID:    "command-main",
			Type:  workflowNodeTypeDeviceCommand,
			Label: "Device Command",
			Data: map[string]any{
				"device_id": "device-1",
				"action":    "push_voice_message",
				"params": map[string]any{
					"message": "提醒：${input}",
					"nested": map[string]any{
						"body": "${input}",
					},
					"items": []any{"播报 ${input}", "unchanged"},
				},
			},
		}},
		Edges: []models.AgentWorkflowEdge{{
			ID:           "edge-text-command",
			Source:       "text-main",
			SourceHandle: "text",
			Target:       "command-main",
			TargetHandle: "trigger",
		}},
	}
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
	params := devices.commands[0].Params
	if got := params["message"]; got != "提醒：今天有雨，记得关窗。" {
		t.Fatalf("message param = %#v", got)
	}
	nested, ok := params["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested param = %#v", params["nested"])
	}
	if got := nested["body"]; got != "今天有雨，记得关窗。" {
		t.Fatalf("nested body = %#v", got)
	}
	items, ok := params["items"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("items param = %#v", params["items"])
	}
	if got := items[0]; got != "播报 今天有雨，记得关窗。" {
		t.Fatalf("items[0] = %#v", got)
	}
}
