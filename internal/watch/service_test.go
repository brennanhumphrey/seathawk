package watch

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/brennanhumphrey/seathawk/internal/store"
	"github.com/brennanhumphrey/seathawk/internal/vt"
)

type fakeVTClient struct {
	studentData vt.StudentData
	search      vt.FoseSearchResponse
	err         error
}

func (f fakeVTClient) StudentData(ctx context.Context, authtoken string) (vt.StudentData, error) {
	return f.studentData, f.err
}

func (f fakeVTClient) SearchByCRN(ctx context.Context, term, crn string) (vt.FoseSearchResponse, error) {
	return f.search, f.err
}

func TestAddCreatesActiveWatch(t *testing.T) {
	db := newWatchTestDB(t)
	saveWatchTestSession(t, db)
	svc := Service{DB: db, VTClient: validFakeVTClient(), Now: fixedWatchNow}

	got, eval, err := svc.Add(context.Background(), CreateAddInput{Term: "202609", CRN: "60058"})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if got.Mode != ModeAdd || got.Term != "202609" || got.AddCRN != "60058" || !got.Active {
		t.Fatalf("unexpected watch: %+v", got)
	}
	if got.DropCRN.Valid {
		t.Fatalf("DropCRN.Valid = true, want false")
	}
	if !got.LastSeenStat.Valid || got.LastSeenStat.String != string(SectionFull) {
		t.Fatalf("LastSeenStat = %+v, want full", got.LastSeenStat)
	}
	if !got.NextPollAt.Valid {
		t.Fatal("NextPollAt is invalid")
	}
	if eval.SectionStatus != SectionFull {
		t.Fatalf("SectionStatus = %q, want full", eval.SectionStatus)
	}
}

func TestSwapCreatesActiveWatch(t *testing.T) {
	db := newWatchTestDB(t)
	saveWatchTestSession(t, db)
	client := validFakeVTClient()
	client.studentData.Registered = map[string][]string{"202609": []string{"60900|CS 3304||N|3|UG|misc"}}
	svc := Service{DB: db, VTClient: client, Now: fixedWatchNow}

	got, eval, err := svc.Swap(context.Background(), CreateSwapInput{
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
	if eval.SectionStatus != SectionFull || !eval.RegisteredDrop {
		t.Fatalf("unexpected evaluation: %+v", eval)
	}
}

func TestServiceValidation(t *testing.T) {
	tests := []struct {
		name string
		call func(Service) error
	}{
		{name: "invalid add term", call: func(s Service) error {
			_, _, err := s.Add(context.Background(), CreateAddInput{Term: "2026", CRN: "60058"})
			return err
		}},
		{name: "invalid add crn", call: func(s Service) error {
			_, _, err := s.Add(context.Background(), CreateAddInput{Term: "202609", CRN: "6005"})
			return err
		}},
		{name: "invalid swap add crn", call: func(s Service) error {
			_, _, err := s.Swap(context.Background(), CreateSwapInput{Term: "202609", AddCRN: "abcde", DropCRN: "60900"})
			return err
		}},
		{name: "missing swap drop crn", call: func(s Service) error {
			_, _, err := s.Swap(context.Background(), CreateSwapInput{Term: "202609", AddCRN: "60058"})
			return err
		}},
		{name: "identical swap crns", call: func(s Service) error {
			_, _, err := s.Swap(context.Background(), CreateSwapInput{Term: "202609", AddCRN: "60058", DropCRN: "60058"})
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
	db := newWatchTestDB(t)
	saveWatchTestSession(t, db)
	svc := Service{DB: db, VTClient: validFakeVTClient(), Now: fixedWatchNow}

	got, _, err := svc.Add(context.Background(), CreateAddInput{Term: " 202609 ", CRN: " 60058 "})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if got.Term != "202609" || got.AddCRN != "60058" {
		t.Fatalf("input was not normalized: %+v", got)
	}
}

func TestEnableDisable(t *testing.T) {
	db := newWatchTestDB(t)
	saveWatchTestSession(t, db)
	svc := Service{DB: db, VTClient: validFakeVTClient(), Now: fixedWatchNow}
	watch, _, err := svc.Add(context.Background(), CreateAddInput{Term: "202609", CRN: "60058"})
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
	saveWatchTestSession(t, db)
	svc := Service{DB: db, VTClient: validFakeVTClient(), Now: fixedWatchNow}
	watch, _, err := svc.Add(context.Background(), CreateAddInput{Term: "202609", CRN: "60058"})
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

func TestAddRequiresCurrentSession(t *testing.T) {
	svc := Service{DB: newWatchTestDB(t), VTClient: validFakeVTClient(), Now: fixedWatchNow}
	_, _, err := svc.Add(context.Background(), CreateAddInput{Term: "202609", CRN: "60058"})
	if err == nil {
		t.Fatal("Add returned nil error")
	}
}

func TestAddRejectsAlreadyRegisteredCRN(t *testing.T) {
	db := newWatchTestDB(t)
	saveWatchTestSession(t, db)
	client := validFakeVTClient()
	client.studentData.Registered = map[string][]string{"202609": []string{"60058|CS 3304||N|3|UG|misc"}}
	svc := Service{DB: db, VTClient: client, Now: fixedWatchNow}

	_, eval, err := svc.Add(context.Background(), CreateAddInput{Term: "202609", CRN: "60058"})
	if err == nil {
		t.Fatal("Add returned nil error")
	}
	if !hasReject(eval, RejectAlreadyRegisteredAdd) {
		t.Fatalf("HardRejects = %v, want %s", eval.HardRejects, RejectAlreadyRegisteredAdd)
	}
}

func TestAddRejectsCanceledCRN(t *testing.T) {
	db := newWatchTestDB(t)
	saveWatchTestSession(t, db)
	client := validFakeVTClient()
	client.search = foseSearch("60058", "C")
	svc := Service{DB: db, VTClient: client, Now: fixedWatchNow}

	_, eval, err := svc.Add(context.Background(), CreateAddInput{Term: "202609", CRN: "60058"})
	if err == nil {
		t.Fatal("Add returned nil error")
	}
	if !hasReject(eval, RejectSectionCanceled) {
		t.Fatalf("HardRejects = %v, want %s", eval.HardRejects, RejectSectionCanceled)
	}
}

func TestSwapRejectsMissingDropRegistration(t *testing.T) {
	db := newWatchTestDB(t)
	saveWatchTestSession(t, db)
	svc := Service{DB: db, VTClient: validFakeVTClient(), Now: fixedWatchNow}

	_, eval, err := svc.Swap(context.Background(), CreateSwapInput{Term: "202609", AddCRN: "60058", DropCRN: "60900"})
	if err == nil {
		t.Fatal("Swap returned nil error")
	}
	if !hasReject(eval, RejectMissingDropRegistration) {
		t.Fatalf("HardRejects = %v, want %s", eval.HardRejects, RejectMissingDropRegistration)
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

func saveWatchTestSession(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := store.SaveSession(context.Background(), db, store.Session{
		Authtoken:   "token",
		PersID:      "person",
		PersIDProof: "proof",
		CapturedAt:  fixedWatchNow(),
		Status:      "valid",
	}); err != nil {
		t.Fatalf("SaveSession returned error: %v", err)
	}
}

func validFakeVTClient() fakeVTClient {
	return fakeVTClient{
		studentData: studentDataWithTicket(),
		search:      foseSearch("60058", "F"),
	}
}

func fixedWatchNow() time.Time {
	return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
}
