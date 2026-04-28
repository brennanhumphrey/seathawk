package register

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/brennanhumphrey/seathawk/internal/store"
	"github.com/brennanhumphrey/seathawk/internal/storetest"
	"github.com/brennanhumphrey/seathawk/internal/vt"
	"github.com/brennanhumphrey/seathawk/internal/watch"
)

type fakeVTClient struct {
	studentData []vt.StudentData
	search      vt.FoseSearchResponse
	preflight   vt.PreflightResponse
	cart        vt.CartResponse
	register    vt.ShockabsorberResponse
	statuses    []vt.ShockabsorberResponse

	cartAddCalls  int
	registerCalls int
	statusCalls   int
}

func (f *fakeVTClient) StudentData(ctx context.Context, authtoken string) (vt.StudentData, error) {
	if len(f.studentData) == 0 {
		return vt.StudentData{}, nil
	}
	out := f.studentData[0]
	if len(f.studentData) > 1 {
		f.studentData = f.studentData[1:]
	}
	return out, nil
}

func (f *fakeVTClient) SearchByCRN(ctx context.Context, term, crn string) (vt.FoseSearchResponse, error) {
	return f.search, nil
}

func (f *fakeVTClient) Preflight(ctx context.Context, authtoken, term string, crns []string) (vt.PreflightResponse, error) {
	return f.preflight, nil
}

func (f *fakeVTClient) CartAdd(ctx context.Context, input vt.CartAddInput) (vt.CartResponse, error) {
	f.cartAddCalls++
	return f.cart, nil
}

func (f *fakeVTClient) ShockabsorberRegister(ctx context.Context, input vt.ShockabsorberRegisterInput) (vt.ShockabsorberResponse, error) {
	f.registerCalls++
	return f.register, nil
}

func (f *fakeVTClient) ShockabsorberStatus(ctx context.Context, input vt.ShockabsorberStatusInput) (vt.ShockabsorberResponse, error) {
	f.statusCalls++
	if len(f.statuses) == 0 {
		return vt.ShockabsorberResponse{Body: "PROCESSED", Code: 200}, nil
	}
	out := f.statuses[0]
	if len(f.statuses) > 1 {
		f.statuses = f.statuses[1:]
	}
	return out, nil
}

func TestAttemptAddConfirmed(t *testing.T) {
	db := storetest.NewDB(t)
	now := fixedRegisterNow()
	saveRegisterTestSession(t, db, now)
	watchRow := saveRegisterTestWatch(t, db, watch.ModeAdd, now)
	client := validRegisterClient()
	client.studentData = []vt.StudentData{
		studentDataWithRegistration(false),
		studentDataWithRegistration(true),
	}

	result, err := Service{
		DB:                 db,
		VTClient:           client,
		Now:                func() time.Time { return now },
		StatusPollInterval: time.Nanosecond,
		MaxStatusPolls:     2,
	}.AttemptAdd(context.Background(), AttemptAddInput{WatchID: watchRow.ID})
	if err != nil {
		t.Fatalf("AttemptAdd returned error: %v", err)
	}
	if result.Outcome != OutcomeConfirmed || !result.Registered {
		t.Fatalf("result = %+v, want confirmed registered", result)
	}
	if client.cartAddCalls != 1 || client.registerCalls != 1 || client.statusCalls != 1 {
		t.Fatalf("calls cart=%d register=%d status=%d, want 1/1/1", client.cartAddCalls, client.registerCalls, client.statusCalls)
	}
	updated, err := store.WatchByID(context.Background(), db, watchRow.ID)
	if err != nil {
		t.Fatalf("WatchByID returned error: %v", err)
	}
	if updated.Active {
		t.Fatal("confirmed registration did not disable watch")
	}
	if !updated.LastAttemptAt.Valid {
		t.Fatal("LastAttemptAt was not set")
	}
}

func TestAttemptAddSkipsClosedSectionBeforeMutation(t *testing.T) {
	db := storetest.NewDB(t)
	now := fixedRegisterNow()
	saveRegisterTestSession(t, db, now)
	watchRow := saveRegisterTestWatch(t, db, watch.ModeAdd, now)
	client := validRegisterClient()
	client.search.Results[0].Stat = "F"

	result, err := Service{
		DB:       db,
		VTClient: client,
		Now:      func() time.Time { return now },
	}.AttemptAdd(context.Background(), AttemptAddInput{WatchID: watchRow.ID})
	if err != nil {
		t.Fatalf("AttemptAdd returned error: %v", err)
	}
	if result.Outcome != OutcomeSkipped || !strings.Contains(result.Message, "section is not open") {
		t.Fatalf("result = %+v, want skipped section message", result)
	}
	if client.cartAddCalls != 0 || client.registerCalls != 0 {
		t.Fatalf("mutation calls cart=%d register=%d, want none", client.cartAddCalls, client.registerCalls)
	}
}

