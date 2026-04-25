// Package session owns the credential import and validation workflow.
//
// It sits between CLI code, the VT client, and SQLite persistence: captured
// credentials are never trusted until studentdata confirms they match the
// authenticated VT session.
package session

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/brennanhumphrey/seathawk/internal/store"
	"github.com/brennanhumphrey/seathawk/internal/vt"
)

const (
	StatusValid   = "valid"
	StatusInvalid = "invalid"
)

// CapturedCredentials is the JSON shape copied from the VT credential bookmarklet.
type CapturedCredentials struct {
	CapturedAt  time.Time               `json:"captured_at"`
	Authtoken   string                  `json:"authtoken"`
	PersID      string                  `json:"pers_id"`
	PersIDProof string                  `json:"pers_id_proof"`
	Name        string                  `json:"name"`
	RegTickets  []vt.RegistrationTicket `json:"reg_tickets"`
}

// StudentDataClient is the VT client behavior needed for session validation.
type StudentDataClient interface {
	StudentData(ctx context.Context, authtoken string) (vt.StudentData, error)
}

// Service coordinates session validation with local persistence.
type Service struct {
	DB       *sql.DB
	VTClient StudentDataClient
	// Now is injectable so validation timestamps can be tested deterministically.
	Now func() time.Time
}

// Import validates captured credentials against VT and stores them as the newest session.
func (s Service) Import(ctx context.Context, payload CapturedCredentials) (store.Session, error) {
	if payload.Authtoken == "" {
		return store.Session{}, fmt.Errorf("authtoken is required")
	}
	if payload.PersID == "" {
		return store.Session{}, fmt.Errorf("pers_id is required")
	}
	if payload.PersIDProof == "" {
		return store.Session{}, fmt.Errorf("pers_id_proof is required")
	}

	studentData, err := s.client().StudentData(ctx, payload.Authtoken)
	if err != nil {
		return store.Session{}, fmt.Errorf("validate imported session: %w", err)
	}
	if studentData.Pers.ID != payload.PersID {
		return store.Session{}, fmt.Errorf("imported session identity does not match VT studentdata")
	}
	if studentData.Pers.IDProof == "" {
		return store.Session{}, fmt.Errorf("VT studentdata did not include a session proof")
	}

	now := s.now()
	capturedAt := payload.CapturedAt
	if capturedAt.IsZero() {
		// Some manual imports may omit captured_at; keep the row usable by
		// recording local import time instead of rejecting otherwise valid creds.
		capturedAt = now
	}

	session := store.Session{
		Authtoken:       payload.Authtoken,
		PersID:          payload.PersID,
		PersIDProof:     studentData.Pers.IDProof,
		CapturedAt:      capturedAt.UTC(),
		LastValidatedAt: sql.NullTime{Time: now, Valid: true},
		Status:          StatusValid,
	}
	if err := store.SaveSession(ctx, s.DB, session); err != nil {
		return store.Session{}, err
	}

	return store.CurrentSession(ctx, s.DB)
}

// ValidateCurrent re-checks the stored session against VT studentdata.
func (s Service) ValidateCurrent(ctx context.Context) (store.Session, error) {
	current, err := store.CurrentSession(ctx, s.DB)
	if err != nil {
		return store.Session{}, err
	}

	now := s.now()
	studentData, err := s.client().StudentData(ctx, current.Authtoken)
	if err != nil {
		// Preserve the row for audit/history, but mark it unusable for future
		// workflows until the user imports or validates fresh credentials.
		return s.markCurrent(ctx, current, StatusInvalid, now, "", fmt.Errorf("validate stored session: %w", err))
	}
	if studentData.Pers.ID != current.PersID {
		return s.markCurrent(ctx, current, StatusInvalid, now, "", fmt.Errorf("stored session identity no longer matches VT studentdata"))
	}
	if studentData.Pers.IDProof == "" {
		return s.markCurrent(ctx, current, StatusInvalid, now, "", fmt.Errorf("VT studentdata did not include a session proof"))
	}

	return s.markCurrent(ctx, current, StatusValid, now, studentData.Pers.IDProof, nil)
}

// Current returns the newest stored session without contacting VT.
func (s Service) Current(ctx context.Context) (store.Session, error) {
	return store.CurrentSession(ctx, s.DB)
}

func (s Service) markCurrent(ctx context.Context, current store.Session, status string, at time.Time, persIDProof string, cause error) (store.Session, error) {
	if err := store.UpdateSessionValidation(ctx, s.DB, current.ID, status, at, persIDProof); err != nil {
		if cause != nil {
			return store.Session{}, fmt.Errorf("%v; also failed to update session status: %w", cause, err)
		}
		return store.Session{}, err
	}

	updated, err := store.CurrentSession(ctx, s.DB)
	if err != nil {
		if cause != nil {
			return store.Session{}, fmt.Errorf("%v; also failed to reload session: %w", cause, err)
		}
		return store.Session{}, err
	}
	if cause != nil {
		// Return the updated row and the validation failure so CLI callers can
		// show the new status while still exiting non-zero.
		return updated, cause
	}
	return updated, nil
}

func (s Service) client() StudentDataClient {
	return s.VTClient
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}
