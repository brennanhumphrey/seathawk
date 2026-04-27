package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Start SeatHawk",
		RunE: func(cmd *cobra.Command, args []string) error {
			// The run command is the daemon boot path. For Phase 1 it only
			// proves that config and persistence are wired correctly.
			cfg, _, cleanup, err := openAppDB(cmd)
			if err != nil {
				return err
			}
			defer cleanup()

			fmt.Println("seathawk booted successfully")
			fmt.Printf("config: %s\n", cfg.ConfigPath)
			fmt.Printf("database: %s\n", cfg.DatabasePath)

			return nil
		},
	}

	return cmd
}
