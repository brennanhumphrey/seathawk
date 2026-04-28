// Package register owns explicit, user-triggered VT registration attempts.
//
// This package is intentionally separate from watch polling. Watches describe
// local intent and polling observes VT state, while register performs the
// write-side workflow that can actually change a student's schedule.
package register

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/brennanhumphrey/seathawk/internal/sessionstatus"
	"github.com/brennanhumphrey/seathawk/internal/store"
	"github.com/brennanhumphrey/seathawk/internal/vt"
	"github.com/brennanhumphrey/seathawk/internal/watch"
)

const (
	// DefaultStatusPollInterval follows the VT API reference's status-polling
	// guidance while keeping manual attempts responsive.
	DefaultStatusPollInterval = 2 * time.Second
	// DefaultMaxStatusPolls caps one manual attempt at roughly one minute.
	DefaultMaxStatusPolls = 30

	defaultGradeMode = "N"
	defaultRegInfo   = "E"
)

const (
	phaseEvaluate  = "evaluate"
	phasePreflight = "preflight"
	phaseCartAdd   = "cart_add"
	phaseRegister  = "register"
	phaseStatus    = "status"
	phaseConfirm   = "confirm"
)

// AttemptOutcome is the normalized result of one manual registration attempt.
type AttemptOutcome string

const (
	// OutcomeSkipped means SeatHawk stopped before a VT mutation because the
	// watch was not currently safe or actionable.
	OutcomeSkipped AttemptOutcome = "skipped"
	// OutcomeSubmitted means SeatHawk submitted the registration but has not
	// proven the final studentdata state yet.
	OutcomeSubmitted AttemptOutcome = "submitted"
	// OutcomeConfirmed means fresh studentdata shows the target CRN registered.
	OutcomeConfirmed AttemptOutcome = "confirmed"
	// OutcomeFailed means VT returned a terminal non-success or preflight block.
	OutcomeFailed AttemptOutcome = "failed"
	// OutcomeAmbiguous means the queue outcome was unclear and studentdata did
	// not prove success.
	OutcomeAmbiguous AttemptOutcome = "ambiguous"
)

// VTClient is the VT behavior needed for a single-CRN add attempt.
//
// The interface deliberately includes both read-only and write methods because
// registration orchestration must guard every mutation with fresh reads.
type VTClient interface {
	StudentData(ctx context.Context, authtoken string) (vt.StudentData, error)
	SearchByCRN(ctx context.Context, term, crn string) (vt.FoseSearchResponse, error)
	Preflight(ctx context.Context, authtoken, term string, crns []string) (vt.PreflightResponse, error)
	CartAdd(ctx context.Context, input vt.CartAddInput) (vt.CartResponse, error)
	ShockabsorberRegister(ctx context.Context, input vt.ShockabsorberRegisterInput) (vt.ShockabsorberResponse, error)
	ShockabsorberStatus(ctx context.Context, input vt.ShockabsorberStatusInput) (vt.ShockabsorberResponse, error)
}

// Service coordinates one explicit registration attempt with VT and SQLite.
//
// It does not run in the background and does not select watches by schedule.
// The CLI passes a specific watch ID so the user remains in control while this
// dangerous path is first proven.
type Service struct {
	DB       *sql.DB
	VTClient VTClient
	// Now is injectable so attempt timestamps can be tested deterministically.
	Now func() time.Time
	// StatusPollInterval controls delay between shockabsorber status checks.
	StatusPollInterval time.Duration
	// MaxStatusPolls caps how many status checks follow one register call.
	MaxStatusPolls int
}

// AttemptAddInput identifies the add watch to attempt.
type AttemptAddInput struct {
	WatchID int64
}

// AttemptAddResult is the user-facing summary of one attempt.
//
// The CLI prints this structure instead of inspecting low-level VT responses.
// Attempt audit rows still preserve phase-level details for debugging.
type AttemptAddResult struct {
	Watch      store.Watch
	Evaluation watch.Evaluation
	Outcome    AttemptOutcome
	Registered bool
	Message    string
}

