package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestSaveWatchAndWatchByID(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	now := fixedStoreTime()

	saved, err := SaveWatch(ctx, db, Watch{
		Term:      "202609",
		Mode:      "add",
		AddCRN:    "60058",
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("SaveWatch returned error: %v", err)
	}

	got, err := WatchByID(ctx, db, saved.ID)
	if err != nil {
		t.Fatalf("WatchByID returned error: %v", err)
	}
	if got.Term != "202609" || got.Mode != "add" || got.AddCRN != "60058" || !got.Active {
		t.Fatalf("unexpected watch: %+v", got)
	}
	if got.DropCRN.Valid {
		t.Fatalf("DropCRN.Valid = true, want false")
	}
}

func TestSaveWatchSwap(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	now := fixedStoreTime()

	got, err := SaveWatch(ctx, db, Watch{
		Term:      "202609",
		Mode:      "swap",
		AddCRN:    "60058",
		DropCRN:   sql.NullString{String: "60900", Valid: true},
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("SaveWatch returned error: %v", err)
	}
	if !got.DropCRN.Valid || got.DropCRN.String != "60900" {
		t.Fatalf("DropCRN = %+v, want 60900", got.DropCRN)
	}
}

func TestListWatches(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	now := fixedStoreTime()

	inputs := []Watch{
		{Term: "202606", Mode: "add", AddCRN: "11111", Active: true, CreatedAt: now, UpdatedAt: now},
		{Term: "202609", Mode: "add", AddCRN: "22222", Active: false, CreatedAt: now, UpdatedAt: now},
	}
	for _, input := range inputs {
		if _, err := SaveWatch(ctx, db, input); err != nil {
			t.Fatalf("SaveWatch returned error: %v", err)
		}
	}

	got, err := ListWatches(ctx, db)
	if err != nil {
		t.Fatalf("ListWatches returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("watch count = %d, want 2", len(got))
	}
	if got[0].AddCRN != "11111" || got[1].AddCRN != "22222" {
		t.Fatalf("unexpected watch order: %+v", got)
	}
	if got[1].Active {
		t.Fatalf("disabled watch was not returned as inactive: %+v", got[1])
	}
}

func TestListActiveWatches(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	now := fixedStoreTime()

	inputs := []Watch{
		{Term: "202606", Mode: "add", AddCRN: "11111", Active: true, CreatedAt: now, UpdatedAt: now},
		{Term: "202609", Mode: "add", AddCRN: "22222", Active: false, CreatedAt: now, UpdatedAt: now},
		{Term: "202612", Mode: "add", AddCRN: "33333", Active: true, CreatedAt: now, UpdatedAt: now},
	}
	for _, input := range inputs {
		if _, err := SaveWatch(ctx, db, input); err != nil {
			t.Fatalf("SaveWatch returned error: %v", err)
		}
	}

	got, err := ListActiveWatches(ctx, db)
	if err != nil {
		t.Fatalf("ListActiveWatches returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("watch count = %d, want 2", len(got))
	}
	if got[0].AddCRN != "11111" || got[1].AddCRN != "33333" {
		t.Fatalf("unexpected active watches: %+v", got)
	}
}

func TestListDueActiveWatches(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	now := fixedStoreTime()

	inputs := []Watch{
		{Term: "202606", Mode: "add", AddCRN: "11111", Active: true, NextPollAt: sql.NullTime{}, CreatedAt: now, UpdatedAt: now},
		{Term: "202609", Mode: "add", AddCRN: "22222", Active: true, NextPollAt: sql.NullTime{Time: now.Add(-time.Minute), Valid: true}, CreatedAt: now, UpdatedAt: now},
		{Term: "202612", Mode: "add", AddCRN: "33333", Active: true, NextPollAt: sql.NullTime{Time: now.Add(time.Minute), Valid: true}, CreatedAt: now, UpdatedAt: now},
		{Term: "202612", Mode: "add", AddCRN: "44444", Active: false, NextPollAt: sql.NullTime{Time: now.Add(-time.Minute), Valid: true}, CreatedAt: now, UpdatedAt: now},
	}
	for _, input := range inputs {
		if _, err := SaveWatch(ctx, db, input); err != nil {
			t.Fatalf("SaveWatch returned error: %v", err)
		}
	}

	got, err := ListDueActiveWatches(ctx, db, now)
	if err != nil {
		t.Fatalf("ListDueActiveWatches returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("watch count = %d, want 2", len(got))
	}
	if got[0].AddCRN != "11111" || got[1].AddCRN != "22222" {
		t.Fatalf("unexpected due watches: %+v", got)
	}
}

func TestSetWatchActive(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	now := fixedStoreTime()

	saved, err := SaveWatch(ctx, db, Watch{
		Term:      "202609",
		Mode:      "add",
		AddCRN:    "60058",
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("SaveWatch returned error: %v", err)
	}

	updatedAt := now.Add(time.Hour)
	if err := SetWatchActive(ctx, db, saved.ID, false, updatedAt); err != nil {
		t.Fatalf("SetWatchActive returned error: %v", err)
	}

	got, err := WatchByID(ctx, db, saved.ID)
	if err != nil {
		t.Fatalf("WatchByID returned error: %v", err)
	}
	if got.Active {
		t.Fatal("Active = true, want false")
	}
	if !got.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("UpdatedAt = %s, want %s", got.UpdatedAt, updatedAt)
	}
}

func TestUpdateWatchEvaluation(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	now := fixedStoreTime()

	saved, err := SaveWatch(ctx, db, Watch{
		Term:      "202609",
		Mode:      "add",
		AddCRN:    "60058",
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("SaveWatch returned error: %v", err)
	}

	nextPollAt := now.Add(30 * time.Second)
	updatedAt := now.Add(time.Minute)
	if err := UpdateWatchEvaluation(ctx, db, saved.ID,
		sql.NullString{String: "full", Valid: true},
		sql.NullTime{Time: nextPollAt, Valid: true},
		updatedAt,
	); err != nil {
		t.Fatalf("UpdateWatchEvaluation returned error: %v", err)
	}

	got, err := WatchByID(ctx, db, saved.ID)
	if err != nil {
		t.Fatalf("WatchByID returned error: %v", err)
	}
	if !got.LastSeenStat.Valid || got.LastSeenStat.String != "full" {
		t.Fatalf("LastSeenStat = %+v, want full", got.LastSeenStat)
	}
	if !got.NextPollAt.Valid || !got.NextPollAt.Time.Equal(nextPollAt) {
		t.Fatalf("NextPollAt = %+v, want %s", got.NextPollAt, nextPollAt)
	}
	if !got.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("UpdatedAt = %s, want %s", got.UpdatedAt, updatedAt)
	}
}

func TestDeleteWatch(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	now := fixedStoreTime()

	saved, err := SaveWatch(ctx, db, Watch{
		Term:      "202609",
		Mode:      "add",
		AddCRN:    "60058",
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("SaveWatch returned error: %v", err)
	}
	if err := DeleteWatch(ctx, db, saved.ID); err != nil {
		t.Fatalf("DeleteWatch returned error: %v", err)
	}
	if _, err := WatchByID(ctx, db, saved.ID); !errors.Is(err, ErrNoWatch) {
		t.Fatalf("WatchByID error = %v, want ErrNoWatch", err)
	}
}

func TestWatchNotFound(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	if _, err := WatchByID(ctx, db, 999); !errors.Is(err, ErrNoWatch) {
		t.Fatalf("WatchByID error = %v, want ErrNoWatch", err)
	}
	if err := SetWatchActive(ctx, db, 999, true, fixedStoreTime()); !errors.Is(err, ErrNoWatch) {
		t.Fatalf("SetWatchActive error = %v, want ErrNoWatch", err)
	}
	if err := UpdateWatchEvaluation(ctx, db, 999, sql.NullString{}, sql.NullTime{}, fixedStoreTime()); !errors.Is(err, ErrNoWatch) {
		t.Fatalf("UpdateWatchEvaluation error = %v, want ErrNoWatch", err)
	}
	if err := DeleteWatch(ctx, db, 999); !errors.Is(err, ErrNoWatch) {
		t.Fatalf("DeleteWatch error = %v, want ErrNoWatch", err)
	}
}

func fixedStoreTime() time.Time {
	return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
}