func TestAttemptAddPreflightBlockStopsBeforeCart(t *testing.T) {
	db := storetest.NewDB(t)
	now := fixedRegisterNow()
	saveRegisterTestSession(t, db, now)
	watchRow := saveRegisterTestWatch(t, db, watch.ModeAdd, now)
	client := validRegisterClient()
	client.preflight = vt.PreflightResponse{
		RegCourseErrors: map[string]string{"60058": "Prerequisite not met"},
	}

	result, err := Service{
		DB:       db,
		VTClient: client,
		Now:      func() time.Time { return now },
	}.AttemptAdd(context.Background(), AttemptAddInput{WatchID: watchRow.ID})
	if err != nil {
		t.Fatalf("AttemptAdd returned error: %v", err)
	}
	if result.Outcome != OutcomeFailed {
		t.Fatalf("outcome = %s, want failed", result.Outcome)
	}
	if client.cartAddCalls != 0 || client.registerCalls != 0 {
		t.Fatalf("mutation calls cart=%d register=%d, want none", client.cartAddCalls, client.registerCalls)
	}
}

func TestAttemptAddUsesCartOptionsWhenHoursHTMLIsMissing(t *testing.T) {
	db := storetest.NewDB(t)
	now := fixedRegisterNow()
	saveRegisterTestSession(t, db, now)
	watchRow := saveRegisterTestWatch(t, db, watch.ModeAdd, now)
	client := validRegisterClient()
	client.studentData = []vt.StudentData{
		studentDataWithRegistration(false),
		studentDataWithRegistration(true),
	}
	client.search.Results[0].HoursHTML = ""
	client.search.Results[0].CartOptions = `{"credit_hrs":{"cartField":"p_hours","enabled":true,"options":[{"value":"3","label":"3","default":true,"selected":"selected"}]}}`

	result, err := Service{
		DB:                 db,
		VTClient:           client,
		Now:                func() time.Time { return now },
		StatusPollInterval: time.Nanosecond,
		MaxStatusPolls:     2,
	}.AttemptAdd(context.Background(), AttemptAddInput{WatchID: watchRow.ID})
	if err != nil {
		t.Fatalf("AttemptAdd returned error: %v", err)
	}
	if result.Outcome != OutcomeConfirmed {
		t.Fatalf("outcome = %s, want confirmed", result.Outcome)
	}
	if client.cartAddCalls != 1 {
		t.Fatalf("cartAddCalls = %d, want 1", client.cartAddCalls)
	}
}

func TestAttemptAddUnsupportedSwapWatch(t *testing.T) {
	db := storetest.NewDB(t)
	now := fixedRegisterNow()
	saveRegisterTestSession(t, db, now)
	watchRow := saveRegisterTestWatch(t, db, watch.ModeSwap, now)

	_, err := Service{
		DB:       db,
		VTClient: validRegisterClient(),
		Now:      func() time.Time { return now },
	}.AttemptAdd(context.Background(), AttemptAddInput{WatchID: watchRow.ID})
	if err == nil {
		t.Fatal("AttemptAdd returned nil error for swap watch")
	}
}

