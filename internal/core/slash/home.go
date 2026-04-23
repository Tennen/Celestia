package slash

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/storage"
	"github.com/google/uuid"
)

func (s *Service) runHome(ctx context.Context, req models.ProjectInputRequest, args []string) (string, map[string]any, error) {
	if len(args) == 0 || equalRef(args[0], "list") {
		query := ""
		if len(args) > 1 {
			query = strings.Join(args[1:], " ")
		}
		output, count, err := s.homeList(ctx, query)
		return output, map[string]any{"domain": "home", "action": "list", "count": count}, err
	}
	if equalRef(args[0], "help") {
		return homeHelp(), map[string]any{"domain": "home", "action": "help"}, nil
	}
	if equalRef(args[0], "action") {
		if len(args) < 3 {
			return "", map[string]any{"domain": "home", "action": "direct"}, errors.New("usage: /home action <device> <action> [key=value ...]")
		}
		params, _, err := parseSlashParams(args[3:])
		if err != nil {
			return "", map[string]any{"domain": "home", "action": "direct"}, err
		}
		output, metadata, err := s.executeDirectDeviceAction(ctx, req, args[1], args[2], params)
		return output, metadata, err
	}
	if len(args) < 2 {
		return "", map[string]any{"domain": "home", "action": "control"}, errors.New("usage: /home <device> <command> [value|key=value ...]")
	}
	output, metadata, err := s.executeHomeControl(ctx, req, args[0], args[1], args[2:])
	return output, metadata, err
}

func (s *Service) homeList(ctx context.Context, query string) (string, int, error) {
	devices, err := s.registry.List(ctx, storage.DeviceFilter{Query: strings.TrimSpace(query)})
	if err != nil {
		return "", 0, err
	}
	sort.SliceStable(devices, func(i, j int) bool { return devices[i].Name < devices[j].Name })
	lines := []string{"Home devices:"}
	for _, device := range devices {
		_, view, err := s.deviceView(ctx, device)
		if err != nil {
			return "", 0, err
		}
		controls := make([]string, 0, len(view.Controls))
		for _, control := range view.Controls {
			if control.Disabled || !control.Visible {
				continue
			}
			controls = append(controls, firstNonEmpty(control.Label, control.ID))
		}
		stateText := "offline"
		if view.Device.Online {
			stateText = "online"
		}
		line := fmt.Sprintf("- %s (%s, %s)", firstNonEmpty(view.Device.Name, view.Device.ID), view.Device.ID, stateText)
		if len(controls) > 0 {
			line += " controls: " + strings.Join(controls, ", ")
		}
		lines = append(lines, line)
	}
	if len(devices) == 0 {
		lines = append(lines, "- no devices matched")
	}
	return strings.Join(lines, "\n"), len(devices), nil
}

func (s *Service) executeDirectDeviceAction(
	ctx context.Context,
	req models.ProjectInputRequest,
	deviceRef string,
	action string,
	params map[string]any,
) (string, map[string]any, error) {
	device, view, err := s.resolveDevice(ctx, deviceRef)
	if err != nil {
		return "", map[string]any{"domain": "home", "action": "direct"}, err
	}
	response, err := s.executeCommand(ctx, actorOrInput(req), device, models.CommandRequest{
		DeviceID:  device.ID,
		Action:    strings.TrimSpace(action),
		Params:    cloneMap(params),
		RequestID: uuid.NewString(),
	})
	metadata := map[string]any{
		"domain":      "home",
		"action":      "direct",
		"device_id":   device.ID,
		"device_name": firstNonEmpty(view.Device.Name, device.Name, device.ID),
		"command":     strings.TrimSpace(action),
		"accepted":    response.Accepted,
	}
	if err != nil {
		return "", metadata, err
	}
	return fmt.Sprintf("Home command accepted: %s / %s", firstNonEmpty(view.Device.Name, device.Name, device.ID), action), metadata, nil
}

