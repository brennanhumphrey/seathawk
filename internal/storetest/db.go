package storetest

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/brennanhumphrey/seathawk/internal/store"
)

func NewDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	return db
}
