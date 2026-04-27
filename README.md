# SeatHawk

SeatHawk is a single-user Go CLI for monitoring Virginia Tech course seats and,
eventually, attempting registration with the user's own local VT session.

Current status: the app can import/validate a VT session and create live-checked
local watch definitions. It does **not** poll continuously or register/drop
courses yet.

## Quick Start

Run the CLI:

```sh
go run ./cmd/seathawk run
```

This creates the default config and SQLite database under:

```text
~/.config/seathawk/
```

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
docs/              VT API reference and planning notes
```

## How The Current Pieces Connect

The CLI layer parses commands and prints human-readable output. It should stay
thin. Domain packages decide what valid work means, and the store package only
does SQLite reads/writes.

```text
CLI command
  -> config.Load + store.Open + store.Migrate
  -> session.Service or watch.Service
  -> store helpers
  -> local SQLite database
```

Session commands construct a VT client because they validate the copied
authtoken against `studentdata`. Watch creation also constructs a VT client, but
only for read-only checks: `studentdata` for current registration state and
`fose` search for CRN existence/current section status.

The `watches` table has fields such as `last_seen_stat`, `next_poll_at`, and
`last_attempt_at`. Watch creation now fills the initial section status and first
poll time. The daemon polling loop and registration-attempt history still come
later.

## Verification

Run the full test and vet suite:

```sh
go test ./...
go vet ./...
```