func (s *Service) executeHomeControl(ctx context.Context, req models.ProjectInputRequest, deviceRef string, commandRef string, argText []string) (string, map[string]any, error) {
	device, view, err := s.resolveDevice(ctx, deviceRef)
	if err != nil {
		return "", map[string]any{"domain": "home", "action": "control"}, err
	}
	params, values, err := parseSlashParams(argText)
	if err != nil {
		return "", map[string]any{"domain": "home", "action": "control", "device_id": device.ID}, err
	}
	commandReq, controlName, err := s.resolveControlCommand(device, view, commandRef, params, values)
	if err != nil {
		return "", map[string]any{"domain": "home", "action": "control", "device_id": device.ID}, err
	}
	commandReq.DeviceID = device.ID
	commandReq.RequestID = uuid.NewString()
	response, err := s.executeCommand(ctx, actorOrInput(req), device, commandReq)
	metadata := map[string]any{
		"domain":       "home",
		"action":       "control",
		"device_id":    device.ID,
		"device_name":  firstNonEmpty(view.Device.Name, device.Name, device.ID),
		"control_name": controlName,
		"command":      commandReq.Action,
		"accepted":     response.Accepted,
	}
	if err != nil {
		return "", metadata, err
	}
	return fmt.Sprintf("Home command accepted: %s / %s", firstNonEmpty(view.Device.Name, device.Name, device.ID), controlName), metadata, nil
}

func (s *Service) resolveControlCommand(
	device models.Device,
	view models.DeviceView,
	commandRef string,
	params map[string]any,
	values []string,
) (models.CommandRequest, string, error) {
	if on, ok := boolCommandRef(commandRef); ok {
		toggle, err := singleToggle(view.Controls)
		if err != nil {
			return models.CommandRequest{}, "", err
		}
		req, err := s.controls.ResolveToggle(device, view.State, toggle.ID, on)
		return req, firstNonEmpty(toggle.Label, toggle.ID), err
	}
	control, ok := findControl(view.Controls, commandRef)
	if !ok {
		return models.CommandRequest{}, "", fmt.Errorf("control %q not found on %s", strings.TrimSpace(commandRef), firstNonEmpty(view.Device.Name, device.ID))
	}
	if control.Disabled {
		return models.CommandRequest{}, "", fmt.Errorf("control %q is disabled", firstNonEmpty(control.Label, control.ID))
	}
	switch control.Kind {
	case models.DeviceControlKindToggle:
		on, ok := boolParam(params, "on")
		if !ok && len(values) > 0 {
			on, ok = parseBoolWord(values[0])
		}
		if !ok {
			return models.CommandRequest{}, "", fmt.Errorf("toggle %q requires on/off", firstNonEmpty(control.Label, control.ID))
		}
		req, err := s.controls.ResolveToggle(device, view.State, control.ID, on)
		return req, firstNonEmpty(control.Label, control.ID), err
	case models.DeviceControlKindAction:
		req, err := s.controls.ResolveAction(device, view.State, control.ID)
		for key, value := range params {
			if req.Params == nil {
				req.Params = map[string]any{}
			}
			req.Params[key] = value
		}
		return req, firstNonEmpty(control.Label, control.ID), err
	case models.DeviceControlKindSelect, models.DeviceControlKindNumber:
		if control.Command == nil || strings.TrimSpace(control.Command.Action) == "" {
			return models.CommandRequest{}, "", fmt.Errorf("control %q is not executable", firstNonEmpty(control.Label, control.ID))
		}
		out := models.CommandRequest{
			DeviceID: device.ID,
			Action:   strings.TrimSpace(control.Command.Action),
			Params:   cloneMap(control.Command.Params),
		}
		valueParam := strings.TrimSpace(control.Command.ValueParam)
		if valueParam != "" && len(values) > 0 {
			out.Params[valueParam] = coerceSlashValue(values[0])
		}
		for key, value := range params {
			out.Params[key] = value
		}
		if valueParam != "" {
			if _, ok := out.Params[valueParam]; !ok {
				return models.CommandRequest{}, "", fmt.Errorf("control %q requires %s", firstNonEmpty(control.Label, control.ID), valueParam)
			}
		}
		return out, firstNonEmpty(control.Label, control.ID), nil
	default:
		return models.CommandRequest{}, "", fmt.Errorf("unsupported control kind %q", control.Kind)
	}
}

