package control

import (
	"sort"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

func buildHomeDeviceCatalog(device models.Device, view models.DeviceView, specs []models.DeviceControlSpec) homeDeviceCatalog {
	actionCounts := make(map[string]int)
	for _, spec := range specs {
		for action := range homeCommandActions(spec) {
			actionCounts[action]++
		}
	}

	controlByID := make(map[string]models.DeviceControl, len(view.Controls))
	for _, control := range view.Controls {
		controlByID[control.ID] = control
	}

	commands := make([]homeResolvedCommand, 0, len(specs))
	for _, spec := range specs {
		control, ok := controlByID[spec.ID]
		if !ok {
			continue
		}
		command, ok := buildHomeCommand(control, spec, actionCounts)
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
	deviceAliases = appendUniqueHomeName(deviceAliases, view.Device.DefaultName)
	if strings.TrimSpace(device.Name) != strings.TrimSpace(view.Device.Name) {
		deviceAliases = appendUniqueHomeName(deviceAliases, device.Name)
	}

	deviceView := HomeDevice{
		ID:      device.ID,
		Name:    firstNonEmpty(view.Device.Name, device.Name, device.ID),
		Aliases: deviceAliases,
	}
	for _, command := range commands {
		deviceView.Commands = append(deviceView.Commands, command.view)
	}
	return homeDeviceCatalog{
		device:   deviceView,
		model:    device,
		commands: commands,
	}
}

func buildHomeCommand(control models.DeviceControl, spec models.DeviceControlSpec, actionCounts map[string]int) (homeResolvedCommand, bool) {
	if control.Disabled || spec.Disabled {
		return homeResolvedCommand{}, false
	}

	commandView := HomeCommand{
		Name:    firstNonEmpty(control.Label, spec.Label, spec.ID),
		Aliases: homeCommandAliases(control, spec, actionCounts),
	}

	switch spec.Kind {
	case models.DeviceControlKindToggle:
		if spec.OnCommand == nil || spec.OffCommand == nil {
			return homeResolvedCommand{}, false
		}
		if spec.OnCommand.Action == spec.OffCommand.Action {
			commandView.Action = strings.TrimSpace(spec.OnCommand.Action)
		}
		commandView.Params = []HomeCommandParam{{
			Name:     "on",
			Type:     models.DeviceCommandParamTypeBoolean,
			Required: true,
		}}
		return homeResolvedCommand{
			view:       commandView,
			kind:       spec.Kind,
			onCommand:  cloneHomeControlCommand(spec.OnCommand),
			offCommand: cloneHomeControlCommand(spec.OffCommand),
		}, true
	case models.DeviceControlKindAction:
		if spec.Command == nil || strings.TrimSpace(spec.Command.Action) == "" {
			return homeResolvedCommand{}, false
		}
		commandView.Action = strings.TrimSpace(spec.Command.Action)
		commandView.Params = cloneHomeCommandParams(spec.Command.ParamsSpec)
		commandView.Defaults = homePublicDefaults(spec.Command.Params, commandView.Params)
		return homeResolvedCommand{
			view:    commandView,
			kind:    spec.Kind,
			command: cloneHomeControlCommand(spec.Command),
		}, true
	case models.DeviceControlKindSelect:
		if spec.Command == nil || strings.TrimSpace(spec.Command.Action) == "" || strings.TrimSpace(spec.Command.ValueParam) == "" {
			return homeResolvedCommand{}, false
		}
		commandView.Action = strings.TrimSpace(spec.Command.Action)
		commandView.Params = []HomeCommandParam{{
			Name:     strings.TrimSpace(spec.Command.ValueParam),
			Type:     models.DeviceCommandParamTypeString,
			Required: true,
			Options:  cloneOptions(spec.Options),
		}}
		return homeResolvedCommand{
			view:    commandView,
			kind:    spec.Kind,
			command: cloneHomeControlCommand(spec.Command),
		}, true
	case models.DeviceControlKindNumber:
		if spec.Command == nil || strings.TrimSpace(spec.Command.Action) == "" || strings.TrimSpace(spec.Command.ValueParam) == "" {
			return homeResolvedCommand{}, false
		}
		commandView.Action = strings.TrimSpace(spec.Command.Action)
		commandView.Params = []HomeCommandParam{{
			Name:     strings.TrimSpace(spec.Command.ValueParam),
			Type:     models.DeviceCommandParamTypeNumber,
			Required: true,
			Min:      cloneNumberPtr(spec.Min),
			Max:      cloneNumberPtr(spec.Max),
			Step:     cloneNumberPtr(spec.Step),
			Unit:     spec.Unit,
		}}
		return homeResolvedCommand{
			view:    commandView,
			kind:    spec.Kind,
			command: cloneHomeControlCommand(spec.Command),
		}, true
	default:
		return homeResolvedCommand{}, false
	}
}

func homeCommandAliases(control models.DeviceControl, spec models.DeviceControlSpec, actionCounts map[string]int) []string {
	aliases := make([]string, 0, 4)
	aliases = appendUniqueHomeName(aliases, control.DefaultLabel)
	aliases = appendUniqueHomeName(aliases, spec.ID)
	for action := range homeCommandActions(spec) {
		if actionCounts[action] == 1 {
			aliases = appendUniqueHomeName(aliases, action)
		}
	}
	return aliases
}

func homeCommandActions(spec models.DeviceControlSpec) map[string]struct{} {
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
