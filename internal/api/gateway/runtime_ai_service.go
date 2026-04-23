package gateway

import (
	"context"
	"errors"
	"net/http"

	corecontrol "github.com/chentianyu/celestia/internal/core/control"
)

func (s *RuntimeService) ListAIDevices(ctx context.Context, filter DeviceFilter) ([]AIDevice, error) {
	items, err := s.runtime.Home.ListCatalog(ctx, corecontrol.HomeFilter{
		PluginID: filter.PluginID,
		Kind:     filter.Kind,
		Query:    filter.Query,
	})
	if err != nil {
		return nil, mapHomeError(err)
	}
	out := make([]AIDevice, 0, len(items))
	for _, item := range items {
		out = append(out, toAIDevice(item))
	}
	return out, nil
}

func (s *RuntimeService) ExecuteAICommand(ctx context.Context, req AICommandRequest) (AICommandResult, error) {
	result, err := s.runtime.Home.Execute(ctx, corecontrol.HomeRequest{
		Target:     req.Target,
		DeviceID:   req.DeviceID,
		DeviceName: req.DeviceName,
		Command:    req.Command,
		Action:     req.Action,
		Actor:      req.Actor,
		Params:     req.Params,
		Values:     req.Values,
	})
	if err != nil {
		return AICommandResult{}, mapHomeError(err)
	}
	return AICommandResult{
		Device: AIResolvedDevice{
			ID:   result.Device.ID,
			Name: result.Device.Name,
		},
		Command: AIResolvedCommand{
			Name:   result.Command.Name,
			Action: result.Command.Action,
			Target: result.Command.Target,
			Params: result.Command.Params,
		},
		Decision: result.Decision,
		Result:   result.Result,
	}, nil
}

func toAIDevice(item corecontrol.HomeDevice) AIDevice {
	out := AIDevice{
		ID:      item.ID,
		Name:    item.Name,
		Aliases: append([]string{}, item.Aliases...),
	}
	for _, command := range item.Commands {
		out.Commands = append(out.Commands, AICommand{
			Name:     command.Name,
			Aliases:  append([]string{}, command.Aliases...),
			Action:   command.Action,
			Params:   toAICommandParams(command.Params),
			Defaults: cloneAIParamsMap(command.Defaults),
		})
	}
	return out
}

func toAICommandParams(input []corecontrol.HomeCommandParam) []AICommandParam {
	if len(input) == 0 {
		return nil
	}
	out := make([]AICommandParam, 0, len(input))
	for _, item := range input {
		out = append(out, AICommandParam{
			Name:     item.Name,
			Type:     item.Type,
			Required: item.Required,
			Default:  item.Default,
			Options:  cloneAIOptions(item.Options),
			Min:      cloneAINumber(item.Min),
			Max:      cloneAINumber(item.Max),
			Step:     cloneAINumber(item.Step),
			Unit:     item.Unit,
		})
	}
	return out
}

func mapHomeError(err error) error {
	if err == nil {
		return nil
	}

	var invalid *corecontrol.ValidationError
	if errors.As(err, &invalid) {
		return statusError(http.StatusBadRequest, invalid)
	}

	var missing *corecontrol.NotFoundError
	if errors.As(err, &missing) {
		return &StatusError{
			StatusCode: http.StatusNotFound,
			Err: &ReferenceNotFoundError{
				Field: missing.Field,
				Value: missing.Value,
				Err:   missing,
			},
		}
	}

	var ambiguous *corecontrol.AmbiguousReferenceError
	if errors.As(err, &ambiguous) {
		return &StatusError{
			StatusCode: http.StatusConflict,
			Err: &AmbiguousReferenceError{
				Field:   ambiguous.Field,
				Value:   ambiguous.Value,
				Matches: toAIResolveMatches(ambiguous.Matches),
			},
		}
	}

	var denied *corecontrol.PolicyDeniedError
	if errors.As(err, &denied) {
		return &StatusError{
			StatusCode: http.StatusForbidden,
			Err:        &PolicyDeniedError{Decision: denied.Decision},
		}
	}

	var execution *corecontrol.CommandExecutionError
	if errors.As(err, &execution) {
		return statusError(http.StatusBadGateway, execution)
	}

	return statusError(http.StatusInternalServerError, err)
}

func toAIResolveMatches(input []corecontrol.HomeResolveMatch) []AIResolveMatch {
	if len(input) == 0 {
		return nil
	}
	out := make([]AIResolveMatch, 0, len(input))
	for _, item := range input {
		out = append(out, AIResolveMatch{
			DeviceID:   item.DeviceID,
			DeviceName: item.DeviceName,
			Room:       item.Room,
			Command:    item.Command,
			Action:     item.Action,
			Target:     item.Target,
		})
	}
	return out
}
