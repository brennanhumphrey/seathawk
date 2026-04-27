package watch

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/brennanhumphrey/seathawk/internal/store"
	"github.com/brennanhumphrey/seathawk/internal/vt"
)

// VTClient is the read-only VT behavior needed to sanity-check watches.
//
// Keeping this interface small lets tests exercise watch evaluation without
// touching classes.vt.edu, while production can pass *vt.Client directly.
type VTClient interface {
	StudentData(ctx context.Context, authtoken string) (vt.StudentData, error)
	SearchByCRN(ctx context.Context, term, crn string) (vt.FoseSearchResponse, error)
}

// SectionStatus is SeatHawk's normalized view of VT's compact fose stat code.
type SectionStatus string

const (
	SectionOpen     SectionStatus = "open"
	SectionFull     SectionStatus = "full"
	SectionWaitlist SectionStatus = "waitlist"
	SectionCanceled SectionStatus = "canceled"
	SectionUnknown  SectionStatus = "unknown"
)

// WindowStatus is the current registration-window state for a watch.
type WindowStatus string

const (
	WindowReady             WindowStatus = "ready"
	WindowNotOpen           WindowStatus = "not_open"
	WindowClosed            WindowStatus = "closed"
	WindowNoTicket          WindowStatus = "no_ticket"
	WindowActionUnavailable WindowStatus = "action_unavailable"
	WindowUnknown           WindowStatus = "unknown"
)

// Note is non-fatal context about the watch's current live state.
type Note string

const (
	NoteSectionFull          Note = "section is currently full; this is the normal watch state"
	NoteSectionOpen          Note = "section is currently open"
	NoteSectionWaitlist      Note = "section is currently waitlisted; waitlist behavior is not implemented yet"
	NoteSectionUnknown       Note = "section status is unknown"
	NoteWindowNotOpen        Note = "registration window is not open yet"
	NoteWindowClosed         Note = "registration window appears closed"
	NoteNoRegistrationWindow Note = "no registration ticket currently exists for this term"
	NoteActionUnavailable    Note = "registration ticket does not currently allow the needed action"
)

// RejectReason explains why a watch should not be created.
type RejectReason string

const (
	RejectCRNNotFound             RejectReason = "crn_not_found"
	RejectSectionCanceled         RejectReason = "section_canceled"
	RejectAlreadyRegisteredAdd    RejectReason = "already_registered_add"
	RejectMissingDropRegistration RejectReason = "missing_drop_registration"
)

// Evaluation is the read-only state SeatHawk observed for one watch.
//
// Full and not-yet-open states are represented as notes, not hard rejects,
// because watching full courses before a registration window opens is the main
// product use case. HardRejects are reserved for invalid or dangerous watch
// definitions.
type Evaluation struct {
	SectionStatus        SectionStatus
	SectionTitle         string
	AlreadyRegisteredAdd bool
	RegisteredDrop       bool
	Window               WindowStatus
	Notes                []Note
	HardRejects          []RejectReason
}

// HasHardRejects reports whether this evaluation should block watch creation.
func (e Evaluation) HasHardRejects() bool {
	return len(e.HardRejects) > 0
}

// Evaluate fetches live VT state and evaluates watch against it.
func Evaluate(ctx context.Context, client VTClient, session store.Session, watch store.Watch, now time.Time) (Evaluation, error) {
	if client == nil {
		return Evaluation{}, fmt.Errorf("VT client is nil")
	}
	studentData, err := client.StudentData(ctx, session.Authtoken)
	if err != nil {
		return Evaluation{}, fmt.Errorf("fetch studentdata: %w", err)
	}
	search, err := client.SearchByCRN(ctx, watch.Term, watch.AddCRN)
	if err != nil {
		return Evaluation{}, fmt.Errorf("search add CRN: %w", err)
	}

	return evaluateFromSnapshots(watch, studentData, search, now)
}

