# SeatHawk

SeatHawk is a single-user Go CLI for monitoring Virginia Tech course seats and,
eventually, attempting registration with the user's own local VT session.

Current status: the app can import/validate a VT session, create live-checked
local watch definitions, run a continuous polling daemon, run an explicit
manual single-CRN add attempt, and optionally let the daemon auto-attempt
eligible add watches. Daemon registration is off unless `--auto-register` is
passed.

## Quick Start

Run the daemon:

```sh
go run ./cmd/seathawk run
```

This creates the default config and SQLite database under:

```text
~/.config/seathawk/
```

The daemon keeps running until you stop it with Ctrl-C.

## Import A VT Session

1. Log in to the VT registration page in your browser.
2. Run the authtoken bookmarklet from [docs/VT_API_REFERENCE.md](docs/VT_API_REFERENCE.md).
3. Import the copied JSON:

```sh
pbpaste | go run ./cmd/seathawk session import -
```

Check the stored session:

```sh
go run ./cmd/seathawk session validate
go run ./cmd/seathawk session show
```

SeatHawk stores the authtoken locally, calls `studentdata`, and derives the
current `pers_id` / `pers_id_proof` needed by later registration phases.

## Manage Watches

Watches are local records of what you want SeatHawk to attempt later. Creating a
watch now performs read-only VT sanity checks, but it does not register or drop
anything.

There are two watch modes:

- `add`: watch one CRN and eventually try to add it.
- `swap`: watch one CRN to add while explicitly naming a CRN to drop.

Swap is separate on purpose. Later registration code must treat add/drop
workflows as potentially destructive and reconcile the result carefully against
fresh `studentdata`.

Watch creation requires an imported valid session because SeatHawk checks your
current `studentdata` before saving. This prevents watches that are already
satisfied or unsafe, such as watching an add CRN you are already registered for
or creating a swap whose drop CRN is not currently in your schedule.

Full sections are valid watch targets. In practice, most watches are expected to
start with a full CRN and wait until a future polling phase sees it open.

### Term Codes

VT calls the selected academic term `srcdb` in the public course-search API and
`term_code` in registration-related API calls. SeatHawk exposes this as the
`--term` flag.

Term codes are six digits: the four-digit year plus a two-digit term suffix.
Known examples:

- `202601`: Spring 2026
- `202606`: Summer 2026
- `202609`: Fall 2026
- `202612`: Winter 2026-2027

Use the term code that matches the course-search term you are looking at on
classes.vt.edu. SeatHawk validates that the value is exactly six digits, then VT
determines whether that term currently exists and whether your session has a
registration ticket for it.

Create an add watch:

```sh
go run ./cmd/seathawk watch add --term 202609 --crn 60058
```

Create a swap watch:

```sh
go run ./cmd/seathawk watch swap --term 202609 --add-crn 60058 --drop-crn 60900
```

List watches:

```sh
go run ./cmd/seathawk watch list
```

Run one read-only manual poll pass for watches whose `next_poll_at` time has arrived:

```sh
go run ./cmd/seathawk watch poll
```

Force a read-only poll pass for every active watch, ignoring `next_poll_at`:

```sh
go run ./cmd/seathawk watch poll --all
```

Polling refreshes local watch metadata such as `last_seen_stat` and
`next_poll_at`. It only reads VT `studentdata` and `fose`; it does not add a
class to the cart, preflight registration, register, drop, or swap anything.

## Attempt Manual Registration

Manual registration attempts are available separately from daemon polling. This
is useful for proving a watch and session before allowing unattended
auto-registration.

Attempt one existing add watch:

```sh
go run ./cmd/seathawk register attempt <watch-id> --confirm
```

The `--confirm` flag is required because this command can submit a real VT
registration attempt. The command currently supports add watches only. Swap
registration is not enabled yet.

Before any VT write, SeatHawk:

- loads the current valid session
- fetches fresh `studentdata`
- searches the add CRN in `fose`
- verifies the CRN exists, is open, is not already registered, and the
  registration window allows add
- runs VT `preflight`

If those checks pass, SeatHawk stages the CRN with `cart_add`, submits exactly
one `shockabsorber register` call, polls `shockabsorber status`, and then
confirms the final state with fresh `studentdata`. Fresh `studentdata` is the
source of truth; shockabsorber text is treated as diagnostic output, not final
proof.

SeatHawk records attempt audit rows locally. On confirmed manual success, the
watch is disabled so it will not be attempted again. Failed or ambiguous manual
attempts keep the watch active, but update `last_attempt_at`. Cart cleanup is
intentionally not automatic yet.

## Run The Daemon

After importing a valid session and creating watches, start continuous read-only
polling:

```sh
go run ./cmd/seathawk run
```

The daemon polls once immediately, then repeats on SeatHawk's default poll
interval. Each pass respects `next_poll_at`, so active watches are only checked
when they are due. Logs are written to stderr and include the config path,
database path, checked watch count, watch IDs, observed section status, and any
per-watch errors.

Stop the daemon with Ctrl-C. Without extra flags, the daemon is read-only: it
does not add a class to the cart, preflight registration, register, drop, or
swap anything.

To allow unattended registration attempts for eligible add watches, start the
daemon with:

```sh
go run ./cmd/seathawk run --auto-register
```

Auto-registration only applies to add watches. Swap watches remain read-only in
this phase. When auto mode sees an active add watch that is open and whose
registration window is ready, it hands the watch to the same registration flow
used by `register attempt`. That flow still performs fresh `studentdata`,
`fose`, and `preflight` checks before writing to VT.

Confirmed automatic success disables the watch. Failed or ambiguous automatic
attempts also disable the watch so SeatHawk cannot repeatedly submit VT writes
without manual review. Full sections and not-yet-open registration windows keep
polling normally.

Pause and resume watches:

```sh
go run ./cmd/seathawk watch disable <id>
go run ./cmd/seathawk watch enable <id>
```

Remove a watch:

```sh
go run ./cmd/seathawk watch remove <id>
```

## Project Structure

```text
cmd/seathawk/      CLI executable entrypoint
internal/cli/      Cobra commands and command output
internal/config/   Local config loading
internal/store/    SQLite setup and persistence helpers
internal/session/  VT session import and validation workflow
internal/vt/       VT protocol client and response parsing
internal/watch/    Local watch validation and workflow rules
internal/register/ Explicit manual registration-attempt orchestration
internal/automation/ Daemon pass coordination and auto-attempt decisions
docs/              VT API reference and planning notes
```

## How The Current Pieces Connect

The CLI layer parses commands and prints human-readable output. It should stay
thin. Domain packages decide what valid work means, and the store package only
does SQLite reads/writes.

```text
CLI command
  -> config.Load + store.Open + store.Migrate
  -> session.Service, watch.Service, register.Service, or automation.Service
  -> store helpers
  -> local SQLite database
```

Session commands construct a VT client because they validate the copied
authtoken against `studentdata`. Watch creation, manual polling, and the daemon
also construct a VT client, but only for read-only checks: `studentdata` for
current registration state and `fose` search for CRN existence/current section
status.

The `watches` table has fields such as `last_seen_stat`, `next_poll_at`, and
`last_attempt_at`. Watch creation fills the initial section status and first
poll time. `watch poll` and `run` refresh the read-only scheduler metadata.
`register attempt` reuses the same watch evaluation rules before performing the
VT write sequence and records phase-level history in the `attempts` table.
`run --auto-register` connects polling to that same registration service only
after the daemon observes an open, registration-ready add watch.

## Verification

Run the full test and vet suite:

```sh
go test ./...
go vet ./...
```
