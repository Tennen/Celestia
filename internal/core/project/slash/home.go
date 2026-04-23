package slash

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/chentianyu/celestia/internal/core/control"
	"github.com/chentianyu/celestia/internal/models"
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
		result, err := s.home.Execute(ctx, control.HomeRequest{
			DeviceName: args[1],
			Action:     strings.TrimSpace(args[2]),
			Actor:      actorOrInput(req),
			Params:     params,
		})
		if err != nil {
			return "", map[string]any{"domain": "home", "action": "direct"}, err
		}
		metadata := map[string]any{
			"domain":      "home",
			"action":      "direct",
			"device_id":   result.Device.ID,
			"device_name": result.Device.Name,
			"command":     result.Command.Action,
			"accepted":    result.Result.Accepted,
		}
		return fmt.Sprintf("Home command accepted: %s / %s", result.Device.Name, result.Command.Name), metadata, nil
	}

	primary, fallback, err := buildSlashHomeRequest(actorOrInput(req), args)
	if err != nil {
		return "", map[string]any{"domain": "home", "action": "control"}, err
	}
	result, err := s.home.Execute(ctx, primary)
	if err != nil && fallback != nil {
		var missing *control.NotFoundError
		if errors.As(err, &missing) && (equalRef(missing.Field, "device") || equalRef(missing.Field, "scope")) {
			result, err = s.home.Execute(ctx, *fallback)
		}
	}
	if err != nil {
		return "", map[string]any{"domain": "home", "action": "control"}, err
	}
	metadata := map[string]any{
		"domain":       "home",
		"action":       "control",
		"device_id":    result.Device.ID,
		"device_name":  result.Device.Name,
		"control_name": result.Command.Name,
		"command":      result.Command.Action,
		"accepted":     result.Result.Accepted,
	}
	return fmt.Sprintf("Home command accepted: %s / %s", result.Device.Name, result.Command.Name), metadata, nil
}

func (s *Service) homeList(ctx context.Context, query string) (string, int, error) {
	views, err := s.home.ListViews(ctx, control.HomeFilter{Query: strings.TrimSpace(query)})
	if err != nil {
		return "", 0, err
	}
	sort.SliceStable(views, func(i, j int) bool { return views[i].Device.Name < views[j].Device.Name })

	lines := []string{"Home devices:"}
	for _, view := range views {
		controls := make([]string, 0, len(view.Controls))
		for _, item := range view.Controls {
			if item.Disabled || !item.Visible {
				continue
			}
			controls = append(controls, firstNonEmpty(item.Label, item.ID))
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
	if len(views) == 0 {
		lines = append(lines, "- no devices matched")
	}
	return strings.Join(lines, "\n"), len(views), nil
}

func buildSlashHomeRequest(actor string, args []string) (control.HomeRequest, *control.HomeRequest, error) {
	if len(args) == 0 {
		return control.HomeRequest{}, nil, errors.New("usage: /home <device> <command> [value|key=value ...]")
	}
	if strings.Contains(args[0], ".") {
		params, values, err := parseSlashParams(args[1:])
		if err != nil {
			return control.HomeRequest{}, nil, err
		}
		return control.HomeRequest{
			Target: args[0],
			Actor:  actor,
			Params: params,
			Values: values,
		}, nil, nil
	}
	if len(args) == 1 {
		return control.HomeRequest{
			Command: args[0],
			Actor:   actor,
		}, nil, nil
	}

	params, values, err := parseSlashParams(args[2:])
	if err != nil {
		return control.HomeRequest{}, nil, err
	}
	fallbackParams, fallbackValues, err := parseSlashParams(args[1:])
	if err != nil {
		return control.HomeRequest{}, nil, err
	}
	fallback := control.HomeRequest{
		Command: args[0],
		Actor:   actor,
		Params:  fallbackParams,
		Values:  fallbackValues,
	}
	return control.HomeRequest{
		DeviceName: args[0],
		Command:    args[1],
		Actor:      actor,
		Params:     params,
		Values:     values,
	}, &fallback, nil
}
