package automation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/brennanhumphrey/seathawk/internal/register"
	"github.com/brennanhumphrey/seathawk/internal/store"
	"github.com/brennanhumphrey/seathawk/internal/watch"
)

type fakePoller struct {
	calls  int
	report watch.PollReport
	err    error
}

func (f *fakePoller) PollDue(ctx context.Context, input watch.PollInput) (watch.PollReport, error) {
	f.calls++
	return f.report, f.err
}

type fakeRegisterer struct {
	calls   int
	inputs  []register.AttemptAddInput
	results []register.AttemptAddResult
	errs    []error
}

func (f *fakeRegisterer) AttemptAdd(ctx context.Context, input register.AttemptAddInput) (register.AttemptAddResult, error) {
	f.calls++
	f.inputs = append(f.inputs, input)
	index := f.calls - 1

	var result register.AttemptAddResult
	if index < len(f.results) {
		result = f.results[index]
	}
	var err error
	if index < len(f.errs) {
		err = f.errs[index]
	}
	return result, err
}

type fakeDisabler struct {
	calls int
	ids   []int64
	err   error
}

func (f *fakeDisabler) Disable(ctx context.Context, id int64) (store.Watch, error) {
	f.calls++
	f.ids = append(f.ids, id)
	return store.Watch{ID: id, Active: false}, f.err
}

func TestProcessDueReadOnlyDoesNotAttempt(t *testing.T) {
	poller := &fakePoller{report: pollReport(openReadyAddWatch(7))}
	registerer := &fakeRegisterer{}

	report, err := Service{Poller: poller, Registerer: registerer}.ProcessDue(context.Background(), ProcessInput{})
	if err != nil {
		t.Fatalf("ProcessDue returned error: %v", err)
	}
	if poller.calls != 1 {
		t.Fatalf("poll calls = %d, want 1", poller.calls)
	}
	if registerer.calls != 0 {
		t.Fatalf("register calls = %d, want 0", registerer.calls)
	}
	if len(report.Attempts) != 0 {
		t.Fatalf("attempt count = %d, want 0", len(report.Attempts))
	}
}

func TestProcessDueAutoAttemptsEligibleAddWatch(t *testing.T) {
	poller := &fakePoller{report: pollReport(openReadyAddWatch(7))}
	registerer := &fakeRegisterer{results: []register.AttemptAddResult{{
		Watch:      store.Watch{ID: 7, Active: false},
		Outcome:    register.OutcomeConfirmed,
		Registered: true,
		Message:    "registration confirmed in studentdata",
	}}}
	disabler := &fakeDisabler{}

	report, err := Service{Poller: poller, Registerer: registerer, Disabler: disabler}.ProcessDue(context.Background(), ProcessInput{AutoRegister: true})
	if err != nil {
		t.Fatalf("ProcessDue returned error: %v", err)
	}
	if registerer.calls != 1 || registerer.inputs[0].WatchID != 7 {
		t.Fatalf("register calls/input = %d/%+v, want watch 7", registerer.calls, registerer.inputs)
	}
	if disabler.calls != 0 {
		t.Fatalf("disable calls = %d, want 0 because confirmed attempts disable inside register.Service", disabler.calls)
	}
	if len(report.Attempts) != 1 || report.Attempts[0].Result.Outcome != register.OutcomeConfirmed {
		t.Fatalf("unexpected attempts: %+v", report.Attempts)
	}
}

func TestProcessDueSkipsIneligiblePollResults(t *testing.T) {
	poller := &fakePoller{report: watch.PollReport{
		CheckedAt: fixedAutomationNow(),
		Results: []watch.PollResult{
			openReadyAddWatch(1),
			{Watch: store.Watch{ID: 2, Active: true, Mode: watch.ModeAdd}, Evaluation: watch.Evaluation{SectionStatus: watch.SectionFull, Window: watch.WindowReady}},
			{Watch: store.Watch{ID: 3, Active: true, Mode: watch.ModeAdd}, Evaluation: watch.Evaluation{SectionStatus: watch.SectionOpen, Window: watch.WindowNotOpen}},
			{Watch: store.Watch{ID: 4, Active: true, Mode: watch.ModeSwap}, Evaluation: watch.Evaluation{SectionStatus: watch.SectionOpen, Window: watch.WindowReady}},
			{Watch: store.Watch{ID: 5, Active: false, Mode: watch.ModeAdd}, Evaluation: watch.Evaluation{SectionStatus: watch.SectionOpen, Window: watch.WindowReady}},
			{Watch: store.Watch{ID: 6, Active: true, Mode: watch.ModeAdd}, Evaluation: watch.Evaluation{SectionStatus: watch.SectionOpen, Window: watch.WindowReady}, Err: errors.New("poll failed")},
		},
	}}
	registerer := &fakeRegisterer{results: []register.AttemptAddResult{{Watch: store.Watch{ID: 1}, Outcome: register.OutcomeConfirmed, Registered: true}}}

	_, err := Service{Poller: poller, Registerer: registerer, Disabler: &fakeDisabler{}}.ProcessDue(context.Background(), ProcessInput{AutoRegister: true})
	if err != nil {
		t.Fatalf("ProcessDue returned error: %v", err)
	}
	if registerer.calls != 1 || registerer.inputs[0].WatchID != 1 {
		t.Fatalf("register calls/input = %d/%+v, want only watch 1", registerer.calls, registerer.inputs)
	}
}

