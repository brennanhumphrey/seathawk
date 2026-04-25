package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
)

// initialSchema is embedded so installed binaries can initialize the database
// without requiring a migrations directory on disk.
//
//go:embed migrations/001_init.sql
var initialSchema string

// Migrate applies the initial database schema.
//
// This deliberately stays minimal until schema evolution becomes a real
// problem. The important Phase 1 behavior is that a fresh database can boot
// into a known shape.
func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, initialSchema); err != nil {
		return fmt.Errorf("apply migration: %w", err)
	}

	return nil
}
