package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveSessionAndCurrentSession(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)

	saved, err := SaveSession(ctx, db, Session{
		Authtoken:       "token",
		PersID:          "person",
		PersIDProof:     "proof",
		CapturedAt:      now,
		LastValidatedAt: sql.NullTime{Time: now.Add(time.Minute), Valid: true},
		Status:          "valid",
	})
	if err != nil {
		t.Fatalf("SaveSession returned error: %v", err)
	}
	if saved.ID == 0 {
		t.Fatal("SaveSession returned session without ID")
	}

	got, err := CurrentSession(ctx, db)
	if err != nil {
		t.Fatalf("CurrentSession returned error: %v", err)
	}
	if got.Authtoken != "token" || got.PersID != "person" || got.PersIDProof != "proof" || got.Status != "valid" {
		t.Fatalf("unexpected session: %+v", got)
	}
	if !got.LastValidatedAt.Valid {
		t.Fatal("LastValidatedAt is invalid")
	}
}

func TestSaveSessionAppendsHistoryAndCurrentSessionReturnsNewest(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)

	for _, token := range []string{"old", "new"} {
		if _, err := SaveSession(ctx, db, Session{
			Authtoken:   token,
			PersID:      token + "-person",
			PersIDProof: token + "-proof",
			CapturedAt:  now,
			Status:      "valid",
		}); err != nil {
			t.Fatalf("SaveSession(%q) returned error: %v", token, err)
		}
	}

	got, err := CurrentSession(ctx, db)
	if err != nil {
		t.Fatalf("CurrentSession returned error: %v", err)
	}
	if got.Authtoken != "new" {
		t.Fatalf("Authtoken = %q, want new", got.Authtoken)
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions`).Scan(&count); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	if count != 2 {
		t.Fatalf("session count = %d, want 2", count)
	}
}

func TestCurrentSessionEmpty(t *testing.T) {
	db := newTestDB(t)
	_, err := CurrentSession(context.Background(), db)
	if !errors.Is(err, ErrNoSession) {
		t.Fatalf("CurrentSession error = %v, want ErrNoSession", err)
	}
}

func TestUpdateSessionValidation(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	if _, err := SaveSession(ctx, db, Session{
		Authtoken:   "token",
		PersID:      "person",
		PersIDProof: "proof",
		CapturedAt:  now,
		Status:      "unknown",
	}); err != nil {
		t.Fatalf("SaveSession returned error: %v", err)
	}

	current, err := CurrentSession(ctx, db)
	if err != nil {
		t.Fatalf("CurrentSession returned error: %v", err)
	}

	validatedAt := now.Add(time.Hour)
	if err := UpdateSessionValidation(ctx, db, current.ID, "invalid", validatedAt, "fresh-proof"); err != nil {
		t.Fatalf("UpdateSessionValidation returned error: %v", err)
	}

	got, err := CurrentSession(ctx, db)
	if err != nil {
		t.Fatalf("CurrentSession returned error: %v", err)
	}
	if got.Status != "invalid" {
		t.Fatalf("Status = %q, want invalid", got.Status)
	}
	if got.PersIDProof != "fresh-proof" {
		t.Fatalf("PersIDProof = %q, want fresh-proof", got.PersIDProof)
	}
	if !got.LastValidatedAt.Valid {
		t.Fatal("LastValidatedAt is invalid")
	}
}

func newTestDB(t *testing.T) *sql.DB {
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