// AttemptAdd runs one manual single-CRN add attempt for an existing watch.
//
// The method submits shockabsorber register at most once. If the queue response
// is ambiguous, it reads fresh studentdata instead of retrying, because VT's
// register action is state-sensitive and can be destructive when repeated.
func (s Service) AttemptAdd(ctx context.Context, input AttemptAddInput) (AttemptAddResult, error) {
	if s.DB == nil {
		return AttemptAddResult{}, fmt.Errorf("database is nil")
	}
	if s.VTClient == nil {
		return AttemptAddResult{}, fmt.Errorf("VT client is nil")
	}
	if input.WatchID <= 0 {
		return AttemptAddResult{}, fmt.Errorf("watch id must be a positive integer")
	}

	now := s.now()
	watchRow, err := store.WatchByID(ctx, s.DB, input.WatchID)
	if err != nil {
		return AttemptAddResult{}, err
	}
	result := AttemptAddResult{Watch: watchRow}
	if watchRow.Mode != watch.ModeAdd {
		return result, fmt.Errorf("manual registration attempts currently support add watches only")
	}

	session, err := currentValidSession(ctx, s.DB)
	if err != nil {
		return result, err
	}

	studentData, err := s.VTClient.StudentData(ctx, session.Authtoken)
	if err != nil {
		return result, fmt.Errorf("fetch studentdata: %w", err)
	}
	search, err := s.VTClient.SearchByCRN(ctx, watchRow.Term, watchRow.AddCRN)
	if err != nil {
		return result, fmt.Errorf("search add CRN: %w", err)
	}

	evaluation, err := watch.EvaluateFromSnapshots(watchRow, studentData, search, now)
	if err != nil {
		return result, err
	}
	result.Evaluation = evaluation

	section, err := selectedAddSection(search, watchRow.AddCRN)
	if err != nil {
		return s.finishSkipped(ctx, result, phaseEvaluate, "add CRN was not found in fose search", true, now)
	}
	if evaluation.HasHardRejects() {
		return s.finishSkipped(ctx, result, phaseEvaluate, "watch is no longer safe to attempt: "+formatRejects(evaluation.HardRejects), true, now)
	}
	if evaluation.SectionStatus != watch.SectionOpen {
		return s.finishSkipped(ctx, result, phaseEvaluate, "section is not open", false, now)
	}
	if evaluation.Window != watch.WindowReady {
		return s.finishSkipped(ctx, result, phaseEvaluate, "registration window is not ready", false, now)
	}

	preflight, err := s.VTClient.Preflight(ctx, session.Authtoken, watchRow.Term, []string{watchRow.AddCRN})
	if err != nil {
		return result, fmt.Errorf("preflight: %w", err)
	}
	if preflightBlocked(preflight, watchRow.AddCRN) {
		if saveErr := s.saveAttempt(ctx, watchRow.ID, phasePreflight, OutcomeFailed, preflight, now); saveErr != nil {
			return result, saveErr
		}
		if err := store.UpdateWatchLastAttemptAt(ctx, s.DB, watchRow.ID, now, now); err != nil {
			return result, err
		}
		result.Outcome = OutcomeFailed
		result.Message = "preflight reported a registration blocker"
		return result, nil
	}
	if err := s.saveAttempt(ctx, watchRow.ID, phasePreflight, OutcomeSubmitted, preflight, now); err != nil {
		return result, err
	}

	hours, err := creditHoursFromSection(section)
	if err != nil {
		return s.finishSkipped(ctx, result, phaseCartAdd, err.Error(), false, now)
	}
	cart, err := s.VTClient.CartAdd(ctx, vt.CartAddInput{
		Authtoken: session.Authtoken,
		Term:      watchRow.Term,
		CRN:       watchRow.AddCRN,
		Hours:     hours,
		GradeMode: defaultGradeMode,
		RegInfo:   defaultRegInfo,
	})
	if err != nil {
		return result, fmt.Errorf("cart add: %w", err)
	}
	if err := s.saveAttempt(ctx, watchRow.ID, phaseCartAdd, OutcomeSubmitted, cart, now); err != nil {
		return result, err
	}

	ticket, err := findAddTicket(studentData, watchRow.Term, now)
	if err != nil {
		return s.finishSkipped(ctx, result, phaseRegister, err.Error(), false, now)
	}
	timeTicket, err := vt.BuildTimeTicket(ticket, studentData.Pers.ID)
	if err != nil {
		return result, fmt.Errorf("build time ticket: %w", err)
	}
	credentials := vt.ShockabsorberCredentials{
		Authtoken:     session.Authtoken,
		PersonID:      studentData.Pers.ID,
		PersonIDProof: studentData.Pers.IDProof,
	}
	replayURL, err := vt.BuildSingleReplayURL(watchRow.Term, watchRow.AddCRN)
	if err != nil {
		return result, fmt.Errorf("build replay URL: %w", err)
	}
	registerResponse, err := s.VTClient.ShockabsorberRegister(ctx, vt.ShockabsorberRegisterInput{
		Credentials: credentials,
		TimeTicket:  timeTicket,
		URLReplay:   replayURL,
	})
	if err != nil {
		return result, fmt.Errorf("shockabsorber register: %w", err)
	}
	if err := s.saveAttempt(ctx, watchRow.ID, phaseRegister, OutcomeSubmitted, registerResponse, now); err != nil {
		return result, err
	}

	statusOutcome, err := s.pollStatus(ctx, watchRow.ID, credentials, timeTicket, now)
	if err != nil {
		return result, err
	}

	finalStudentData, err := s.VTClient.StudentData(ctx, session.Authtoken)
	if err != nil {
		return result, fmt.Errorf("confirm studentdata: %w", err)
	}
	registered, err := registeredForAddCRN(finalStudentData, watchRow.Term, watchRow.AddCRN)
	if err != nil {
		return result, err
	}
	result.Registered = registered

	outcome := OutcomeFailed
	message := "registration was not confirmed in studentdata"
	if registered {
		outcome = OutcomeConfirmed
		message = "registration confirmed in studentdata"
	} else if statusOutcome == OutcomeAmbiguous {
		outcome = OutcomeAmbiguous
		message = "registration outcome is ambiguous; studentdata did not confirm the add"
	}

	if err := s.saveAttempt(ctx, watchRow.ID, phaseConfirm, outcome, map[string]any{"registered": registered}, now); err != nil {
		return result, err
	}
	if err := store.UpdateWatchLastAttemptAt(ctx, s.DB, watchRow.ID, now, now); err != nil {
		return result, err
	}
	if registered {
		if err := store.SetWatchActive(ctx, s.DB, watchRow.ID, false, now); err != nil {
			return result, err
		}
	}
	updated, err := store.WatchByID(ctx, s.DB, watchRow.ID)
	if err != nil {
		return result, err
	}

	result.Watch = updated
	result.Outcome = outcome
	result.Message = message
	return result, nil
}

