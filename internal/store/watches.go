package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ErrNoWatch means no watch exists for the requested ID.
var ErrNoWatch = errors.New("no watch found")

const watchColumns = `
	id,
	term,
	mode,
	add_crn,
	drop_crn,
	active,
	last_seen_stat,
	next_poll_at,
	last_attempt_at,
	created_at,
	updated_at
`

// Watch is one durable user intent row from the watches table.
//
// The watch row is intentionally broader than the current Phase 5 CLI needs.
// Term/Mode/AddCRN/DropCRN describe the user's desired future action. Active
// lets the user pause that intent without deleting it. LastSeenStat,
// NextPollAt, and LastAttemptAt are scheduler state for later phases, when the
// daemon starts polling VT and attempting registration.
type Watch struct {
	ID            int64
	Term          string
	Mode          string // "add" or "swap"; stored explicitly so drop_crn is not the source of truth.
	AddCRN        string
	DropCRN       sql.NullString // Only valid for swap watches.
	Active        bool
	LastSeenStat  sql.NullString // Last normalized fose section status, for example "full" or "open".
	NextPollAt    sql.NullTime   // Future scheduler timestamp for the next read-only availability check.
	LastAttemptAt sql.NullTime   // Future registration-attempt timestamp.
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// SaveWatch appends a new watch and returns the stored row.
//
// Store functions assume callers already made domain decisions. Validation such
// as "swap watches need different add/drop CRNs" belongs in internal/watch, not
// in this SQL layer.
func SaveWatch(ctx context.Context, db *sql.DB, watch Watch) (Watch, error) {
	result, err := db.ExecContext(ctx, `
		INSERT INTO watches (
			term,
			mode,
			add_crn,
			drop_crn,
			active,
			last_seen_stat,
			next_poll_at,
			last_attempt_at,
			created_at,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		watch.Term,
		watch.Mode,
		watch.AddCRN,
		nullStringValue(watch.DropCRN),
		boolToInt(watch.Active),
		nullStringValue(watch.LastSeenStat),
		nullTimeValue(watch.NextPollAt),
		nullTimeValue(watch.LastAttemptAt),
		watch.CreatedAt.UTC(),
		watch.UpdatedAt.UTC(),
	)
	if err != nil {
		return Watch{}, fmt.Errorf("insert watch: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Watch{}, fmt.Errorf("read inserted watch id: %w", err)
	}
	return WatchByID(ctx, db, id)
}

// ListWatches returns every stored watch in creation order.
//
// Disabled watches are intentionally included. "Inactive" means paused, not
// forgotten, and hiding paused watches would make the CLI misleading.
func ListWatches(ctx context.Context, db *sql.DB) ([]Watch, error) {
	return listWatchesByQuery(ctx, db, fmt.Sprintf(`
		SELECT %s
		FROM watches
		ORDER BY id ASC
	`, watchColumns))
}

// ListActiveWatches returns every active watch in creation order.
func ListActiveWatches(ctx context.Context, db *sql.DB) ([]Watch, error) {
	return listWatchesByQuery(ctx, db, fmt.Sprintf(`
		SELECT %s
		FROM watches
		WHERE active = 1
		ORDER BY id ASC
	`, watchColumns))
}

// ListDueActiveWatches returns active watches whose poll time has arrived.
func ListDueActiveWatches(ctx context.Context, db *sql.DB, now time.Time) ([]Watch, error) {
	return listWatchesByQuery(ctx, db, fmt.Sprintf(`
		SELECT %s
		FROM watches
		WHERE active = 1
			AND (next_poll_at IS NULL OR next_poll_at <= ?)
		ORDER BY next_poll_at IS NOT NULL, next_poll_at ASC, id ASC
	`, watchColumns), now.UTC())
}

func listWatchesByQuery(ctx context.Context, db *sql.DB, query string, args ...any) ([]Watch, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list watches: %w", err)
	}
	defer rows.Close()

	var watches []Watch
	for rows.Next() {
		watch, err := scanWatch(rows)
		if err != nil {
			return nil, err
		}
		watches = append(watches, watch)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate watches: %w", err)
	}
	return watches, nil
}

// WatchByID returns one watch by primary key.
func WatchByID(ctx context.Context, db *sql.DB, id int64) (Watch, error) {
	row := db.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT %s
		FROM watches
		WHERE id = ?
	`, watchColumns), id)

	watch, err := scanWatch(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Watch{}, ErrNoWatch
	}
	if err != nil {
		return Watch{}, fmt.Errorf("read watch: %w", err)
	}
	return watch, nil
}

