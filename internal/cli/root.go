// Package cli defines SeatHawk's command-line interface.
//
// CLI code should stay thin: commands parse user input and orchestrate work,
// while config, storage, VT protocol logic, and registration workflows live in
// their own packages.
package cli

import (
	"context"

	"github.com/spf13/cobra"
)

var configPath string

// Execute builds the root command and runs SeatHawk's CLI.
func Execute(ctx context.Context) error {
	return newRootCmd().ExecuteContext(ctx)
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "seathawk",
		Short:         "VT seat opening and auto-registration tool",
		Long:          "SeatHawk is a single-user daemon and CLI for monitoring seats and attempting registration.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(newRunCmd())
	cmd.AddCommand(newSessionCmd())

	cmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to config file")

	return cmd
}
