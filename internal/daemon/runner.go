// Package daemon owns SeatHawk's process-level polling loop.
//
// The daemon coordinates timing, logging, and shutdown. It deliberately delegates
// VT/domain decisions to internal/automation so this package never grows
// registration behavior of its own.
package daemon

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/brennanhumphrey/seathawk/internal/automation"
	"github.com/brennanhumphrey/seathawk/internal/watch"
)

// Processor is the per-pass automation behavior the daemon needs.
type Processor interface {
	ProcessDue(ctx context.Context, input automation.ProcessInput) (automation.ProcessReport, error)
}

// Logger is the logging behavior the daemon needs.
type Logger interface {
	Printf(format string, args ...any)
}

// Runner repeatedly performs daemon work until its context ends.
//
// By default the work is read-only polling. When AutoRegister is true, Runner
// passes that opt-in to the automation layer, which may attempt eligible add
// watches through the registration service.
type Runner struct {
	Processor    Processor
	AutoRegister bool
	Logger       Logger
	Interval     time.Duration
	Sleep        func(context.Context, time.Duration) error
}

// Run starts the continuous daemon loop.
//
// It polls once immediately, then sleeps for the configured interval before
// processing again. Pass-level errors are logged and retried later because
// expired sessions, VT outages, and transient network failures should not crash
// the daemon process.
func (r Runner) Run(ctx context.Context) error {
	if r.Processor == nil {
		return fmt.Errorf("daemon processor is nil")
	}

	logger := r.logger()
	interval := r.interval()
	sleep := r.sleep()

	logger.Printf("daemon starting interval=%s auto_register=%t", interval, r.AutoRegister)
	for {
		// Treat cancellation as normal shutdown. Ctrl-C/SIGTERM should stop the
		// daemon without surfacing an error to the CLI.
		if ctx.Err() != nil {
			return nil
		}

		// The daemon intentionally uses the due-watch path only. Forced polling
		// stays behind `watch poll --all` so unattended runs respect next_poll_at.
		report, err := r.Processor.ProcessDue(ctx, automation.ProcessInput{AutoRegister: r.AutoRegister})
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			// Session expiry, VT downtime, and network failures are expected
			// operational states. Log them and retry on the next interval.
			logger.Printf("poll error: %v", err)
		} else {
			logProcessReport(logger, report)
		}

		// Sleep after every pass, including empty or failed passes, so the daemon
		// cannot hot-loop when there is nothing to do or VT is temporarily broken.
		if err := sleep(ctx, interval); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("daemon sleep: %w", err)
		}
	}
}

// interval resolves the configured poll interval to the daemon default.
func (r Runner) interval() time.Duration {
	if r.Interval > 0 {
		return r.Interval
	}
	return watch.DefaultPollInterval
}

// logger resolves the configured logger to the process default logger.
func (r Runner) logger() Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return log.Default()
}

// sleep resolves the configured sleep hook to the real context-aware sleeper.
func (r Runner) sleep() func(context.Context, time.Duration) error {
	if r.Sleep != nil {
		return r.Sleep
	}
	return defaultSleep
}

// defaultSleep waits for duration or returns early when the context is canceled.
func defaultSleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// logProcessReport writes a compact daemon-oriented summary of one pass.
func logProcessReport(logger Logger, report automation.ProcessReport) {
	checkedAt := report.PollReport.CheckedAt.UTC().Format(time.RFC3339)
	if len(report.PollReport.Results) == 0 {
		logger.Printf("poll checked_at=%s watches=0", checkedAt)
	} else {
		logger.Printf("poll checked_at=%s watches=%d", checkedAt, len(report.PollReport.Results))
		for _, result := range report.PollReport.Results {
			logPollResult(logger, result)
		}
	}

	for _, attempt := range report.Attempts {
		logAttemptResult(logger, attempt)
	}
}

// logPollResult writes one watch polling result.
func logPollResult(logger Logger, result watch.PollResult) {
	status := string(result.Evaluation.SectionStatus)
	if status == "" && result.Watch.LastSeenStat.Valid {
		status = result.Watch.LastSeenStat.String
	}

	if result.Err != nil {
		logger.Printf("poll watch id=%d active=%t mode=%s term=%s add_crn=%s status=%s error=%v",
			result.Watch.ID,
			result.Watch.Active,
			result.Watch.Mode,
			result.Watch.Term,
			result.Watch.AddCRN,
			status,
			result.Err,
		)
		return
	}

	logger.Printf("poll watch id=%d active=%t mode=%s term=%s add_crn=%s status=%s",
		result.Watch.ID,
		result.Watch.Active,
		result.Watch.Mode,
		result.Watch.Term,
		result.Watch.AddCRN,
		status,
	)
}

// logAttemptResult writes one automatic registration attempt result.
func logAttemptResult(logger Logger, attempt automation.AttemptResult) {
	if attempt.Err != nil {
		logger.Printf("attempt watch id=%d error=%v", attempt.Watch.ID, attempt.Err)
		return
	}

	logger.Printf("attempt watch id=%d outcome=%s registered=%t message=%q",
		attempt.Watch.ID,
		attempt.Result.Outcome,
		attempt.Result.Registered,
		attempt.Result.Message,
	)
}
