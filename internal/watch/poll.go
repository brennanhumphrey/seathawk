package watch

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/brennanhumphrey/seathawk/internal/store"
	"github.com/brennanhumphrey/seathawk/internal/vt"
)

// PollInput configures one read-only polling pass.
type PollInput struct {
	// All ignores next_poll_at and checks every active watch. Disabled watches
	// are still skipped because disabled means "do not consider for automation."
	All bool
}

// PollResult is the outcome of polling one watch.
type PollResult struct {
	Watch      store.Watch
	Evaluation Evaluation
	Err        error
}

// PollReport summarizes one read-only polling pass.
type PollReport struct {
	CheckedAt time.Time
	Results   []PollResult
}

// PollDue evaluates active watches whose next poll time has arrived.
//
// This is the single-pass polling unit that a future daemon loop can call
// repeatedly. It reads VT state but never mutates VT: no cart, preflight,
// shockabsorber, or registration calls happen here.
func (s Service) PollDue(ctx context.Context, input PollInput) (PollReport, error) {
	if s.VTClient == nil {
		return PollReport{}, fmt.Errorf("VT client is nil")
	}

	now := s.now()
	session, err := s.currentSession(ctx)
	if err != nil {
		return PollReport{}, err
	}

	watches, err := s.watchesToPoll(ctx, input, now)
	if err != nil {
		return PollReport{}, err
	}

	report := PollReport{
		CheckedAt: now,
		Results:   make([]PollResult, 0, len(watches)),
	}

	if len(watches) == 0 {
		return report, nil
	}

	// studentdata contains the user's current registration state and reg_tickets.
	// Fetch it once per pass so multiple watches use a consistent snapshot.
	studentData, err := s.VTClient.StudentData(ctx, session.Authtoken)
	if err != nil {
		return PollReport{}, fmt.Errorf("fetch studentdata: %w", err)
	}

	for _, watch := range watches {
		report.Results = append(report.Results, s.pollWatch(ctx, watch, studentData, now))
	}
	return report, nil
}

func (s Service) watchesToPoll(ctx context.Context, input PollInput, now time.Time) ([]store.Watch, error) {
	if input.All {
		return store.ListActiveWatches(ctx, s.DB)
	}
	return store.ListDueActiveWatches(ctx, s.DB, now)
}

func (s Service) pollWatch(ctx context.Context, watch store.Watch, studentData vt.StudentData, now time.Time) PollResult {
	result := PollResult{Watch: watch}

	// Fose is checked per watch because availability is tied to the specific
	// add CRN. studentdata is shared for the whole pass by PollDue.
	search, err := s.VTClient.SearchByCRN(ctx, watch.Term, watch.AddCRN)
	if err != nil {
		result.Err = fmt.Errorf("search add CRN: %w", err)
		result.Watch = s.rescheduleAfterPollError(ctx, watch, now)
		return result
	}

	evaluation, err := evaluateFromSnapshots(watch, studentData, search, now)
	if err != nil {
		result.Err = err
		result.Watch = s.rescheduleAfterPollError(ctx, watch, now)
		return result
	}
	result.Evaluation = evaluation

	if evaluation.HasHardRejects() {
		result.Err = fmt.Errorf("watch rejected: %s", formatRejects(evaluation.HardRejects))
		result.Watch = s.disableRejectedWatch(ctx, watch, evaluation, now)
		return result
	}

	nextPollAt := nextPollTime(now)
	if err := store.UpdateWatchEvaluation(ctx, s.DB, watch.ID, watchStatusForStorage(evaluation), sql.NullTime{Time: nextPollAt, Valid: true}, now); err != nil {
		result.Err = err
		return result
	}

	updated, err := store.WatchByID(ctx, s.DB, watch.ID)
	if err != nil {
		result.Err = err
		return result
	}

	result.Watch = updated
	return result
}

func (s Service) rescheduleAfterPollError(ctx context.Context, watch store.Watch, now time.Time) store.Watch {
	// A transient VT/parsing error should not make the watch hot-loop. Keep the
	// previous observed status and push the next poll out by the normal interval.
	nextPollAt := nextPollTime(now)
	_ = store.UpdateWatchEvaluation(ctx, s.DB, watch.ID, watch.LastSeenStat, sql.NullTime{Time: nextPollAt, Valid: true}, now)
	updated, err := store.WatchByID(ctx, s.DB, watch.ID)
	if err != nil {
		return watch
	}
	return updated
}

func (s Service) disableRejectedWatch(ctx context.Context, watch store.Watch, evaluation Evaluation, now time.Time) store.Watch {
	// Hard rejects mean the local intent is no longer safe or valid to automate:
	// wrong term/CRN, canceled section, already registered add target, or an
	// unsafe swap drop target. Disable the watch so future polls skip it.
	_ = store.UpdateWatchEvaluation(ctx, s.DB, watch.ID, watchStatusForStorage(evaluation), sql.NullTime{}, now)
	_ = store.SetWatchActive(ctx, s.DB, watch.ID, false, now)
	updated, err := store.WatchByID(ctx, s.DB, watch.ID)
	if err != nil {
		return watch
	}
	return updated
}

func watchStatusForStorage(evaluation Evaluation) sql.NullString {
	if evaluation.SectionStatus == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: string(evaluation.SectionStatus), Valid: true}
}