// selectedAddSection returns the exact fose result for the watch's add CRN.
//
// fose searches can return arrays, so registration uses an explicit match
// instead of assuming the first result is the requested section.
func selectedAddSection(search vt.FoseSearchResponse, crn string) (vt.FoseResult, error) {
	for _, result := range search.Results {
		if result.CRN == crn {
			return result, nil
		}
	}
	return vt.FoseResult{}, fmt.Errorf("add CRN was not found in fose search")
}

var leadingHoursPattern = regexp.MustCompile(`^\s*([0-9]+(?:\.[0-9]+)?)`)

// creditHoursFromSection extracts VT's cart_add hours value from fose data.
//
// cart_add requires a plain value such as "3", but fose exposes human text such
// as "3 Credit Hours" for some sections. Other sections omit hours_html and put
// the selected/default value inside the JSON-encoded cart_opts.credit_hrs
// payload. If neither source yields a value, the safe behavior is to stop before
// writing to the VT cart.
func creditHoursFromSection(section vt.FoseResult) (string, error) {
	match := leadingHoursPattern.FindStringSubmatch(section.HoursHTML)
	if len(match) >= 2 {
		return match[1], nil
	}

	hours, err := defaultCartOptionValue(section.CartOptions, "credit_hrs")
	if err != nil {
		return "", fmt.Errorf("parse section cart options: %w", err)
	}
	if hours != "" {
		return hours, nil
	}
	return "", fmt.Errorf("section credit hours could not be parsed")
}

type foseCartOptions struct {
	CreditHours cartOptionGroup `json:"credit_hrs"`
}

type cartOptionGroup struct {
	Options []cartOption `json:"options"`
}

type cartOption struct {
	Value    string `json:"value"`
	Default  bool   `json:"default"`
	Selected string `json:"selected"`
}

