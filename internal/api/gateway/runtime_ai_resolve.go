package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/google/uuid"
)

type aiResolvedTarget struct {
	catalog aiDeviceCatalog
	command aiResolvedCommand
}

func (s *RuntimeService) ExecuteAICommand(ctx context.Context, req AICommandRequest) (AICommandResult, error) {
	scopeRef, commandRef, err := parseAICommandRequest(req)
	if err != nil {
		return AICommandResult{}, statusError(http.StatusBadRequest, err)
	}

	action := strings.TrimSpace(req.Action)
	var (
		catalog         aiDeviceCatalog
		resolvedCommand aiResolvedCommand
		params          map[string]any
	)

	switch {
	case action != "":
		catalog, err = s.resolveAIDeviceCatalog(ctx, req.DeviceID, scopeRef)
		if err != nil {
			return AICommandResult{}, err
		}
		params = cloneAIParamsMap(req.Params)
		resolvedCommand = aiResolvedCommand{
			view: AICommand{
				Name: firstNonEmpty(commandRef, action),
			},
		}
	case strings.TrimSpace(req.DeviceID) != "":
		catalog, resolvedCommand, params, action, err = s.resolveAICommandForDeviceID(ctx, strings.TrimSpace(req.DeviceID), commandRef, req.Params)
		if err != nil {
			return AICommandResult{}, err
		}
	case scopeRef != "":
		catalog, resolvedCommand, params, action, err = s.resolveAICommandForScope(ctx, scopeRef, commandRef, req.Params)
		if err != nil {
			return AICommandResult{}, err
		}
	default:
		catalog, resolvedCommand, params, action, err = s.resolveAICommandGlobally(ctx, commandRef, req.Params)
		if err != nil {
			return AICommandResult{}, err
		}
	}

	execResult, err := s.executeDeviceCommand(ctx, actorOrAI(req.Actor), catalog.model, models.CommandRequest{
		DeviceID:  catalog.model.ID,
		Action:    action,
		Params:    params,
		RequestID: uuid.NewString(),
	})
	if err != nil {
		return AICommandResult{}, err
	}

	deviceName := catalog.device.Name
	if deviceName == "" {
		deviceName = catalog.model.Name
	}
	commandName := firstNonEmpty(resolvedCommand.view.Name, commandRef, action)
	return AICommandResult{
		Device: AIResolvedDevice{
			ID:   catalog.model.ID,
			Name: deviceName,
		},
		Command: AIResolvedCommand{
			Name:   commandName,
			Action: action,
			Target: buildAITarget(deviceName, commandName),
			Params: params,
		},
		Decision: execResult.Decision,
		Result:   execResult.Result,
	}, nil
}

func (s *RuntimeService) resolveAICommandForDeviceID(
	ctx context.Context,
	deviceID string,
	commandRef string,
	input map[string]any,
) (aiDeviceCatalog, aiResolvedCommand, map[string]any, string, error) {
	if commandRef == "" {
		return aiDeviceCatalog{}, aiResolvedCommand{}, nil, "", statusError(http.StatusBadRequest, errors.New("command, action, or target is required"))
	}
	model, view, err := s.loadDeviceByID(ctx, deviceID)
	if err != nil {
		return aiDeviceCatalog{}, aiResolvedCommand{}, nil, "", err
	}
	catalog := buildAIDeviceCatalog(model, view, s.runtime.Controls.Specs(model))
	command, err := resolveAICommand(catalog, commandRef)
	if err != nil {
		return aiDeviceCatalog{}, aiResolvedCommand{}, nil, "", err
	}
	action, params, err := command.buildRequest(input)
	if err != nil {
		return aiDeviceCatalog{}, aiResolvedCommand{}, nil, "", statusError(http.StatusBadRequest, err)
	}
	return catalog, command, params, action, nil
}

func (s *RuntimeService) resolveAICommandForScope(
	ctx context.Context,
	scopeRef string,
	commandRef string,
	input map[string]any,
) (aiDeviceCatalog, aiResolvedCommand, map[string]any, string, error) {
	if commandRef == "" {
		return aiDeviceCatalog{}, aiResolvedCommand{}, nil, "", statusError(http.StatusBadRequest, errors.New("command, action, or target is required"))
	}
	catalogs, err := s.loadAIDeviceCatalogs(ctx, DeviceFilter{})
	if err != nil {
		return aiDeviceCatalog{}, aiResolvedCommand{}, nil, "", err
	}
	target, err := resolveAICommandInScope(catalogs, scopeRef, commandRef)
	if err != nil {
		return aiDeviceCatalog{}, aiResolvedCommand{}, nil, "", err
	}
	action, params, err := target.command.buildRequest(input)
	if err != nil {
		return aiDeviceCatalog{}, aiResolvedCommand{}, nil, "", statusError(http.StatusBadRequest, err)
	}
	return target.catalog, target.command, params, action, nil
}

