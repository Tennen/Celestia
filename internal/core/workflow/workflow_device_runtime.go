package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/core/audit"
	"github.com/chentianyu/celestia/internal/core/policy"
	"github.com/chentianyu/celestia/internal/core/registry"
	"github.com/chentianyu/celestia/internal/core/state"
	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

type WorkflowDeviceRuntime interface {
	DeviceState(context.Context, string) (models.DeviceStateSnapshot, bool, error)
	ExecuteDeviceCommand(context.Context, string, string, string, map[string]any) error
}

type workflowCommandExecutor interface {
	ExecuteCommand(context.Context, models.Device, models.CommandRequest) (models.CommandResponse, error)
}

type workflowDeviceRuntime struct {
	registry *registry.Service
	state    *state.Service
	policy   *policy.Service
	audit    *audit.Service
	executor workflowCommandExecutor
}

func NewWorkflowDeviceRuntime(
	registrySvc *registry.Service,
	stateSvc *state.Service,
	policySvc *policy.Service,
	auditSvc *audit.Service,
	executor workflowCommandExecutor,
) WorkflowDeviceRuntime {
	return &workflowDeviceRuntime{
		registry: registrySvc,
		state:    stateSvc,
		policy:   policySvc,
		audit:    auditSvc,
		executor: executor,
	}
}

func (s *Service) SetWorkflowDeviceRuntime(runtime WorkflowDeviceRuntime) {
	s.workflowDevices = runtime
}

func (r *workflowDeviceRuntime) DeviceState(ctx context.Context, deviceID string) (models.DeviceStateSnapshot, bool, error) {
	if r == nil || r.state == nil {
		return models.DeviceStateSnapshot{}, false, errors.New("workflow device state runtime is not configured")
	}
	return r.state.Get(ctx, strings.TrimSpace(deviceID))
}

func (r *workflowDeviceRuntime) ExecuteDeviceCommand(ctx context.Context, actor string, deviceID string, action string, params map[string]any) error {
	if r == nil || r.registry == nil || r.policy == nil || r.audit == nil || r.executor == nil {
		return errors.New("workflow device command runtime is not configured")
	}
	device, ok, err := r.registry.Get(ctx, strings.TrimSpace(deviceID))
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("device %q not found", strings.TrimSpace(deviceID))
	}
	action = strings.TrimSpace(action)
	if action == "" {
		return errors.New("device command action is required")
	}
	clonedParams := cloneWorkflowParams(params)
	decision := r.policy.Evaluate(actor, action)
	auditRecord := models.AuditRecord{
		ID:        uuid.NewString(),
		Actor:     actor,
		DeviceID:  device.ID,
		Action:    action,
		Params:    clonedParams,
		Allowed:   decision.Allowed,
		RiskLevel: decision.RiskLevel,
		CreatedAt: time.Now().UTC(),
	}
	if !decision.Allowed {
		auditRecord.Result = "denied"
		_ = r.audit.Append(ctx, auditRecord)
		return fmt.Errorf("action %q denied: %s", action, decision.Reason)
	}
	resp, err := r.executor.ExecuteCommand(ctx, device, models.CommandRequest{
		DeviceID:  device.ID,
		Action:    action,
		Params:    cloneWorkflowParams(params),
		RequestID: uuid.NewString(),
	})
	if err != nil {
		auditRecord.Result = "failed"
		_ = r.audit.Append(ctx, auditRecord)
		return fmt.Errorf("action %q failed: %v", action, err)
	}
	if !resp.Accepted {
		auditRecord.Result = "failed"
		_ = r.audit.Append(ctx, auditRecord)
		return fmt.Errorf("action %q rejected: %s", action, strings.TrimSpace(resp.Message))
	}
	auditRecord.Result = "accepted"
	_ = r.audit.Append(ctx, auditRecord)
	return nil
}

func cloneWorkflowParams(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	raw, err := json.Marshal(src)
	if err != nil {
		out := make(map[string]any, len(src))
		for key, value := range src {
			out[key] = value
		}
		return out
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return map[string]any{}
	}
	return out
}
