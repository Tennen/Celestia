package main

import (
	gatewayapi "github.com/chentianyu/celestia/internal/api/gateway"
	"github.com/spf13/cobra"
)

func newDevicesCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "devices",
		Short: "Query devices and dispatch commands",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDevicesList(cmd, opts, "", "", "")
		},
	}
	cmd.AddCommand(newDevicesListCommand(opts))
	cmd.AddCommand(newDevicesGetCommand(opts))
	cmd.AddCommand(newDevicesAliasCommand(opts))
	cmd.AddCommand(newDevicesControlCommand(opts))
	cmd.AddCommand(newDevicesCommandCommand(opts))
	cmd.AddCommand(newDevicesToggleCommand(opts))
	cmd.AddCommand(newDevicesActionCommand(opts))
	return cmd
}

func newDevicesListCommand(opts *rootOptions) *cobra.Command {
	var pluginID string
	var kind string
	var query string
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List devices",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDevicesList(cmd, opts, pluginID, kind, query)
		},
	}
	cmd.Flags().StringVar(&pluginID, "plugin-id", "", "Filter by plugin id")
	cmd.Flags().StringVar(&kind, "kind", "", "Filter by device kind")
	cmd.Flags().StringVar(&query, "q", "", "Search by name/alias")
	return cmd
}

func runDevicesList(cmd *cobra.Command, opts *rootOptions, pluginID, kind, query string) error {
	ctx, cancel := opts.context(cmd.Context())
	defer cancel()

	items, err := opts.service().ListDevices(ctx, gatewayapi.DeviceFilter{
		PluginID: pluginID,
		Kind:     kind,
		Query:    query,
	})
	if err != nil {
		return err
	}
	return writeOutput(cmd, opts.output, items)
}

func newDevicesGetCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "get <device-id>",
		Short: "Get one device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := opts.context(cmd.Context())
			defer cancel()

			item, err := opts.service().GetDevice(ctx, args[0])
			if err != nil {
				return err
			}
			return writeOutput(cmd, opts.output, item)
		},
	}
}

func newDevicesAliasCommand(opts *rootOptions) *cobra.Command {
	var alias string
	cmd := &cobra.Command{
		Use:   "alias <device-id>",
		Short: "Update device alias",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("alias") {
				return errRequiredFlag("--alias")
			}

			ctx, cancel := opts.context(cmd.Context())
			defer cancel()

			payload, err := opts.service().UpdateDevicePreference(ctx, gatewayapi.UpdateDevicePreferenceRequest{
				DeviceID: args[0],
				Alias:    alias,
			})
			if err != nil {
				return err
			}
			return writeOutput(cmd, opts.output, payload)
		},
	}
	cmd.Flags().StringVar(&alias, "alias", "", "Device alias (empty string resets)")
	return cmd
}

func newDevicesControlCommand(opts *rootOptions) *cobra.Command {
	var alias string
	var visibleRaw string
	cmd := &cobra.Command{
		Use:   "control <device-id> <control-id>",
		Short: "Update control alias/visibility",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("alias") && !cmd.Flags().Changed("visible") {
				return errRequiredFlag("--alias or --visible")
			}
			visible, err := parseVisibleFlag(visibleRaw)
			if err != nil {
				return err
			}

			ctx, cancel := opts.context(cmd.Context())
			defer cancel()

			payload, err := opts.service().UpdateControlPreference(ctx, gatewayapi.UpdateControlPreferenceRequest{
				DeviceID:  args[0],
				ControlID: args[1],
				Alias:     alias,
				Visible:   visible,
			})
			if err != nil {
				return err
			}
			return writeOutput(cmd, opts.output, payload)
		},
	}
	cmd.Flags().StringVar(&alias, "alias", "", "Control alias (empty string resets)")
	cmd.Flags().StringVar(&visibleRaw, "visible", "", "Control visibility: true or false")
	return cmd
}

func newDevicesCommandCommand(opts *rootOptions) *cobra.Command {
	var action string
	var paramsJSON string
	cmd := &cobra.Command{
		Use:   "command <device-id>",
		Short: "Send normalized command to a device",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if action == "" {
				return errRequiredFlag("--action")
			}
			params, err := parseJSONMap(paramsJSON)
			if err != nil {
				return err
			}

			ctx, cancel := opts.context(cmd.Context())
			defer cancel()

			payload, err := opts.service().SendDeviceCommand(ctx, gatewayapi.DeviceCommandRequest{
				DeviceID: args[0],
				Actor:    opts.actor,
				Action:   action,
				Params:   params,
			})
			if err != nil {
				return err
			}
			return writeOutput(cmd, opts.output, payload)
		},
	}
	cmd.Flags().StringVar(&action, "action", "", "Normalized command action")
	cmd.Flags().StringVar(&paramsJSON, "params-json", "", "Command params JSON object")
	return cmd
}

func newDevicesToggleCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "toggle <device-id.control-id> <on|off>",
		Short: "Toggle a quick control",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			on, err := parseOnOff(args[1])
			if err != nil {
				return err
			}

			ctx, cancel := opts.context(cmd.Context())
			defer cancel()

			payload, err := opts.service().ToggleControl(ctx, gatewayapi.ToggleControlRequest{
				CompoundControlID: args[0],
				Actor:             opts.actor,
				On:                on,
			})
			if err != nil {
				return err
			}
			return writeOutput(cmd, opts.output, payload)
		},
	}
}

func newDevicesActionCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "action <device-id.control-id>",
		Short: "Run an action quick control",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := opts.context(cmd.Context())
			defer cancel()

			payload, err := opts.service().RunActionControl(ctx, gatewayapi.ActionControlRequest{
				CompoundControlID: args[0],
				Actor:             opts.actor,
			})
			if err != nil {
				return err
			}
			return writeOutput(cmd, opts.output, payload)
		},
	}
}