func (s *Service) resolveDevice(ctx context.Context, ref string) (models.Device, models.DeviceView, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return models.Device{}, models.DeviceView{}, errors.New("device is required")
	}
	devices, err := s.registry.List(ctx, storage.DeviceFilter{})
	if err != nil {
		return models.Device{}, models.DeviceView{}, err
	}
	matches := []models.Device{}
	for _, device := range devices {
		viewDevice := device
		if pref, ok, err := s.store.GetDevicePreference(ctx, device.ID); err != nil {
			return models.Device{}, models.DeviceView{}, err
		} else if ok && strings.TrimSpace(pref.Alias) != "" {
			viewDevice.Alias = pref.Alias
			viewDevice.Name = pref.Alias
		}
		if deviceMatches(viewDevice, ref) {
			matches = append(matches, device)
		}
	}
	switch len(matches) {
	case 0:
		return models.Device{}, models.DeviceView{}, fmt.Errorf("device %q not found", ref)
	case 1:
		return s.deviceView(ctx, matches[0])
	default:
		names := make([]string, 0, len(matches))
		for _, device := range matches {
			names = append(names, firstNonEmpty(device.Name, device.ID)+" ("+device.ID+")")
		}
		return models.Device{}, models.DeviceView{}, fmt.Errorf("device %q is ambiguous: %s", ref, strings.Join(names, ", "))
	}
}

func (s *Service) deviceView(ctx context.Context, device models.Device) (models.Device, models.DeviceView, error) {
	stateSnapshot, _, err := s.state.Get(ctx, device.ID)
	if err != nil {
		return models.Device{}, models.DeviceView{}, err
	}
	view := s.controls.BuildView(device, stateSnapshot)
	if pref, ok, err := s.store.GetDevicePreference(ctx, device.ID); err != nil {
		return models.Device{}, models.DeviceView{}, err
	} else if ok && strings.TrimSpace(pref.Alias) != "" {
		view.Device.DefaultName = view.Device.Name
		view.Device.Alias = pref.Alias
		view.Device.Name = pref.Alias
	}
	prefs, err := s.store.ListDeviceControlPreferences(ctx, device.ID)
	if err != nil {
		return models.Device{}, models.DeviceView{}, err
	}
	return device, s.controls.ApplyPreferences(view, prefs), nil
}

func (s *Service) executeCommand(ctx context.Context, actor string, device models.Device, req models.CommandRequest) (models.CommandResponse, error) {
	if strings.TrimSpace(req.Action) == "" {
		return models.CommandResponse{}, errors.New("command action is required")
	}
	decision := s.policy.Evaluate(actor, req.Action)
	auditRecord := models.AuditRecord{
		ID:        uuid.NewString(),
		Actor:     actor,
		DeviceID:  device.ID,
		Action:    req.Action,
		Params:    cloneMap(req.Params),
		Allowed:   decision.Allowed,
		RiskLevel: decision.RiskLevel,
		CreatedAt: time.Now().UTC(),
	}
	if !decision.Allowed {
		auditRecord.Result = "denied"
		_ = s.audit.Append(ctx, auditRecord)
		return models.CommandResponse{}, fmt.Errorf("command %q denied: %s", req.Action, decision.Reason)
	}
	if s.executor == nil {
		return models.CommandResponse{}, errors.New("command executor is not available")
	}
	resp, err := s.executor.ExecuteCommand(ctx, device, req)
	if err != nil {
		auditRecord.Result = "failed"
		_ = s.audit.Append(ctx, auditRecord)
		return resp, fmt.Errorf("command %q failed: %w", req.Action, err)
	}
	if !resp.Accepted {
		auditRecord.Result = "failed"
		_ = s.audit.Append(ctx, auditRecord)
		return resp, fmt.Errorf("command %q rejected: %s", req.Action, strings.TrimSpace(resp.Message))
	}
	auditRecord.Result = "accepted"
	_ = s.audit.Append(ctx, auditRecord)
	return resp, nil
}