func TestProcessDueContinuesAfterAttemptError(t *testing.T) {
	poller := &fakePoller{report: watch.PollReport{
		CheckedAt: fixedAutomationNow(),
		Results: []watch.PollResult{
			openReadyAddWatch(7),
			openReadyAddWatch(8),
		},
	}}
	registerer := &fakeRegisterer{
		results: []register.AttemptAddResult{
			{Watch: store.Watch{ID: 7}},
			{Watch: store.Watch{ID: 8}, Outcome: register.OutcomeConfirmed, Registered: true},
		},
		errs: []error{errors.New("register failed"), nil},
	}

	report, err := Service{Poller: poller, Registerer: registerer, Disabler: &fakeDisabler{}}.ProcessDue(context.Background(), ProcessInput{AutoRegister: true})
	if err != nil {
		t.Fatalf("ProcessDue returned error: %v", err)
	}
	if registerer.calls != 2 {
		t.Fatalf("register calls = %d, want 2", registerer.calls)
	}
	if len(report.Attempts) != 2 || report.Attempts[0].Err == nil || report.Attempts[1].Err != nil {
		t.Fatalf("unexpected attempts: %+v", report.Attempts)
	}
}

func TestProcessDueDisablesFailedAndAmbiguousAutoAttempts(t *testing.T) {
	poller := &fakePoller{report: watch.PollReport{
		CheckedAt: fixedAutomationNow(),
		Results: []watch.PollResult{
			openReadyAddWatch(7),
			openReadyAddWatch(8),
		},
	}}
	registerer := &fakeRegisterer{results: []register.AttemptAddResult{
		{Watch: store.Watch{ID: 7, Active: true}, Outcome: register.OutcomeFailed},
		{Watch: store.Watch{ID: 8, Active: true}, Outcome: register.OutcomeAmbiguous},
	}}
	disabler := &fakeDisabler{}

	report, err := Service{Poller: poller, Registerer: registerer, Disabler: disabler}.ProcessDue(context.Background(), ProcessInput{AutoRegister: true})
	if err != nil {
		t.Fatalf("ProcessDue returned error: %v", err)
	}
	if disabler.calls != 2 || disabler.ids[0] != 7 || disabler.ids[1] != 8 {
		t.Fatalf("disabled ids = %+v, want [7 8]", disabler.ids)
	}
	if report.Attempts[0].Watch.Active || report.Attempts[1].Watch.Active {
		t.Fatalf("attempt watches were not marked disabled: %+v", report.Attempts)
	}
}

func TestProcessDueReturnsPollError(t *testing.T) {
	_, err := Service{Poller: &fakePoller{err: errors.New("studentdata failed")}}.ProcessDue(context.Background(), ProcessInput{})
	if err == nil {
		t.Fatal("ProcessDue returned nil error")
	}
}

func TestProcessDueValidatesDependencies(t *testing.T) {
	if _, err := (Service{}).ProcessDue(context.Background(), ProcessInput{}); err == nil {
		t.Fatal("ProcessDue returned nil error for missing poller")
	}
	if _, err := (Service{Poller: &fakePoller{}}).ProcessDue(context.Background(), ProcessInput{AutoRegister: true}); err == nil {
		t.Fatal("ProcessDue returned nil error for missing registerer")
	}
	if _, err := (Service{Poller: &fakePoller{}, Registerer: &fakeRegisterer{}}).ProcessDue(context.Background(), ProcessInput{AutoRegister: true}); err == nil {
		t.Fatal("ProcessDue returned nil error for missing disabler")
	}
}

func pollReport(results ...watch.PollResult) watch.PollReport {
	return watch.PollReport{CheckedAt: fixedAutomationNow(), Results: results}
}

func openReadyAddWatch(id int64) watch.PollResult {
	return watch.PollResult{
		Watch: store.Watch{ID: id, Active: true, Mode: watch.ModeAdd, Term: "202609", AddCRN: "60058"},
		Evaluation: watch.Evaluation{
			SectionStatus: watch.SectionOpen,
			Window:        watch.WindowReady,
		},
	}
}

func fixedAutomationNow() time.Time {
	return time.Date(2026, 4, 28, 14, 0, 0, 0, time.UTC)
}
