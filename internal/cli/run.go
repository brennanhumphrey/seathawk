package cli

import (
	"fmt"
	"log"

	"github.com/brennanhumphrey/seathawk/internal/automation"
	"github.com/brennanhumphrey/seathawk/internal/daemon"
	"github.com/brennanhumphrey/seathawk/internal/register"
	"github.com/brennanhumphrey/seathawk/internal/vt"
	watchsvc "github.com/brennanhumphrey/seathawk/internal/watch"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var autoRegister bool

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the watch daemon with optional auto-registration",
		Long: "Run the watch daemon.\n\n" +
			"By default this command is read-only: it polls due watches and updates\n" +
			"local metadata, but it does not write to VT registration endpoints.\n" +
			"Pass --auto-register to allow eligible add watches to use the same\n" +
			"registration flow as `register attempt`.",
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
			logger.Printf("auto_register=%t", autoRegister)

			watchService := watchsvc.Service{DB: db, VTClient: client}
			registerService := register.Service{DB: db, VTClient: client}
			runner := daemon.Runner{
				Processor: automation.Service{
					Poller:     watchService,
					Registerer: registerService,
					Disabler:   watchService,
				},
				AutoRegister: autoRegister,
				Logger:       logger,
			}
			return runner.Run(cmd.Context())
		},
	}

	cmd.Flags().BoolVar(&autoRegister, "auto-register", false, "Allow the daemon to automatically attempt eligible add watches")
	return cmd
}
