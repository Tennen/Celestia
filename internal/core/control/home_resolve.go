package control

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

func (s *HomeService) Execute(ctx context.Context, req HomeRequest) (HomeResult, error) {
	scopeRef, commandRef, err := parseHomeCommandRequest(req)
	if err != nil {
		return HomeResult{}, &ValidationError{Err: err}
	}

	action := strings.TrimSpace(req.Action)
	var (
		catalog         homeDeviceCatalog
		resolvedCommand homeResolvedCommand
		params          map[string]any
	)

	switch {
	case action != "":
		catalog, err = s.resolveHomeDeviceCatalog(ctx, req.DeviceID, scopeRef)
		if err != nil {
			return HomeResult{}, err
		}
		params = cloneParams(req.Params)
		resolvedCommand = homeResolvedCommand{
			view: HomeCommand{
				Name: firstNonEmpty(commandRef, action),
			},
		}
	case strings.TrimSpace(req.DeviceID) != "":
		catalog, resolvedCommand, params, action, err = s.resolveHomeCommandForDeviceID(ctx, strings.TrimSpace(req.DeviceID), commandRef, req.Params, req.Values)
		if err != nil {
			return HomeResult{}, err
		}
	case scopeRef != "":
		catalog, resolvedCommand, params, action, err = s.resolveHomeCommandForScope(ctx, scopeRef, commandRef, req.Params, req.Values)
		if err != nil {
			return HomeResult{}, err
		}
	default:
		catalog, resolvedCommand, params, action, err = s.resolveHomeCommandGlobally(ctx, commandRef, req.Params, req.Values)
		if err != nil {
			return HomeResult{}, err
		}
	}

	decision, response, err := s.executeCommand(ctx, actorOrHome(req.Actor), catalog.model, models.CommandRequest{
		DeviceID:  catalog.model.ID,
		Action:    action,
		Params:    params,
		RequestID: uuid.NewString(),
	})
	if err != nil {
		return HomeResult{}, err
	}

	deviceName := catalog.device.Name
	if deviceName == "" {
		deviceName = catalog.model.Name
	}
	commandName := firstNonEmpty(resolvedCommand.view.Name, commandRef, action)
	return HomeResult{
		Device: HomeResolvedDevice{
			ID:   catalog.model.ID,
			Name: deviceName,
		},
		Command: HomeResolvedCommand{
			Name:   commandName,
			Action: action,
			Target: buildHomeTarget(deviceName, commandName),
			Params: params,
		},
		Decision: decision,
		Result:   response,
	}, nil
}

func (s *HomeService) executeCommand(ctx context.Context, actor string, device models.Device, req models.CommandRequest) (models.PolicyDecision, models.CommandResponse, error) {
	if strings.TrimSpace(req.Action) == "" {
		return models.PolicyDecision{}, models.CommandResponse{}, &ValidationError{Err: errors.New("command action is required")}
	}
	if s.executor == nil {
		return models.PolicyDecision{}, models.CommandResponse{}, errors.New("command executor is not available")
	}

	decision := s.policy.Evaluate(actor, req.Action)
	auditRecord := models.AuditRecord{
		ID:        uuid.NewString(),
		Actor:     actor,
		DeviceID:  device.ID,
		Action:    req.Action,
		Params:    cloneParams(req.Params),
		Allowed:   decision.Allowed,
		RiskLevel: decision.RiskLevel,
		CreatedAt: time.Now().UTC(),
	}
	if !decision.Allowed {
		auditRecord.Result = "denied"
		_ = s.audit.Append(ctx, auditRecord)
		return decision, models.CommandResponse{}, &PolicyDeniedError{Decision: decision}
	}

	response, err := s.executor.ExecuteCommand(ctx, device, req)
	if err != nil {
		auditRecord.Result = "failed"
		_ = s.audit.Append(ctx, auditRecord)
		return decision, models.CommandResponse{}, &CommandExecutionError{Err: err}
	}

	auditRecord.Result = "accepted"
	if err := s.audit.Append(ctx, auditRecord); err != nil {
		return decision, models.CommandResponse{}, err
	}
	return decision, response, nil
}