func (s *RuntimeService) resolveAICommandGlobally(
	ctx context.Context,
	commandRef string,
	input map[string]any,
) (aiDeviceCatalog, aiResolvedCommand, map[string]any, string, error) {
	if commandRef == "" {
		return aiDeviceCatalog{}, aiResolvedCommand{}, nil, "", statusError(http.StatusBadRequest, errors.New("command, action, or target is required"))
	}
	catalogs, err := s.loadAIDeviceCatalogs(ctx, DeviceFilter{})
	if err != nil {
		return aiDeviceCatalog{}, aiResolvedCommand{}, nil, "", err
	}
	target, err := resolveAICommandAcrossCatalogs(catalogs, commandRef)
	if err != nil {
		return aiDeviceCatalog{}, aiResolvedCommand{}, nil, "", err
	}
	action, params, err := target.command.buildRequest(input)
	if err != nil {
		return aiDeviceCatalog{}, aiResolvedCommand{}, nil, "", statusError(http.StatusBadRequest, err)
	}
	return target.catalog, target.command, params, action, nil
}

func (s *RuntimeService) resolveAIDeviceCatalog(ctx context.Context, deviceID, deviceRef string) (aiDeviceCatalog, error) {
	if trimmed := strings.TrimSpace(deviceID); trimmed != "" {
		model, view, err := s.loadDeviceByID(ctx, trimmed)
		if err != nil {
			return aiDeviceCatalog{}, err
		}
		return buildAIDeviceCatalog(model, view, s.runtime.Controls.Specs(model)), nil
	}
	if strings.TrimSpace(deviceRef) == "" {
		return aiDeviceCatalog{}, statusError(http.StatusBadRequest, errors.New("device_id, device_name, or target is required"))
	}
	catalogs, err := s.loadAIDeviceCatalogs(ctx, DeviceFilter{})
	if err != nil {
		return aiDeviceCatalog{}, err
	}
	return resolveAIDevice(catalogs, deviceRef)
}

func resolveAIDevice(catalogs []aiDeviceCatalog, deviceRef string) (aiDeviceCatalog, error) {
	var matches []aiDeviceCatalog
	for _, item := range catalogs {
		if aiMatches(item.device.Name, item.device.Aliases, deviceRef) {
			matches = append(matches, item)
		}
	}
	switch len(matches) {
	case 0:
		return aiDeviceCatalog{}, statusError(http.StatusNotFound, errors.New("device not found"))
	case 1:
		return matches[0], nil
	default:
		return aiDeviceCatalog{}, ambiguousAIScopeError("device", deviceRef, matches)
	}
}

func resolveAICommandInScope(catalogs []aiDeviceCatalog, scopeRef, commandRef string) (aiResolvedTarget, error) {
	candidates := filterAIScopeCatalogs(catalogs, scopeRef)
	if len(candidates) == 0 {
		return aiResolvedTarget{}, statusError(http.StatusNotFound, errors.New("device or room not found"))
	}
	matches := collectAICommandMatches(candidates, commandRef)
	switch len(matches) {
	case 0:
		return aiResolvedTarget{}, statusError(http.StatusNotFound, fmt.Errorf("command %q not found in %q", strings.TrimSpace(commandRef), strings.TrimSpace(scopeRef)))
	case 1:
		return matches[0], nil
	default:
		return aiResolvedTarget{}, ambiguousAICommandError("target", buildAITarget(scopeRef, commandRef), matches)
	}
}

func resolveAICommandAcrossCatalogs(catalogs []aiDeviceCatalog, commandRef string) (aiResolvedTarget, error) {
	matches := collectAICommandMatches(catalogs, commandRef)
	switch len(matches) {
	case 0:
		return aiResolvedTarget{}, statusError(http.StatusNotFound, fmt.Errorf("command %q not found", strings.TrimSpace(commandRef)))
	case 1:
		return matches[0], nil
	default:
		return aiResolvedTarget{}, ambiguousAICommandError("command", commandRef, matches)
	}
}

func resolveAICommand(device aiDeviceCatalog, commandRef string) (aiResolvedCommand, error) {
	var matches []aiResolvedCommand
	for _, item := range device.commands {
		if aiMatches(item.view.Name, item.view.Aliases, commandRef) {
			matches = append(matches, item)
		}
	}
	switch len(matches) {
	case 0:
		return aiResolvedCommand{}, statusError(http.StatusNotFound, fmt.Errorf("command %q not found on device", strings.TrimSpace(commandRef)))
	case 1:
		return matches[0], nil
	default:
		return aiResolvedCommand{}, ambiguousAICommandsForDevice(device, commandRef, matches)
	}
}