// defaultCartOptionValue extracts the browser-selected cart option from cart_opts.
//
// VT serializes cart_opts as a JSON string inside the fose result instead of as
// nested JSON. The registration flow only needs credit_hrs today, but this
// helper keeps the selected/default option rules in one place.
func defaultCartOptionValue(raw string, group string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}

	var options foseCartOptions
	if err := json.Unmarshal([]byte(raw), &options); err != nil {
		return "", err
	}

	var optionGroup cartOptionGroup
	switch group {
	case "credit_hrs":
		optionGroup = options.CreditHours
	default:
		return "", fmt.Errorf("unsupported cart option group %q", group)
	}

	for _, option := range optionGroup.Options {
		value := strings.TrimSpace(option.Value)
		if value == "" {
			continue
		}
		if option.Default || strings.EqualFold(strings.TrimSpace(option.Selected), "selected") {
			return value, nil
		}
	}

	return "", nil
}

// preflightBlocked reports whether VT preflight returned a real blocker.
//
// VT commonly returns "||" for a CRN with no actionable error. Any non-empty
// course text other than that sentinel, or any non-course error, blocks the
// registration attempt before cart_add.
func preflightBlocked(response vt.PreflightResponse, crn string) bool {
	if len(response.RegNonCourseErrors) > 0 {
		return true
	}
	courseError := strings.TrimSpace(response.RegCourseErrors[crn])
	return courseError != "" && courseError != "||"
}

// findAddTicket returns the active add-capable registration ticket for a term.
//
// This duplicates the window check at the point of time_ticket construction so
// an attempt cannot proceed with a stale or action-incompatible ticket.
func findAddTicket(studentData vt.StudentData, term string, now time.Time) (vt.RegistrationTicket, error) {
	for _, ticket := range studentData.RegTickets {
		if ticket.Term != term {
			continue
		}
		start, err := vt.ParseTimestamp(ticket.StartDate)
		if err != nil {
			return vt.RegistrationTicket{}, fmt.Errorf("parse ticket start: %w", err)
		}
		end, err := vt.ParseTimestamp(ticket.EndDate)
		if err != nil {
			return vt.RegistrationTicket{}, fmt.Errorf("parse ticket end: %w", err)
		}
		if now.Before(start) || now.After(end) {
			return vt.RegistrationTicket{}, fmt.Errorf("registration ticket is not active")
		}
		if !ticketHasAction(ticket, "add") {
			return vt.RegistrationTicket{}, fmt.Errorf("registration ticket does not allow add")
		}
		return ticket, nil
	}
	return vt.RegistrationTicket{}, fmt.Errorf("no registration ticket exists for term")
}

// registeredForAddCRN confirms final add state from studentdata reg.<term>.
//
// shockabsorber responses are useful diagnostics, but studentdata is the source
// of truth for whether the student's schedule actually changed.
func registeredForAddCRN(studentData vt.StudentData, term string, crn string) (bool, error) {
	for _, raw := range studentData.Registered[term] {
		section, err := vt.ParseRegisteredSection(raw)
		if err != nil {
			return false, fmt.Errorf("parse registered section: %w", err)
		}
		if section.CRN == crn {
			return true, nil
		}
	}
	return false, nil
}

// currentValidSession loads the newest session that is safe for VT writes.
func currentValidSession(ctx context.Context, db *sql.DB) (store.Session, error) {
	session, err := store.CurrentSession(ctx, db)
	if err != nil {
		return store.Session{}, fmt.Errorf("load current session: %w", err)
	}
	if session.Status != sessionstatus.StatusValid {
		return store.Session{}, fmt.Errorf("current session is %s; run session validate or import fresh credentials", session.Status)
	}
	return session, nil
}

// pollStatus polls shockabsorber after the single register submission.
//
// A status error or poll-limit timeout is ambiguous, not proof of failure. The
// caller must still reconcile with fresh studentdata before reporting the final
// outcome.
func (s Service) pollStatus(ctx context.Context, watchID int64, credentials vt.ShockabsorberCredentials, timeTicket string, now time.Time) (AttemptOutcome, error) {
	for i := 0; i < s.maxStatusPolls(); i++ {
		if i > 0 {
			timer := time.NewTimer(s.statusPollInterval())
			select {
			case <-ctx.Done():
				timer.Stop()
				return OutcomeAmbiguous, ctx.Err()
			case <-timer.C:
			}
		}

		response, err := s.VTClient.ShockabsorberStatus(ctx, vt.ShockabsorberStatusInput{
			Credentials: credentials,
			TimeTicket:  timeTicket,
		})
		if err != nil {
			if saveErr := s.saveAttempt(ctx, watchID, phaseStatus, OutcomeAmbiguous, map[string]string{"error": err.Error()}, now); saveErr != nil {
				return OutcomeAmbiguous, saveErr
			}
			return OutcomeAmbiguous, nil
		}
		if err := s.saveAttempt(ctx, watchID, phaseStatus, OutcomeSubmitted, response, now); err != nil {
			return OutcomeAmbiguous, err
		}
		if !strings.EqualFold(strings.TrimSpace(response.Body), "WAIT") {
			return OutcomeSubmitted, nil
		}
	}
	if err := s.saveAttempt(ctx, watchID, phaseStatus, OutcomeAmbiguous, map[string]string{"reason": "status poll limit reached"}, now); err != nil {
		return OutcomeAmbiguous, err
	}
	return OutcomeAmbiguous, nil
}

