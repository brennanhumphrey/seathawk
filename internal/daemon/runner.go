// Package daemon owns SeatHawk's process-level polling loop.
//
// The daemon coordinates timing, logging, and shutdown. It deliberately delegates
// VT/domain decisions to internal/watch so this package never grows registration
// behavior of its own.
package daemon

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/brennanhumphrey/seathawk/internal/watch"
)

// Poller is the watch polling behavior the daemon needs.
type Poller interface {
	PollDue(ctx context.Context, input watch.PollInput) (watch.PollReport, error)
}

// Logger is the logging behavior the daemon needs.
type Logger interface {
	Printf(format string, args ...any)
}

// Runner repeatedly performs read-only watch polling until its context ends.
type Runner struct {
	Poller   Poller
	Logger   Logger
	Interval time.Duration
	Sleep    func(context.Context, time.Duration) error
}

// Run starts the continuous read-only polling loop.
//
// It polls once immediately, then sleeps for the configured interval before
// polling again. Poll pass errors are logged and retried later because expired
// sessions, VT outages, and transient network failures should not crash the
// daemon process.
func (r Runner) Run(ctx context.Context) error {
	if r.Poller == nil {
		return fmt.Errorf("daemon poller is nil")
	}

	logger := r.logger()
	interval := r.interval()
	sleep := r.sleep()

	logger.Printf("daemon starting interval=%s", interval)
	for {
		// Treat cancellation as normal shutdown. Ctrl-C/SIGTERM should stop the
		// daemon without surfacing an error to the CLI.
		if ctx.Err() != nil {
			return nil
		}

		// The daemon intentionally uses the due-watch path only. Forced polling
		// stays behind `watch poll --all` so unattended runs respect next_poll_at.
		report, err := r.Poller.PollDue(ctx, watch.PollInput{})
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			// Session expiry, VT downtime, and network failures are expected
			// operational states. Log them and retry on the next interval.
			logger.Printf("poll error: %v", err)
		} else {
			logPollReport(logger, report)
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

// logPollReport writes a compact daemon-oriented summary of one polling pass.
func logPollReport(logger Logger, report watch.PollReport) {
	checkedAt := report.CheckedAt.UTC().Format(time.RFC3339)
	if len(report.Results) == 0 {
		logger.Printf("poll checked_at=%s watches=0", checkedAt)
		return
	}

	logger.Printf("poll checked_at=%s watches=%d", checkedAt, len(report.Results))
	for _, result := range report.Results {
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
			continue
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
}
