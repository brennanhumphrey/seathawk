package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/brennanhumphrey/seathawk/internal/automation"
	"github.com/brennanhumphrey/seathawk/internal/register"
	"github.com/brennanhumphrey/seathawk/internal/store"
	"github.com/brennanhumphrey/seathawk/internal/watch"
)

type fakeProcessor struct {
	calls   int
	inputs  []automation.ProcessInput
	reports []automation.ProcessReport
	errs    []error
}

func (f *fakeProcessor) ProcessDue(ctx context.Context, input automation.ProcessInput) (automation.ProcessReport, error) {
	f.calls++
	f.inputs = append(f.inputs, input)

	index := f.calls - 1
	var report automation.ProcessReport
	if index < len(f.reports) {
		report = f.reports[index]
	}
	var err error
	if index < len(f.errs) {
		err = f.errs[index]
	}
	return report, err
}

type fakeLogger struct {
	lines []string
}

func (l *fakeLogger) Printf(format string, args ...any) {
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}

func TestRunnerProcessesImmediately(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	processor := &fakeProcessor{}
	sleepCalls := 0
	runner := Runner{
		Processor: processor,
		Logger:    &fakeLogger{},
		Sleep: func(ctx context.Context, duration time.Duration) error {
			sleepCalls++
			cancel()
			return ctx.Err()
		},
	}

	if err := runner.Run(ctx); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if processor.calls != 1 {
		t.Fatalf("process calls = %d, want 1", processor.calls)
	}
	if sleepCalls != 1 {
		t.Fatalf("sleep calls = %d, want 1", sleepCalls)
	}
	if processor.inputs[0].AutoRegister {
		t.Fatal("daemon passed AutoRegister=true by default")
	}
}

func TestRunnerPassesAutoRegister(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	processor := &fakeProcessor{}
	runner := Runner{
		Processor:    processor,
		AutoRegister: true,
		Logger:       &fakeLogger{},
		Sleep: func(ctx context.Context, duration time.Duration) error {
			cancel()
			return ctx.Err()
		},
	}

	if err := runner.Run(ctx); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !processor.inputs[0].AutoRegister {
		t.Fatal("daemon did not pass AutoRegister=true")
	}
}

func TestRunnerUsesDefaultInterval(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var slept time.Duration
	runner := Runner{
		Processor: &fakeProcessor{},
		Logger:    &fakeLogger{},
		Sleep: func(ctx context.Context, duration time.Duration) error {
			slept = duration
			cancel()
			return ctx.Err()
		},
	}

	if err := runner.Run(ctx); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if slept != watch.DefaultPollInterval {
		t.Fatalf("sleep duration = %s, want %s", slept, watch.DefaultPollInterval)
	}
}

func TestRunnerUsesConfiguredInterval(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	want := 5 * time.Second
	var slept time.Duration
	runner := Runner{
		Processor: &fakeProcessor{},
		Logger:    &fakeLogger{},
		Interval:  want,
		Sleep: func(ctx context.Context, duration time.Duration) error {
			slept = duration
			cancel()
			return ctx.Err()
		},
	}

	if err := runner.Run(ctx); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if slept != want {
		t.Fatalf("sleep duration = %s, want %s", slept, want)
	}
}

func TestRunnerStopsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Runner{
		Processor: &fakeProcessor{},
		Logger:    &fakeLogger{},
	}.Run(ctx)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}

func TestRunnerContinuesAfterProcessError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	processor := &fakeProcessor{
		errs: []error{errors.New("studentdata failed"), nil},
	}
	sleepCalls := 0
	logger := &fakeLogger{}
	runner := Runner{
		Processor: processor,
		Logger:    logger,
		Sleep: func(ctx context.Context, duration time.Duration) error {
			sleepCalls++
			if sleepCalls == 2 {
				cancel()
				return ctx.Err()
			}
			return nil
		},
	}

	if err := runner.Run(ctx); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if processor.calls != 2 {
		t.Fatalf("process calls = %d, want 2", processor.calls)
	}
	if !strings.Contains(strings.Join(logger.lines, "\n"), "poll error: studentdata failed") {
		t.Fatalf("logs missing poll error: %v", logger.lines)
	}
}

func TestRunnerRejectsMissingProcessor(t *testing.T) {
	err := Runner{Logger: &fakeLogger{}}.Run(context.Background())
	if err == nil {
		t.Fatal("Run returned nil error")
	}
}

func TestLogProcessReport(t *testing.T) {
	logger := &fakeLogger{}
	report := automation.ProcessReport{
		PollReport: watch.PollReport{
			CheckedAt: time.Date(2026, 4, 28, 14, 0, 0, 0, time.UTC),
			Results: []watch.PollResult{
				{
					Watch: store.Watch{ID: 1, Active: true, Mode: watch.ModeAdd, Term: "202609", AddCRN: "60058"},
					Evaluation: watch.Evaluation{
						SectionStatus: watch.SectionOpen,
					},
				},
				{
					Watch: store.Watch{ID: 2, Active: false, Mode: watch.ModeAdd, Term: "202609", AddCRN: "60059"},
					Err:   errors.New("watch rejected: crn_not_found"),
				},
			},
		},
		Attempts: []automation.AttemptResult{
			{
				Watch: store.Watch{ID: 1},
				Result: register.AttemptAddResult{
					Outcome:    register.OutcomeConfirmed,
					Registered: true,
					Message:    "registration confirmed in studentdata",
				},
			},
			{Watch: store.Watch{ID: 3}, Err: errors.New("attempt failed")},
		},
	}

	logProcessReport(logger, report)
	logs := strings.Join(logger.lines, "\n")
	for _, want := range []string{
		"poll checked_at=2026-04-28T14:00:00Z watches=2",
		"id=1",
		"status=open",
		"id=2",
		"error=watch rejected: crn_not_found",
		"attempt watch id=1 outcome=confirmed registered=true",
		"message=\"registration confirmed in studentdata\"",
		"attempt watch id=3 error=attempt failed",
	} {
		if !strings.Contains(logs, want) {
			t.Fatalf("logs missing %q:\n%s", want, logs)
		}
	}
}

func TestLogProcessReportEmpty(t *testing.T) {
	logger := &fakeLogger{}
	logProcessReport(logger, automation.ProcessReport{
		PollReport: watch.PollReport{CheckedAt: time.Date(2026, 4, 28, 14, 0, 0, 0, time.UTC)},
	})

	if got := strings.Join(logger.lines, "\n"); !strings.Contains(got, "watches=0") {
		t.Fatalf("logs missing empty poll count: %s", got)
	}
}
