package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	gatewayapi "github.com/chentianyu/celestia/internal/api/gateway"
	"github.com/spf13/cobra"
)

func newHomeCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "home",
		Short: "Resolve and execute home shortcuts",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHomeCommand(cmd, opts, args)
		},
	}
}

func runHomeCommand(cmd *cobra.Command, opts *rootOptions, args []string) error {
	if len(args) == 0 || equalCLIRef(args[0], "list") {
		query := ""
		if len(args) > 1 {
			query = strings.Join(args[1:], " ")
		}
		return runHomeList(cmd, opts, query)
	}
	if equalCLIRef(args[0], "action") {
		if len(args) < 3 {
			return fmt.Errorf("usage: celctl home action <device> <raw_action> [key=value ...]")
		}
		params, _, err := parseHomeCLIParams(args[3:])
		if err != nil {
			return err
		}
		return runHomeExecute(cmd, opts, gatewayapi.AICommandRequest{
			DeviceName: args[1],
			Action:     strings.TrimSpace(args[2]),
			Actor:      opts.actor,
			Params:     params,
		})
	}

	primary, fallback, err := buildCLIHomeRequest(opts.actor, args)
	if err != nil {
		return err
	}
	if err := runHomeExecute(cmd, opts, primary); err != nil {
		var missing *gatewayapi.ReferenceNotFoundError
		field := ""
		if errors.As(err, &missing) {
			field = normalizeCLIRef(missing.Field)
		}
		if fallback == nil || field != "device" && field != "scope" {
			return err
		}
		return runHomeExecute(cmd, opts, *fallback)
	}
	return nil
}

func runHomeList(cmd *cobra.Command, opts *rootOptions, query string) error {
	ctx, cancel := opts.context(cmd.Context())
	defer cancel()

	items, err := opts.service().ListAIDevices(ctx, gatewayapi.DeviceFilter{Query: strings.TrimSpace(query)})
	if err != nil {
		return err
	}
	return writeOutput(cmd, opts.output, items)
}

func runHomeExecute(cmd *cobra.Command, opts *rootOptions, req gatewayapi.AICommandRequest) error {
	ctx, cancel := opts.context(cmd.Context())
	defer cancel()

	result, err := opts.service().ExecuteAICommand(ctx, req)
	if err != nil {
		return err
	}
	return writeOutput(cmd, opts.output, result)
}

func buildCLIHomeRequest(actor string, args []string) (gatewayapi.AICommandRequest, *gatewayapi.AICommandRequest, error) {
	if len(args) == 0 {
		return gatewayapi.AICommandRequest{}, nil, fmt.Errorf("usage: celctl home <device> <command> [value|key=value ...]")
	}
	if strings.Contains(args[0], ".") {
		params, values, err := parseHomeCLIParams(args[1:])
		if err != nil {
			return gatewayapi.AICommandRequest{}, nil, err
		}
		return gatewayapi.AICommandRequest{
			Target: args[0],
			Actor:  actor,
			Params: params,
			Values: values,
		}, nil, nil
	}
	if len(args) == 1 {
		return gatewayapi.AICommandRequest{
			Command: args[0],
			Actor:   actor,
		}, nil, nil
	}

	params, values, err := parseHomeCLIParams(args[2:])
	if err != nil {
		return gatewayapi.AICommandRequest{}, nil, err
	}
	fallbackParams, fallbackValues, err := parseHomeCLIParams(args[1:])
	if err != nil {
		return gatewayapi.AICommandRequest{}, nil, err
	}
	fallback := gatewayapi.AICommandRequest{
		Command: args[0],
		Actor:   actor,
		Params:  fallbackParams,
		Values:  fallbackValues,
	}
	return gatewayapi.AICommandRequest{
		DeviceName: args[0],
		Command:    args[1],
		Actor:      actor,
		Params:     params,
		Values:     values,
	}, &fallback, nil
}

func parseHomeCLIParams(args []string) (map[string]any, []string, error) {
	params := map[string]any{}
	values := []string{}
	for _, arg := range args {
		trimmed := strings.TrimSpace(arg)
		if trimmed == "" {
			continue
		}
		key, value, ok := strings.Cut(trimmed, "=")
		if !ok {
			values = append(values, trimmed)
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, nil, fmt.Errorf("invalid param %q", arg)
		}
		params[key] = coerceCLIValue(value)
	}
	return params, values, nil
}

func equalCLIRef(a string, b string) bool {
	return normalizeCLIRef(a) == normalizeCLIRef(b) && normalizeCLIRef(a) != ""
}

func normalizeCLIRef(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "", "_", "", "-", "", ".", "")
	return replacer.Replace(value)
}

func coerceCLIValue(value string) any {
	trimmed := strings.TrimSpace(value)
	switch normalizeCLIRef(trimmed) {
	case "on", "true", "1", "open", "enable", "enabled", "开", "打开", "开启":
		return true
	case "off", "false", "0", "close", "disable", "disabled", "关", "关闭":
		return false
	}
	if number, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return number
	}
	return trimmed
}
