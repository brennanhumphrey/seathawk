package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveAttemptAndAttemptByID(t *testing.T) {
	db := newAttemptTestDB(t)
	ctx := context.Background()
	watch := saveAttemptTestWatch(t, db)
	createdAt := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)

	saved, err := SaveAttempt(ctx, db, Attempt{
		WatchID:     watch.ID,
		Phase:       "cart_add",
		OutcomeKind: "submitted",
		RawResponse: sql.NullString{String: `{"cart":[]}`, Valid: true},
		CreatedAt:   createdAt,
	})
	if err != nil {
		t.Fatalf("SaveAttempt returned error: %v", err)
	}
	if saved.ID == 0 {
		t.Fatal("SaveAttempt returned attempt without ID")
	}
	if saved.WatchID != watch.ID || saved.Phase != "cart_add" || saved.OutcomeKind != "submitted" {
		t.Fatalf("unexpected saved attempt: %+v", saved)
	}
	if !saved.RawResponse.Valid || saved.RawResponse.String != `{"cart":[]}` {
		t.Fatalf("RawResponse = %+v, want JSON payload", saved.RawResponse)
	}

	loaded, err := AttemptByID(ctx, db, saved.ID)
	if err != nil {
		t.Fatalf("AttemptByID returned error: %v", err)
	}
	if loaded.ID != saved.ID || loaded.CreatedAt.IsZero() {
		t.Fatalf("unexpected loaded attempt: %+v", loaded)
	}
}

func TestUpdateWatchLastAttemptAt(t *testing.T) {
	db := newAttemptTestDB(t)
	ctx := context.Background()
	watch := saveAttemptTestWatch(t, db)
	attemptedAt := time.Date(2026, 4, 28, 12, 30, 0, 0, time.UTC)

	if err := UpdateWatchLastAttemptAt(ctx, db, watch.ID, attemptedAt, attemptedAt); err != nil {
		t.Fatalf("UpdateWatchLastAttemptAt returned error: %v", err)
	}

	updated, err := WatchByID(ctx, db, watch.ID)
	if err != nil {
		t.Fatalf("WatchByID returned error: %v", err)
	}
	if !updated.LastAttemptAt.Valid || !updated.LastAttemptAt.Time.Equal(attemptedAt) {
		t.Fatalf("LastAttemptAt = %+v, want %s", updated.LastAttemptAt, attemptedAt)
	}
}

func newAttemptTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	return db
}

func saveAttemptTestWatch(t *testing.T, db *sql.DB) Watch {
	t.Helper()

	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
	watch, err := SaveWatch(context.Background(), db, Watch{
		Term:      "202606",
		Mode:      "add",
		AddCRN:    "60058",
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("SaveWatch returned error: %v", err)
	}
	return watch
}
