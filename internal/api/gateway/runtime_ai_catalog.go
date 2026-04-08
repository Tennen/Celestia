package gateway

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
	"github.com/chentianyu/celestia/internal/storage"
)

type aiDeviceCatalog struct {
	device   AIDevice
	model    models.Device
	commands []aiResolvedCommand
}

type aiResolvedCommand struct {
	view       AICommand
	kind       models.DeviceControlKind
	command    *models.DeviceControlCommand
	onCommand  *models.DeviceControlCommand
	offCommand *models.DeviceControlCommand
}

func (s *RuntimeService) ListAIDevices(ctx context.Context, filter DeviceFilter) ([]AIDevice, error) {
	catalogs, err := s.loadAIDeviceCatalogs(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]AIDevice, 0, len(catalogs))
	for _, item := range catalogs {
		out = append(out, item.device)
	}
	return out, nil
}

func (s *RuntimeService) loadAIDeviceCatalogs(ctx context.Context, filter DeviceFilter) ([]aiDeviceCatalog, error) {
	devices, views, err := s.loadDeviceViews(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]aiDeviceCatalog, 0, len(devices))
	for _, device := range devices {
		view, ok := views[device.ID]
		if !ok {
			continue
		}
		out = append(out, buildAIDeviceCatalog(device, view, s.runtime.Controls.Specs(device)))
	}
	return out, nil
}

func (s *RuntimeService) loadDeviceByID(ctx context.Context, deviceID string) (models.Device, models.DeviceView, error) {
	device, ok, err := s.runtime.Registry.Get(ctx, deviceID)
	if err != nil {
		return models.Device{}, models.DeviceView{}, statusError(http.StatusInternalServerError, err)
	}
	if !ok {
		return models.Device{}, models.DeviceView{}, statusError(http.StatusNotFound, errors.New("device not found"))
	}
	state, _, err := s.runtime.State.Get(ctx, device.ID)
	if err != nil {
		return models.Device{}, models.DeviceView{}, statusError(http.StatusInternalServerError, err)
	}
	view, err := s.deviceView(ctx, device, state)
	if err != nil {
		return models.Device{}, models.DeviceView{}, statusError(http.StatusInternalServerError, err)
	}
	return device, view, nil
}

func (s *RuntimeService) loadDeviceViews(ctx context.Context, filter DeviceFilter) ([]models.Device, map[string]models.DeviceView, error) {
	devices, err := s.runtime.Registry.List(ctx, storage.DeviceFilter{
		PluginID: filter.PluginID,
		Kind:     filter.Kind,
		Query:    filter.Query,
	})
	if err != nil {
		return nil, nil, statusError(http.StatusInternalServerError, err)
	}
	states, err := s.runtime.State.List(ctx, storage.StateFilter{})
	if err != nil {
		return nil, nil, statusError(http.StatusInternalServerError, err)
	}
	stateMap := make(map[string]models.DeviceStateSnapshot, len(states))
	for _, item := range states {
		stateMap[item.DeviceID] = item
	}
	views := make(map[string]models.DeviceView, len(devices))
	for _, device := range devices {
		view, err := s.deviceView(ctx, device, stateMap[device.ID])
		if err != nil {
			return nil, nil, statusError(http.StatusInternalServerError, err)
		}
		views[device.ID] = view
	}
	return devices, views, nil
}

func buildAIDeviceCatalog(device models.Device, view models.DeviceView, specs []models.DeviceControlSpec) aiDeviceCatalog {
	actionCounts := make(map[string]int)
	for _, spec := range specs {
		for action := range aiCommandActions(spec) {
			actionCounts[action]++
		}
	}

	controlByID := make(map[string]models.DeviceControl, len(view.Controls))
	for _, control := range view.Controls {
		controlByID[control.ID] = control
	}

	commands := make([]aiResolvedCommand, 0, len(specs))
	for _, spec := range specs {
		control, ok := controlByID[spec.ID]
		if !ok {
			continue
		}
		command, ok := buildAICommand(control, spec, actionCounts)
		if !ok {
			continue
		}
		commands = append(commands, command)
	}

	sort.SliceStable(commands, func(i, j int) bool {
		if commands[i].view.Name != commands[j].view.Name {
			return commands[i].view.Name < commands[j].view.Name
		}
		return commands[i].view.Action < commands[j].view.Action
	})

	deviceAliases := make([]string, 0, 2)
	deviceAliases = appendUniqueAIName(deviceAliases, view.Device.DefaultName)
	if strings.TrimSpace(device.Name) != strings.TrimSpace(view.Device.Name) {
		deviceAliases = appendUniqueAIName(deviceAliases, device.Name)
	}

	deviceView := AIDevice{
		ID:      device.ID,
		Name:    firstNonEmpty(view.Device.Name, device.Name, device.ID),
		Aliases: deviceAliases,
	}
	for _, command := range commands {
		deviceView.Commands = append(deviceView.Commands, command.view)
	}
	return aiDeviceCatalog{
		device:   deviceView,
		model:    device,
		commands: commands,
	}
}

