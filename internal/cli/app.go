package cli

import (
	"database/sql"
	"fmt"

	"github.com/brennanhumphrey/seathawk/internal/config"
	"github.com/brennanhumphrey/seathawk/internal/store"
	"github.com/spf13/cobra"
)

// openAppDB is the shared CLI bootstrap path for commands that need local
// state.
//
// Commands should not each reimplement config loading, SQLite opening, and
// migration application. Keeping that here makes new commands behave the same
// way with default config paths, custom --config paths, and database setup.
func openAppDB(cmd *cobra.Command) (config.Config, *sql.DB, func(), error) {
	cfg, err := config.Load(configPathFromCommand(cmd))
	if err != nil {
		return config.Config{}, nil, nil, fmt.Errorf("load config: %w", err)
	}

	db, err := store.Open(cmd.Context(), cfg.DatabasePath)
	if err != nil {
		return config.Config{}, nil, nil, fmt.Errorf("open database: %w", err)
	}
	cleanup := func() {
		_ = db.Close()
	}

	if err := store.Migrate(cmd.Context(), db); err != nil {
		cleanup()
		return config.Config{}, nil, nil, fmt.Errorf("migrate database: %w", err)
	}

	return cfg, db, cleanup, nil
}

func configPathFromCommand(cmd *cobra.Command) string {
	flag := cmd.Root().PersistentFlags().Lookup("config")
	if flag == nil {
		return ""
	}
	return flag.Value.String()
}
