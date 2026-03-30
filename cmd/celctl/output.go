package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func writeOutput(cmd *cobra.Command, format string, payload any) error {
	writer := cmd.OutOrStdout()
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "json":
		encoder := json.NewEncoder(writer)
		encoder.SetEscapeHTML(false)
		return encoder.Encode(payload)
	case "pretty":
		raw, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return err
		}
		if _, err := writer.Write(raw); err != nil {
			return err
		}
		if _, err := writer.Write([]byte("\n")); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func validateOutputFormat(format string) error {
	normalized := strings.ToLower(strings.TrimSpace(format))
	if normalized == "json" || normalized == "pretty" {
		return nil
	}
	return fmt.Errorf("unsupported --output %q, expected json or pretty", format)
}

func parseJSONMap(raw string) (map[string]any, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func parseVisibleFlag(raw string) (*bool, error) {
	trimmed := strings.TrimSpace(strings.ToLower(raw))
	if trimmed == "" {
		return nil, nil
	}
	switch trimmed {
	case "true", "1", "yes", "on":
		value := true
		return &value, nil
	case "false", "0", "no", "off":
		value := false
		return &value, nil
	default:
		return nil, fmt.Errorf("invalid --visible value %q, expected true or false", raw)
	}
}

func parseOnOff(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "on", "true", "1":
		return true, nil
	case "off", "false", "0":
		return false, nil
	default:
		return false, fmt.Errorf("invalid state %q, expected on or off", value)
	}
}
