package main

import (
	"context"

	gatewayapi "github.com/chentianyu/celestia/internal/api/gateway"
	"github.com/spf13/cobra"
)

func newPluginsCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "plugins",
		Aliases: []string{"plugin"},
		Short:   "Manage plugin lifecycle and config",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPluginsList(cmd, opts)
		},
	}
	cmd.AddCommand(newPluginsListCommand(opts))
	cmd.AddCommand(newPluginsCatalogCommand(opts))
	cmd.AddCommand(newPluginsInstallCommand(opts))
	cmd.AddCommand(newPluginsConfigCommand(opts))
	cmd.AddCommand(newPluginsEnableCommand(opts))
	cmd.AddCommand(newPluginsDisableCommand(opts))
	cmd.AddCommand(newPluginsDiscoverCommand(opts))
	cmd.AddCommand(newPluginsDeleteCommand(opts))
	cmd.AddCommand(newPluginsLogsCommand(opts))
	return cmd
}

func newPluginsListCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List installed plugins",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPluginsList(cmd, opts)
		},
	}
}

func runPluginsList(cmd *cobra.Command, opts *rootOptions) error {
	ctx, cancel := opts.context(cmd.Context())
	defer cancel()

	items, err := opts.service().ListPlugins(ctx)
	if err != nil {
		return err
	}
	return writeOutput(cmd, opts.output, items)
}

func newPluginsCatalogCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "catalog",
		Short: "List plugin catalog entries",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := opts.context(cmd.Context())
			defer cancel()

			items, err := opts.service().ListCatalogPlugins(ctx)
			if err != nil {
				return err
			}
			return writeOutput(cmd, opts.output, items)
		},
	}
}

func newPluginsInstallCommand(opts *rootOptions) *cobra.Command {
	var binaryPath string
	var configJSON string
	var metadataJSON string

	cmd := &cobra.Command{
		Use:   "install <plugin-id>",
		Short: "Install a plugin from catalog",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := parseJSONMap(configJSON)
			if err != nil {
				return err
			}
			metadata, err := parseJSONMap(metadataJSON)
			if err != nil {
				return err
			}

			ctx, cancel := opts.context(cmd.Context())
			defer cancel()

			record, err := opts.service().InstallPlugin(ctx, gatewayapi.InstallPluginRequest{
				PluginID:   args[0],
				BinaryPath: binaryPath,
				Config:     config,
				Metadata:   metadata,
			})
			if err != nil {
				return err
			}
			return writeOutput(cmd, opts.output, record)
		},
	}
	cmd.Flags().StringVar(&binaryPath, "binary-path", "", "Plugin binary path override")
	cmd.Flags().StringVar(&configJSON, "config-json", "", "Plugin config JSON object")
	cmd.Flags().StringVar(&metadataJSON, "metadata-json", "", "Plugin metadata JSON object")
	return cmd
}

func newPluginsConfigCommand(opts *rootOptions) *cobra.Command {
	var configJSON string
	cmd := &cobra.Command{
		Use:   "config <plugin-id>",
		Short: "Update plugin config",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := parseJSONMap(configJSON)
			if err != nil {
				return err
			}
			if config == nil {
				return errRequiredFlag("--config-json")
			}

			ctx, cancel := opts.context(cmd.Context())
			defer cancel()

			record, err := opts.service().UpdatePluginConfig(ctx, gatewayapi.UpdatePluginConfigRequest{
				PluginID: args[0],
				Config:   config,
			})
			if err != nil {
				return err
			}
			return writeOutput(cmd, opts.output, record)
		},
	}
	cmd.Flags().StringVar(&configJSON, "config-json", "", "Plugin config JSON object")
	return cmd
}

func newPluginsEnableCommand(opts *rootOptions) *cobra.Command {
	return newPluginActionCommand(opts, "enable", "Enable a plugin", "enabled", func(ctx context.Context, svc gatewayapi.Service, pluginID string) error {
		return svc.EnablePlugin(ctx, pluginID)
	})
}

func newPluginsDisableCommand(opts *rootOptions) *cobra.Command {
	return newPluginActionCommand(opts, "disable", "Disable a plugin", "disabled", func(ctx context.Context, svc gatewayapi.Service, pluginID string) error {
		return svc.DisablePlugin(ctx, pluginID)
	})
}

func newPluginsDiscoverCommand(opts *rootOptions) *cobra.Command {
	return newPluginActionCommand(opts, "discover", "Run plugin discovery", "discovered", func(ctx context.Context, svc gatewayapi.Service, pluginID string) error {
		return svc.DiscoverPlugin(ctx, pluginID)
	})
}

func newPluginsDeleteCommand(opts *rootOptions) *cobra.Command {
	return newPluginActionCommand(opts, "delete", "Uninstall a plugin", "deleted", func(ctx context.Context, svc gatewayapi.Service, pluginID string) error {
		return svc.DeletePlugin(ctx, pluginID)
	})
}

func newPluginsLogsCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "logs <plugin-id>",
		Short: "Fetch recent plugin logs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := opts.context(cmd.Context())
			defer cancel()

			payload, err := opts.service().GetPluginLogs(ctx, args[0])
			if err != nil {
				return err
			}
			return writeOutput(cmd, opts.output, payload)
		},
	}
}

func newPluginActionCommand(opts *rootOptions, use, short, action string, fn func(context.Context, gatewayapi.Service, string) error) *cobra.Command {
	return &cobra.Command{
		Use:   use + " <plugin-id>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := opts.context(cmd.Context())
			defer cancel()

			svc := opts.service()
			if err := fn(ctx, svc, args[0]); err != nil {
				return err
			}
			return writeOutput(cmd, opts.output, map[string]any{
				"ok":        true,
				"plugin_id": args[0],
				"action":    action,
			})
		},
	}
}