func TestCreditHoursFromSection(t *testing.T) {
	tests := []struct {
		name    string
		section vt.FoseResult
		want    string
		wantErr bool
	}{
		{
			name:    "hours html",
			section: vt.FoseResult{HoursHTML: "3 Credit Hours"},
			want:    "3",
		},
		{
			name: "cart options",
			section: vt.FoseResult{
				CartOptions: `{"credit_hrs":{"options":[{"value":"3","default":true,"selected":"selected"}]}}`,
			},
			want: "3",
		},
		{
			name: "missing",
			section: vt.FoseResult{
				CartOptions: `{"grade_mode":{"options":[{"value":"N","default":true}]}}`,
			},
			wantErr: true,
		},
		{
			name: "malformed cart options",
			section: vt.FoseResult{
				CartOptions: `{`,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := creditHoursFromSection(tt.section)
			if tt.wantErr {
				if err == nil {
					t.Fatal("creditHoursFromSection returned nil error")
				}
				return
			}
			if err != nil {
				t.Fatalf("creditHoursFromSection returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("creditHoursFromSection = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAttemptAddAmbiguousStatusStillConfirmsStudentData(t *testing.T) {
	db := storetest.NewDB(t)
	now := fixedRegisterNow()
	saveRegisterTestSession(t, db, now)
	watchRow := saveRegisterTestWatch(t, db, watch.ModeAdd, now)
	client := validRegisterClient()
	client.studentData = []vt.StudentData{
		studentDataWithRegistration(false),
		studentDataWithRegistration(false),
	}
	client.statuses = []vt.ShockabsorberResponse{
		{Body: "WAIT", Code: 200},
		{Body: "WAIT", Code: 200},
	}

	result, err := Service{
		DB:                 db,
		VTClient:           client,
		Now:                func() time.Time { return now },
		StatusPollInterval: time.Nanosecond,
		MaxStatusPolls:     2,
	}.AttemptAdd(context.Background(), AttemptAddInput{WatchID: watchRow.ID})
	if err != nil {
		t.Fatalf("AttemptAdd returned error: %v", err)
	}
	if result.Outcome != OutcomeAmbiguous || result.Registered {
		t.Fatalf("result = %+v, want ambiguous unregistered", result)
	}
	if client.registerCalls != 1 || client.statusCalls != 2 {
		t.Fatalf("calls register=%d status=%d, want 1/2", client.registerCalls, client.statusCalls)
	}
}

func validRegisterClient() *fakeVTClient {
	return &fakeVTClient{
		studentData: []vt.StudentData{studentDataWithRegistration(false), studentDataWithRegistration(false)},
		search: vt.FoseSearchResponse{
			SrcDB: "202606",
			Count: 1,
			Results: []vt.FoseResult{{
				CRN:       "60058",
				Title:     "Personal Financial Planning",
				Stat:      "A",
				HoursHTML: "3 Credit Hours",
				SrcDB:     "202606",
			}},
		},
		preflight: vt.PreflightResponse{
			RegCourseErrors:    map[string]string{"60058": "||"},
			RegNonCourseErrors: nil,
		},
		cart:     vt.CartResponse{Cart: []string{"202606|default|60058|3||||AAEC 2104||N|||E|||||"}},
		register: vt.ShockabsorberResponse{Body: "WAIT", Code: 200},
		statuses: []vt.ShockabsorberResponse{{Body: "PROCESSED", Code: 200}},
	}
}

func studentDataWithRegistration(registered bool) vt.StudentData {
	data := vt.StudentData{
		Pers: vt.Person{
			ID:      "person-id",
			IDProof: "person-proof",
		},
		Registered: map[string][]string{},
		RegTickets: []vt.RegistrationTicket{{
			Term:      "202606",
			StartDate: "2026-03-17T07:00:00:000000000-04:00",
			EndDate:   "2026-06-23T23:59:00:000000000-04:00",
			Ticket:    "2026-03-17T07:00:00:000000000-04:00|202606|UG|banner|signed",
			Actions:   []string{"add", "drop", "modify"},
		}},
	}
	if registered {
		data.Registered["202606"] = []string{"60058|AAEC 2104||N|3|UG|misc"}
	}
	return data
}

func saveRegisterTestSession(t *testing.T, db *sql.DB, now time.Time) {
	t.Helper()
	if _, err := store.SaveSession(context.Background(), db, store.Session{
		Authtoken:   "token",
		PersID:      "person-id",
		PersIDProof: "person-proof",
		CapturedAt:  now,
		Status:      "valid",
	}); err != nil {
		t.Fatalf("SaveSession returned error: %v", err)
	}
}

func saveRegisterTestWatch(t *testing.T, db *sql.DB, mode string, now time.Time) store.Watch {
	t.Helper()
	watchRow := store.Watch{
		Term:      "202606",
		Mode:      mode,
		AddCRN:    "60058",
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if mode == watch.ModeSwap {
		watchRow.DropCRN = sql.NullString{String: "60900", Valid: true}
	}
	saved, err := store.SaveWatch(context.Background(), db, watchRow)
	if err != nil {
		t.Fatalf("SaveWatch returned error: %v", err)
	}
	return saved
}

func fixedRegisterNow() time.Time {
	return time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)
}
