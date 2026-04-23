package main

import (
	"context"
	"os"
	"strings"
	"time"

	gatewayapi "github.com/chentianyu/celestia/internal/api/gateway"
	"github.com/spf13/cobra"
)

type rootOptions struct {
	baseURL string
	actor   string
	timeout time.Duration
	output  string
}

func newRootCommand() *cobra.Command {
	opts := rootOptions{
		baseURL: defaultBaseURL(),
		actor:   "celctl",
		timeout: 10 * time.Second,
		output:  "json",
	}

	cmd := &cobra.Command{
		Use:           "celctl",
		Short:         "Celestia CLI for agents and operators",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDashboard(cmd, &opts)
		},
	}

	cmd.PersistentFlags().StringVar(&opts.baseURL, "base-url", opts.baseURL, "Gateway base URL")
	cmd.PersistentFlags().StringVar(&opts.actor, "actor", opts.actor, "Actor value for command requests")
	cmd.PersistentFlags().DurationVar(&opts.timeout, "timeout", opts.timeout, "Per-command timeout")
	cmd.PersistentFlags().StringVar(&opts.output, "output", opts.output, "Output format: json|pretty")
	cmd.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		return validateOutputFormat(opts.output)
	}

	cmd.AddCommand(newHealthCommand(&opts))
	cmd.AddCommand(newDashboardCommand(&opts))
	cmd.AddCommand(newPluginsCommand(&opts))
	cmd.AddCommand(newDevicesCommand(&opts))
	cmd.AddCommand(newHomeCommand(&opts))
	cmd.AddCommand(newEventsCommand(&opts))
	cmd.AddCommand(newAuditsCommand(&opts))
	return cmd
}

func (o *rootOptions) service() gatewayapi.Service {
	return gatewayapi.NewHTTPService(o.baseURL, o.timeout)
}

func (o *rootOptions) context(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	if o.timeout <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, o.timeout)
}

func defaultBaseURL() string {
	if value := strings.TrimSpace(os.Getenv("CELESTIA_URL")); value != "" {
		return value
	}
	return "http://127.0.0.1:8080"
}
