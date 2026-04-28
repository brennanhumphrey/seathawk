package cli

import (
	"fmt"
	"log"

	"github.com/brennanhumphrey/seathawk/internal/daemon"
	"github.com/brennanhumphrey/seathawk/internal/vt"
	watchsvc "github.com/brennanhumphrey/seathawk/internal/watch"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the read-only watch polling daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, db, cleanup, err := openAppDB(cmd)
			if err != nil {
				return err
			}
			defer cleanup()

			client, err := vt.NewClient(vt.ClientConfig{})
			if err != nil {
				return fmt.Errorf("create VT client: %w", err)
			}

			logger := log.New(cmd.ErrOrStderr(), "seathawk: ", log.LstdFlags)
			logger.Printf("config=%s", cfg.ConfigPath)
			logger.Printf("database=%s", cfg.DatabasePath)

			runner := daemon.Runner{
				Poller: watchsvc.Service{DB: db, VTClient: client},
				Logger: logger,
			}
			return runner.Run(cmd.Context())
		},
	}

	return cmd
}