func (s *HomeService) resolveHomeCommandForDeviceID(
	ctx context.Context,
	deviceID string,
	commandRef string,
	input map[string]any,
	values []string,
) (homeDeviceCatalog, homeResolvedCommand, map[string]any, string, error) {
	if commandRef == "" {
		return homeDeviceCatalog{}, homeResolvedCommand{}, nil, "", &ValidationError{Err: errors.New("command, action, or target is required")}
	}
	model, view, err := s.loadDeviceByID(ctx, deviceID)
	if err != nil {
		return homeDeviceCatalog{}, homeResolvedCommand{}, nil, "", err
	}
	catalog := buildHomeDeviceCatalog(model, view, s.controls.Specs(model))
	command, err := resolveHomeCommand(catalog, commandRef)
	if err != nil {
		return homeDeviceCatalog{}, homeResolvedCommand{}, nil, "", err
	}
	action, params, err := command.buildRequest(input, values)
	if err != nil {
		return homeDeviceCatalog{}, homeResolvedCommand{}, nil, "", &ValidationError{Err: err}
	}
	return catalog, command, params, action, nil
}

func (s *HomeService) resolveHomeCommandForScope(
	ctx context.Context,
	scopeRef string,
	commandRef string,
	input map[string]any,
	values []string,
) (homeDeviceCatalog, homeResolvedCommand, map[string]any, string, error) {
	if commandRef == "" {
		return homeDeviceCatalog{}, homeResolvedCommand{}, nil, "", &ValidationError{Err: errors.New("command, action, or target is required")}
	}
	catalogs, err := s.loadDeviceCatalogs(ctx, HomeFilter{})
	if err != nil {
		return homeDeviceCatalog{}, homeResolvedCommand{}, nil, "", err
	}
	target, err := resolveHomeCommandInScope(catalogs, scopeRef, commandRef)
	if err != nil {
		return homeDeviceCatalog{}, homeResolvedCommand{}, nil, "", err
	}
	action, params, err := target.command.buildRequest(input, values)
	if err != nil {
		return homeDeviceCatalog{}, homeResolvedCommand{}, nil, "", &ValidationError{Err: err}
	}
	return target.catalog, target.command, params, action, nil
}

func (s *HomeService) resolveHomeCommandGlobally(
	ctx context.Context,
	commandRef string,
	input map[string]any,
	values []string,
) (homeDeviceCatalog, homeResolvedCommand, map[string]any, string, error) {
	if commandRef == "" {
		return homeDeviceCatalog{}, homeResolvedCommand{}, nil, "", &ValidationError{Err: errors.New("command, action, or target is required")}
	}
	catalogs, err := s.loadDeviceCatalogs(ctx, HomeFilter{})
	if err != nil {
		return homeDeviceCatalog{}, homeResolvedCommand{}, nil, "", err
	}
	target, err := resolveHomeCommandAcrossCatalogs(catalogs, commandRef)
	if err != nil {
		return homeDeviceCatalog{}, homeResolvedCommand{}, nil, "", err
	}
	action, params, err := target.command.buildRequest(input, values)
	if err != nil {
		return homeDeviceCatalog{}, homeResolvedCommand{}, nil, "", &ValidationError{Err: err}
	}
	return target.catalog, target.command, params, action, nil
}

func (s *HomeService) resolveHomeDeviceCatalog(ctx context.Context, deviceID, deviceRef string) (homeDeviceCatalog, error) {
	if trimmed := strings.TrimSpace(deviceID); trimmed != "" {
		model, view, err := s.loadDeviceByID(ctx, trimmed)
		if err != nil {
			return homeDeviceCatalog{}, err
		}
		return buildHomeDeviceCatalog(model, view, s.controls.Specs(model)), nil
	}
	if strings.TrimSpace(deviceRef) == "" {
		return homeDeviceCatalog{}, &ValidationError{Err: errors.New("device_id, device_name, or target is required")}
	}
	catalogs, err := s.loadDeviceCatalogs(ctx, HomeFilter{})
	if err != nil {
		return homeDeviceCatalog{}, err
	}
	return resolveHomeDevice(catalogs, deviceRef)
}