func parseAICommandRequest(req AICommandRequest) (string, string, error) {
	scopeRef := strings.TrimSpace(req.DeviceName)
	commandRef := strings.TrimSpace(req.Command)
	target := strings.TrimSpace(req.Target)
	if target == "" {
		return scopeRef, commandRef, nil
	}
	if scope, command, ok := splitAIQualifiedTarget(target); ok {
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

func splitAIQualifiedTarget(target string) (string, string, bool) {
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

func filterAIScopeCatalogs(catalogs []aiDeviceCatalog, scopeRef string) []aiDeviceCatalog {
	if strings.TrimSpace(scopeRef) == "" {
		return catalogs
	}
	matches := make([]aiDeviceCatalog, 0, len(catalogs))
	for _, item := range catalogs {
		if aiScopeMatches(item, scopeRef) {
			matches = append(matches, item)
		}
	}
	return matches
}

func aiScopeMatches(item aiDeviceCatalog, scopeRef string) bool {
	if aiMatches(item.device.Name, item.device.Aliases, scopeRef) {
		return true
	}
	room := strings.TrimSpace(item.model.Room)
	return room != "" && normalizeAIRef(room) == normalizeAIRef(scopeRef)
}

func collectAICommandMatches(catalogs []aiDeviceCatalog, commandRef string) []aiResolvedTarget {
	matches := make([]aiResolvedTarget, 0)
	for _, catalog := range catalogs {
		for _, command := range catalog.commands {
			if aiMatches(command.view.Name, command.view.Aliases, commandRef) {
				matches = append(matches, aiResolvedTarget{
					catalog: catalog,
					command: command,
				})
			}
		}
	}
	return matches
}

func ambiguousAIScopeError(field, value string, catalogs []aiDeviceCatalog) error {
	resolved := make([]AIResolveMatch, 0, len(catalogs))
	for _, match := range catalogs {
		resolved = append(resolved, AIResolveMatch{
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
	return &StatusError{
		StatusCode: http.StatusConflict,
		Err: &AmbiguousReferenceError{
			Field:   field,
			Value:   value,
			Matches: resolved,
		},
	}
}

func ambiguousAICommandsForDevice(device aiDeviceCatalog, commandRef string, commands []aiResolvedCommand) error {
	resolved := make([]AIResolveMatch, 0, len(commands))
	for _, match := range commands {
		resolved = append(resolved, AIResolveMatch{
			DeviceID:   device.device.ID,
			DeviceName: device.device.Name,
			Room:       device.model.Room,
			Command:    match.view.Name,
			Action:     match.view.Action,
			Target:     buildAITarget(device.device.Name, match.view.Name),
		})
	}
	sort.SliceStable(resolved, func(i, j int) bool {
		if resolved[i].Command != resolved[j].Command {
			return resolved[i].Command < resolved[j].Command
		}
		return resolved[i].Action < resolved[j].Action
	})
	return &StatusError{
		StatusCode: http.StatusConflict,
		Err: &AmbiguousReferenceError{
			Field:   "command",
			Value:   commandRef,
			Matches: resolved,
		},
	}
}

func ambiguousAICommandError(field, value string, matches []aiResolvedTarget) error {
	resolved := make([]AIResolveMatch, 0, len(matches))
	for _, match := range matches {
		resolved = append(resolved, AIResolveMatch{
			DeviceID:   match.catalog.device.ID,
			DeviceName: match.catalog.device.Name,
			Room:       match.catalog.model.Room,
			Command:    match.command.view.Name,
			Action:     match.command.view.Action,
			Target:     buildAITarget(match.catalog.device.Name, match.command.view.Name),
		})
	}
	sort.SliceStable(resolved, func(i, j int) bool {
		if resolved[i].Target != resolved[j].Target {
			return resolved[i].Target < resolved[j].Target
		}
		return resolved[i].Action < resolved[j].Action
	})
	return &StatusError{
		StatusCode: http.StatusConflict,
		Err: &AmbiguousReferenceError{
			Field:   field,
			Value:   value,
			Matches: resolved,
		},
	}
}

func buildAITarget(deviceName, commandName string) string {
	if strings.TrimSpace(deviceName) == "" || strings.TrimSpace(commandName) == "" {
		return ""
	}
	return strings.TrimSpace(deviceName) + "." + strings.TrimSpace(commandName)
}

func actorOrAI(actor string) string {
	if trimmed := strings.TrimSpace(actor); trimmed != "" {
		return trimmed
	}
	return "ai"
}
