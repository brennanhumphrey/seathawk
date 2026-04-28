package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/brennanhumphrey/seathawk/internal/store"
	"github.com/brennanhumphrey/seathawk/internal/watch"
)

type fakePoller struct {
	calls   int
	inputs  []watch.PollInput
	reports []watch.PollReport
	errs    []error
}

func (f *fakePoller) PollDue(ctx context.Context, input watch.PollInput) (watch.PollReport, error) {
	f.calls++
	f.inputs = append(f.inputs, input)

	index := f.calls - 1
	var report watch.PollReport
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

func TestRunnerPollsImmediately(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	poller := &fakePoller{}
	sleepCalls := 0
	runner := Runner{
		Poller: poller,
		Logger: &fakeLogger{},
		Sleep: func(ctx context.Context, duration time.Duration) error {
			sleepCalls++
			cancel()
			return ctx.Err()
		},
	}

	if err := runner.Run(ctx); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if poller.calls != 1 {
		t.Fatalf("poll calls = %d, want 1", poller.calls)
	}
	if sleepCalls != 1 {
		t.Fatalf("sleep calls = %d, want 1", sleepCalls)
	}
	if poller.inputs[0].All {
		t.Fatal("daemon poll used All=true")
	}
}

func TestRunnerUsesDefaultInterval(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var slept time.Duration
	runner := Runner{
		Poller: &fakePoller{},
		Logger: &fakeLogger{},
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
		Poller:   &fakePoller{},
		Logger:   &fakeLogger{},
		Interval: want,
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
		Poller: &fakePoller{},
		Logger: &fakeLogger{},
	}.Run(ctx)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}

func TestRunnerContinuesAfterPollError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	poller := &fakePoller{
		errs: []error{errors.New("studentdata failed"), nil},
	}
	sleepCalls := 0
	logger := &fakeLogger{}
	runner := Runner{
		Poller: poller,
		Logger: logger,
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
	if poller.calls != 2 {
		t.Fatalf("poll calls = %d, want 2", poller.calls)
	}
	if !strings.Contains(strings.Join(logger.lines, "\n"), "poll error: studentdata failed") {
		t.Fatalf("logs missing poll error: %v", logger.lines)
	}
}

func TestRunnerRejectsMissingPoller(t *testing.T) {
	err := Runner{Logger: &fakeLogger{}}.Run(context.Background())
	if err == nil {
		t.Fatal("Run returned nil error")
	}
}

func TestLogPollReport(t *testing.T) {
	logger := &fakeLogger{}
	report := watch.PollReport{
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
	}

	logPollReport(logger, report)
	logs := strings.Join(logger.lines, "\n")
	for _, want := range []string{
		"poll checked_at=2026-04-28T14:00:00Z watches=2",
		"id=1",
		"status=open",
		"id=2",
		"error=watch rejected: crn_not_found",
	} {
		if !strings.Contains(logs, want) {
			t.Fatalf("logs missing %q:\n%s", want, logs)
		}
	}
}

func TestLogPollReportEmpty(t *testing.T) {
	logger := &fakeLogger{}
	logPollReport(logger, watch.PollReport{CheckedAt: time.Date(2026, 4, 28, 14, 0, 0, 0, time.UTC)})

	if got := strings.Join(logger.lines, "\n"); !strings.Contains(got, "watches=0") {
		t.Fatalf("logs missing empty poll count: %s", got)
	}
}