func resolveHomeDevice(catalogs []homeDeviceCatalog, deviceRef string) (homeDeviceCatalog, error) {
	var matches []homeDeviceCatalog
	for _, item := range catalogs {
		if homeMatches(item.device.Name, item.device.Aliases, deviceRef) {
			matches = append(matches, item)
		}
	}
	switch len(matches) {
	case 0:
		return homeDeviceCatalog{}, &NotFoundError{
			Field: "device",
			Value: deviceRef,
			Err:   errors.New("device not found"),
		}
	case 1:
		return matches[0], nil
	default:
		return homeDeviceCatalog{}, ambiguousHomeScopeError("device", deviceRef, matches)
	}
}

func resolveHomeCommandInScope(catalogs []homeDeviceCatalog, scopeRef, commandRef string) (homeResolvedTarget, error) {
	candidates := filterHomeScopeCatalogs(catalogs, scopeRef)
	if len(candidates) == 0 {
		return homeResolvedTarget{}, &NotFoundError{
			Field: "scope",
			Value: scopeRef,
			Err:   errors.New("device or room not found"),
		}
	}
	matches := collectHomeCommandMatches(candidates, commandRef)
	switch len(matches) {
	case 0:
		return homeResolvedTarget{}, &NotFoundError{
			Field: "command",
			Value: commandRef,
			Err:   fmt.Errorf("command %q not found in %q", strings.TrimSpace(commandRef), strings.TrimSpace(scopeRef)),
		}
	case 1:
		return matches[0], nil
	default:
		return homeResolvedTarget{}, ambiguousHomeCommandError("target", buildHomeTarget(scopeRef, commandRef), matches)
	}
}

func resolveHomeCommandAcrossCatalogs(catalogs []homeDeviceCatalog, commandRef string) (homeResolvedTarget, error) {
	matches := collectHomeCommandMatches(catalogs, commandRef)
	switch len(matches) {
	case 0:
		return homeResolvedTarget{}, &NotFoundError{
			Field: "command",
			Value: commandRef,
			Err:   fmt.Errorf("command %q not found", strings.TrimSpace(commandRef)),
		}
	case 1:
		return matches[0], nil
	default:
		return homeResolvedTarget{}, ambiguousHomeCommandError("command", commandRef, matches)
	}
}

func resolveHomeCommand(device homeDeviceCatalog, commandRef string) (homeResolvedCommand, error) {
	var matches []homeResolvedCommand
	for _, item := range device.commands {
		if homeMatches(item.view.Name, item.view.Aliases, commandRef) {
			matches = append(matches, item)
		}
	}
	switch len(matches) {
	case 0:
		return homeResolvedCommand{}, &NotFoundError{
			Field: "command",
			Value: commandRef,
			Err:   fmt.Errorf("command %q not found on device", strings.TrimSpace(commandRef)),
		}
	case 1:
		return matches[0], nil
	default:
		return homeResolvedCommand{}, ambiguousHomeCommandsForDevice(device, commandRef, matches)
	}
}

func parseHomeCommandRequest(req HomeRequest) (string, string, error) {
	scopeRef := strings.TrimSpace(req.DeviceName)
	commandRef := strings.TrimSpace(req.Command)
	target := strings.TrimSpace(req.Target)
	if target == "" {
		return scopeRef, commandRef, nil
	}
	if scope, command, ok := splitHomeQualifiedTarget(target); ok {
		if scopeRef == "" {
			scopeRef = scope
		}
		if commandRef == "" {
			commandRef = command
		}
		return scopeRef, commandRef, nil
	}
	if req.Action == "" && commandRef == "" {
		commandRef = target
		return scopeRef, commandRef, nil
	}
	if scopeRef == "" {
		scopeRef = target
	}
	return scopeRef, commandRef, nil
}

func splitHomeQualifiedTarget(target string) (string, string, bool) {
	trimmed := strings.TrimSpace(target)
	index := strings.LastIndex(trimmed, ".")
	if index <= 0 || index == len(trimmed)-1 {
		return "", "", false
	}
	scopeRef := strings.TrimSpace(trimmed[:index])
	commandRef := strings.TrimSpace(trimmed[index+1:])
	if scopeRef == "" || commandRef == "" {
		return "", "", false
	}
	return scopeRef, commandRef, true
}

