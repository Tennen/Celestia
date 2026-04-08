package control

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/chentianyu/celestia/internal/models"
)

type Service struct{}

func New() *Service {
	return &Service{}
}

func (s *Service) BuildView(device models.Device, state models.DeviceStateSnapshot) models.DeviceView {
	return models.DeviceView{
		Device:   device,
		State:    state,
		Controls: s.List(device, state),
	}
}

func (s *Service) ApplyPreferences(view models.DeviceView, prefs []models.DeviceControlPreference) models.DeviceView {
	indexed := make(map[string]models.DeviceControlPreference, len(prefs))
	for _, pref := range prefs {
		indexed[pref.ControlID] = pref
	}
	for idx := range view.Controls {
		control := view.Controls[idx]
		control.DefaultLabel = control.Label
		control.Visible = true
		if pref, ok := indexed[control.ID]; ok {
			control.Alias = pref.Alias
			control.Visible = pref.Visible
			if strings.TrimSpace(pref.Alias) != "" {
				control.Label = pref.Alias
			}
		}
		view.Controls[idx] = control
	}
	return view
}

func (s *Service) List(device models.Device, state models.DeviceStateSnapshot) []models.DeviceControl {
	specs := controlSpecs(device, state)
	out := make([]models.DeviceControl, 0, len(specs))
	for _, item := range specs {
		out = append(out, item.view)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if controlKindOrder(out[i].Kind) != controlKindOrder(out[j].Kind) {
			return controlKindOrder(out[i].Kind) < controlKindOrder(out[j].Kind)
		}
		if out[i].Label != out[j].Label {
			return out[i].Label < out[j].Label
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) ResolveToggle(device models.Device, state models.DeviceStateSnapshot, controlID string, on bool) (models.CommandRequest, error) {
	for _, item := range controlSpecs(device, state) {
		if item.view.Kind != models.DeviceControlKindToggle || item.view.ID != controlID {
			continue
		}
		if item.view.Disabled {
			return models.CommandRequest{}, disabledControlError(item.view)
		}
		command := item.onCommand
		if !on {
			command = item.offCommand
		}
		if command == nil || command.Action == "" {
			return models.CommandRequest{}, fmt.Errorf("toggle %q does not support this state", controlID)
		}
		return models.CommandRequest{
			DeviceID: device.ID,
			Action:   command.Action,
			Params:   cloneParams(command.Params),
		}, nil
	}
	return models.CommandRequest{}, fmt.Errorf("toggle %q not found", controlID)
}

func (s *Service) ResolveAction(device models.Device, state models.DeviceStateSnapshot, controlID string) (models.CommandRequest, error) {
	for _, item := range controlSpecs(device, state) {
		if item.view.Kind != models.DeviceControlKindAction || item.view.ID != controlID {
			continue
		}
		if item.view.Disabled {
			return models.CommandRequest{}, disabledControlError(item.view)
		}
		if item.command == nil || item.command.Action == "" {
			return models.CommandRequest{}, fmt.Errorf("action %q is not executable", controlID)
		}
		return models.CommandRequest{
			DeviceID: device.ID,
			Action:   item.command.Action,
			Params:   cloneParams(item.command.Params),
		}, nil
	}
	return models.CommandRequest{}, fmt.Errorf("action %q not found", controlID)
}

func disabledControlError(control models.DeviceControl) error {
	reason := strings.TrimSpace(control.DisabledReason)
	if reason == "" {
		reason = "control is disabled"
	}
	return errors.New(reason)
}

func ParseCompoundControlID(value string) (string, string, error) {
	index := strings.LastIndex(value, ".")
	if index <= 0 || index == len(value)-1 {
		return "", "", errors.New("control id must look like <device_id>.<control_id>")
	}
	return value[:index], value[index+1:], nil
}
