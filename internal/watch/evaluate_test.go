package watch

import (
	"testing"
	"time"

	"github.com/brennanhumphrey/seathawk/internal/store"
	"github.com/brennanhumphrey/seathawk/internal/vt"
)

func TestEvaluateSectionStatuses(t *testing.T) {
	tests := []struct {
		name       string
		stat       string
		wantStatus SectionStatus
		wantReject RejectReason
	}{
		{name: "open", stat: "A", wantStatus: SectionOpen},
		{name: "full", stat: "F", wantStatus: SectionFull},
		{name: "waitlist", stat: "W", wantStatus: SectionWaitlist},
		{name: "canceled", stat: "C", wantStatus: SectionCanceled, wantReject: RejectSectionCanceled},
		{name: "unknown", stat: "Q", wantStatus: SectionUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EvaluateFromSnapshots(addWatch("60058"), studentDataWithTicket(), foseSearch("60058", tt.stat), fixedWatchNow())
			if err != nil {
				t.Fatalf("EvaluateFromSnapshots returned error: %v", err)
			}
			if got.SectionStatus != tt.wantStatus {
				t.Fatalf("SectionStatus = %q, want %q", got.SectionStatus, tt.wantStatus)
			}
			if tt.wantReject != "" && !hasReject(got, tt.wantReject) {
				t.Fatalf("HardRejects = %v, want %s", got.HardRejects, tt.wantReject)
			}
			if tt.wantReject == "" && got.HasHardRejects() {
				t.Fatalf("unexpected hard rejects: %v", got.HardRejects)
			}
		})
	}
}

func TestEvaluateCRNNotFound(t *testing.T) {
	got, err := EvaluateFromSnapshots(addWatch("60058"), studentDataWithTicket(), vt.FoseSearchResponse{}, fixedWatchNow())
	if err != nil {
		t.Fatalf("EvaluateFromSnapshots returned error: %v", err)
	}
	if !hasReject(got, RejectCRNNotFound) {
		t.Fatalf("HardRejects = %v, want %s", got.HardRejects, RejectCRNNotFound)
	}
}

func TestEvaluateAlreadyRegisteredAdd(t *testing.T) {
	studentData := studentDataWithTicket()
	studentData.Registered = map[string][]string{"202609": []string{"60058|CS 3304||N|3|UG|misc"}}

	got, err := EvaluateFromSnapshots(addWatch("60058"), studentData, foseSearch("60058", "F"), fixedWatchNow())
	if err != nil {
		t.Fatalf("EvaluateFromSnapshots returned error: %v", err)
	}
	if !hasReject(got, RejectAlreadyRegisteredAdd) {
		t.Fatalf("HardRejects = %v, want %s", got.HardRejects, RejectAlreadyRegisteredAdd)
	}
}

func TestEvaluateSwapDropRegistration(t *testing.T) {
	tests := []struct {
		name       string
		registered []string
		wantReject bool
	}{
		{name: "drop registered", registered: []string{"60900|CS 3304||N|3|UG|misc"}},
		{name: "drop missing", registered: nil, wantReject: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			studentData := studentDataWithTicket()
			studentData.Registered = map[string][]string{"202609": tt.registered}

			got, err := EvaluateFromSnapshots(swapWatch("60058", "60900"), studentData, foseSearch("60058", "F"), fixedWatchNow())
			if err != nil {
				t.Fatalf("EvaluateFromSnapshots returned error: %v", err)
			}
			if tt.wantReject && !hasReject(got, RejectMissingDropRegistration) {
				t.Fatalf("HardRejects = %v, want %s", got.HardRejects, RejectMissingDropRegistration)
			}
			if !tt.wantReject && hasReject(got, RejectMissingDropRegistration) {
				t.Fatalf("unexpected missing drop reject: %v", got.HardRejects)
			}
		})
	}
}

func TestEvaluateWindowStatus(t *testing.T) {
	tests := []struct {
		name    string
		tickets []vt.RegistrationTicket
		mode    string
		want    WindowStatus
	}{
		{name: "no ticket", tickets: nil, mode: ModeAdd, want: WindowNoTicket},
		{name: "not open", tickets: []vt.RegistrationTicket{ticket("2026-04-27T12:01:00:000000000+00:00", "2026-04-27T13:00:00:000000000+00:00", "add")}, mode: ModeAdd, want: WindowNotOpen},
		{name: "closed", tickets: []vt.RegistrationTicket{ticket("2026-04-27T10:00:00:000000000+00:00", "2026-04-27T11:00:00:000000000+00:00", "add")}, mode: ModeAdd, want: WindowClosed},
		{name: "ready add", tickets: []vt.RegistrationTicket{ticket("2026-04-27T11:00:00:000000000+00:00", "2026-04-27T13:00:00:000000000+00:00", "add")}, mode: ModeAdd, want: WindowReady},
		{name: "ready swap", tickets: []vt.RegistrationTicket{ticket("2026-04-27T11:00:00:000000000+00:00", "2026-04-27T13:00:00:000000000+00:00", "add", "drop")}, mode: ModeSwap, want: WindowReady},
		{name: "action unavailable", tickets: []vt.RegistrationTicket{ticket("2026-04-27T11:00:00:000000000+00:00", "2026-04-27T13:00:00:000000000+00:00", "regopt")}, mode: ModeAdd, want: WindowActionUnavailable},
	}

	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateWindow(vt.StudentData{RegTickets: tt.tickets}, "202609", tt.mode, now)
			if got != tt.want {
				t.Fatalf("evaluateWindow = %q, want %q", got, tt.want)
			}
		})
	}
}

func addWatch(crn string) store.Watch {
	return store.Watch{Term: "202609", Mode: ModeAdd, AddCRN: crn}
}

func swapWatch(addCRN, dropCRN string) store.Watch {
	watch := addWatch(addCRN)
	watch.Mode = ModeSwap
	watch.DropCRN.Valid = true
	watch.DropCRN.String = dropCRN
	return watch
}

func studentDataWithTicket() vt.StudentData {
	return vt.StudentData{
		Registered: map[string][]string{},
		RegTickets: []vt.RegistrationTicket{
			ticket("2026-04-26T11:00:00:000000000+00:00", "2026-04-26T13:00:00:000000000+00:00", "add", "drop"),
		},
	}
}

func foseSearch(crn, stat string) vt.FoseSearchResponse {
	return vt.FoseSearchResponse{
		Results: []vt.FoseResult{{CRN: crn, Stat: stat, Title: "Software Engineering"}},
	}
}

func ticket(start, end string, actions ...string) vt.RegistrationTicket {
	return vt.RegistrationTicket{Term: "202609", StartDate: start, EndDate: end, Actions: actions}
}

func hasReject(eval Evaluation, reject RejectReason) bool {
	for _, got := range eval.HardRejects {
		if got == reject {
			return true
		}
	}
	return false
}
