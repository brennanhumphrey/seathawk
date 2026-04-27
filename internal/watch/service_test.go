package watch

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/brennanhumphrey/seathawk/internal/store"
	"github.com/brennanhumphrey/seathawk/internal/storetest"
	"github.com/brennanhumphrey/seathawk/internal/vt"
)

type fakeVTClient struct {
	studentData      vt.StudentData
	search           vt.FoseSearchResponse
	searchByCRN      map[string]vt.FoseSearchResponse
	studentDataCalls int
	searchCalls      int
	err              error
	searchErrByCRN   map[string]error
}

func (f *fakeVTClient) StudentData(ctx context.Context, authtoken string) (vt.StudentData, error) {
	f.studentDataCalls++
	return f.studentData, f.err
}

func (f *fakeVTClient) SearchByCRN(ctx context.Context, term, crn string) (vt.FoseSearchResponse, error) {
	f.searchCalls++
	if err := f.searchErrByCRN[crn]; err != nil {
		return vt.FoseSearchResponse{}, err
	}
	if search, ok := f.searchByCRN[crn]; ok {
		return search, f.err
	}
	return f.search, f.err
}

func TestAddCreatesActiveWatch(t *testing.T) {
	db := storetest.NewDB(t)
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
	db := storetest.NewDB(t)
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
			svc := Service{DB: storetest.NewDB(t), Now: fixedWatchNow}
			if err := tt.call(svc); err == nil {
				t.Fatal("call returned nil error")
			}
		})
	}
}

func TestServiceTrimsInput(t *testing.T) {
	db := storetest.NewDB(t)
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
	db := storetest.NewDB(t)
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
	db := storetest.NewDB(t)
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
	svc := Service{DB: storetest.NewDB(t), VTClient: validFakeVTClient(), Now: fixedWatchNow}
	_, _, err := svc.Add(context.Background(), CreateAddInput{Term: "202609", CRN: "60058"})
	if err == nil {
		t.Fatal("Add returned nil error")
	}
}

