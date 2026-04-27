// Package session owns the credential import and validation workflow.
//
// It sits between CLI code, the VT client, and SQLite persistence. The browser
// only needs to capture authtoken; studentdata is the trusted source for the
// identity/proof fields SeatHawk stores for later registration calls.
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
	CapturedAt time.Time `json:"captured_at"`
	Authtoken  string    `json:"authtoken"`

	// Legacy bookmarklets copied these fields too. Import deliberately ignores
	// them because idProof may rotate and studentdata is the authoritative source.
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

// ValidationResult is the outcome of re-checking a stored session.
//
// ValidationErr is separate from the method error so callers can display the
// persisted session status even when VT rejected the credentials.
type ValidationResult struct {
	Session       store.Session
	ValidationErr error
}

// Import validates the captured authtoken and stores the derived VT session fields.
func (s Service) Import(ctx context.Context, payload CapturedCredentials) (store.Session, error) {
	if payload.Authtoken == "" {
		return store.Session{}, fmt.Errorf("authtoken is required")
	}

	studentData, err := s.VTClient.StudentData(ctx, payload.Authtoken)
	if err != nil {
		return store.Session{}, fmt.Errorf("validate imported session: %w", err)
	}
	if studentData.Pers.ID == "" {
		return store.Session{}, fmt.Errorf("VT studentdata did not include a session identity")
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
		PersID:          studentData.Pers.ID,
		PersIDProof:     studentData.Pers.IDProof,
		CapturedAt:      capturedAt.UTC(),
		LastValidatedAt: sql.NullTime{Time: now, Valid: true},
		Status:          StatusValid,
	}
	saved, err := store.SaveSession(ctx, s.DB, session)
	if err != nil {
		return store.Session{}, err
	}

	return saved, nil
}

// ValidateCurrent re-checks the stored session against VT studentdata.
func (s Service) ValidateCurrent(ctx context.Context) (ValidationResult, error) {
	current, err := store.CurrentSession(ctx, s.DB)
	if err != nil {
		return ValidationResult{}, err
	}

	now := s.now()
	studentData, err := s.VTClient.StudentData(ctx, current.Authtoken)
	decision := validValidation(studentData.Pers.IDProof)
	if err != nil {
		// Preserve the row for audit/history, but mark it unusable for future
		// workflows until the user imports or validates fresh credentials.
		decision = invalidValidation(fmt.Errorf("validate stored session: %w", err))
	} else if studentData.Pers.ID != current.PersID {
		decision = invalidValidation(fmt.Errorf("stored session identity no longer matches VT studentdata"))
	} else if studentData.Pers.IDProof == "" {
		decision = invalidValidation(fmt.Errorf("VT studentdata did not include a session proof"))
	}

	updated, err := s.persistValidationDecision(ctx, current, now, decision)
	if err != nil {
		return ValidationResult{}, err
	}
	return ValidationResult{Session: updated, ValidationErr: decision.ValidationErr}, nil
}

// Current returns the newest stored session without contacting VT.
func (s Service) Current(ctx context.Context) (store.Session, error) {
	return store.CurrentSession(ctx, s.DB)
}

type validationDecision struct {
	Status        string
	PersIDProof   string
	ValidationErr error
}

func validValidation(persIDProof string) validationDecision {
	return validationDecision{
		Status:      StatusValid,
		PersIDProof: persIDProof,
	}
}

func invalidValidation(err error) validationDecision {
	return validationDecision{
		Status:        StatusInvalid,
		ValidationErr: err,
	}
}

func (s Service) persistValidationDecision(ctx context.Context, current store.Session, at time.Time, decision validationDecision) (store.Session, error) {
	if err := store.UpdateSessionValidation(ctx, s.DB, current.ID, decision.Status, at, decision.PersIDProof); err != nil {
		if decision.ValidationErr != nil {
			return store.Session{}, fmt.Errorf("%v; also failed to update session status: %w", decision.ValidationErr, err)
		}
		return store.Session{}, err
	}

	updated, err := store.CurrentSession(ctx, s.DB)
	if err != nil {
		if decision.ValidationErr != nil {
			return store.Session{}, fmt.Errorf("%v; also failed to reload session: %w", decision.ValidationErr, err)
		}
		return store.Session{}, err
	}
	return updated, nil
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}
