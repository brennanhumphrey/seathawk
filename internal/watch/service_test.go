package watch

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/brennanhumphrey/seathawk/internal/store"
)

func TestAddCreatesActiveWatch(t *testing.T) {
	svc := Service{DB: newWatchTestDB(t), Now: fixedWatchNow}

	got, err := svc.Add(context.Background(), CreateAddInput{Term: "202609", CRN: "60058"})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if got.Mode != ModeAdd || got.Term != "202609" || got.AddCRN != "60058" || !got.Active {
		t.Fatalf("unexpected watch: %+v", got)
	}
	if got.DropCRN.Valid {
		t.Fatalf("DropCRN.Valid = true, want false")
	}
}

func TestSwapCreatesActiveWatch(t *testing.T) {
	svc := Service{DB: newWatchTestDB(t), Now: fixedWatchNow}

	got, err := svc.Swap(context.Background(), CreateSwapInput{
		Term:    "202609",
		AddCRN:  "60058",
		DropCRN: "60900",
	})
	if err != nil {
		t.Fatalf("Swap returned error: %v", err)
	}
	if got.Mode != ModeSwap || got.AddCRN != "60058" || !got.DropCRN.Valid || got.DropCRN.String != "60900" {
		t.Fatalf("unexpected watch: %+v", got)
	}
}

func TestServiceValidation(t *testing.T) {
	tests := []struct {
		name string
		call func(Service) error
	}{
		{name: "invalid add term", call: func(s Service) error {
			_, err := s.Add(context.Background(), CreateAddInput{Term: "2026", CRN: "60058"})
			return err
		}},
		{name: "invalid add crn", call: func(s Service) error {
			_, err := s.Add(context.Background(), CreateAddInput{Term: "202609", CRN: "6005"})
			return err
		}},
		{name: "invalid swap add crn", call: func(s Service) error {
			_, err := s.Swap(context.Background(), CreateSwapInput{Term: "202609", AddCRN: "abcde", DropCRN: "60900"})
			return err
		}},
		{name: "missing swap drop crn", call: func(s Service) error {
			_, err := s.Swap(context.Background(), CreateSwapInput{Term: "202609", AddCRN: "60058"})
			return err
		}},
		{name: "identical swap crns", call: func(s Service) error {
			_, err := s.Swap(context.Background(), CreateSwapInput{Term: "202609", AddCRN: "60058", DropCRN: "60058"})
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := Service{DB: newWatchTestDB(t), Now: fixedWatchNow}
			if err := tt.call(svc); err == nil {
				t.Fatal("call returned nil error")
			}
		})
	}
}

func TestServiceTrimsInput(t *testing.T) {
	svc := Service{DB: newWatchTestDB(t), Now: fixedWatchNow}

	got, err := svc.Add(context.Background(), CreateAddInput{Term: " 202609 ", CRN: " 60058 "})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if got.Term != "202609" || got.AddCRN != "60058" {
		t.Fatalf("input was not normalized: %+v", got)
	}
}

func TestEnableDisable(t *testing.T) {
	svc := Service{DB: newWatchTestDB(t), Now: fixedWatchNow}
	watch, err := svc.Add(context.Background(), CreateAddInput{Term: "202609", CRN: "60058"})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	disabled, err := svc.Disable(context.Background(), watch.ID)
	if err != nil {
		t.Fatalf("Disable returned error: %v", err)
	}
	if disabled.Active {
		t.Fatal("Disable returned active watch")
	}

	enabled, err := svc.Enable(context.Background(), watch.ID)
	if err != nil {
		t.Fatalf("Enable returned error: %v", err)
	}
	if !enabled.Active {
		t.Fatal("Enable returned inactive watch")
	}
}

func TestRemove(t *testing.T) {
	db := newWatchTestDB(t)
	svc := Service{DB: db, Now: fixedWatchNow}
	watch, err := svc.Add(context.Background(), CreateAddInput{Term: "202609", CRN: "60058"})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	if err := svc.Remove(context.Background(), watch.ID); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	if _, err := store.WatchByID(context.Background(), db, watch.ID); !errors.Is(err, store.ErrNoWatch) {
		t.Fatalf("WatchByID error = %v, want ErrNoWatch", err)
	}
}

func newWatchTestDB(t *testing.T) *sql.DB {
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

func fixedWatchNow() time.Time {
	return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
}