func TestAddRejectsAlreadyRegisteredCRN(t *testing.T) {
	db := storetest.NewDB(t)
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
	db := storetest.NewDB(t)
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

func TestAddRejectDoesNotSaveWatch(t *testing.T) {
	db := storetest.NewDB(t)
	saveWatchTestSession(t, db)
	client := validFakeVTClient()
	client.search = vt.FoseSearchResponse{}
	svc := Service{DB: db, VTClient: client, Now: fixedWatchNow}

	_, eval, err := svc.Add(context.Background(), CreateAddInput{Term: "202609", CRN: "60058"})
	if err == nil {
		t.Fatal("Add returned nil error")
	}
	if !hasReject(eval, RejectCRNNotFound) {
		t.Fatalf("HardRejects = %v, want %s", eval.HardRejects, RejectCRNNotFound)
	}

	watches, err := store.ListWatches(context.Background(), db)
	if err != nil {
		t.Fatalf("ListWatches returned error: %v", err)
	}
	if len(watches) != 0 {
		t.Fatalf("saved watch count = %d, want 0: %+v", len(watches), watches)
	}
}

func TestSwapRejectsMissingDropRegistration(t *testing.T) {
	db := storetest.NewDB(t)
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

func TestPollDueEmpty(t *testing.T) {
	db := storetest.NewDB(t)
	saveWatchTestSession(t, db)
	svc := Service{DB: db, VTClient: validFakeVTClient(), Now: fixedWatchNow}

	report, err := svc.PollDue(context.Background(), PollInput{})
	if err != nil {
		t.Fatalf("PollDue returned error: %v", err)
	}
	if len(report.Results) != 0 {
		t.Fatalf("result count = %d, want 0", len(report.Results))
	}
}

func TestPollDueEvaluatesDueWatches(t *testing.T) {
	db := storetest.NewDB(t)
	saveWatchTestSession(t, db)
	now := fixedWatchNow()
	watch := saveWatchForPoll(t, db, store.Watch{
		Term:       "202609",
		Mode:       ModeAdd,
		AddCRN:     "60058",
		Active:     true,
		NextPollAt: sql.NullTime{Time: now.Add(-time.Minute), Valid: true},
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	client := validFakeVTClient()
	svc := Service{DB: db, VTClient: client, Now: fixedWatchNow}

	report, err := svc.PollDue(context.Background(), PollInput{})
	if err != nil {
		t.Fatalf("PollDue returned error: %v", err)
	}
	if len(report.Results) != 1 {
		t.Fatalf("result count = %d, want 1", len(report.Results))
	}
	if report.Results[0].Watch.ID != watch.ID || report.Results[0].Evaluation.SectionStatus != SectionFull {
		t.Fatalf("unexpected poll result: %+v", report.Results[0])
	}
	if client.studentDataCalls != 1 {
		t.Fatalf("studentdata calls = %d, want 1", client.studentDataCalls)
	}
	if !report.Results[0].Watch.LastSeenStat.Valid || report.Results[0].Watch.LastSeenStat.String != string(SectionFull) {
		t.Fatalf("LastSeenStat = %+v, want full", report.Results[0].Watch.LastSeenStat)
	}
	if !report.Results[0].Watch.NextPollAt.Valid || !report.Results[0].Watch.NextPollAt.Time.Equal(now.Add(30*time.Second)) {
		t.Fatalf("NextPollAt = %+v, want %s", report.Results[0].Watch.NextPollAt, now.Add(30*time.Second))
	}
}

func TestPollDueFetchesStudentDataOnceForMultipleWatches(t *testing.T) {
	db := storetest.NewDB(t)
	saveWatchTestSession(t, db)
	now := fixedWatchNow()
	saveWatchForPoll(t, db, store.Watch{Term: "202609", Mode: ModeAdd, AddCRN: "60058", Active: true, CreatedAt: now, UpdatedAt: now})
	saveWatchForPoll(t, db, store.Watch{Term: "202609", Mode: ModeAdd, AddCRN: "60059", Active: true, CreatedAt: now, UpdatedAt: now})
	client := validFakeVTClient()
	client.searchByCRN = map[string]vt.FoseSearchResponse{
		"60058": foseSearch("60058", "F"),
		"60059": foseSearch("60059", "A"),
	}
	svc := Service{DB: db, VTClient: client, Now: fixedWatchNow}

	report, err := svc.PollDue(context.Background(), PollInput{})
	if err != nil {
		t.Fatalf("PollDue returned error: %v", err)
	}
	if len(report.Results) != 2 {
		t.Fatalf("result count = %d, want 2", len(report.Results))
	}
	if client.studentDataCalls != 1 {
		t.Fatalf("studentdata calls = %d, want 1", client.studentDataCalls)
	}
	if client.searchCalls != 2 {
		t.Fatalf("search calls = %d, want 2", client.searchCalls)
	}
}

func TestPollDueAllIncludesFutureActiveWatches(t *testing.T) {
	db := storetest.NewDB(t)
	saveWatchTestSession(t, db)
	now := fixedWatchNow()
	saveWatchForPoll(t, db, store.Watch{
		Term:       "202609",
		Mode:       ModeAdd,
		AddCRN:     "60058",
		Active:     true,
		NextPollAt: sql.NullTime{Time: now.Add(time.Hour), Valid: true},
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	svc := Service{DB: db, VTClient: validFakeVTClient(), Now: fixedWatchNow}

	report, err := svc.PollDue(context.Background(), PollInput{All: true})
	if err != nil {
		t.Fatalf("PollDue returned error: %v", err)
	}
	if len(report.Results) != 1 {
		t.Fatalf("result count = %d, want 1", len(report.Results))
	}
}

func TestPollDueSkipsDisabledWatches(t *testing.T) {
	db := storetest.NewDB(t)
	saveWatchTestSession(t, db)
	now := fixedWatchNow()
	saveWatchForPoll(t, db, store.Watch{Term: "202609", Mode: ModeAdd, AddCRN: "60058", Active: false, CreatedAt: now, UpdatedAt: now})
	client := validFakeVTClient()
	svc := Service{DB: db, VTClient: client, Now: fixedWatchNow}

	report, err := svc.PollDue(context.Background(), PollInput{All: true})
	if err != nil {
		t.Fatalf("PollDue returned error: %v", err)
	}
	if len(report.Results) != 0 {
		t.Fatalf("result count = %d, want 0", len(report.Results))
	}
	if client.studentDataCalls != 0 {
		t.Fatalf("studentdata calls = %d, want 0", client.studentDataCalls)
	}
}

func TestPollDueContinuesAfterPerWatchError(t *testing.T) {
	db := storetest.NewDB(t)
	saveWatchTestSession(t, db)
	now := fixedWatchNow()
	saveWatchForPoll(t, db, store.Watch{Term: "202609", Mode: ModeAdd, AddCRN: "60058", Active: true, CreatedAt: now, UpdatedAt: now})
	saveWatchForPoll(t, db, store.Watch{Term: "202609", Mode: ModeAdd, AddCRN: "60059", Active: true, CreatedAt: now, UpdatedAt: now})
	client := validFakeVTClient()
	client.searchErrByCRN = map[string]error{"60058": errors.New("upstream failed")}
	client.searchByCRN = map[string]vt.FoseSearchResponse{"60059": foseSearch("60059", "A")}
	svc := Service{DB: db, VTClient: client, Now: fixedWatchNow}

	report, err := svc.PollDue(context.Background(), PollInput{})
	if err != nil {
		t.Fatalf("PollDue returned error: %v", err)
	}
	if len(report.Results) != 2 {
		t.Fatalf("result count = %d, want 2", len(report.Results))
	}
	if report.Results[0].Err == nil {
		t.Fatal("first result error is nil")
	}
	if report.Results[1].Err != nil {
		t.Fatalf("second result error = %v, want nil", report.Results[1].Err)
	}
	if !report.Results[0].Watch.NextPollAt.Valid {
		t.Fatalf("errored watch was not rescheduled: %+v", report.Results[0].Watch)
	}
}

func TestPollWatchSurfacesRescheduleWriteError(t *testing.T) {
	db := storetest.NewDB(t)
	now := fixedWatchNow()
	watch := saveWatchForPoll(t, db, store.Watch{
		Term:      "202609",
		Mode:      ModeAdd,
		AddCRN:    "60058",
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err := db.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	client := validFakeVTClient()
	client.searchErrByCRN = map[string]error{"60058": errors.New("upstream failed")}
	svc := Service{DB: db, VTClient: client, Now: fixedWatchNow}

	result := svc.pollWatch(context.Background(), watch, studentDataWithTicket(), now)
	if result.Err == nil {
		t.Fatal("pollWatch returned nil error")
	}
	if !strings.Contains(result.Err.Error(), "also failed to reschedule watch") {
		t.Fatalf("error = %v, want reschedule write failure context", result.Err)
	}
}

func TestPollDueDisablesHardRejectedWatch(t *testing.T) {
	db := storetest.NewDB(t)
	saveWatchTestSession(t, db)
	now := fixedWatchNow()
	watch := saveWatchForPoll(t, db, store.Watch{
		Term:       "202609",
		Mode:       ModeAdd,
		AddCRN:     "60058",
		Active:     true,
		NextPollAt: sql.NullTime{Time: now.Add(-time.Minute), Valid: true},
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	client := validFakeVTClient()
	client.search = vt.FoseSearchResponse{}
	svc := Service{DB: db, VTClient: client, Now: fixedWatchNow}

	report, err := svc.PollDue(context.Background(), PollInput{})
	if err != nil {
		t.Fatalf("PollDue returned error: %v", err)
	}
	if len(report.Results) != 1 {
		t.Fatalf("result count = %d, want 1", len(report.Results))
	}
	if report.Results[0].Err == nil {
		t.Fatal("poll result error is nil")
	}
	if report.Results[0].Watch.Active {
		t.Fatalf("rejected watch remains active: %+v", report.Results[0].Watch)
	}
	if report.Results[0].Watch.NextPollAt.Valid {
		t.Fatalf("rejected watch still has next poll time: %+v", report.Results[0].Watch.NextPollAt)
	}

	updated, err := store.WatchByID(context.Background(), db, watch.ID)
	if err != nil {
		t.Fatalf("WatchByID returned error: %v", err)
	}
	if updated.Active {
		t.Fatalf("stored watch remains active: %+v", updated)
	}
}

func saveWatchForPoll(t *testing.T, db *sql.DB, watch store.Watch) store.Watch {
	t.Helper()
	saved, err := store.SaveWatch(context.Background(), db, watch)
	if err != nil {
		t.Fatalf("SaveWatch returned error: %v", err)
	}
	return saved
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

func validFakeVTClient() *fakeVTClient {
	return &fakeVTClient{
		studentData: studentDataWithTicket(),
		search:      foseSearch("60058", "F"),
	}
}

func fixedWatchNow() time.Time {
	return time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
}