func filterHomeScopeCatalogs(catalogs []homeDeviceCatalog, scopeRef string) []homeDeviceCatalog {
	if strings.TrimSpace(scopeRef) == "" {
		return catalogs
	}
	matches := make([]homeDeviceCatalog, 0, len(catalogs))
	for _, item := range catalogs {
		if homeScopeMatches(item, scopeRef) {
			matches = append(matches, item)
		}
	}
	return matches
}

func homeScopeMatches(item homeDeviceCatalog, scopeRef string) bool {
	if homeMatches(item.device.Name, item.device.Aliases, scopeRef) {
		return true
	}
	room := strings.TrimSpace(item.model.Room)
	return room != "" && normalizeHomeRef(room) == normalizeHomeRef(scopeRef)
}

func collectHomeCommandMatches(catalogs []homeDeviceCatalog, commandRef string) []homeResolvedTarget {
	matches := make([]homeResolvedTarget, 0)
	for _, catalog := range catalogs {
		for _, command := range catalog.commands {
			if homeMatches(command.view.Name, command.view.Aliases, commandRef) {
				matches = append(matches, homeResolvedTarget{
					catalog: catalog,
					command: command,
				})
			}
		}
	}
	return matches
}

func ambiguousHomeScopeError(field, value string, catalogs []homeDeviceCatalog) error {
	resolved := make([]HomeResolveMatch, 0, len(catalogs))
	for _, match := range catalogs {
		resolved = append(resolved, HomeResolveMatch{
			DeviceID:   match.device.ID,
			DeviceName: match.device.Name,
			Room:       match.model.Room,
		})
	}
	sort.SliceStable(resolved, func(i, j int) bool {
		if resolved[i].DeviceName != resolved[j].DeviceName {
			return resolved[i].DeviceName < resolved[j].DeviceName
		}
		return resolved[i].DeviceID < resolved[j].DeviceID
	})
	return &AmbiguousReferenceError{
		Field:   field,
		Value:   value,
		Matches: resolved,
	}
}

func ambiguousHomeCommandsForDevice(device homeDeviceCatalog, commandRef string, commands []homeResolvedCommand) error {
	resolved := make([]HomeResolveMatch, 0, len(commands))
	for _, match := range commands {
		resolved = append(resolved, HomeResolveMatch{
			DeviceID:   device.device.ID,
			DeviceName: device.device.Name,
			Room:       device.model.Room,
			Command:    match.view.Name,
			Action:     match.view.Action,
			Target:     buildHomeTarget(device.device.Name, match.view.Name),
		})
	}
	sort.SliceStable(resolved, func(i, j int) bool {
		if resolved[i].Command != resolved[j].Command {
			return resolved[i].Command < resolved[j].Command
		}
		return resolved[i].Action < resolved[j].Action
	})
	return &AmbiguousReferenceError{
		Field:   "command",
		Value:   commandRef,
		Matches: resolved,
	}
}

func ambiguousHomeCommandError(field, value string, matches []homeResolvedTarget) error {
	resolved := make([]HomeResolveMatch, 0, len(matches))
	for _, match := range matches {
		resolved = append(resolved, HomeResolveMatch{
			DeviceID:   match.catalog.device.ID,
			DeviceName: match.catalog.device.Name,
			Room:       match.catalog.model.Room,
			Command:    match.command.view.Name,
			Action:     match.command.view.Action,
			Target:     buildHomeTarget(match.catalog.device.Name, match.command.view.Name),
		})
	}
	sort.SliceStable(resolved, func(i, j int) bool {
		if resolved[i].Target != resolved[j].Target {
			return resolved[i].Target < resolved[j].Target
		}
		return resolved[i].Action < resolved[j].Action
	})
	return &AmbiguousReferenceError{
		Field:   field,
		Value:   value,
		Matches: resolved,
	}
}

func buildHomeTarget(deviceName, commandName string) string {
	if strings.TrimSpace(deviceName) == "" || strings.TrimSpace(commandName) == "" {
		return ""
	}
	return strings.TrimSpace(deviceName) + "." + strings.TrimSpace(commandName)
}

func actorOrHome(actor string) string {
	if trimmed := strings.TrimSpace(actor); trimmed != "" {
		return trimmed
	}
	return "home"
}
