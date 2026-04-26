package cli

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/brennanhumphrey/seathawk/internal/config"
	"github.com/brennanhumphrey/seathawk/internal/store"
)

// openAppDB is the shared CLI bootstrap path for commands that need local
// state.
//
// Commands should not each reimplement config loading, SQLite opening, and
// migration application. Keeping that here makes new commands behave the same
// way with default config paths, custom --config paths, and database setup.
func openAppDB(ctx context.Context) (config.Config, *sql.DB, func(), error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return config.Config{}, nil, nil, fmt.Errorf("load config: %w", err)
	}

	db, err := store.Open(ctx, cfg.DatabasePath)
	if err != nil {
		return config.Config{}, nil, nil, fmt.Errorf("open database: %w", err)
	}
	cleanup := func() {
		_ = db.Close()
	}

	if err := store.Migrate(ctx, db); err != nil {
		cleanup()
		return config.Config{}, nil, nil, fmt.Errorf("migrate database: %w", err)
	}

	return cfg, db, cleanup, nil
}
