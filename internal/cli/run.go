package cli

import (
	"fmt"

	"github.com/brennanhumphrey/seathawk/internal/config"
	"github.com/brennanhumphrey/seathawk/internal/store"

	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Start SeatHawk",
		RunE: func(cmd *cobra.Command, args []string) error {
			// The run command is the daemon boot path. For Phase 1 it only
			// proves that config and persistence are wired correctly.
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			db, err := store.Open(cmd.Context(), cfg.DatabasePath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer db.Close()

			if err := store.Migrate(cmd.Context(), db); err != nil {
				return fmt.Errorf("migrate database: %w", err)
			}

			fmt.Println("seathawk booted successfully")
			fmt.Printf("config: %s\n", cfg.ConfigPath)
			fmt.Printf("database: %s\n", cfg.DatabasePath)

			return nil
		},
	}

	return cmd
}