// finishSkipped records a non-mutating stop and optionally disables the watch.
//
// Hard safety rejects disable the watch because repeating them would be unsafe.
// Temporary states like "section is full" or "window not ready" stay active.
func (s Service) finishSkipped(ctx context.Context, result AttemptAddResult, phase string, message string, disable bool, now time.Time) (AttemptAddResult, error) {
	if err := s.saveAttempt(ctx, result.Watch.ID, phase, OutcomeSkipped, map[string]string{"message": message}, now); err != nil {
		return result, err
	}
	if err := store.UpdateWatchLastAttemptAt(ctx, s.DB, result.Watch.ID, now, now); err != nil {
		return result, err
	}
	if disable {
		if err := store.SetWatchActive(ctx, s.DB, result.Watch.ID, false, now); err != nil {
			return result, err
		}
	}
	updated, err := store.WatchByID(ctx, s.DB, result.Watch.ID)
	if err != nil {
		return result, err
	}
	result.Watch = updated
	result.Outcome = OutcomeSkipped
	result.Message = message
	return result, nil
}

// saveAttempt writes one phase-level audit row for a registration attempt.
//
// Raw payloads are limited to VT responses or local summaries that do not
// include authtoken, pers_id, or pers_id_proof.
func (s Service) saveAttempt(ctx context.Context, watchID int64, phase string, outcome AttemptOutcome, raw any, now time.Time) error {
	attempt := store.Attempt{
		WatchID:     watchID,
		Phase:       phase,
		OutcomeKind: string(outcome),
		CreatedAt:   now,
	}
	if raw != nil {
		encoded, err := json.Marshal(raw)
		if err != nil {
			return fmt.Errorf("marshal attempt payload: %w", err)
		}
		attempt.RawResponse = sql.NullString{String: string(encoded), Valid: true}
	}
	if _, err := store.SaveAttempt(ctx, s.DB, attempt); err != nil {
		return err
	}
	return nil
}

// now returns the service clock in UTC.
//
// Tests inject Now so registration-attempt timestamps and active-window checks
// stay deterministic.
func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

// statusPollInterval returns the configured shockabsorber status delay.
//
// A zero value means use DefaultStatusPollInterval, keeping production setup
// simple while allowing tests to run without real sleeps.
func (s Service) statusPollInterval() time.Duration {
	if s.StatusPollInterval > 0 {
		return s.StatusPollInterval
	}
	return DefaultStatusPollInterval
}

// maxStatusPolls returns the configured status poll cap.
//
// The cap prevents a queued or stuck shockabsorber request from hanging a
// manual CLI attempt indefinitely.
func (s Service) maxStatusPolls() int {
	if s.MaxStatusPolls > 0 {
		return s.MaxStatusPolls
	}
	return DefaultMaxStatusPolls
}

// ticketHasAction reports whether a registration ticket allows an action.
//
// VT action strings come from studentdata and are treated case-insensitively to
// avoid rejecting a valid ticket because of presentation differences.
func ticketHasAction(ticket vt.RegistrationTicket, want string) bool {
	for _, action := range ticket.Actions {
		if strings.EqualFold(strings.TrimSpace(action), want) {
			return true
		}
	}
	return false
}

// formatRejects converts watch hard rejects into a stable CLI/audit message.
func formatRejects(rejects []watch.RejectReason) string {
	parts := make([]string, 0, len(rejects))
	for _, reject := range rejects {
		parts = append(parts, string(reject))
	}
	return strings.Join(parts, ", ")
}
