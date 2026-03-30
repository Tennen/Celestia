package main

import "github.com/spf13/cobra"

func newHealthCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Get gateway health",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := opts.context(cmd.Context())
			defer cancel()

			payload, err := opts.service().Health(ctx)
			if err != nil {
				return err
			}
			return writeOutput(cmd, opts.output, payload)
		},
	}
}

func newDashboardCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "dashboard",
		Short: "Get dashboard summary",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDashboard(cmd, opts)
		},
	}
}

func runDashboard(cmd *cobra.Command, opts *rootOptions) error {
	ctx, cancel := opts.context(cmd.Context())
	defer cancel()

	payload, err := opts.service().Dashboard(ctx)
	if err != nil {
		return err
	}
	return writeOutput(cmd, opts.output, payload)
}
