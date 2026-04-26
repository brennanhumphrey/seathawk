// Package watch owns local watch-management workflow rules.
//
// A watch is durable user intent, not a VT request. This package deliberately
// avoids VT calls so users can configure watches before later polling and
// registration phases exist.
//
// The split from internal/store is intentional: store knows how to read and
// write rows, while watch knows what a valid SeatHawk watch means. That keeps
// SQL helpers simple and prevents Cobra command code from accumulating domain
// rules.
package watch

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/brennanhumphrey/seathawk/internal/store"
)

const (
	// ModeAdd is a watch that will eventually register one CRN.
	ModeAdd = "add"
	// ModeSwap is a watch that will eventually add one CRN and drop another.
	ModeSwap = "swap"
)

// Service coordinates watch validation with local persistence.
//
// It is the boundary between CLI input and durable state. Future polling code
// should be able to depend on watches created here without re-validating basic
// invariants like term/CRN shape or swap add/drop differences.
type Service struct {
	DB *sql.DB
	// Now is injectable so timestamp behavior can be tested deterministically.
	Now func() time.Time
}

// CreateAddInput is the user input needed to create an add watch.
//
// CRN is named generically here because add watches only have one target CRN.
type CreateAddInput struct {
	Term string
	CRN  string
}

// CreateSwapInput is the user input needed to create a swap watch.
//
// AddCRN and DropCRN are separate fields because swap intent is potentially
// destructive. The app should never infer a drop target from schedule state.
type CreateSwapInput struct {
	Term    string
	AddCRN  string
	DropCRN string
}

// Add creates an active watch for adding one CRN.
//
// This only records intent. It does not check whether the CRN exists, whether a
// seat is open, or whether the user's registration window is active; those are
// read-only evaluation concerns for the next phase.
func (s Service) Add(ctx context.Context, input CreateAddInput) (store.Watch, error) {
	term, err := validateTerm(input.Term)
	if err != nil {
		return store.Watch{}, err
	}
	crn, err := validateCRN(input.CRN)
	if err != nil {
		return store.Watch{}, err
	}

	now := s.now()
	return store.SaveWatch(ctx, s.DB, store.Watch{
		Term:      term,
		Mode:      ModeAdd,
		AddCRN:    crn,
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	})
}

// Swap creates an active watch for adding one CRN while dropping another.
//
// Keeping swap as a first-class mode matters because later registration logic
// must use stricter guardrails and reconciliation for add/drop workflows.
func (s Service) Swap(ctx context.Context, input CreateSwapInput) (store.Watch, error) {
	term, err := validateTerm(input.Term)
	if err != nil {
		return store.Watch{}, err
	}
	addCRN, err := validateCRN(input.AddCRN)
	if err != nil {
		return store.Watch{}, fmt.Errorf("add crn: %w", err)
	}
	dropCRN, err := validateCRN(input.DropCRN)
	if err != nil {
		return store.Watch{}, fmt.Errorf("drop crn: %w", err)
	}
	if addCRN == dropCRN {
		return store.Watch{}, fmt.Errorf("add_crn and drop_crn must be different")
	}

	now := s.now()
	return store.SaveWatch(ctx, s.DB, store.Watch{
		Term:      term,
		Mode:      ModeSwap,
		AddCRN:    addCRN,
		DropCRN:   sql.NullString{String: dropCRN, Valid: true},
		Active:    true,
		CreatedAt: now,
		UpdatedAt: now,
	})
}

// List returns all watches, including disabled watches.
func (s Service) List(ctx context.Context) ([]store.Watch, error) {
	return store.ListWatches(ctx, s.DB)
}

// Enable marks one watch active and returns the updated row.
func (s Service) Enable(ctx context.Context, id int64) (store.Watch, error) {
	if err := store.SetWatchActive(ctx, s.DB, id, true, s.now()); err != nil {
		return store.Watch{}, err
	}
	return store.WatchByID(ctx, s.DB, id)
}

// Disable marks one watch inactive and returns the updated row.
func (s Service) Disable(ctx context.Context, id int64) (store.Watch, error) {
	if err := store.SetWatchActive(ctx, s.DB, id, false, s.now()); err != nil {
		return store.Watch{}, err
	}
	return store.WatchByID(ctx, s.DB, id)
}

// Remove deletes one watch.
func (s Service) Remove(ctx context.Context, id int64) error {
	// Hard delete is acceptable while watches have no attempt history. Once
	// attempts are populated, remove should preserve auditability.
	return store.DeleteWatch(ctx, s.DB, id)
}

func validateTerm(term string) (string, error) {
	term = normalize(term)
	if !isNDigits(term, 6) {
		return "", fmt.Errorf("term must be exactly 6 digits")
	}
	return term, nil
}

func validateCRN(crn string) (string, error) {
	crn = normalize(crn)
	if !isNDigits(crn, 5) {
		return "", fmt.Errorf("crn must be exactly 5 digits")
	}
	return crn, nil
}

func normalize(value string) string {
	return strings.TrimSpace(value)
}

func isNDigits(value string, n int) bool {
	if len(value) != n {
		return false
	}
	for _, r := range value {
		// Use unicode.IsDigit instead of byte math so validation remains obvious
		// and correct even if future inputs come from non-terminal sources.
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}
