package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrNoAttempt means no registration attempt exists for the requested ID.
var ErrNoAttempt = errors.New("no attempt found")

const attemptColumns = `
	id,
	watch_id,
	phase,
	outcome_kind,
	raw_response,
	created_at
`

// Attempt is one durable audit event from the attempts table.
//
// Registration can cross several VT systems: read-only checks, cart staging,
// shockabsorber submission, status polling, and final studentdata
// reconciliation. Attempt rows preserve those important phase boundaries
// without forcing the rest of the application to parse CLI output or logs.
type Attempt struct {
	ID          int64
	WatchID     int64
	Phase       string
	OutcomeKind string
	RawResponse sql.NullString
	CreatedAt   time.Time
}

// SaveAttempt appends one registration-attempt audit event and returns it.
//
// Store deliberately does not interpret Phase or OutcomeKind. Higher layers own
// registration semantics; this layer only persists the timeline.
func SaveAttempt(ctx context.Context, db *sql.DB, attempt Attempt) (Attempt, error) {
	result, err := db.ExecContext(ctx, `
		INSERT INTO attempts (
			watch_id,
			phase,
			outcome_kind,
			raw_response,
			created_at
		) VALUES (?, ?, ?, ?, ?)
	`,
		attempt.WatchID,
		attempt.Phase,
		attempt.OutcomeKind,
		nullStringValue(attempt.RawResponse),
		attempt.CreatedAt.UTC(),
	)
	if err != nil {
		return Attempt{}, fmt.Errorf("insert attempt: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Attempt{}, fmt.Errorf("read inserted attempt id: %w", err)
	}
	return AttemptByID(ctx, db, id)
}

// AttemptByID returns one attempt audit event by primary key.
func AttemptByID(ctx context.Context, db *sql.DB, id int64) (Attempt, error) {
	row := db.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT %s
		FROM attempts
		WHERE id = ?
	`, attemptColumns), id)

	attempt, err := scanAttempt(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Attempt{}, ErrNoAttempt
	}
	if err != nil {
		return Attempt{}, fmt.Errorf("read attempt: %w", err)
	}
	return attempt, nil
}

type attemptScanner interface {
	Scan(dest ...any) error
}

// scanAttempt maps the attempts table column order into an Attempt.
func scanAttempt(scanner attemptScanner) (Attempt, error) {
	var attempt Attempt
	if err := scanner.Scan(
		&attempt.ID,
		&attempt.WatchID,
		&attempt.Phase,
		&attempt.OutcomeKind,
		&attempt.RawResponse,
		&attempt.CreatedAt,
	); err != nil {
		return Attempt{}, err
	}
	return attempt, nil
}
