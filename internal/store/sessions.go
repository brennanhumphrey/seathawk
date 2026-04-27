package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrNoSession means no VT session has been imported yet.
var ErrNoSession = errors.New("no session found")

// Session is one imported VT session row.
//
// Sessions are append-only history. The newest row is treated as the current
// session, while Status tracks whether that row was last validated successfully.
type Session struct {
	ID              int64
	Authtoken       string
	PersID          string
	PersIDProof     string
	CapturedAt      time.Time
	LastValidatedAt sql.NullTime
	Status          string
}

// SaveSession appends session to the local session history and returns the
// inserted row.
func SaveSession(ctx context.Context, db *sql.DB, session Session) (Session, error) {
	result, err := db.ExecContext(ctx, `
		INSERT INTO sessions (
			authtoken,
			pers_id,
			pers_id_proof,
			captured_at,
			last_validated_at,
			status
		) VALUES (?, ?, ?, ?, ?, ?)
	`,
		session.Authtoken,
		session.PersID,
		session.PersIDProof,
		session.CapturedAt.UTC(),
		nullTimeValue(session.LastValidatedAt),
		session.Status,
	)
	if err != nil {
		return Session{}, fmt.Errorf("insert session: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Session{}, fmt.Errorf("read inserted session id: %w", err)
	}
	return sessionByID(ctx, db, id)
}

// CurrentSession returns the newest imported session.
func CurrentSession(ctx context.Context, db *sql.DB) (Session, error) {
	// There is no active flag yet. In the current single-user model, importing
	// credentials makes that newest row the current session.
	row := db.QueryRowContext(ctx, `
		SELECT
			id,
			authtoken,
			pers_id,
			pers_id_proof,
			captured_at,
			last_validated_at,
			status
		FROM sessions
		ORDER BY id DESC
		LIMIT 1
	`)

	session, err := scanSession(row)
	if errors.Is(err, ErrNoSession) {
		return Session{}, ErrNoSession
	}
	if err != nil {
		return Session{}, fmt.Errorf("read current session: %w", err)
	}

	return session, nil
}

func sessionByID(ctx context.Context, db *sql.DB, id int64) (Session, error) {
	row := db.QueryRowContext(ctx, `
		SELECT
			id,
			authtoken,
			pers_id,
			pers_id_proof,
			captured_at,
			last_validated_at,
			status
		FROM sessions
		WHERE id = ?
	`, id)

	session, err := scanSession(row)
	if errors.Is(err, ErrNoSession) {
		return Session{}, ErrNoSession
	}
	if err != nil {
		return Session{}, fmt.Errorf("read inserted session: %w", err)
	}
	return session, nil
}

type sessionScanner interface {
	Scan(dest ...any) error
}

func scanSession(row sessionScanner) (Session, error) {
	var session Session
	err := row.Scan(
		&session.ID,
		&session.Authtoken,
		&session.PersID,
		&session.PersIDProof,
		&session.CapturedAt,
		&session.LastValidatedAt,
		&session.Status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNoSession
	}
	if err != nil {
		return Session{}, err
	}
	return session, nil
}

// UpdateSessionValidation updates validation metadata for one stored session.
//
// Validation modifies the selected row in-place instead of appending a new row;
// importing credentials is what creates session history. If persIDProof is
// non-empty, it also refreshes the stored proof because VT may rotate idProof
// between studentdata calls.
func UpdateSessionValidation(ctx context.Context, db *sql.DB, id int64, status string, validatedAt time.Time, persIDProof string) error {
	result, err := db.ExecContext(ctx, `
		UPDATE sessions
		SET
			status = ?,
			last_validated_at = ?,
			pers_id_proof = COALESCE(NULLIF(?, ''), pers_id_proof)
		WHERE id = ?
	`, status, validatedAt.UTC(), persIDProof, id)
	if err != nil {
		return fmt.Errorf("update session validation: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read session validation update count: %w", err)
	}
	if rows == 0 {
		return ErrNoSession
	}
	return nil
}

func nullTimeValue(value sql.NullTime) any {
	if !value.Valid {
		return nil
	}
	return value.Time.UTC()
}