func evaluateFromSnapshots(watch store.Watch, studentData vt.StudentData, search vt.FoseSearchResponse, now time.Time) (Evaluation, error) {
	evaluation := Evaluation{
		SectionStatus: SectionUnknown,
		Window:        evaluateWindow(studentData, watch.Term, watch.Mode, now),
	}

	section, found := sectionFromSearch(search, watch.AddCRN)
	if !found {
		evaluation.HardRejects = append(evaluation.HardRejects, RejectCRNNotFound)
		return evaluation, nil
	}

	evaluation.SectionTitle = section.Title
	evaluation.SectionStatus = sectionStatus(section.Stat)

	addRegistered, err := registeredForCRN(studentData, watch.Term, watch.AddCRN)
	if err != nil {
		return Evaluation{}, err
	}
	evaluation.AlreadyRegisteredAdd = addRegistered
	if addRegistered {
		evaluation.HardRejects = append(evaluation.HardRejects, RejectAlreadyRegisteredAdd)
	}

	if watch.Mode == ModeSwap {
		dropRegistered := false
		if watch.DropCRN.Valid {
			dropRegistered, err = registeredForCRN(studentData, watch.Term, watch.DropCRN.String)
			if err != nil {
				return Evaluation{}, err
			}
		}
		evaluation.RegisteredDrop = dropRegistered
		if !dropRegistered {
			evaluation.HardRejects = append(evaluation.HardRejects, RejectMissingDropRegistration)
		}
	}

	evaluation.addSectionStatusNotes()
	evaluation.addWindowNotes()
	return evaluation, nil
}

func (e *Evaluation) addSectionStatusNotes() {
	switch e.SectionStatus {
	case SectionOpen:
		e.Notes = append(e.Notes, NoteSectionOpen)
	case SectionFull:
		e.Notes = append(e.Notes, NoteSectionFull)
	case SectionWaitlist:
		e.Notes = append(e.Notes, NoteSectionWaitlist)
	case SectionCanceled:
		e.HardRejects = append(e.HardRejects, RejectSectionCanceled)
	case SectionUnknown:
		e.Notes = append(e.Notes, NoteSectionUnknown)
	}
}

func (e *Evaluation) addWindowNotes() {
	switch e.Window {
	case WindowNotOpen:
		e.Notes = append(e.Notes, NoteWindowNotOpen)
	case WindowClosed:
		e.Notes = append(e.Notes, NoteWindowClosed)
	case WindowNoTicket:
		e.Notes = append(e.Notes, NoteNoRegistrationWindow)
	case WindowActionUnavailable:
		e.Notes = append(e.Notes, NoteActionUnavailable)
	}
}

func registeredForCRN(studentData vt.StudentData, term string, crn string) (bool, error) {
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

func sectionFromSearch(search vt.FoseSearchResponse, crn string) (vt.FoseResult, bool) {
	for _, result := range search.Results {
		if result.CRN == crn {
			return result, true
		}
	}
	return vt.FoseResult{}, false
}

func sectionStatus(stat string) SectionStatus {
	switch strings.ToUpper(strings.TrimSpace(stat)) {
	case "A":
		return SectionOpen
	case "F":
		return SectionFull
	case "W":
		return SectionWaitlist
	case "C":
		return SectionCanceled
	default:
		return SectionUnknown
	}
}

func evaluateWindow(studentData vt.StudentData, term string, mode string, now time.Time) WindowStatus {
	for _, ticket := range studentData.RegTickets {
		if ticket.Term != term {
			continue
		}
		start, err := vt.ParseTimestamp(ticket.StartDate)
		if err != nil {
			return WindowUnknown
		}
		end, err := vt.ParseTimestamp(ticket.EndDate)
		if err != nil {
			return WindowUnknown
		}
		if now.Before(start) {
			return WindowNotOpen
		}
		if now.After(end) {
			return WindowClosed
		}
		if ticketAllowsMode(ticket, mode) {
			return WindowReady
		}
		return WindowActionUnavailable
	}
	return WindowNoTicket
}

func ticketAllowsMode(ticket vt.RegistrationTicket, mode string) bool {
	switch mode {
	case ModeAdd:
		return hasAction(ticket.Actions, "add")
	case ModeSwap:
		return hasAction(ticket.Actions, "add") && (hasAction(ticket.Actions, "drop") || hasAction(ticket.Actions, "modify"))
	default:
		return false
	}
}

func hasAction(actions []string, want string) bool {
	for _, action := range actions {
		if strings.EqualFold(strings.TrimSpace(action), want) {
			return true
		}
	}
	return false
}
