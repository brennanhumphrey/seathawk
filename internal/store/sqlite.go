// Package store owns local persistence setup for SeatHawk.
//
// It opens SQLite, applies the embedded schema, and provides focused query
// helpers for durable app state. Higher-level packages own workflow decisions;
// this package should stay close to database reads and writes.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Open creates the database directory, opens SQLite, and verifies the
// connection before returning it.
func Open(ctx context.Context, dbPath string) (*sql.DB, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("database path is empty")
	}

	dbPath = filepath.Clean(dbPath)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, fmt.Errorf("create database dir: %w", err)
	}
	if err := ensureDatabaseFile(dbPath); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("sql open: %w", err)
	}
	// Keep one SQLite connection through database/sql to avoid self-inflicted
	// "database is locked" errors in this single-process app.
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if err := configureSQLite(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

// ensureDatabaseFile creates the SQLite file with owner-only permissions before
// the SQLite driver has a chance to create it with broader umask-derived modes.
func ensureDatabaseFile(dbPath string) error {
	file, err := os.OpenFile(dbPath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("create database file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close database file: %w", err)
	}

	// Session credentials are stored in this local SQLite file for v1. Chmod
	// also tightens permissions on existing files created by older versions.
	if err := os.Chmod(dbPath, 0o600); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("set database permissions: %w", err)
	}
	return nil
}

// configureSQLite applies connection-scoped SQLite settings SeatHawk relies on.
func configureSQLite(ctx context.Context, db *sql.DB) error {
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
	}
	for _, pragma := range pragmas {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("apply sqlite setting %q: %w", pragma, err)
		}
	}
	return nil
}