// SetWatchActive updates whether a watch should be considered by future polling.
func SetWatchActive(ctx context.Context, db *sql.DB, id int64, active bool, updatedAt time.Time) error {
	result, err := db.ExecContext(ctx, `
		UPDATE watches
		SET
			active = ?,
			updated_at = ?
		WHERE id = ?
	`, boolToInt(active), updatedAt.UTC(), id)
	if err != nil {
		return fmt.Errorf("update watch active: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read watch active update count: %w", err)
	}
	if rows == 0 {
		return ErrNoWatch
	}
	return nil
}

// UpdateWatchEvaluation records the latest read-only evaluation metadata.
//
// This is the polling counterpart to SaveWatch's initial metadata insert:
// future daemon runs can refresh seat status and next poll time without
// changing the user's watch intent.
func UpdateWatchEvaluation(ctx context.Context, db *sql.DB, id int64, lastSeenStat sql.NullString, nextPollAt sql.NullTime, updatedAt time.Time) error {
	result, err := db.ExecContext(ctx, `
		UPDATE watches
		SET
			last_seen_stat = ?,
			next_poll_at = ?,
			updated_at = ?
		WHERE id = ?
	`, nullStringValue(lastSeenStat), nullTimeValue(nextPollAt), updatedAt.UTC(), id)
	if err != nil {
		return fmt.Errorf("update watch evaluation: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read watch evaluation update count: %w", err)
	}
	if rows == 0 {
		return ErrNoWatch
	}
	return nil
}

// UpdateWatchLastAttemptAt records when SeatHawk last tried to register a watch.
//
// This is separate from UpdateWatchEvaluation because read-only polling and
// write-side registration attempts are different scheduler events. Keeping the
// SQL helpers separate makes call sites more explicit about which kind of state
// they are changing.
func UpdateWatchLastAttemptAt(ctx context.Context, db *sql.DB, id int64, attemptedAt time.Time, updatedAt time.Time) error {
	result, err := db.ExecContext(ctx, `
		UPDATE watches
		SET
			last_attempt_at = ?,
			updated_at = ?
		WHERE id = ?
	`, attemptedAt.UTC(), updatedAt.UTC(), id)
	if err != nil {
		return fmt.Errorf("update watch last attempt: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read watch last attempt update count: %w", err)
	}
	if rows == 0 {
		return ErrNoWatch
	}
	return nil
}

// DeleteWatch hard-deletes one watch row.
//
// Watches with attempt history are protected by the attempts foreign key. If a
// user wants to stop future work on an attempted watch, higher layers should
// disable it instead of deleting the audit trail.
func DeleteWatch(ctx context.Context, db *sql.DB, id int64) error {
	result, err := db.ExecContext(ctx, `DELETE FROM watches WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete watch: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read watch delete count: %w", err)
	}
	if rows == 0 {
		return ErrNoWatch
	}
	return nil
}

type watchScanner interface {
	Scan(dest ...any) error
}

func scanWatch(scanner watchScanner) (Watch, error) {
	var watch Watch
	var active int
	if err := scanner.Scan(
		&watch.ID,
		&watch.Term,
		&watch.Mode,
		&watch.AddCRN,
		&watch.DropCRN,
		&active,
		&watch.LastSeenStat,
		&watch.NextPollAt,
		&watch.LastAttemptAt,
		&watch.CreatedAt,
		&watch.UpdatedAt,
	); err != nil {
		return Watch{}, err
	}
	watch.Active = intToBool(active)
	return watch, nil
}
