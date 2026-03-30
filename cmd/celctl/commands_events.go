package main

import (
	gatewayapi "github.com/chentianyu/celestia/internal/api/gateway"
	"github.com/spf13/cobra"
)

func newEventsCommand(opts *rootOptions) *cobra.Command {
	var pluginID string
	var deviceID string
	var limit int

	cmd := &cobra.Command{
		Use:   "events",
		Short: "List runtime events",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := opts.context(cmd.Context())
			defer cancel()

			items, err := opts.service().ListEvents(ctx, gatewayapi.EventFilter{
				PluginID: pluginID,
				DeviceID: deviceID,
				Limit:    limit,
			})
			if err != nil {
				return err
			}
			return writeOutput(cmd, opts.output, items)
		},
	}
	cmd.Flags().StringVar(&pluginID, "plugin-id", "", "Filter by plugin id")
	cmd.Flags().StringVar(&deviceID, "device-id", "", "Filter by device id")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max events to return")
	return cmd
}

func newAuditsCommand(opts *rootOptions) *cobra.Command {
	var deviceID string
	var limit int

	cmd := &cobra.Command{
		Use:   "audits",
		Short: "List audit records",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := opts.context(cmd.Context())
			defer cancel()

			items, err := opts.service().ListAudits(ctx, gatewayapi.AuditFilter{
				DeviceID: deviceID,
				Limit:    limit,
			})
			if err != nil {
				return err
			}
			return writeOutput(cmd, opts.output, items)
		},
	}
	cmd.Flags().StringVar(&deviceID, "device-id", "", "Filter by device id")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max audit records to return")
	return cmd
}
