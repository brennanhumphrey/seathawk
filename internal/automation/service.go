// Package automation coordinates unattended daemon passes.
//
// The package sits between read-only watch polling and explicit registration
// attempts. It is the boundary where SeatHawk may decide that a polled watch is
// actionable enough to hand to the registration package.
package automation

import (
	"context"
	"fmt"

	"github.com/brennanhumphrey/seathawk/internal/register"
	"github.com/brennanhumphrey/seathawk/internal/store"
	"github.com/brennanhumphrey/seathawk/internal/watch"
)

// Poller is the read-only polling behavior automation needs.
//
// Production uses watch.Service. Tests use a fake so automation decisions can
// be exercised without contacting VT.
type Poller interface {
	PollDue(ctx context.Context, input watch.PollInput) (watch.PollReport, error)
}

// Registerer is the registration behavior automation needs.
//
// Production uses register.Service. Automation never stages carts or calls
// shockabsorber directly; it delegates to the registration package so every
// mutation still gets fresh safety checks.
type Registerer interface {
	AttemptAdd(ctx context.Context, input register.AttemptAddInput) (register.AttemptAddResult, error)
}

// Disabler is the watch behavior automation uses after unsafe auto outcomes.
//
// Production uses watch.Service.Disable. Keeping this as an interface avoids
// putting SQL or watch-state rules in the daemon package.
type Disabler interface {
	Disable(ctx context.Context, id int64) (store.Watch, error)
}

// Service coordinates one daemon pass when auto-registration may be enabled.
type Service struct {
	Poller     Poller
	Registerer Registerer
	Disabler   Disabler
}

// ProcessInput configures one automation pass.
type ProcessInput struct {
	// AutoRegister allows eligible add watches to be handed to Registerer.
	//
	// When false, ProcessDue is a read-only polling pass.
	AutoRegister bool
}

// ProcessReport summarizes one automation pass.
type ProcessReport struct {
	PollReport watch.PollReport
	Attempts   []AttemptResult
}

// AttemptResult is the daemon-facing result of one automatic attempt.
type AttemptResult struct {
	Watch store.Watch
	// Result is populated when Registerer returned a structured outcome.
	Result register.AttemptAddResult
	Err    error
}

// ProcessDue polls due watches and optionally attempts eligible add watches.
//
// This is the only package-level workflow that can turn a read-only poll result
// into a VT write. The pre-filter here is deliberately conservative, but it is
// not the source of truth: register.Service re-fetches studentdata/fose and
// rechecks all safety conditions immediately before mutating VT.
func (s Service) ProcessDue(ctx context.Context, input ProcessInput) (ProcessReport, error) {
	if s.Poller == nil {
		return ProcessReport{}, fmt.Errorf("automation poller is nil")
	}
	if input.AutoRegister && s.Registerer == nil {
		return ProcessReport{}, fmt.Errorf("automation registerer is nil")
	}
	if input.AutoRegister && s.Disabler == nil {
		return ProcessReport{}, fmt.Errorf("automation disabler is nil")
	}

	pollReport, err := s.Poller.PollDue(ctx, watch.PollInput{})
	if err != nil {
		return ProcessReport{}, err
	}

	report := ProcessReport{PollReport: pollReport}
	if !input.AutoRegister {
		return report, nil
	}

	for _, result := range pollReport.Results {
		if !shouldAttempt(result) {
			continue
		}

		attempt := AttemptResult{Watch: result.Watch}
		// AttemptAdd performs fresh reads before any write. The poll result only
		// decides whether this watch is worth handing to the registration layer.
		attempt.Result, attempt.Err = s.Registerer.AttemptAdd(ctx, register.AttemptAddInput{WatchID: result.Watch.ID})
		if attempt.Result.Watch.ID != 0 {
			attempt.Watch = attempt.Result.Watch
		}
		if attempt.Err == nil && shouldDisableAfterAutomaticAttempt(attempt.Result) {
			disabled, err := s.Disabler.Disable(ctx, result.Watch.ID)
			if err != nil {
				attempt.Err = fmt.Errorf("disable watch after automatic attempt: %w", err)
			} else {
				attempt.Watch = disabled
				attempt.Result.Watch = disabled
			}
		}
		report.Attempts = append(report.Attempts, attempt)
	}

	return report, nil
}

// shouldAttempt reports whether a poll result is eligible for auto-registration.
//
// This is only a daemon pre-filter. It prevents obvious non-actionable watches
// from reaching the registration package, but it does not replace the fresh
// safety checks inside register.Service.AttemptAdd.
func shouldAttempt(result watch.PollResult) bool {
	if result.Err != nil || !result.Watch.Active {
		return false
	}
	if result.Watch.Mode != watch.ModeAdd {
		return false
	}
	if result.Evaluation.HasHardRejects() {
		return false
	}
	return result.Evaluation.SectionStatus == watch.SectionOpen &&
		result.Evaluation.Window == watch.WindowReady
}

// shouldDisableAfterAutomaticAttempt reports whether auto mode should stop a watch.
//
// Confirmed attempts are already disabled by register.Service. Failed and
// ambiguous outcomes are disabled here so the daemon cannot repeatedly submit
// VT writes without the user reviewing what happened.
func shouldDisableAfterAutomaticAttempt(result register.AttemptAddResult) bool {
	switch result.Outcome {
	case register.OutcomeFailed, register.OutcomeAmbiguous:
		return true
	default:
		return false
	}
}