func buildAICommand(control models.DeviceControl, spec models.DeviceControlSpec, actionCounts map[string]int) (aiResolvedCommand, bool) {
	if control.Disabled || spec.Disabled {
		return aiResolvedCommand{}, false
	}
	commandView := AICommand{
		Name:    firstNonEmpty(control.Label, spec.Label, spec.ID),
		Aliases: aiCommandAliases(control, spec, actionCounts),
	}
	switch spec.Kind {
	case models.DeviceControlKindToggle:
		if spec.OnCommand == nil || spec.OffCommand == nil {
			return aiResolvedCommand{}, false
		}
		if spec.OnCommand.Action == spec.OffCommand.Action {
			commandView.Action = strings.TrimSpace(spec.OnCommand.Action)
		}
		commandView.Params = []AICommandParam{{
			Name:     "on",
			Type:     models.DeviceCommandParamTypeBoolean,
			Required: true,
		}}
		return aiResolvedCommand{
			view:       commandView,
			kind:       spec.Kind,
			onCommand:  cloneAIControlCommand(spec.OnCommand),
			offCommand: cloneAIControlCommand(spec.OffCommand),
		}, true
	case models.DeviceControlKindAction:
		if spec.Command == nil || strings.TrimSpace(spec.Command.Action) == "" {
			return aiResolvedCommand{}, false
		}
		commandView.Action = strings.TrimSpace(spec.Command.Action)
		commandView.Params = cloneAICommandParams(spec.Command.ParamsSpec)
		commandView.Defaults = aiPublicDefaults(spec.Command.Params, commandView.Params)
		return aiResolvedCommand{
			view:    commandView,
			kind:    spec.Kind,
			command: cloneAIControlCommand(spec.Command),
		}, true
	case models.DeviceControlKindSelect:
		if spec.Command == nil || strings.TrimSpace(spec.Command.Action) == "" || strings.TrimSpace(spec.Command.ValueParam) == "" {
			return aiResolvedCommand{}, false
		}
		commandView.Action = strings.TrimSpace(spec.Command.Action)
		commandView.Params = []AICommandParam{{
			Name:     strings.TrimSpace(spec.Command.ValueParam),
			Type:     models.DeviceCommandParamTypeString,
			Required: true,
			Options:  cloneAIOptions(spec.Options),
		}}
		return aiResolvedCommand{
			view:    commandView,
			kind:    spec.Kind,
			command: cloneAIControlCommand(spec.Command),
		}, true
	case models.DeviceControlKindNumber:
		if spec.Command == nil || strings.TrimSpace(spec.Command.Action) == "" || strings.TrimSpace(spec.Command.ValueParam) == "" {
			return aiResolvedCommand{}, false
		}
		commandView.Action = strings.TrimSpace(spec.Command.Action)
		commandView.Params = []AICommandParam{{
			Name:     strings.TrimSpace(spec.Command.ValueParam),
			Type:     models.DeviceCommandParamTypeNumber,
			Required: true,
			Min:      cloneAINumber(spec.Min),
			Max:      cloneAINumber(spec.Max),
			Step:     cloneAINumber(spec.Step),
			Unit:     spec.Unit,
		}}
		return aiResolvedCommand{
			view:    commandView,
			kind:    spec.Kind,
			command: cloneAIControlCommand(spec.Command),
		}, true
	default:
		return aiResolvedCommand{}, false
	}
}

func aiCommandAliases(control models.DeviceControl, spec models.DeviceControlSpec, actionCounts map[string]int) []string {
	aliases := make([]string, 0, 4)
	aliases = appendUniqueAIName(aliases, control.DefaultLabel)
	aliases = appendUniqueAIName(aliases, spec.ID)
	for action := range aiCommandActions(spec) {
		if actionCounts[action] == 1 {
			aliases = appendUniqueAIName(aliases, action)
		}
	}
	return aliases
}

func aiCommandActions(spec models.DeviceControlSpec) map[string]struct{} {
	out := map[string]struct{}{}
	if spec.Command != nil && strings.TrimSpace(spec.Command.Action) != "" {
		out[strings.TrimSpace(spec.Command.Action)] = struct{}{}
	}
	if spec.OnCommand != nil && strings.TrimSpace(spec.OnCommand.Action) != "" {
		out[strings.TrimSpace(spec.OnCommand.Action)] = struct{}{}
	}
	if spec.OffCommand != nil && strings.TrimSpace(spec.OffCommand.Action) != "" {
		out[strings.TrimSpace(spec.OffCommand.Action)] = struct{}{}
	}
	return out
}
