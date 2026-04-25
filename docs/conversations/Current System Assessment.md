> From: https://chatgpt.com/c/69e90bbd-6254-83ea-8414-c0cf748f5169

# you asked

Thoughts on all this so far and where we are currenrly?

---

# chatgpt response

I’m pulling the threads together instead of just echoing the previous back-and-forth. The main thing I’m checking is whether the conversation has actually resolved the load-bearing unknowns, or just gotten more articulate about them.

I have the gist. I’m opening the strongest chunks so I can pin this down with exact citations instead of hand-waving.

Here’s my read.

## What is actually solid

The **core architecture is fine**. The shared-polling model with a `sections` layer and a separate per-user `watch_subscriptions` layer is the right shape. The web server handles CRUD and user-facing requests, the worker does background polling and registration, and both coordinate through Postgres rather than calling each other directly. That part is not the problem anymore.  

The worker model is also basically settled: one worker process for MVP, with goroutine pools inside it, a fast scheduling loop, and per-section `next_poll_at` timestamps so you are not checking everything at once. Conceptually, that piece is understood well enough to build.  

## Where you actually are

You are **not blocked by backend design**. You are blocked by two empirical unknowns and one policy problem.

First, **shockabsorber with `authtoken` alone is still unproven**, and that is the load-bearing assumption in the whole public-SaaS design. Multiple parts of the conversation correctly identified this as the thing to test before building more backend code. If that fails, the bookmarklet-plus-token model is not your product, full stop.  

Second, **token lifetime is still unknown**, and that directly determines whether the UX is viable or annoying garbage. If the token dies quickly or on logout, the whole “set it and forget it” story falls apart. The conversation already laid out the right empirical checks: 1h, 6h, 24h, 48h, and logout behavior. Until that is tested, the user experience is speculation. 

Third, and this is the uncomfortable one, **public SaaS looks like the wrong shape for this project**. Based on the quoted acceptable-use language in the file, handing your service VT access tokens so your servers act on the user’s behalf fits the policy framing of sharing or transferring access codes. The discussion in the file is blunt about this: public SaaS with server-side token storage is the version most exposed on policy grounds, and it also puts users at risk, not just you.  

## My actual opinion

Right now, you have **an architecture that makes sense for a product you probably should not deploy in that form**. That is where you are.

More bluntly: the conversation got very good at explaining a backend that is internally coherent, but it also kept drifting past the fact that the **real go/no-go questions were never answered**. The backend explanations were useful, but they also created the illusion of progress. The actual progress gate is still: can `authtoken` do the full registration flow, how long does it last, and are you willing to build something whose public version appears to sit in direct tension with VT’s policy posture.   

So my position is:

- **If you insist on public SaaS**, you are still in validation mode, not build mode.
- **If your goal is a real shippable project**, the smarter direction is probably a **local tool, browser extension, or client-side app** that keeps credentials on the student’s machine and uses your backend, if any, only for non-sensitive coordination or public seat monitoring. That is the direction the conversation itself kept converging toward once the policy implications were taken seriously.  

## Current status in one sentence

**Architecture: mostly figured out. Product shape: not resolved. Critical assumptions: not yet proven. Public SaaS: likely the wrong bet.**  

If you want, I’ll give you the exact next-step decision tree from here: what to test first, what result kills public SaaS, and what local-first architecture I’d use instead.

---

# you asked

talking more about the actually building the opensource version of the tool from scratch - the directory struture, how it should be built, etc

---

# chatgpt response

I’m switching from product-risk talk to implementation shape. I’m checking the uploaded material for the local/open-source direction so I can anchor the build plan to what you already established, not invent a parallel project.

I have enough. The main design decision is whether this is “an extension with some glue” or “a real local app.” My view is that extension-first sounds simpler but is actually the worse foundation if you want something reliable.

Yes. And for the open-source version, you should **stop thinking like you’re building a SaaS**.

The earlier thread already landed on the real shift: once this is a local-install tool, you no longer need shared auth, user accounts, a hosted database, or public infrastructure. The local version is simpler precisely because everything stays on the user’s machine.  

## My recommendation

**Build one local Go daemon first. Not a browser extension first. Not a polished desktop app first.**

Extension-first sounds simpler, but it is a worse foundation for long-running polling, local persistence, retry logic, logs, and packaging. Build the engine first. Add UI later.

So the shape should be:

- **Go core** for polling, registration, persistence, scheduling
- **SQLite** for local state
- **CLI** for setup and debugging
- **launchd** on macOS to keep it running in the background
- **Optional UI later** as either:
  - a local web UI served by the daemon, or
  - a thin Tauri wrapper over that UI

That gets you a real tool fast instead of a pretty shell around unfinished logic.

---

## What the local open-source architecture should look like

For the OSS/local version, the “backend” still exists, but it is **local**:

```text
User
  -> CLI / local UI
      -> local daemon (Go)
          -> SQLite
          -> VT public seat-status endpoints
          -> VT registration endpoints using the user's own local session/token
```

No hosted web server.  
No multi-tenant Postgres.  
No cloud worker.  
No auth system.

That matches the earlier conclusion that the local-tool version avoids the whole “store other people’s credentials on your server” problem and removes most of the SaaS machinery.  

---

## The repo structure I would use

This is the structure I’d actually build.

```text
vtwatch/
├── README.md
├── LICENSE
├── Makefile
├── .gitignore
├── .github/
│   └── workflows/
│       ├── ci.yml
│       └── release.yml
├── docs/
│   ├── architecture.md
│   ├── api-notes.md
│   ├── local-security.md
│   ├── state-machine.md
│   └── acceptable-use.md
├── cmd/
│   ├── vtwatch/
│   │   └── main.go          # CLI entrypoint
│   └── vtwatchd/
│       └── main.go          # local daemon entrypoint
├── internal/
│   ├── app/
│   │   ├── daemon.go
│   │   ├── wiring.go
│   │   └── lifecycle.go
│   ├── config/
│   │   ├── config.go
│   │   └── paths.go
│   ├── logging/
│   │   └── logger.go
│   ├── store/
│   │   ├── sqlite/
│   │   │   ├── db.go
│   │   │   ├── migrations.go
│   │   │   ├── watches.go
│   │   │   ├── sessions.go
│   │   │   ├── attempts.go
│   │   │   └── events.go
│   │   └── models/
│   │       ├── watch.go
│   │       ├── session.go
│   │       ├── attempt.go
│   │       └── event.go
│   ├── vt/
│   │   ├── fose/
│   │   │   ├── client.go
│   │   │   ├── types.go
│   │   │   └── parser.go
│   │   ├── registrar/
│   │   │   ├── preflight.go
│   │   │   ├── cart_add.go
│   │   │   ├── shockabsorber.go
│   │   │   ├── status.go
│   │   │   └── errors.go
│   │   └── session/
│   │       ├── source.go
│   │       ├── import_token.go
│   │       └── validate.go
│   ├── watch/
│   │   ├── scheduler.go
│   │   ├── poller.go
│   │   ├── detector.go
│   │   ├── service.go
│   │   └── states.go
│   ├── register/
│   │   ├── service.go
│   │   ├── runner.go
│   │   ├── reconcile.go
│   │   └── policy.go
│   ├── notify/
│   │   ├── macos.go
│   │   └── stdout.go
│   └── httpapi/             # optional local API for a future UI
│       ├── server.go
│       ├── routes.go
│       └── handlers.go
├── migrations/
│   ├── 001_init.sql
│   ├── 002_attempt_logs.sql
│   └── 003_events.sql
├── testdata/
│   ├── fose/
│   ├── preflight/
│   ├── cart_add/
│   └── shockabsorber/
├── scripts/
│   ├── dev.sh
│   ├── install-launchd.sh
│   └── uninstall-launchd.sh
└── web/                    # only add later if you actually need UI
    ├── package.json
    ├── src/
    └── vite.config.ts
```

---

## Why this structure is right

### `cmd/`
Two executables:

- `vtwatch` = human-facing CLI
- `vtwatchd` = long-running local daemon

That split matters. The CLI is for setup, inspection, and manual actions. The daemon is the thing that keeps polling and attempting registration.

### `internal/vt/`
This is your reverse-engineered adapter layer. Keep it isolated.

Do not scatter VT request logic across the app. That is how projects like this become unreadable garbage.

You want one place for:
- seat-status polling
- session validation
- preflight
- cart add
- shockabsorber register
- result parsing

### `internal/watch/`
This is the seat-monitoring domain.

Responsibilities:
- maintain watch definitions
- calculate `next_poll_at`
- poll due watches
- detect state transitions like `F -> A`
- enqueue a registration attempt locally

### `internal/register/`
Separate from watch logic.

Responsibilities:
- run the registration flow
- handle retries
- classify failures
- reconcile weird states
- stop duplicate attempts

Do **not** jam this into `watch/`. Polling and registration are different concerns.

### `internal/store/sqlite/`
Local tool means local persistence. SQLite is the right call.

You do not need Postgres for a single-user local daemon. Using Postgres here would be fake sophistication.

### `internal/httpapi/`
Only if you add a UI later.

Don’t start here. But if you later build a React or Tauri frontend, this gives the frontend a clean local API instead of coupling UI code directly to the daemon internals.

---

## What the local database should hold

Keep it small.

### `watches`
```sql
id
term_code
crn
drop_crn
status              -- active, paused, success, blocked, failed
last_seen_stat
next_poll_at
created_at
updated_at
```

### `sessions`
```sql
id
auth_token
cookie_blob         -- only if you actually need it
last_validated_at
expires_at          -- nullable, because you may not know
status              -- valid, expired, unknown
```

### `attempts`
```sql
id
watch_id
phase               -- preflight, cart_add, register, status_poll
result              -- success, blocked, failed, unknown
response_blob
created_at
```

### `events`
```sql
id
watch_id
kind                -- seat_opened, session_expired, registration_success, etc.
message
created_at
```

That is enough for an OSS local tool.

You do **not** need a full event bus, distributed queue, or analytics warehouse. Don’t be ridiculous.

---

## How the app should actually run

### CLI examples

```bash
vtwatch session import
vtwatch watch add --term 202608 --crn 12345 --drop 54321
vtwatch watch list
vtwatch daemon start
vtwatch daemon status
vtwatch logs tail
```

### Daemon loop

Your daemon should do roughly this:

1. load config
2. open SQLite
3. start scheduler ticker
4. claim due watches
5. poll seat status
6. if a seat opens, run registration flow
7. write logs/events
8. send local notification
9. repeat

That is the local equivalent of the worker concept you already spent time understanding. The difference is that now it is all single-user and on-device, not split across web server plus cloud worker. 

---

## Build order

This is the part where I’m going to push back.

**Do not start by building the UI.**  
That would be backwards.

Build in this order:

### Phase 1: core polling only
- config loading
- SQLite persistence
- `fose` client
- watch scheduler
- CLI commands: add/list/remove/run
- structured logs

Goal: user can locally watch a CRN and see state changes.

### Phase 2: session import + validation
- import token/session locally
- validate session
- record expiration/failure states
- manual dry-run checks

Goal: prove you can use the local session reliably.

### Phase 3: registration flow
- preflight
- cart add
- shockabsorber submit
- result reconciliation
- retry policy
- local notifications

Goal: end-to-end registration works on one machine.

### Phase 4: background operation on macOS
- `launchd` plist
- install/uninstall scripts
- log locations
- auto-start on login if desired

Goal: user does not need to keep a terminal open.

### Phase 5: optional UI
- local HTTP API
- simple React UI or Tauri shell
- watch creation, status, logs, session health

Goal: make it nicer, not more correct.

---

## What I would not build yet

Do **not** start with:

- Electron
- Tauri
- browser extension as the main product
- plugin system
- separate polling and registration daemons
- remote sync
- cloud relay
- multi-user anything
- Postgres
- queue infrastructure

That is all premature.

The thread already established that local-install removes the need for most of the SaaS architecture. If you recreate that complexity locally, you’ve learned nothing.  

---

## My opinion on the tech stack

For your machine and for this project:

### Core
- **Go**
- **SQLite**
- **Cobra** for CLI if you want clean subcommands
- standard `net/http`
- `context.Context` everywhere
- `slog` or `zap` for logs

### macOS integration
- `launchd` for daemon management
- native notifications through a small wrapper or shell out to `osascript` initially

### UI later
- local web UI with React/Vite
- or Tauri only after the core works

That gets you a clean ARM-native path on macOS without building a bloated desktop stack too early.

---

## The one architectural choice that matters most

You need to decide whether the session source is:

1. **manual paste/import**, or
2. **browser-assisted local capture**

I would start with **manual import** because it is simpler and easier to debug.

If that proves annoying, then add either:
- a bookmarklet that POSTs to `localhost`, or
- a small extension that sends the session data to the local daemon

But again, that is **phase 2 or 3**, not day 1.

---

## Bottom line

If this were my repo, I would build:

- **one Go monorepo**
- **two binaries**: `vtwatch` and `vtwatchd`
- **SQLite for local state**
- **no hosted backend**
- **CLI first**
- **launchd for persistence on macOS**
- **optional UI later**

That is the cleanest open-source version.

If you want, next I’ll turn this into a concrete starter skeleton with actual file contents for `cmd/`, `internal/watch/`, `internal/vt/fose/`, and the first SQLite migration.

---

# you asked

and also the plan would probably involve hosting it on some free tier instance of some platform

---

# chatgpt response

This changes the shape again. A “local/open-source tool” and a “free-tier hosted instance” are not the same thing, and the distinction matters both technically and on the policy side.

No. Be careful here.

The moment you say **“host it on some free-tier instance”**, you are no longer describing the clean local/open-source version we were just talking about. You are drifting back toward the SaaS shape, and that reintroduces the exact problems you were trying to remove.

## The hard distinction

### Local OSS version
- single-user
- runs on the student’s machine
- no public-facing infrastructure
- no token storage liability on your side
- requests come from each user’s own IP, not one central server IP  

### Hosted instance
Once you centralize anything important, you start reintroducing:
- server-side token/session handling
- one shared outbound IP
- a central point VT can notice/block
- a public-service posture instead of “user runs their own tool”  

So first: **what exactly are you hosting?** Because that answer changes whether this is still the OSS/local design or just SaaS with extra coping language.

## The versions that make sense

### 1. Good: host only distribution stuff
This is fine:
- GitHub repo
- GitHub Releases for binaries
- docs site
- install script
- update manifest
- maybe a landing page

That does **not** change the local architecture. The actual daemon still runs locally. This is the version I’d recommend.

### 2. Maybe acceptable: host a thin, non-sensitive helper
This is the farthest I’d go:
- release metadata
- anonymous telemetry if users explicitly opt in
- maybe a static catalog/cache of public course metadata

But the hosted helper should **not**:
- hold VT tokens
- call registration endpoints
- perform shockabsorber requests
- manage watches on behalf of users

Once it does those things, you are back in central-service territory.

### 3. Bad idea: host the polling/registration engine
This is just the SaaS version again, whether it’s on Fly/Railway/Render/free tier or not.

And that version is exactly the one the prior material kept flagging as the risky shape:
- public/commercial app posture is flagged as high risk / likely not permitted in the earlier project notes  
- all traffic comes from one server IP, which makes the stampede/bot signature much worse  
- it throws away the “no public-facing infrastructure” and “no token storage liability” advantages of the local version 

## My opinion

If your real plan is:

> “open-source version, but also probably hosted on a free tier instance”

then you are mixing two incompatible strategies.

That is muddled thinking.

You need to pick one of these:

### Path A: true local OSS tool
Build:
- local Go daemon
- local SQLite
- launchd service on macOS
- optional local web UI
- GitHub Releases for binaries

Host only docs and downloads.

### Path B: hosted service
Then be honest that it is a hosted service, not the nice clean local version. That means:
- central DB
- worker
- token/session storage
- rate limiting
- abuse controls
- much worse policy exposure

You do not get to keep the benefits of local architecture while quietly centralizing the hard part.

## What I would do

I would do **Path A** and keep hosting extremely boring:

- GitHub repo
- GitHub Releases
- static docs site
- maybe a Homebrew tap later for install convenience on macOS

That’s it.

The actual tool should remain:
- local daemon
- local session/token handling
- local watch state
- local registration execution

If you need background reliability, solve that with `launchd`, not with a cloud worker.

## If you insist on a hosted component

Then make it **credential-blind** and **non-authoritative**.

Acceptable hosted roles:
- binary distribution
- changelog / release metadata
- optional anonymous metrics
- maybe public-seat metadata cache

Unacceptable hosted roles, if you want to stay on the local-tool side:
- storing auth tokens
- executing registration
- owning the watch queue
- doing protected VT calls for users

That line matters.

## Bottom line

**Host the software, not the automation.**

If you host the automation, you’re back in SaaS land, and all the complexity and risk comes roaring back.  

The next useful step is to decide whether you want:
1. a pure local daemon with GitHub Releases, or
2. a hybrid design with a tiny hosted helper that never touches credentials.

---

# you asked

also, correct me if i am wrong, but that project structure seems extremely complicated. Lets think about if this is the best/optimal structure for the project, weighing pros and cons and alternatives and come to a conclusion in terms of best practices for golang projects

---

# chatgpt response

You’re right. That structure was too heavy for a first serious Go project. I’m checking current Go guidance rather than defending it, because a lot of “best practice” advice in Go is cargo cult.

Yes. You were right.

That structure was **too complicated for the stage and probably too complicated for the project**. In Go, the default should be **start simple and add structure only when the code forces you to**. The official Go docs explicitly show very small layouts for a basic package or a basic command, and recommend introducing `internal` only when supporting packages are actually needed. ([Go.dev](https://go.dev/doc/modules/layout))
## The core mistake

The mistake was treating “possible future complexity” as if it were “current design requirement.”

That is how people end up with:
- ten directories before they have 2,000 lines of code
- packages with one file each
- fake architecture names like `service`, `model`, `app`, `runner`, `policy`, `lifecycle`
- boundaries that look clean on paper but slow you down in practice

That is not idiomatic Go. Go tends to favor **fewer packages, clearer names, and flatter layouts** unless there is a real reason to split. The Go team’s own guidance is much more minimal than the “standard project layout” repo people copy from blog posts. ([Go.dev](https://go.dev/doc/modules/layout))
## What Go best practice actually points toward

A few principles matter here.

### 1. One module, minimal layout
Go’s official guidance starts with a single `go.mod` at the repo root and a very simple directory structure. A basic command can be just a few files in one directory, and only later does it suggest adding `internal` for supporting packages. ([Go.dev](https://go.dev/doc/modules/layout))
### 2. Use `internal` for private code
`internal` is real and useful. It is not just a naming convention. The Go toolchain enforces that code outside the parent subtree cannot import it, and the Go docs recommend keeping packages in `internal` as much as possible when you do not want to support them as public APIs. ([Go.dev](https://go.dev/doc/go1.4))
### 3. Use `cmd/` only when it helps
`cmd/` is a common convention, especially for repositories with multiple commands or mixed content, but it is not mandatory. The official docs say it is useful in mixed repositories and for server-style projects with multiple binaries, not something you must use from day one for every repo. ([Go.dev](https://go.dev/doc/modules/layout))
### 4. Ignore `pkg/` unless you have a concrete reason
The popular `golang-standards/project-layout` repo says outright that it is **not** an official standard, says it is overkill for learning, PoCs, and simple projects, and even notes that some patterns in it are not universally accepted. Russ Cox also criticized the idea that such complex layouts represent how most Go repos are structured. ([GitHub](https://github.com/golang-standards/project-layout/blob/master/README.md))
### 5. Package names should be short and direct
Effective Go recommends short, lowercase, single-word package names, with the package name matching the base directory name. That pushes against deep folder trees full of vague abstraction names. ([Go.dev](https://go.dev/doc/effective_go))
## The alternatives, honestly evaluated

### Option A: super flat
```text
vtwatch/
  go.mod
  main.go
  config.go
  store.go
  vt.go
  watch.go
  register.go
```

**Pros**
- fastest way to get moving
- easiest to understand initially
- no fake architecture

**Cons**
- `package main` grows messy fast
- harder to test reusable logic cleanly
- awkward once persistence, polling, registration, and notifications all exist

This is fine for a spike. It is not where I would leave the project.

---

### Option B: balanced Go layout
```text
vtwatch/
  go.mod
  go.sum
  README.md
  cmd/
    vtwatch/
      main.go
  internal/
    config/
    store/
    vt/
    watch/
    register/
    notify/
  migrations/
  testdata/
  scripts/
```

**Pros**
- still simple
- clear separation of concerns
- keeps non-public code private with `internal`
- easy to grow without a rewrite
- works well for an OSS repo that may include scripts/docs/testdata

**Cons**
- slightly more setup than a flat repo
- you need discipline to avoid turning every concept into its own package

This is the sweet spot.

---

### Option C: enterprise cosplay
```text
internal/app
internal/domain
internal/service
internal/repository
internal/model
internal/policy
internal/runner
internal/lifecycle
internal/httpapi
cmd/vtwatch
cmd/vtwatchd
web/
docs/
...
```

**Pros**
- looks impressive in a screenshot
- supports a lot of hypothetical growth

**Cons**
- too many boundaries too early
- unclear package responsibilities
- harder navigation
- more refactoring friction
- invites Java-style layering instead of idiomatic Go

This is the version I gave you earlier. It was too much.

## My conclusion

For **best practices in Go**, the best structure for this project is **Option B**, but trimmed aggressively.

Not the flattest possible layout, and not the giant “industry template” layout.

## What I would actually use

For your project, I would start here:

```text
vtwatch/
  go.mod
  go.sum
  README.md

  cmd/
    vtwatch/
      main.go

  internal/
    config/
      config.go
    store/
      sqlite.go
      watch_repo.go
    vt/
      client.go
      types.go
    watch/
      service.go
      scheduler.go
    register/
      service.go
    notify/
      notify.go

  migrations/
    001_init.sql

  testdata/
  scripts/
```

## Why this is the right compromise

- **`cmd/vtwatch`**: one binary, thin `main.go`
- **`internal/config`**: config loading only
- **`internal/store`**: SQLite and persistence
- **`internal/vt`**: all VT-specific request/response code in one place
- **`internal/watch`**: polling and watch management
- **`internal/register`**: registration flow
- **`internal/notify`**: notifications, probably tiny
- **`migrations/`**: optional, but reasonable once SQLite schema stops fitting in one string literal

That is enough structure to stay sane without turning the repo into a maze.

## What I would deliberately avoid

I would **not** start with:

- `pkg/`
- `internal/app`
- `internal/model`
- `internal/repository`
- `internal/service` as a dumping ground
- `internal/httpapi`
- separate `vtwatch` and `vtwatchd` binaries
- a `web/` directory
- multiple modules

Unless the code earns those abstractions, they are just noise.

## One more correction

You also do **not** need to decide the final package structure up front.

In Go, a good rule is:

- start with 4 to 6 packages max
- split a package only when it has become confusing, not when you can imagine it becoming confusing
- merge packages if the boundary feels artificial

Packages are not decorations. They are API boundaries. Treat them like a cost.

## My final recommendation

Use this as the baseline:

```text
vtwatch/
  go.mod
  README.md
  cmd/vtwatch/main.go
  internal/config/
  internal/store/
  internal/vt/
  internal/watch/
  internal/register/
  internal/notify/
  migrations/
  testdata/
  scripts/
```

That is the best balance of:
- idiomatic Go
- room to grow
- low complexity
- clean OSS repo shape

And yes, it is substantially simpler than what I proposed earlier.

The next thing worth doing is pressure-testing this package split against the actual features you need in v1, because there is a decent chance even this can be simplified further.

---

# you asked

and also when i say "hosted" i just mean someone can host it thensleves on a cloud server instead of their local machine so they don't have to worry about never shutting their machine off

---

# chatgpt response

That’s a materially different constraint. “Self-hostable by one user” is a legitimate middle ground, but it changes what the repo should optimize for.

The right way to think about that is “single-user, single-tenant, deployable anywhere,” not “local app” versus “SaaS.” That leads to a cleaner design than either extreme.

Yes, that is a real distinction, and it changes my recommendation.

You are not describing SaaS. You are describing a **single-user, self-hostable daemon** that can run either:

- on the user’s own machine, or
- on the user’s own VPS/cloud box

That is a valid middle ground.

## The right framing

Stop thinking in terms of:

- local app
- hosted service

The better framing is:

- **single-tenant**
- **deployable anywhere**

That is the architecture you want.

And that actually fits Go better too, because the official Go guidance is to keep module layout simple and only introduce extra structure like `internal` and `cmd` when it helps, not because a template said so. ([Go.dev](https://go.dev/doc/modules/layout?utm_source=chatgpt.com))The popular `golang-standards/project-layout` repo explicitly says it is **not** an official standard and can be overkill for simple projects and learning work. ([GitHub](https://github.com/golang-standards/project-layout?utm_source=chatgpt.com))
## What this means architecturally

The core should be:

- one user
- one process
- one SQLite database
- one config file
- one scheduler/worker loop
- optional HTTP UI
- deployable on macOS or Linux

So the project should not be “desktop app first” and it should not be “multi-user web app first.”

It should be a **daemon-first app**.

## The best structure for that

I would simplify further and build this:

```text
vtwatch/
  go.mod
  go.sum
  README.md

  cmd/
    vtwatch/
      main.go

  internal/
    config/
      config.go
    store/
      sqlite.go
      watches.go
      sessions.go
    vt/
      client.go
      fose.go
      register.go
      types.go
    watch/
      service.go
      scheduler.go
    web/
      server.go
      handlers.go
    notify/
      notify.go

  migrations/
    001_init.sql

  deploy/
    Dockerfile
    docker-compose.yml
    vtwatch.service

  scripts/
  testdata/
```

## Why this is better than the earlier version

### 1. One binary, not two
Earlier I suggested separate binaries like `vtwatch` and `vtwatchd`. For this project, I think that is probably unnecessary at first.

Use one binary with subcommands:

```bash
vtwatch run
vtwatch serve
vtwatch watch add --term 202608 --crn 12345
vtwatch session import
vtwatch watch list
```

Or even just:

```bash
vtwatch run
```

where `run` starts the scheduler and optional web server.

That keeps the repo simpler.

### 2. `internal/web` is now justified
Previously I pushed against adding a web layer too early. With **self-hosting on a VPS**, that changes.

A tiny web UI becomes useful because otherwise the UX is ugly:
- SSH in
- paste token manually
- edit watches from terminal
- inspect logs over SSH

That is fine for you, but not great if the point is “someone can host it themselves and not keep their laptop on.”

So now I do think a **small local/private web UI** is warranted.

Not a public multi-user frontend. Just a thin admin interface for the one user who owns the instance.

### 3. SQLite still makes sense
Do not jump to Postgres just because it can run in the cloud.

If this is still:
- one user
- one instance
- low write volume
- no concurrent multi-tenant workload

then SQLite is still the right default. A VPS with a mounted volume or bind-mounted Docker volume is fine.

If it later becomes multi-user, that is a different project.

## The key design consequence you were about to miss

This self-hosted model introduces one genuinely important issue:

### Session/token capture is now different

If the app runs locally, a bookmarklet posting to `localhost` is easy.

If the app runs on a remote VPS, `localhost` is no longer the browser’s machine. So you need a different import path.

That means one of these:

### Option A
A small authenticated web page on the self-hosted instance where the user pastes/imports the session/token.

### Option B
A CLI command over SSH where the user pastes the token manually.

### Option C
A bookmarklet that posts to the user’s configured remote host instead of localhost.

Of those, **Option A** is probably the cleanest if you want self-hosting to be usable.

This is one of the few reasons I now think `internal/web` belongs in v1.

## So what should the app actually be?

This is my updated recommendation:

### Core identity
A **single-user daemon** with:
- scheduler
- registration logic
- SQLite persistence
- optional web UI
- local notifications where available
- deployable locally or remotely

### Deployment targets
- **macOS**: launchd
- **Linux/VPS**: systemd or Docker Compose
- **maybe containerized** by default for easier self-hosting

### Not included
- multi-user auth
- hosted central control plane
- shared DB
- public API
- SaaS account system

## Best-practice conclusion for Go

Given what you actually mean by “hosted,” the best-practice Go conclusion is:

- **one module**
- **one binary**
- **a few focused `internal` packages**
- **no fake enterprise layering**
- **a thin `web` package only because remote self-hosting makes it practical**
- **deployment files in `deploy/`**
- **SQLite until proven otherwise**

That is a much cleaner and more honest structure than either:
- “pure local desktop app,” or
- “future-SaaS style worker/web-server split”

## Final recommendation

I would now aim for this exact shape:

```text
vtwatch/
  cmd/vtwatch/main.go
  internal/config/
  internal/store/
  internal/vt/
  internal/watch/
  internal/web/
  internal/notify/
  migrations/
  deploy/
```

And the operating model would be:

- **single-user**
- **single-instance**
- **runs anywhere**
- **local or self-hosted**
- **same codebase for both**

That is the architecture I’d back.

The next useful step is to decide whether `internal/web` should be in v1 or whether you want to force CLI-only for the first cut and add web only after the core loop works.

---

# you asked

and also cobra for the cli

---

# chatgpt response

I’m checking the current Go and Cobra guidance before I give you a yes or no. This is one of those areas where people repeat stale “best practices” that were never really standard.

Yes. **Cobra makes sense here.**

Not because “real Go CLIs use Cobra.” That’s fake reasoning. It makes sense because your tool naturally wants **subcommands**, **flags**, **help text**, and probably **shell completion**. Cobra is explicitly built around command trees, flags, automatic help, suggestions, aliases, completion, and man page generation. ([GitHub](https://github.com/spf13/cobra/blob/main/README.md))
## My recommendation

Use **Cobra for the CLI**, but keep it **thin**.

That means:

- Cobra handles parsing, help, subcommands, and flag wiring
- your actual logic lives outside the CLI layer
- no Viper at first
- no giant autogenerated CLI forest unless the app actually needs it

Cobra’s own generator creates a barebones app and expects you to edit `cmd/root.go`, then add commands from there. That’s fine as a starting point, but you do **not** need to let the generator dictate the whole architecture. ([GitHub](https://github.com/spf13/cobra-cli/blob/main/README.md))
## Why Cobra is a good fit for this project

Your commands are naturally shaped like this:

```bash
vtwatch run
vtwatch watch add --term 202608 --crn 12345 --drop 54321
vtwatch watch list
vtwatch watch pause <id>
vtwatch session import
vtwatch session validate
vtwatch serve
vtwatch doctor
```

That maps directly onto Cobra’s command model, where commands represent actions and flags modify them. Cobra’s docs literally describe the pattern as `APPNAME COMMAND ARG --FLAG`. ([GitHub](https://github.com/spf13/cobra/blob/main/README.md))
For this tool, that is a better fit than trying to force everything into flat flags like:

```bash
vtwatch --add-watch --term 202608 --crn 12345 --drop 54321
```

That’s worse. It gets ugly fast.

## The important caveat

Do **not** let Cobra infect the rest of the codebase.

A common mistake is building the app as if Cobra commands **are** the application. That turns every feature into command glue and leaves you with logic trapped in `RunE` functions.

Bad:

```go
var addWatchCmd = &cobra.Command{
    Use: "add",
    RunE: func(cmd *cobra.Command, args []string) error {
        // parse flags
        // open database
        // create services
        // call VT
        // business rules
        // logging
        // notifications
        return nil
    },
}
```

That’s CLI-driven spaghetti.

Better:

```go
var addWatchCmd = &cobra.Command{
    Use: "add",
    RunE: func(cmd *cobra.Command, args []string) error {
        return app.AddWatch(cmd.Context(), AddWatchInput{
            Term: term,
            CRN: crn,
            DropCRN: dropCRN,
        })
    },
}
```

Cobra should be a transport layer, not your domain model.

## The structure I’d use with Cobra

Given everything you said so far, I would use this:

```text
vtwatch/
  go.mod
  go.sum
  README.md

  cmd/
    vtwatch/
      main.go

  internal/
    cli/
      root.go
      run.go
      serve.go
      doctor.go
      watch.go
      watch_add.go
      watch_list.go
      session.go
      session_import.go
      session_validate.go

    config/
      config.go

    store/
      sqlite.go
      watches.go
      sessions.go

    vt/
      client.go
      fose.go
      register.go
      types.go

    watch/
      service.go
      scheduler.go

    web/
      server.go
      handlers.go

    notify/
      notify.go

  migrations/
    001_init.sql

  deploy/
    Dockerfile
    docker-compose.yml
    vtwatch.service
```

## Why this is better than the earlier structures

### `cmd/vtwatch/main.go`
Tiny. Ideally just:

```go
package main

import (
    "os"

    "github.com/you/vtwatch/internal/cli"
)

func main() {
    os.Exit(cli.Execute())
}
```

### `internal/cli/`
All Cobra wiring lives here.

That gives you:
- clean subcommand files
- isolated flag definitions
- easy testing of command behavior if needed
- no leakage of Cobra types into the rest of the app

### `internal/watch`, `internal/store`, `internal/vt`
These are still your real app.

The CLI calls into them. It should not replace them.

## Should you use `cobra-cli init`?

You can, but I would treat it as scaffolding, not architecture.

The Cobra generator is intended to bootstrap a project and add commands, and it works from within a Go module. ([GitHub](https://github.com/spf13/cobra-cli/blob/main/README.md))
My actual opinion:

- `cobra-cli init` is fine if you want a fast start
- but I would probably **reorganize the generated files almost immediately**
- specifically, I would not leave the whole app architecture implied by the default scaffold

So: **use Cobra**, maybe **use `cobra-cli` once**, but don’t become loyal to the generated tree.

## Should you use Viper too?

Probably **no**, at least initially.

Cobra mentions optional integration with Viper, and `cobra-cli init --viper` can wire that up. ([GitHub](https://github.com/spf13/cobra/blob/main/README.md))
I would not do that yet.

Why:
- Viper adds more moving parts than you need at the beginning
- config loading for this project is simple enough with stdlib plus a small config package
- Viper is often added because people want to feel “production-ready,” not because they need it

Start with:
- flags
- env vars where useful
- maybe one config file you parse yourself

Add Viper only if config complexity becomes real.

## When Cobra is the wrong choice

If this project were just:

```bash
vtwatch --config config.yaml
```

with no subcommands, then Cobra would be unnecessary. The standard library `flag` package would be enough.

But that’s not your app. Your app clearly wants command groups:
- `watch`
- `session`
- `run`
- `serve`
- `doctor`

So for this project, Cobra is justified.

## My conclusion

**Yes, use Cobra.**

But the best version is:

- **one binary**
- **Cobra only in `internal/cli`**
- **business logic outside the CLI**
- **no Viper yet**
- **no giant generated command jungle unless the commands earn it**

If I were locking this in, I’d use:

```text
cmd/vtwatch/main.go
internal/cli/
internal/store/
internal/vt/
internal/watch/
internal/web/
```

with Cobra in `internal/cli` and nowhere else. That’s the cleanest answer.

If you want, I’ll sketch the exact Cobra command tree and what files each command should live in.

---

# you asked

alright so lets summarize what we've settled on/concluded here for me to start the project

---

# chatgpt response

Yes. Here’s the clean summary of where we landed.

## What this project is

**A single-user, single-tenant Go daemon** that can run either:

- locally on the user’s machine, or
- on a server/VPS the user hosts themselves

It is **not** a public SaaS.
It is **not** multi-user.
It should be **deployable anywhere** with the same codebase.

That is the right framing.

---

## Core product direction

### We are optimizing for:
- one user per instance
- one process
- one SQLite database
- one config file
- one scheduler loop
- optional small web UI
- local or self-hosted deployment

### We are not building:
- a hosted multi-user backend
- accounts/auth for your own service
- a shared Postgres instance
- a worker/web-server/cloud split
- enterprise architecture for imaginary scale

---

## Architecture conclusion

### Best architectural model
**Daemon-first app**

Meaning:
- the core is a long-running background process
- it owns polling, persistence, scheduling, and registration
- UI is secondary
- CLI is for control/setup/debugging
- web UI is optional but useful, especially for self-hosting

### Why
Because “local only” is too narrow, but “SaaS backend” is the wrong shape.
The right middle ground is:

**single-user daemon that can run anywhere**

---

## Repo structure conclusion

You were right that the earlier structure was too complicated.

### Best structure for v1
A **simple one-module Go repo** with a **small number of real packages**.

```text
vtwatch/
  go.mod
  go.sum
  README.md

  cmd/
    vtwatch/
      main.go

  internal/
    cli/
    config/
    store/
    vt/
    watch/
    web/
    notify/

  migrations/
  deploy/
  scripts/
  testdata/
```

### Package responsibilities

- `cmd/vtwatch`
  - tiny entrypoint
- `internal/cli`
  - Cobra command wiring only
- `internal/config`
  - config loading and path handling
- `internal/store`
  - SQLite and persistence
- `internal/vt`
  - all VT-specific request/response logic
- `internal/watch`
  - scheduler, watch management, polling logic
- `internal/web`
  - small single-user admin UI/API
- `internal/notify`
  - notifications/log output

### Important constraint
Do **not** explode this into ten more packages unless the code actually forces it.

---

## CLI conclusion

### Yes, use Cobra
But keep it thin.

Cobra should handle:
- subcommands
- flags
- help text
- completions

It should **not** contain your app logic.

### CLI commands we roughly expect
```bash
vtwatch run
vtwatch serve
vtwatch watch add
vtwatch watch list
vtwatch watch pause
vtwatch session import
vtwatch session validate
vtwatch doctor
```

### Also concluded
- use Cobra
- **do not** use Viper yet
- keep config handling simple

---

## Storage conclusion

### Use SQLite
Not Postgres.

Why:
- single-user
- single-instance
- low complexity
- portable
- works locally and on a VPS
- less operational garbage

If the project later becomes truly multi-user, that is a different architecture.

---

## Deployment conclusion

### Supported modes
- **macOS local** via `launchd`
- **Linux/VPS self-hosted** via `systemd` or Docker Compose

### What “hosted” means here
Not your hosted service.
It means **the user can host their own instance** so their laptop does not need to stay on.

That distinction matters.

### So `deploy/` should contain things like:
- `Dockerfile`
- `docker-compose.yml`
- `vtwatch.service`

---

## Web UI conclusion

Earlier, web UI was optional.  
After clarifying self-hosting, we concluded:

### `internal/web` is justified
Because a remote self-hosted instance is annoying if everything requires SSH + CLI.

So a **small private admin UI** makes sense for:
- session import/validation
- managing watches
- viewing status/logs
- basic control

### What it is not
- not a multi-user frontend
- not a public app
- not a heavy SPA-first product

Thin and functional.

---

## Build philosophy conclusion

### What to optimize for
- correctness
- clarity
- simple boundaries
- ability to run locally or remotely
- easy iteration

### What to avoid
- overengineering
- fake “best practice” folder hierarchies
- separate binaries unless necessary
- abstractions for imagined future needs
- treating package structure like branding

The rule is:

**start simple, split only when the code earns it**

---

## Final project identity

This is the sentence I’d use to keep yourself honest:

> **vtwatch is a single-user Go daemon with a Cobra CLI, SQLite persistence, VT integration logic, and an optional small web UI, designed to run either locally or on a user-owned server.**

That is the settled shape.

---

## Recommended v1 order of work

### 1. Bootstrap repo
- Go module
- Cobra root command
- basic package layout
- SQLite setup

### 2. Core watch loop
- watch model
- persistence
- scheduler
- polling path

### 3. VT session handling
- import
- validate
- store locally

### 4. Registration flow
- preflight
- cart add
- register
- retries / result handling

### 5. Web UI
- basic admin/status pages
- manage watches
- session status

### 6. Deployment
- `launchd`
- `systemd` or Docker Compose

---

## What we have explicitly rejected

We have rejected:

- public SaaS architecture
- multi-user service design
- shared hosted backend
- Postgres for v1
- heavy repo structure
- Viper for now
- CLI logic as business logic
- “extension first” as the main architecture
- separate worker/server binaries at the start

---

## Bottom line

You should start the project as:

- **one Go module**
- **one binary**
- **Cobra CLI**
- **SQLite**
- **small `internal` package set**
- **daemon-first**
- **optional thin web UI**
- **local or self-hosted single-user deployment**

That is the cleanest and most defensible starting point.

The next concrete step is to lock the exact package/file skeleton and the initial Cobra command tree.

---

# you asked

Here is a ton of useful information, documentation, etc I put together from my extensive testing and documenting - lets use it to guide the building of the project and whatnot

---

# chatgpt response

Yes. This is now the **single most useful artifact in the project**.

It changes the conversation from “architecture based on assumptions” to “architecture based on verified behavior.” In particular, it resolves several earlier unknowns: VT is using token auth, `authtoken` alone works for both `sisproxy` and `shockabsorber`, the real cart is `default`, `studentdata` is the key endpoint, and the end-to-end registration flow is now concrete instead of inferred. 

## What this means for the project

We should stop designing around old uncertainty and design around **this document as source of truth**.

That means the implementation should be built around these facts:

- `studentdata` is the anchor endpoint because it returns identity, `pers` credentials, registration tickets, cart, current registrations, waitlist, and history in one call. 
- `sisproxy` is GET with query params and JSONP-wrapped responses, so a JSONP stripper is a first-class utility, not an afterthought. 
- `shockabsorber register/status` are the dangerous operations and need careful request construction, especially `time_ticket`, `url_replay`, form body credentials, and the `default` cart. 
- `studentdata` confirmation after a registration attempt is mandatory because a status timeout does **not** reliably mean failure. 
- self-hosted or local is still the right product shape, and the doc itself explicitly pushes against hosted credential-sharing models. 

## What it changes from earlier discussion

A few earlier cautions were valid at the time, but this doc supersedes some of them.

### Resolved now
- The “does shockabsorber work with `authtoken` only?” question is no longer open. Your doc says yes, empirically. 
- The “is cookie auth required?” question is no longer open for VT’s banner/token config. Your doc says no. 
- The “is `PRIMARY` the cart?” question is no longer open. It is `default`, and using the wrong cart can break registration flow. 

### Still important
- Session expiry still matters. Your doc says `authtoken` is typically stable for about 24 hours or until logout, and `_pers_id_proof` may rotate, so refresh/validation logic still belongs in v1. 
- The safe retry protocol matters a lot. Blind retry on ambiguous registration results is a real footgun. 
- Rate limiting is not optional. Your doc is explicit about polite polling and global limits. 

## How this should drive the codebase

The package structure we settled on still works, but now each package has a much clearer responsibility.

### `internal/vt`
This becomes the most important package.

It should own:

- endpoint construction
- JSONP stripping
- VT timestamp normalization/parsing
- `time_ticket` building
- request/response types
- cart string parsing
- student record parsing
- safe registration flow helpers

This package should be the only place that “knows VT.”

### `internal/store`
The doc makes it obvious what you actually need to persist:

- credentials/session material
- watch definitions
- watch state
- attempt logs
- maybe recent studentdata-derived snapshots for debugging

SQLite is still correct for this. The doc does not justify anything heavier. 

### `internal/watch`
This should only do:

- watch lifecycle
- scheduling
- polling `fose`
- detecting transitions like `F/W -> A`
- enqueueing a registration attempt

It should **not** know how `shockabsorber` works internally.

### `internal/web`
This is more justified now because the doc clearly implies a credential import flow and session health flow. A small admin UI for:
- pasting/importing captured creds
- seeing current token/session status
- managing watches
- viewing recent attempts/errors

is useful for self-hosted usage.

### `internal/cli`
Cobra still makes sense. It becomes the control plane:
- `session import`
- `session validate`
- `watch add`
- `watch list`
- `run`
- `doctor`

## Best-practice implementation order

Your own doc already gives the correct order. We should follow it almost exactly, because it is rational and risk-aware. 

### Phase 1: pure utilities and safe reads
Implement first:

1. JSONP stripper
2. VT timestamp parser
3. `fose search`
4. `studentdata`
5. `cart_read`

That gives you:
- network layer
- parsing layer
- auth validation
- safe inspection tools

### Phase 2: reversible writes
Then:

6. `cart_add`
7. `cart_remove`
8. `preflight`

This is the right middle step because it exercises auth and mutation without immediately touching actual registration.

### Phase 3: registration primitives
Then:

9. `time_ticket` builder
10. `shockabsorber register`
11. `shockabsorber status`
12. registration confirmation through `studentdata`

That is where the dangerous logic begins.

### Phase 4: orchestration
Only after the primitives work:

- watch scheduler
- polling loop
- rate limiting
- retry discipline
- background daemon behavior
- notifications

No, you should not start by writing the daemon loop. That would be backwards.

## The concrete domain model this suggests

This doc gives you a much cleaner data model than we had before.

### Session
Store:
- `authtoken`
- `pers_id`
- `pers_id_proof`
- capture timestamp
- last validation timestamp
- last known expiry/session status

### Watch
Store:
- term
- CRN
- desired action (`add` or `swap`)
- optional `drop_crn`
- polling interval
- current watch state
- last seen `stat`

### Attempt
Store:
- watch id
- phase (`preflight`, `cart_add`, `register`, `status`, `confirm`)
- result
- response body summary
- timestamps

That maps directly to the actual API semantics in the doc. 

## What needs strong guardrails in code

Your doc is very clear about the dangerous edges. These should become hard rules in the implementation.

### Guardrail 1
Before any `register` call, fetch fresh `studentdata` and verify the user is **not already registered** for the target CRN unless the action is explicitly a drop. Your doc says calling register on an already-registered CRN can drop them. That is not a soft warning. That needs a hard code-level guard. 

### Guardrail 2
Never retry `shockabsorber register` blindly. Use the safe retry protocol:
- ambiguous result
- fetch `studentdata`
- decide from actual state
- only retry if clearly safe 

### Guardrail 3
Always use `default` cart everywhere. Do not make cart name configurable in v1. Your doc has already done the debugging for that. Exposing it as an option would just create failure modes. 

### Guardrail 4
Rate limit globally, not just per watch. Again, your doc already settled this. 

## What I would change in the repo plan because of this doc

I would add a few very specific things.

### Add a parser-focused test suite
Because this API has:
- JSONP wrappers
- pipe-delimited records
- weird timestamps
- nontrivial ticket construction

you want a lot of deterministic unit tests in `testdata/`.

### Add fixtures immediately
Your doc explicitly recommends capturing real redacted responses as fixtures. That is exactly right. Make `testdata/` a first-class citizen from day one. 

### Add a `doctor` command in v1
Because there are many ways for session/auth/state to go stale, `vtwatch doctor` should probably exist early and check:
- can `studentdata` be fetched
- do credentials parse
- do reg tickets exist for term
- is target action allowed
- is session near expiry

That is much more useful than fancy UI polish.

## What I would not do

I would not treat this doc as “nice reference material.”  
I would treat it as the contract the implementation is built against.

Concretely:
- derive request builders from it
- derive parsing utilities from it
- derive state machine rules from it
- derive tests from it
- derive CLI commands from it

And I would put it in `docs/` or keep it at repo root and explicitly link to it from the README as the implementation reference. The file is too valuable to let it sit as random notes. 

## Bottom line

This document is good enough that it should now drive the build.

The immediate conclusion is:

- the project should stay **single-user/self-hosted**
- the implementation should be **API-first and parser-first**
- `studentdata` should be the center of the auth and confirmation logic
- the registration state machine should follow the exact safe flow in the doc
- v1 should be built in the order your doc already recommends, because that order is correct 

The next useful move is to turn this reference into an **implementation checklist mapped onto the repo structure**, so each package has a concrete v1 scope.

---

# you asked

i should probably manually attempt a simultaneous register/drop with curl before proceeding because i havent really tested that yet

---

# chatgpt response

Yes. You probably **should** do that before going further.

Not because “it might be nice to know,” but because **swap is its own dangerous path**, not just “add with one extra parameter.” Your reference says atomic swap works by doing `cart_add` with `crn_drop=<old>` and then calling the same `register` action, and it explicitly warns that Banner may reject the transaction in edge cases that preflight does not always catch. If you have not manually verified that end-to-end with real requests, then your swap support is still theoretical. 

## Why this matters

You already know:

- add works
- register works
- status polling works
- studentdata confirmation works

But **swap correctness** depends on a few extra things all being true at once:

- `cart_add` actually stores the `crn_drop` in the way Banner expects
- `register` processes that cart entry atomically as add+drop
- the resulting state in `studentdata` reflects exactly one add and one drop
- failure behavior is sane when the add fails, especially whether the drop still happens or not
- preflight and real registration behavior may diverge for dependency/conflict cases 

If your tool will support swaps, that path is load-bearing. You should not code it from documentation alone.

## My recommendation

Do **manual curl testing of swap before implementing swap automation**.

More specifically:

1. Test a **known-safe swap case**
   - a class you are willing to briefly drop
   - a target class with an open seat
   - low-stakes course, not something critical

2. Verify the full sequence manually:
   - `studentdata` before
   - `cart_add` with `crn_drop`
   - inspect cart contents
   - `register`
   - poll `status`
   - `studentdata` after

3. Confirm these exact outcomes:
   - old CRN removed from `reg.<term>`
   - new CRN added to `reg.<term>`
   - cart state makes sense afterward
   - no weird partial state

4. Then test at least one **failure case**
   - add target that should fail or be unavailable
   - see whether the original course is preserved
   - confirm via `studentdata`, not assumptions

That last step matters because your own reference is very clear that ambiguous registration outcomes should be reconciled via `studentdata`, not by guessing from the immediate shockabsorber response. 

## One thing I would push back on

Do **not** jump straight to “simultaneous register/drop with curl” as one big opaque experiment if you have not already manually validated the components around it.

The disciplined sequence is:

- fetch fresh `studentdata`
- verify target CRN is not already registered
- verify drop CRN is actually registered
- `cart_add` with `crn_drop`
- inspect resulting cart entry carefully
- only then call `register`

If you skip those checks, you are not testing swap semantics cleanly. You are just creating noise and risk.

## What I would want proven before coding swap support

You should be able to answer these with evidence:

- Does `cart_add` with `crn_drop` reliably encode the swap in the returned cart entry?
- Does `register` preserve atomicity in practice?
- What does failure look like when the add side fails?
- Can preflight miss a blocker that real register catches?
- After a timeout or ambiguous result, what does `studentdata` show?

Until you know those answers, I would keep swap support out of the app or mark it experimental.

## Bottom line

Yes, test it manually first.

In fact, I would go further: **do not implement automated swap support until you have personally observed one successful swap and one failed swap end-to-end with curl and confirmed both with `studentdata`.** That is the right level of paranoia for this API.

---

# you asked

ok well give me the curl commands to test

---

# chatgpt response

I’m going to give you a sequence you can paste and run, not vague pseudocode. The main thing is to avoid leaking credentials into shell history and to separate “safe inspection” from the actual swap trigger.

Yes. Use this exact flow.

It matches the behavior you documented: `cart_add` with `crn_drop` is how VT encodes a swap, the real cart name is `default`, `register` still uses `action=register`, `url_replay` stays relative, and you should confirm the outcome with `studentdata` before and after instead of trusting the immediate shockabsorber response alone. 

You need one thing prepared before this works:

- a full **7-field** `TIME_TICKET`
- your `AUTHTOKEN`
- `PERS_ID`
- `PERS_ID_PROOF`
- the `TERM`
- `ADD_CRN`
- `DROP_CRN`
- the exact `HOURS` for the section you are adding
- usually `GMOD=N` and `REG_INFO=E` for a normal letter-graded add, but use the exact values VT expects for that section if you know they differ. 

If you do not have `jq`:

```zsh
brew install jq
```

## 1. Load variables securely in zsh

This avoids pasting secrets directly into shell history.

```zsh
BASE='https://classes.vt.edu'

read "TERM?TERM (example 202606): "
read "ADD_CRN?CRN TO ADD: "
read "DROP_CRN?CRN TO DROP: "
read "HOURS?HOURS FOR ADD CRN (example 3): "
read "GMOD?GRADE MODE (usually N): "
read "REG_INFO?REG INFO (usually E): "

read -s "AUTHTOKEN?AUTHTOKEN: "; echo
read -s "PERS_ID?_pers_id: "; echo
read -s "PERS_ID_PROOF?_pers_id_proof: "; echo
read -s "TIME_TICKET?FULL 7-FIELD time_ticket: "; echo
```

## 2. Helper for stripping JSONP

`sisproxy` wraps responses in JSONP, so strip that before piping to `jq`. 

```zsh
strip_jsonp() {
  perl -0777 -pe 's/^[A-Za-z_]+$([\s\S]*)$[;\s]*$/$1/s'
}
```

## 3. Optional sanity check that the add target is actually open

`fose` is public and returns plain JSON. The `stat` field tells you whether the section is open. f```zsh
curl -sS "$BASE/api/?page=fose&route=search" \
  -H 'Content-Type: application/json' \
  --data "{\"other\":{\"srcdb\":\"$TERM\"},\"criteria\":[{\"field\":\"crn\",\"value\":\"$ADD_CRN\"}]}" \
  | jq .
```

You want `results[0].stat == "A"` before bothering with the swap test.

## 4. Snapshot `studentdata` before doing anything

This is your source of truth. It also lets you confirm that the add target is **not** already registered and the drop target **is** registered. f```zsh
curl -sS --get "$BASE/api/" \
  --data-urlencode "page=sisproxy" \
  --data-urlencode "action=studentdata" \
  --data-urlencode "authtoken=$AUTHTOKEN" \
  | strip_jsonp | tee before.studentdata.json \
  | jq --arg term "$TERM" --arg add "$ADD_CRN" --arg drop "$DROP_CRN" '
      [(.reg[$term] // [])[] | split("|")[0]] as $crns
      | {
          registered_crns: $crns,
          add_already_registered: ($crns | index($add) != null),
          drop_currently_registered: ($crns | index($drop) != null)
        }'
```

If `add_already_registered` is `true`, stop.  
If `drop_currently_registered` is `false`, stop.

## 5. Optional preflight

Preflight is worth checking, but it is not the final word. Your own doc notes that Banner can still reject things preflight misses. f```zsh
curl -sS --get "$BASE/api/" \
  --data-urlencode "page=sisproxy" \
  --data-urlencode "action=preflight" \
  --data-urlencode "term_code=$TERM" \
  --data-urlencode "cart_name=default" \
  --data-urlencode "crn_list=$ADD_CRN" \
  --data-urlencode "authtoken=$AUTHTOKEN" \
  | strip_jsonp | tee preflight.json | jq .
```

## 6. Add the target CRN to cart with `crn_drop` set

This is the actual swap encoding step. `cart_name=default` matters. f```zsh
curl -sS --get "$BASE/api/" \
  --data-urlencode "page=sisproxy" \
  --data-urlencode "action=cart_add" \
  --data-urlencode "term_code=$TERM" \
  --data-urlencode "cart_name=default" \
  --data-urlencode "crn=$ADD_CRN" \
  --data-urlencode "crn_drop=$DROP_CRN" \
  --data-urlencode "hours=$HOURS" \
  --data-urlencode "gmod=$GMOD" \
  --data-urlencode "reg_info=$REG_INFO" \
  --data-urlencode "authtoken=$AUTHTOKEN" \
  | strip_jsonp | tee cart_after_add.json | jq .
```

## 7. Read cart and verify the entry is really there

You want to see the new CRN in the default cart before you fire `register`. f```zsh
curl -sS --get "$BASE/api/" \
  --data-urlencode "page=sisproxy" \
  --data-urlencode "action=cart_read" \
  --data-urlencode "authtoken=$AUTHTOKEN" \
  | strip_jsonp | tee cart_read.json | jq .
```

## 8. Build encoded query values for shockabsorber

`time_ticket` and `url_replay` must be URL-encoded when they go in the outer shockabsorber URL. `url_replay` must be relative and point to `action=register`, not `cart_add`. f```zsh
URL_REPLAY="api/?page=sisproxy&action=register&term_code=$TERM&crn=$ADD_CRN&wait_crn=&swap_crn="
TT_ENC=$(jq -rn --arg v "$TIME_TICKET" '$v|@uri')
URL_REPLAY_ENC=$(jq -rn --arg v "$URL_REPLAY" '$v|@uri')
```

## 9. Fire the actual swap via shockabsorber

This is the dangerous call. The form body must include `authtoken`, `_pers_id`, `_pers_id_proof`, and `_pers_real_id`, and for a normal non-impersonation session `_pers_real_id` should equal `_pers_id`. f```zsh
curl -sS -X POST \
  "$BASE/api/?page=shockabsorber&time_ticket=$TT_ENC&action=register&cart_name=default&url_replay=$URL_REPLAY_ENC" \
  --data-urlencode "authtoken=$AUTHTOKEN" \
  --data-urlencode "_pers_id=$PERS_ID" \
  --data-urlencode "_pers_id_proof=$PERS_ID_PROOF" \
  --data-urlencode "_pers_real_id=$PERS_ID" \
  | tee register.json | jq .
```

If it works normally, you will usually get something like:

```json
{"body":"WAIT","code":200,"data":{"id":"..."}}
```

## 10. Poll status until it stops saying `WAIT`

Status uses the same `time_ticket`, `cart_name`, and form-body credentials. Polling every 1 to 2 seconds is reasonable. f```zsh
for i in {1..30}; do
  echo "status poll $i"

  RESP=$(
    curl -sS -X POST \
      "$BASE/api/?page=shockabsorber&time_ticket=$TT_ENC&action=status&cart_name=default" \
      --data-urlencode "authtoken=$AUTHTOKEN" \
      --data-urlencode "_pers_id=$PERS_ID" \
      --data-urlencode "_pers_id_proof=$PERS_ID_PROOF" \
      --data-urlencode "_pers_real_id=$PERS_ID"
  )

  echo "$RESP" | tee "status_$i.json" | jq .

  BODY=$(echo "$RESP" | jq -r '.body // empty')
  if [[ "$BODY" != "WAIT" ]]; then
    break
  fi

  sleep 2
done
```

## 11. Confirm the real final state with `studentdata`

This is the step that matters most. Your doc is explicit that ambiguous shockabsorber results should be resolved by checking `studentdata`, not by guessing. f```zsh
curl -sS --get "$BASE/api/" \
  --data-urlencode "page=sisproxy" \
  --data-urlencode "action=studentdata" \
  --data-urlencode "authtoken=$AUTHTOKEN" \
  | strip_jsonp | tee after.studentdata.json \
  | jq --arg term "$TERM" --arg add "$ADD_CRN" --arg drop "$DROP_CRN" '
      [(.reg[$term] // [])[] | split("|")[0]] as $crns
      | {
          registered_crns: $crns,
          add_now_registered: ($crns | index($add) != null),
          drop_still_registered: ($crns | index($drop) != null)
        }'
```

### What success looks like
- `add_now_registered: true`
- `drop_still_registered: false`

### What failure looks like
- `add_now_registered: false`
- `drop_still_registered: true`

### What would be alarming
- both false
- both true
- cart/registration state that does not match the status response

That is exactly why this final check exists.

## 12. If the status call times out or looks weird

Do **not** immediately rerun `register`. Your own notes already settled the safe protocol: check `studentdata` first, then decide whether the swap actually happened. Blind retry is how you create accidental destructive behavior. f## Practical advice

Do this with:
- a low-stakes section
- an open seat
- a course you are willing to briefly add and remove
- a drop target you can safely risk for testing

Do **not** use a course you cannot afford to lose.

If you want, next I can turn this into a reusable `swap_test.zsh` script with prompts, sanity checks, and cleaner output.

---

# you asked

well i need the commands or whatever to get the tokens/security things i need as well

---

# chatgpt response

Yes. The cleanest way is **not** `curl` first.

You need to get these from a real logged-in `classes.vt.edu` browser session:

- `authtoken`
- `pers_id`
- `pers_id_proof`

Those are the three credentials your reference says you need, and it also identifies the stable client-side JS paths for them. It further says `reg_tickets` can be refreshed later via `studentdata`, so you do **not** need to capture a prebuilt `time_ticket` from the page if you do not want to. 

## Best way: DevTools console on `classes.vt.edu`

1. Log in to `classes.vt.edu`
2. Open DevTools
3. Go to the **Console**
4. Paste this:

```javascript
(async () => {
  const r = await fetch('/api/?page=sisproxy&action=studentdata&authtoken=' + sam.auth.token);
  const t = await r.text();
  const m = t.match(/^setRecord$([\s\S]*)$[;\s]*$/);
  if (!m) throw new Error('failed to parse studentdata response');
  const d = JSON.parse(m[1]);

  const creds = {
    captured_at: new Date().toISOString(),
    authtoken: sam.auth.token,
    pers_id: d.pers.id,
    pers_id_proof: d.pers.idProof,
    name: d.pers.fn,
    reg_tickets: d.reg_tickets
  };

  console.log(creds);
  await navigator.clipboard.writeText(JSON.stringify(creds, null, 2));
  console.log('Copied credentials JSON to clipboard');
})();
```

That is basically the readable bookmarklet flow from your reference, just easier to debug in the console first. Your document explicitly identifies:

- `sam.auth.token`
- `sam.record.getProperty('pers').id`
- `sam.record.getProperty('pers').idProof`

as the stable JS paths, and also provides this bookmarklet-style credential capture flow. f## Even simpler console snippet

If you only want the raw values right now:

```javascript
console.log({
  authtoken: sam.auth.token,
  pers_id: sam.record.getProperty('pers').id,
  pers_id_proof: sam.record.getProperty('pers').idProof
});
```

That gets the essentials directly from the page runtime, which your reference says is where VT exposes them client-side. f## If you want an actual bookmarklet

Your reference already has the right pattern. Minified into one line, use:

```javascript
javascript:(async()=>{try{const r=await fetch('/api/?page=sisproxy&action=studentdata&authtoken='+sam.auth.token);const t=await r.text();const m=t.match(/^setRecord$([\s\S]*)$[;\s]*$/);if(!m)throw new Error('failed to parse studentdata response');const d=JSON.parse(m[1]);const creds={captured_at:new Date().toISOString(),authtoken:sam.auth.token,pers_id:d.pers.id,pers_id_proof:d.pers.idProof,name:d.pers.fn,reg_tickets:d.reg_tickets};await navigator.clipboard.writeText(JSON.stringify(creds,null,2));alert('VT credentials copied to clipboard. Paste into the tool.')}catch(e){alert('Capture failed: '+e.message)}})();
```

That is directly aligned with the capture approach documented in your reference. fil Then use `curl` to fetch fresh `studentdata`

Once you have the token and IDs, use `studentdata` as your first authenticated sanity check. Your reference is very explicit that this is the single most valuable endpoint because it returns identity, `pers`, cart, registrations, and `reg_tickets`. fil`zsh
BASE='https://classes.vt.edu'
AUTHTOKEN='paste_authtoken_here'

curl -sS --get "$BASE/api/" \
  --data-urlencode "page=sisproxy" \
  --data-urlencode "action=studentdata" \
  --data-urlencode "authtoken=$AUTHTOKEN"
```

Since `studentdata` is JSONP-wrapped, strip it before parsing:

```zsh
strip_jsonp() {
  perl -0777 -pe 's/^[A-Za-z_]+$([\s\S]*)$[;\s]*$/$1/s'
}

curl -sS --get "$BASE/api/" \
  --data-urlencode "page=sisproxy" \
  --data-urlencode "action=studentdata" \
  --data-urlencode "authtoken=$AUTHTOKEN" \
  | strip_jsonp | jq .
```

## If you want to pull just the pieces you need

```zsh
curl -sS --get "$BASE/api/" \
  --data-urlencode "page=sisproxy" \
  --data-urlencode "action=studentdata" \
  --data-urlencode "authtoken=$AUTHTOKEN" \
  | strip_jsonp \
  | jq '{pers_id: .pers.id, pers_id_proof: .pers.idProof, reg_tickets: .reg_tickets}'
```

That gives you everything needed to later build the 7-field `time_ticket` for the target term, which your reference says is assembled from `reg_tickets[i].ticket`, the parsed `start_date` in milliseconds, and `pers_id`. filecmportant correction

You do **not** need some separate magic command to “get the security things” from the network layer first.

The correct flow is:

1. get `authtoken`, `pers_id`, `pers_id_proof` from the logged-in page runtime
2. call `studentdata`
3. use `studentdata` to obtain `reg_tickets`
4. build `time_ticket`

That is the cleanest path, and it is exactly what your own reference supports. filecractical recommendation

Start with the simple console snippet, not the bookmarklet. Confirm the values exist. Then run the `studentdata` curl and confirm it parses correctly. Only after that should you move on to the swap test.

If you want, next I’ll give you:
- a **console snippet that prints the fully assembled 7-field `time_ticket`** for a chosen term, or
- a **single zsh script** that prompts for the values and runs the whole swap test end to end.

---

# chatgpt response

Yep. You got **almost all the way there**.

The good news is that your run already proved a lot:

- `fose` search worked and showed `60058` is open. 
- `studentdata` check showed you were registered for `60900` and not already in `60058`. 
- `preflight` returned the usual soft `||` state with no global errors. 
- `cart_add` with `crn_drop=60900` staged the swap correctly. You can see `60058` in cart and `60900` showing as a drop entry with `D`. That is exactly the behavior your reference describes for swap staging. 

So no, the whole approach is not wrong. The failure is narrowed to the **final shockabsorber request construction**. 

## Two actual problems in your shell session

### 1. `TERM` was a bad variable name
That one is on me. `TERM` is the shell environment variable for terminal type. Overwriting it with `202606` is why zsh threw:

`can't find terminal definition for 202606` 

Use `VT_TERM` or `TERM_CODE`, not `TERM`.

### 2. You pasted commands with literal `\n`
This is the bigger one.

Your transcript shows commands like:

- `strip_jsonp() {\n ... \n}`
- `URL_REPLAY="..."\n`
- `TT_ENC=$(...)\n`

Those are not real newlines. They are literal backslash-n text. That means some of your variables very likely ended up with an extra trailing `n`, which would absolutely corrupt `url_replay` and `time_ticket`. Your own reference says malformed `time_ticket` or bad `url_replay` are common causes of shockabsorber failure. 

So the immediate fix is: **rerun the final part cleanly, with real newlines, and with `VT_TERM` instead of `TERM`.**

---

## Use this corrected sequence

### 1. Set clean variables

```zsh
BASE='https://classes.vt.edu'
VT_TERM='202606'
ADD_CRN='60058'
DROP_CRN='60900'
HOURS='3'
GMOD='N'
REG_INFO='E'
```

### 2. Prompt for secrets again

```zsh
read -s "AUTHTOKEN?AUTHTOKEN: "; echo
read -s "PERS_ID?_pers_id: "; echo
read -s "PERS_ID_PROOF?_pers_id_proof: "; echo
read -s "TIME_TICKET?FULL 7-FIELD time_ticket: "; echo
```

### 3. Rebuild the helper cleanly

```zsh
strip_jsonp() {
  perl -0777 -pe 's/^[A-Za-z_]+$([\s\S]*)$[;\s]*$/$1/s'
}
```

### 4. Sanity check the `time_ticket` really has 7 fields

```zsh
printf '%s\n' "$TIME_TICKET" | awk -F'|' '{print "time_ticket fields:", NF}'
```

You want:

```text
time_ticket fields: 7
```

If it is not 7, stop.

### 5. Build `url_replay` cleanly

Your reference is explicit that `url_replay` must be relative, point to `action=register`, and not include `authtoken`. f```zsh
URL_REPLAY="api/?page=sisproxy&action=register&term_code=$VT_TERM&crn=$ADD_CRN&wait_crn=&swap_crn="
```

### 6. Encode both pieces

```zsh
TT_ENC=$(jq -rn --arg v "$TIME_TICKET" '$v|@uri')
URL_REPLAY_ENC=$(jq -rn --arg v "$URL_REPLAY" '$v|@uri')
```

### 7. Print them in escaped form to catch trailing garbage

```zsh
printf 'URL_REPLAY=%q\n' "$URL_REPLAY"
printf 'TT_ENC=%q\n' "$TT_ENC"
printf 'URL_REPLAY_ENC=%q\n' "$URL_REPLAY_ENC"
```

If you see a trailing `n` anywhere, the variable is bad. Reassign it.

### 8. Fire the register call again

```zsh
curl -sS -X POST \
  "$BASE/api/?page=shockabsorber&time_ticket=$TT_ENC&action=register&cart_name=default&url_replay=$URL_REPLAY_ENC" \
  --data-urlencode "authtoken=$AUTHTOKEN" \
  --data-urlencode "_pers_id=$PERS_ID" \
  --data-urlencode "_pers_id_proof=$PERS_ID_PROOF" \
  --data-urlencode "_pers_real_id=$PERS_ID" \
  | tee register.json | jq .
```

If that comes back with `WAIT`, you are back on track. That matches the documented normal behavior. f### 9. Poll status

```zsh
for i in {1..30}; do
  echo "status poll $i"

  RESP=$(
    curl -sS -X POST \
      "$BASE/api/?page=shockabsorber&time_ticket=$TT_ENC&action=status&cart_name=default" \
      --data-urlencode "authtoken=$AUTHTOKEN" \
      --data-urlencode "_pers_id=$PERS_ID" \
      --data-urlencode "_pers_id_proof=$PERS_ID_PROOF" \
      --data-urlencode "_pers_real_id=$PERS_ID"
  )

  echo "$RESP" | tee "status_$i.json" | jq .

  BODY=$(echo "$RESP" | jq -r '.body // empty')
  if [[ "$BODY" != "WAIT" ]]; then
    break
  fi

  sleep 2
done
```

### 10. Confirm with `studentdata`

Your reference is explicit that this is the real source of truth after registration attempts. f```zsh
curl -sS --get "$BASE/api/" \
  --data-urlencode "page=sisproxy" \
  --data-urlencode "action=studentdata" \
  --data-urlencode "authtoken=$AUTHTOKEN" \
  | strip_jsonp | tee after.studentdata.json \
  | jq --arg term "$VT_TERM" --arg add "$ADD_CRN" --arg drop "$DROP_CRN" '
      [(.reg[$term] // [])[] | split("|")[0]] as $crns
      | {
          registered_crns: $crns,
          add_now_registered: ($crns | index($add) != null),
          drop_still_registered: ($crns | index($drop) != null)
        }'
```

---

## If it still returns 400

Then the next most likely issue is your `TIME_TICKET`, not the swap logic.

At that point, do this before anything else:

```zsh
curl -sS --get "$BASE/api/" \
  --data-urlencode "page=sisproxy" \
  --data-urlencode "action=studentdata" \
  --data-urlencode "authtoken=$AUTHTOKEN" \
  | strip_jsonp | jq '{pers: .pers, reg_tickets: .reg_tickets}'
```

Then verify:
- the target term ticket exists
- `.ticket` is not null
- `.actions` contains `"add"`

Your reference says if the ticket is null or the action is not allowed, you should abort instead of trying to register. f## Bottom line

You are not stuck on VT behavior. You are stuck on **shell-command corruption**.

The evidence from your transcript says:
- swap staging worked
- the final request was malformed
- the biggest culprits are `TERM` clobbering and literal `\n` pollution in your variables fRun the corrected block above cleanly and you should get a much more useful result. If you want, paste the next `register.json` and I’ll tell you exactly what it means.

---

# you asked

noy my terminal is like fucked up?

╭─    ~ ··············································································································································································································································································································································   05:44:46 PM
╰─❯ clear
'202606': unknown terminal type.

╭─    ~ ··············································································································································································································································································································································   05:44:48 PM
╰─❯ c
'202606': unknown terminal type.

---

# chatgpt response

Yes. You clobbered your shell’s `TERM` variable.

`TERM` is supposed to be something like `xterm-256color`, `screen-256color`, or `tmux-256color`. You set it to `202606`, which is why programs like `clear` now think your terminal type is literally `202606` and fail with `unknown terminal type`. Your transcript already showed the first sign of that earlier with `zsh: can't find terminal definition for 202606`. 

## Fix it right now

Run this:

```zsh
export TERM=xterm-256color
```

Then test:

```zsh
clear
echo $TERM
```

You should see:

```text
xterm-256color
```

## If that does not fully fix it

You may be inside tmux, Warp, iTerm, or another terminal that prefers a different value. Try these in order:

```zsh
export TERM=xterm-256color
# or
export TERM=screen-256color
# or
export TERM=tmux-256color
```

Then test `clear` again.

## To see what your terminal probably should be

Run:

```zsh
infocmp | head
```

If that still complains, just set:

```zsh
export TERM=xterm-256color
```

and move on for now.

## Important

Do **not** use `TERM` as your semester variable again.

Use:

```zsh
VT_TERM=202606
```

not:

```zsh
TERM=202606
```

That was the bug.

## Clean recovery sequence

Run these exactly:

```zsh
export TERM=xterm-256color
unset VT_TERM ADD_CRN DROP_CRN HOURS GMOD REG_INFO AUTHTOKEN PERS_ID PERS_ID_PROOF TIME_TICKET URL_REPLAY TT_ENC URL_REPLAY_ENC
hash -r
clear
```

Then restart the setup with:

```zsh
VT_TERM='202606'
```

not `TERM='202606'`.

If `clear` still errors after `export TERM=xterm-256color`, tell me whether you’re in Terminal.app, iTerm2, Warp, or tmux.

---

# you asked

how do i get the full 7 field time ticket?

---

# chatgpt response

The easiest way is: **do not assemble it by hand first**. Let VT’s own page code do it for you.

Your reference says the browser already exposes a helper:

- `foseCart.timeTickets.mustGetTimeTicketParam(term)` → returns the fully assembled **7-field** `time_ticket`
- `foseCart.timeTickets.termTickets[term]` → cached structured ticket data if you want to inspect it manually 

## Fastest path: browser console on `classes.vt.edu`

Open DevTools Console while logged into `classes.vt.edu`, then run:

```javascript
const term = "202606";
console.log(foseCart.timeTickets.mustGetTimeTicketParam(term));
```

That should print the full 7-field string directly. 

If you also want to inspect the raw ticket object:

```javascript
const term = "202606";
console.log(foseCart.timeTickets.termTickets[term]);
```

## If that helper is unavailable, build it from `studentdata`

Your reference gives the exact construction algorithm:

1. fetch `studentdata`
2. find the `reg_tickets` entry for the target term
3. take its 5-field `.ticket`
4. parse `.start_date` into Unix milliseconds
5. append `|<timestamp_ms>|<pers_id>`

Final format:

```text
<start_iso>|<term>|<career_or_empty>|<banner_hash>|<signed_hash>|<timestamp_ms>|<pers_id>
```

That is the full 7-field `time_ticket`. 

## Console snippet that builds it for you

Run this in the browser console on `classes.vt.edu`:

```javascript
(async () => {
  const term = "202606";

  const r = await fetch('/api/?page=sisproxy&action=studentdata&authtoken=' + sam.auth.token);
  const t = await r.text();
  const m = t.match(/^setRecord$([\s\S]*)$[;\s]*$/);
  if (!m) throw new Error('failed to parse studentdata');

  const d = JSON.parse(m[1]);
  const rt = d.reg_tickets.find(x => x.term === term);

  if (!rt) throw new Error('no reg_ticket for term ' + term);
  if (!rt.ticket) throw new Error('ticket is null for term ' + term);
  if (!rt.actions || !rt.actions.includes('add')) {
    throw new Error('registration window not active for add');
  }

  const normalized = rt.start_date.replace(
    /(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}):(\d+[-+]\d{2}:\d{2})/,
    '$1.$2'
  );

  const startMillis = new Date(normalized).getTime();
  const persId = d.pers.id;
  const timeTicket = `${rt.ticket}|${startMillis}|${persId}`;

  console.log({ regTicket: rt, startMillis, persId, timeTicket });
  await navigator.clipboard.writeText(timeTicket);
  console.log('Copied full 7-field time_ticket to clipboard');
})();
```

That matches the exact algorithm in your doc, including the weird VT timestamp normalization step. f## Important checks

Before trusting the result, confirm:

- `rt.ticket` is not `null`
- `rt.actions` contains `"add"` for an add/swap test
- the output has **7 pipe-delimited fields** fQuick shell check after pasting it:

```zsh
printf '%s\n' "$TIME_TICKET" | awk -F'|' '{print NF}'
```

You want:

```text
7
```

## If you want the simplest answer

Use this in the browser console:

```javascript
foseCart.timeTickets.mustGetTimeTicketParam("202606")
```

That is the shortest path to the full 7-field ticket. fIf you want, I’ll give you one copy-paste console snippet that prints **all** of these at once: `authtoken`, `pers_id`, `pers_id_proof`, and the full 7-field `time_ticket`.

---

# chatgpt response

Good. This test told you something important.

## What happened

The **register call succeeded**. You got a valid queue response:

- `{"body":"WAIT","code":200,"data":{"id":"144497"}}`

Then status returned a terminal success:

- `{"body":"PROCESSED","code":200,...}`

And the payload shows only **one success**:

- `ticketResponse.reg_success["60058"]`
- message: `"This class has been added to your schedule"` 

Then `studentdata` confirmed the final state:

- `60058` is now registered
- `60900` is **still** registered 

So the result is not ambiguous. Your request did **not** perform an atomic swap. It performed an **add only**.

## Why this matters

This is a direct contradiction of the current assumption in your reference that:

> `cart_add` with `crn_drop=X` plus normal `register` will atomically drop X and add Y. 

At least in this real test case, that assumption did **not** hold.

That means your docs need to be downgraded from:

- “swap works like this”

to:

- “this is the hypothesized mechanism, but current empirical evidence shows add-only behavior in at least one case”

That is a meaningful correction.

## What your evidence actually proves

You now have evidence for all of these:

1. `cart_add` with `crn_drop` **does** stage something in the cart  
   You saw both:
   - `202606|default|60058|...`
   - `202606|default|60900|...|D|...`
   in the cart response. 

2. `shockabsorber register` **does** process the add  
   The status response explicitly says `60058` was added. 

3. The drop portion was **not** executed  
   `studentdata` after the operation still shows both CRNs registered. 

So the current state of knowledge is:

**`crn_drop` affects cart staging, but that alone is not sufficient to produce a real add+drop transaction in the way you expected.**

## The likely conclusion

Your implementation model for swap is still incomplete.

The most likely possibilities are:

- VT’s UI sends some additional parameter or cart state you are not reproducing
- the drop entry in cart is only advisory until some other condition is met
- swap requires a slightly different replay shape or request context than the plain add flow
- your documentation inferred swap semantics from the cart behavior, but Banner’s actual commit behavior is narrower than that inference

Right now, you should **not** code swap support as “solved.”

## What I would change in the docs immediately

Add a note like this:

> Empirical test on 2026-04-22: `cart_add(..., crn_drop=<registered_crn>)` successfully staged both an add entry and a drop-marked cart entry, but `shockabsorber register` only added the target CRN and did not drop the original registered CRN. Status returned `PROCESSED` with `reg_success` only for the added CRN, and `studentdata` after showed both CRNs registered. Therefore, atomic swap behavior is not yet confirmed and should be treated as unresolved. 

Also add `"PROCESSED"` to your observed shockabsorber terminal statuses. Your reference currently mentions `"WAIT"` and `"OK"` as observed bodies, but now you have a real `"PROCESSED"` terminal response with useful nested result data. 

## Best next step

Do **not** guess from here. Capture a **real browser-driven swap** from the VT UI and diff it.

That means:

1. perform a manual swap in the site UI with a disposable class
2. in DevTools Network, capture:
   - the `cart_add` request
   - the `shockabsorber register` request
   - the `status` response
3. compare them byte-for-byte against your curl flow

That is the fastest way to find what you are missing.

Because no, the theory is not enough anymore. Your curl test just falsified it.

## Practical immediate cleanup

You are now double-registered in both `60058` and `60900`, so clean that up deliberately. Do **not** blindly fire more registration calls until you decide exactly which CRN you want to keep. Your own reference is explicit that `register` can be destructive if used carelessly on already-registered sections. 

## Bottom line

This was a successful test.

Not because swap worked, but because it **didn’t**, and now you know that your current swap model is incomplete. That is exactly the kind of thing you needed to learn before writing the feature. 

If you want, I’ll help you turn this result into an updated “known facts / unknowns” section for the reference doc, plus the exact browser-network capture plan to resolve swap behavior.

---

# you asked

i went into the registration ui and manually did an atomic add/drop, and this is what gets sent:

page=shockabsorber&time_ticket=2026-03-17T07%3A00%3A00%3A000000000-04%3A00%blahblahblah7C4J3hVN%2FBqjc1yqKpDrW6Yw%3D%3D%7CMaVNpknJF4zGJACjMSVeoQ%7C1773745200000%7CzKNYy3aq24T9f5K25HFUJw%3D%3D&action=register&cart_name=default&url_replay=api%2F%3Fpage%3Dsisproxy%26action%3Dregister%26term_code%3D202606%26crn%3D60058%252C60900%26wait_crn%3D%26swap_crn%3D
authtoken=f60aa8blahblah-4011-ab10-045a550f93fe&_pers_id=zKNYy3aqblahblahFUJw%3D%3D&_pers_id_proof=blahblah%7CAKXVV%2BfogGxlMdkBOei%2B%2Fw%3D%3D&_pers_real_id=zKNYy3aq24T9f5K25HFUJw%3D%3D

---

# chatgpt response

Yep. **That is the missing piece.**

Your previous curl test did **add only** because your `url_replay` only had:

```text
crn=60058
```

But the real VT UI’s atomic add/drop request sends:

```text
crn=60058%2C60900
```

inside the **inner** replay URL, and then the whole `url_replay` gets encoded again for the outer shockabsorber request, which is why you see:

```text
...url_replay=...crn%3D60058%252C60900...
```

That explains your earlier result perfectly: Banner only saw “register 60058”, so it only added `60058` and left `60900` alone. 

## What this means

Your swap model needs to be updated to:

1. `cart_add` with `crn_drop=<DROP_CRN>` to stage the cart
2. `shockabsorber register` with `url_replay` pointing to:
   ```text
   api/?page=sisproxy&action=register&term_code=<TERM>&crn=<ADD_CRN>%2C<DROP_CRN>&wait_crn=&swap_crn=
   ```
3. same form body:
   - `authtoken`
   - `_pers_id`
   - `_pers_id_proof`
   - `_pers_real_id`

So no, the cart staging alone was not enough. The **register replay target itself** carries the add+drop intent.

## The critical difference

### What you sent before
```text
api/?page=sisproxy&action=register&term_code=202606&crn=60058&wait_crn=&swap_crn=
```

### What the UI actually sends for atomic add/drop
```text
api/?page=sisproxy&action=register&term_code=202606&crn=60058%2C60900&wait_crn=&swap_crn=
```

That comma-separated CRN list is the key.

## How to build it correctly in zsh

Use this:

```zsh
INNER_CRNS="${ADD_CRN}%2C${DROP_CRN}"
URL_REPLAY="api/?page=sisproxy&action=register&term_code=$VT_TERM&crn=$INNER_CRNS&wait_crn=&swap_crn="
URL_REPLAY_ENC=$(jq -rn --arg v "$URL_REPLAY" '$v|@uri')
```

Then your final encoded `url_replay` should look structurally like:

```text
api%2F%3Fpage%3Dsisproxy%26action%3Dregister%26term_code%3D202606%26crn%3D60058%252C60900%26wait_crn%3D%26swap_crn%3D
```

That `%252C` in the outer URL is **correct** because the inner replay URL already contains `%2C`.

## Corrected register call

```zsh
curl -sS -X POST \
  "$BASE/api/?page=shockabsorber&time_ticket=$TT_ENC&action=register&cart_name=default&url_replay=$URL_REPLAY_ENC" \
  --data-urlencode "authtoken=$AUTHTOKEN" \
  --data-urlencode "_pers_id=$PERS_ID" \
  --data-urlencode "_pers_id_proof=$PERS_ID_PROOF" \
  --data-urlencode "_pers_real_id=$PERS_ID" \
  | tee register_swap.json | jq .
```

Then poll status the same way as before.

## What I’d update in your reference doc

You should revise the swap section to say:

- `cart_add(..., crn_drop=DROP_CRN)` stages the swap intent in cart
- but **atomic add/drop only occurs when the shockabsorber `url_replay` inner `crn` parameter contains both CRNs as a comma-separated list**
- for example:
  ```text
  crn=ADD_CRN%2CDROP_CRN
  ```

That is the empirical rule now.

## One caution

Before you run the corrected swap, check your current registration state again. You already added the target once in the previous test, so you are not in the same starting condition anymore. If you fire another register call from the wrong state, you could create a different result than the one you intend. Your own reference is explicit that register behavior is state-sensitive and can be destructive if used carelessly. 

## Bottom line

You found the bug.

It was **not**:
- `time_ticket`
- `cart_name`
- `crn_drop`
- form body credentials

It was the fact that for a real atomic add/drop, the UI encodes **both CRNs in the inner `url_replay` `crn` parameter**.

That is the missing mechanism.

---

# you asked

give me the full process to go through one more time

---

# chatgpt response

Yes. Here is the **full corrected end-to-end process** for testing a real atomic add/drop one more time.

## 0. Before you do anything

First, do **not** reuse `TERM` as a variable name. Use `VT_TERM`. You already proved that clobbering `TERM` breaks the terminal, and more importantly it is just the wrong variable for semester codes. 

Second, make sure your **starting registration state is what you think it is**. Your last test added `60058` without dropping `60900`, so you were left registered in both. If you test again from the wrong starting state, the result will not mean what you think it means. 

If your terminal is still weird, fix it first:

```zsh
export TERM=xterm-256color
```

---

## 1. Set the non-secret variables

Use these as examples, but change them to whatever CRNs you are actually testing.

```zsh
BASE='https://classes.vt.edu'
VT_TERM='202606'
ADD_CRN='60058'
DROP_CRN='60900'
HOURS='3'
GMOD='N'
REG_INFO='E'
```

These match the `sisproxy cart_add` parameters your reference documented, including `cart_name=default`, `hours`, `gmod`, and `reg_info`. 

---

## 2. Capture the credentials and time ticket from the logged-in browser session

Open DevTools Console on `classes.vt.edu` while logged in and run this:

```javascript
(async () => {
  const term = "202606";

  const r = await fetch('/api/?page=sisproxy&action=studentdata&authtoken=' + sam.auth.token);
  const t = await r.text();
  const m = t.match(/^setRecord$([\s\S]*)$[;\s]*$/);
  if (!m) throw new Error('failed to parse studentdata');

  const d = JSON.parse(m[1]);
  const rt = d.reg_tickets.find(x => x.term === term);

  if (!rt) throw new Error('no reg_ticket for term ' + term);
  if (!rt.ticket) throw new Error('ticket is null for term ' + term);
  if (!rt.actions || !rt.actions.includes('add')) {
    throw new Error('registration window not active for add');
  }

  const normalized = rt.start_date.replace(
    /(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}):(\d+[-+]\d{2}:\d{2})/,
    '$1.$2'
  );

  const startMillis = new Date(normalized).getTime();
  const creds = {
    authtoken: sam.auth.token,
    pers_id: d.pers.id,
    pers_id_proof: d.pers.idProof,
    time_ticket: `${rt.ticket}|${startMillis}|${d.pers.id}`
  };

  console.log(creds);
})();
```

That follows the exact documented construction rule: use `studentdata.reg_tickets[i].ticket`, normalize `start_date`, convert it to milliseconds, then append `|<timestamp_ms>|<pers_id>`. fNow paste those values into your shell:

```zsh
read -s "AUTHTOKEN?AUTHTOKEN: "; echo
read -s "PERS_ID?_pers_id: "; echo
read -s "PERS_ID_PROOF?_pers_id_proof: "; echo
read -s "TIME_TICKET?FULL 7-FIELD time_ticket: "; echo
```

Sanity check the ticket:

```zsh
printf '%s\n' "$TIME_TICKET" | awk -F'|' '{print "time_ticket fields:", NF}'
```

You want `7`. You already verified this successfully in your later run. f---

## 3. Define the JSONP stripper

`sisproxy` returns JSONP wrappers like `setRecord(...)`, `setCart(...)`, and `preflight(...)`, so this helper is needed for every read/validation call. f```zsh
strip_jsonp() {
  perl -0777 -pe 's/^[A-Za-z_]+$([\s\S]*)$[;\s]*$/$1/s'
}
```

---

## 4. Confirm the add target is open

`fose` is public and returns plain JSON. You want `stat: "A"` before attempting the swap. fil`zsh
curl -sS "$BASE/api/?page=fose&route=search" \
  -H 'Content-Type: application/json' \
  --data "{\"other\":{\"srcdb\":\"$VT_TERM\"},\"criteria\":[{\"field\":\"crn\",\"value\":\"$ADD_CRN\"}]}" \
  | jq .
```

You already saw this work and return `60058` as open. fil-

## 5. Check current registration state before doing anything destructive

This matters because `register` behavior is state-sensitive, and your reference explicitly warns that firing `register` against a CRN you are already registered for can become destructive. fil`zsh
curl -sS --get "$BASE/api/" \
  --data-urlencode "page=sisproxy" \
  --data-urlencode "action=studentdata" \
  --data-urlencode "authtoken=$AUTHTOKEN" \
  | strip_jsonp | tee before.studentdata.json \
  | jq --arg term "$VT_TERM" --arg add "$ADD_CRN" --arg drop "$DROP_CRN" '
      [(.reg[$term] // [])[] | split("|")[0]] as $crns
      | {
          registered_crns: $crns,
          add_already_registered: ($crns | index($add) != null),
          drop_currently_registered: ($crns | index($drop) != null)
        }'
```

For a clean swap test, you want:
- `add_already_registered: false`
- `drop_currently_registered: true`

That matched your earlier pre-test state before the first add-only run. fil-

## 6. Optional but recommended: preflight

Preflight is a safe validation step and your reference recommends it. Just do not mistake a clean preflight for a guaranteed registration success. fil`zsh
curl -sS --get "$BASE/api/" \
  --data-urlencode "page=sisproxy" \
  --data-urlencode "action=preflight" \
  --data-urlencode "term_code=$VT_TERM" \
  --data-urlencode "cart_name=default" \
  --data-urlencode "crn_list=$ADD_CRN" \
  --data-urlencode "authtoken=$AUTHTOKEN" \
  | strip_jsonp | tee preflight.json | jq .
```

You previously got the typical:

```json
{"reg_course_errors":{"60058":"||"},"reg_non-course_errors":[]}
```

which is the soft/non-blocking state you already documented. fil-

## 7. Stage the swap in the cart

This is **not enough by itself**, but it is still required. `cart_add` must include `crn_drop=<DROP_CRN>`, and `cart_name` must be `default`. Your prior test already proved this stages both an add entry and a drop-marked entry in cart. fil`zsh
curl -sS --get "$BASE/api/" \
  --data-urlencode "page=sisproxy" \
  --data-urlencode "action=cart_add" \
  --data-urlencode "term_code=$VT_TERM" \
  --data-urlencode "cart_name=default" \
  --data-urlencode "crn=$ADD_CRN" \
  --data-urlencode "crn_drop=$DROP_CRN" \
  --data-urlencode "hours=$HOURS" \
  --data-urlencode "gmod=$GMOD" \
  --data-urlencode "reg_info=$REG_INFO" \
  --data-urlencode "authtoken=$AUTHTOKEN" \
  | strip_jsonp | tee cart_after_add.json | jq .
```

Then verify cart contents:

```zsh
curl -sS --get "$BASE/api/" \
  --data-urlencode "page=sisproxy" \
  --data-urlencode "action=cart_read" \
  --data-urlencode "authtoken=$AUTHTOKEN" \
  | strip_jsonp | tee cart_read.json | jq .
```

In your prior run, this showed:
- the add entry for `60058`
- a drop-marked `60900` entry with `D` in the cart data fil-

## 8. Build the **correct** `url_replay` for a real atomic add/drop

This was the missing piece. Your manual browser capture showed that VT’s UI does **not** send a replay URL with just `crn=60058`. It sends both CRNs in the inner replay URL:

```text
crn=60058%2C60900
```

and then that whole replay URL gets encoded again for the outer shockabsorber request. That is why your first attempt only added the new course and did not drop the old one. filild it like this:

```zsh
INNER_CRNS="${ADD_CRN}%2C${DROP_CRN}"
URL_REPLAY="api/?page=sisproxy&action=register&term_code=$VT_TERM&crn=$INNER_CRNS&wait_crn=&swap_crn="

TT_ENC=$(jq -rn --arg v "$TIME_TICKET" '$v|@uri')
URL_REPLAY_ENC=$(jq -rn --arg v "$URL_REPLAY" '$v|@uri')
```

Sanity check:

```zsh
printf 'URL_REPLAY=%q\n' "$URL_REPLAY"
printf 'TT_ENC=%q\n' "$TT_ENC"
printf 'URL_REPLAY_ENC=%q\n' "$URL_REPLAY_ENC"
```

For a correct atomic add/drop, the final encoded replay URL should structurally contain `crn%3D60058%252C60900...`, matching what you observed from the real UI. fil-

## 9. Submit the actual register call through shockabsorber

This is the real commit step. `time_ticket` goes in the outer query string, `url_replay` goes in the outer query string, and the form body contains `authtoken`, `_pers_id`, `_pers_id_proof`, and `_pers_real_id`. That exactly matches both your reference and the real browser request you captured. fil`zsh
curl -sS -X POST \
  "$BASE/api/?page=shockabsorber&time_ticket=$TT_ENC&action=register&cart_name=default&url_replay=$URL_REPLAY_ENC" \
  --data-urlencode "authtoken=$AUTHTOKEN" \
  --data-urlencode "_pers_id=$PERS_ID" \
  --data-urlencode "_pers_id_proof=$PERS_ID_PROOF" \
  --data-urlencode "_pers_real_id=$PERS_ID" \
  | tee register_swap.json | jq .
```

A normal queued start looks like:

```json
{"body":"WAIT","code":200,"data":{"id":"..."}}
```

which you already got in the successful non-atomic attempt. fil-

## 10. Poll status

Poll until the body is no longer `WAIT`. Your real run showed `PROCESSED` as a terminal success state with nested `ticketResponse`, so that now belongs in your empirical model too. fil`zsh
for i in {1..30}; do
  echo "status poll $i"

  RESP=$(
    curl -sS -X POST \
      "$BASE/api/?page=shockabsorber&time_ticket=$TT_ENC&action=status&cart_name=default" \
      --data-urlencode "authtoken=$AUTHTOKEN" \
      --data-urlencode "_pers_id=$PERS_ID" \
      --data-urlencode "_pers_id_proof=$PERS_ID_PROOF" \
      --data-urlencode "_pers_real_id=$PERS_ID"
  )

  echo "$RESP" | tee "status_$i.json" | jq .

  BODY=$(echo "$RESP" | jq -r '.body // empty')
  if [[ "$BODY" != "WAIT" ]]; then
    break
  fi

  sleep 2
done
```

In your earlier run, status returned:

```json
{
  "body": "PROCESSED",
  "code": 200,
  "data": {
    "id": "144497",
    "ticketResponse": {
      "reg_success": {
        "60058": {
          "message": "This class has been added to your schedule"
        }
      }
    }
  }
}
```

That proved the earlier request was add-only, not a real swap. fil-

## 11. Confirm the actual final state with `studentdata`

This is the real source of truth. Your reference is explicit that you must confirm via `studentdata`, not assume success/failure from status alone. fil`zsh
curl -sS --get "$BASE/api/" \
  --data-urlencode "page=sisproxy" \
  --data-urlencode "action=studentdata" \
  --data-urlencode "authtoken=$AUTHTOKEN" \
  | strip_jsonp | tee after.studentdata.json \
  | jq --arg term "$VT_TERM" --arg add "$ADD_CRN" --arg drop "$DROP_CRN" '
      [(.reg[$term] // [])[] | split("|")[0]] as $crns
      | {
          registered_crns: $crns,
          add_now_registered: ($crns | index($add) != null),
          drop_still_registered: ($crns | index($drop) != null)
        }'
```

For a **real atomic add/drop**, the result you want is:

- `add_now_registered: true`
- `drop_still_registered: false`

Your previous run showed:
- `add_now_registered: true`
- `drop_still_registered: true`

which is why it was add-only. fil-

## 12. If anything is ambiguous, do not blindly retry

This is not optional. Your reference already settled the safe retry rule:

1. weird timeout or ambiguous result
2. fetch `studentdata`
3. inspect actual registration state
4. only retry if clearly safe

Do **not** hammer `register`. That is how you create accidental destructive behavior. fil-

## The short version of what changed

The corrected full process is the same as before **except for one crucial fix**:

- stage swap with `cart_add ... crn_drop=DROP_CRN`
- then register with `url_replay` whose **inner `crn` contains both CRNs**:
  `crn=ADD_CRN%2CDROP_CRN`

That was the missing mechanism you discovered from the real UI request. fil you want, I can turn all of this into one clean `swap_test.zsh` script so you stop retyping it.

---

# you asked

Alright now with that tested, and this updated doc, lets go over that overview of building the project again, the structure, the architecture, the core concepts, etc

---

# chatgpt response

Yes. And now that you’ve actually verified the ugly parts, the project overview gets sharper.

Before, we were designing around uncertainty. Now the architecture should be built around what you’ve **actually proven**:

- VT uses `authtoken` plus `_pers_id` / `_pers_id_proof` for the protected flow, with no `IDMSESSID` requirement in practice. 
- `studentdata` is the central endpoint because it gives you identity, tickets, cart, current registrations, history, and waitlist state in one call. 
- `sisproxy` is JSONP-wrapped, so parsing is a first-class concern. 
- atomic swap is real, but only when the replay URL is built correctly with **both CRNs** in the inner `crn` parameter. `crn_drop` in `cart_add` alone is not enough. 
- `studentdata` confirmation after registration is mandatory. You do not trust immediate status responses blindly. 

That changes what the project should optimize for.

---

# What this project is

This should be:

> **A single-user, single-instance Go daemon that can run either locally or on a self-hosted VPS, with a CLI for control, a small optional web UI for convenience, SQLite for persistence, and VT-specific integration logic built directly around the verified API behavior.**

Not SaaS.  
Not multi-user.  
Not “web backend first.”

That still holds, and your updated doc strengthens that conclusion, especially around credential handling and policy posture. 

---

# The core concepts

These are the concepts the whole codebase should revolve around.

## 1. `studentdata` is the source of truth
This is the most important design principle.

Use `studentdata` to:
- validate credentials
- fetch `pers_id` / `pers_id_proof`
- fetch `reg_tickets`
- inspect current registrations
- inspect cart
- confirm post-registration state

That means your app should not keep trying to infer reality from stale cached state when it can cheaply ask VT for the authoritative state. 

## 2. There are three real layers of the VT API
Your own reference makes this clean:

- `fose` for public seat/status polling
- `sisproxy` for authenticated state/cart/preflight
- `shockabsorber` for actual registration queueing and Banner commit 

That should map directly into the code structure.

## 3. Parsing is not incidental
No, this is not “just some HTTP calls.”

You have to handle:
- JSONP wrappers from `sisproxy`
- pipe-delimited cart and registration records
- weird VT timestamps
- time ticket construction
- double-encoded swap replay URLs 

That means parser utilities are part of the product, not implementation trivia.

## 4. Registration is a state machine
The actual registration flow is not one request. It is:

1. `fose search`
2. `preflight`
3. `cart_add`
4. `shockabsorber register`
5. `shockabsorber status`
6. `studentdata` confirmation 

And each step can fail independently.

So the app should not be written like “if seat open, call register.” That would be sloppy and wrong.

## 5. Safe retry discipline matters
This is one of the most important operational ideas in the whole project.

If registration is ambiguous:
- do **not** blindly retry
- fetch `studentdata`
- decide based on actual registration state 

That principle should be baked into the registrar logic from day one.

## 6. Swaps are not just adds with one extra flag
This was the big correction.

A real swap requires:
- cart staging with `crn_drop`
- **and** a replay URL whose inner `crn` contains `ADD_CRN,DROP_CRN`  
- and then the outer query string encoding on top of that 

This means swap deserves its own code path, tests, and helpers. Do not hide it as a tiny boolean option inside generic add code.

---

# The architecture

The right architecture is still a **daemon-first app**, but now it is easier to define the pieces precisely.

## High-level shape

```text id="sgqbtv"
User
  -> Cobra CLI / optional local web UI
      -> single Go daemon
          -> SQLite
          -> VT API clients
              -> fose
              -> sisproxy
              -> shockabsorber
```

One process.  
One SQLite DB.  
One scheduler loop.  
One codebase for both local and self-hosted.

## Internal components

### 1. Session manager
Responsible for:
- storing credentials locally
- validating them
- refreshing state from `studentdata`
- checking whether the registration window is active
- optionally doing keepalive calls if you want long-lived sessions 

### 2. Watch manager
Responsible for:
- creating/removing watches
- storing target CRN, term, optional drop CRN
- polling `fose`
- tracking last known seat status
- deciding when a watch becomes eligible to attempt registration

### 3. Registrar
Responsible for:
- preflight
- cart staging
- building `time_ticket`
- building correct replay URLs
- register call
- status polling
- `studentdata` confirmation
- safe retry behavior

This is the part that must be the most conservative.

### 4. Scheduler / daemon loop
Responsible for:
- polling due watches
- spacing network requests
- rate limiting
- managing retries and cooldowns
- running forever

### 5. UI layer
Two surfaces:
- Cobra CLI
- optional small web admin UI

The UI layer should call into services. It should not contain VT protocol logic.

---

# The repo structure

This is the version I’d back now.

```text id="m3lt6h"
vtwatch/
  go.mod
  go.sum
  README.md

  cmd/
    vtwatch/
      main.go

  internal/
    cli/
      root.go
      run.go
      watch.go
      session.go
      doctor.go

    config/
      config.go

    store/
      sqlite.go
      migrations.go
      sessions.go
      watches.go
      attempts.go

    vt/
      fose.go
      sisproxy.go
      shockabsorber.go
      jsonp.go
      timeticket.go
      replay.go
      cartparse.go
      studentdata.go
      types.go

    session/
      service.go

    watch/
      service.go
      scheduler.go

    register/
      service.go
      single.go
      swap.go
      confirm.go

    web/
      server.go
      handlers.go

    notify/
      notify.go

  migrations/
    001_init.sql

  deploy/
    Dockerfile
    docker-compose.yml
    vtwatch.service

  scripts/
  testdata/
```

## Why this is the right level

This is simpler than the earlier overbuilt version, but not so flat that everything turns into `main.go` soup.

### `internal/vt/`
This should be the protocol layer.

And yes, I would keep this package slightly denser than other packages, because that is where most of the weirdness lives:
- JSONP
- parsing
- time ticket logic
- replay URL logic
- cart parsing
- request builders

That weirdness belongs together.

### `internal/register/`
This deserves its own package because the registration flow is not just a request wrapper. It is behavior and policy:
- safe sequencing
- destructive-action guards
- ambiguous-result handling
- confirmation logic

### `internal/session/`
I did not emphasize this earlier enough. With your updated doc, session handling is clearly an actual subsystem:
- credential loading
- validation
- keepalive
- window checks
- ticket readiness

So it earns its own package.

### `internal/web/`
Still optional, but justified if you want remote self-hosting to be tolerable.

---

# The data model

Keep it small and boring.

## `sessions`
Store:
- `authtoken`
- `pers_id`
- `pers_id_proof`
- captured_at
- last_validated_at
- maybe cached latest `studentdata` metadata
- maybe ticket window summary

Since the credentials are powerful, the doc’s advice matters here: local-only storage, strict permissions, ideally OS keychain when you get there. 

## `watches`
Store:
- id
- term
- add_crn
- optional drop_crn
- mode: `add` or `swap`
- active/paused
- poll interval
- last seen seat status
- next_poll_at
- last_attempt_at

## `attempts`
Store:
- watch_id
- phase: `preflight`, `cart_add`, `register`, `status`, `confirm`
- result
- short response summary
- created_at

You want a durable audit trail, because otherwise debugging this thing later will be miserable.

---

# The core flows

## Flow 1: connect credentials
1. user runs bookmarklet or console snippet
2. app stores credentials locally
3. app calls `studentdata`
4. app validates:
   - credentials work
   - ticket exists for target term
   - actions contain what is needed 

## Flow 2: add watch
1. user says “watch CRN X”
2. app stores watch
3. scheduler polls `fose` every configured interval
4. when `stat` becomes `A`, watch moves into registration path 

## Flow 3: single add
1. confirm not already registered
2. preflight
3. cart add
4. build single-CRN replay URL
5. register
6. status poll
7. confirm via `studentdata` 

## Flow 4: atomic swap
1. confirm target add CRN is not already registered
2. confirm drop CRN is currently registered
3. preflight
4. `cart_add(..., crn_drop=DROP_CRN)`
5. build swap replay URL with both CRNs
6. register
7. status poll
8. confirm via `studentdata` that add succeeded **and** drop disappeared 

That last confirmation is crucial. Your real testing is what proved it.

---

# The CLI shape

Cobra still makes sense.

Commands should probably look like this:

```bash id="0p6kcw"
vtwatch run
vtwatch doctor

vtwatch session import
vtwatch session validate
vtwatch session show

vtwatch watch add --term 202606 --crn 60058
vtwatch watch swap --term 202606 --add 60058 --drop 60900
vtwatch watch list
vtwatch watch pause <id>
vtwatch watch remove <id>

vtwatch serve
```

## Why this tree is right
Because “add” and “swap” are different behaviors now. They should not be forced into one ambiguous command with hidden conditionals.

---

# The web UI

Keep it small.

It should do only:
- import/view session health
- list watches
- add/remove/pause watches
- show recent attempts and errors
- show whether the current ticket window is active

Do **not** build a huge frontend first. That would be the same mistake as before, just in a different form.

---

# The testing strategy

Your updated doc already gives the correct testing ladder. Follow it. 

## Unit tests first
Test:
- JSONP stripper
- VT timestamp normalization
- time ticket builder
- replay URL builder
- cart line parser
- registration line parser

## Fixtures
Use `testdata/` heavily:
- `studentdata_response_sample.json`
- `preflight_success.json`
- `cart_add_success.json`
- `cart_add_with_swap.json`
- `shockabsorber_register_wait.json`
- `shockabsorber_register_swap_success.json` 

## Live tests last
Only with disposable CRNs.
Especially for swap.

That is not paranoia. That is basic discipline.

---

# The build order

This is the v1 order I would use now.

## Phase 1
- repo bootstrap
- Cobra root command
- SQLite setup
- config loading

## Phase 2
- `internal/vt/jsonp.go`
- `internal/vt/timeticket.go`
- `internal/vt/fose.go`
- `internal/vt/studentdata.go`

## Phase 3
- `cart_read`
- `cart_add`
- `cart_remove`
- `preflight`

## Phase 4
- single-CRN registrar
- `status` polling
- post-registration confirmation

## Phase 5
- swap registrar
- replay URL builder for `crn=ADD,DROP`
- extra guards for destructive cases

## Phase 6
- watch scheduler
- daemon loop
- rate limiting
- backoff/retry rules

## Phase 7
- small web UI
- deployment files
- self-hosting polish

No, I would not start with the daemon loop before the protocol layer is solid. That would be backwards.

---

# The most important design rules

These are the rules I would treat as non-negotiable.

## Rule 1
`studentdata` is authoritative.

## Rule 2
Never blindly retry `register`.

## Rule 3
Never call single-CRN register on a CRN the user is already registered for unless they explicitly requested a drop. 

## Rule 4
Swap logic must explicitly verify that replay URL contains both CRNs. 

## Rule 5
Everything uses `cart_name=default`. No config option for this. 

## Rule 6
Global rate limiting, not per-watch flooding. 

## Rule 7
Credentials never leave the user’s machine or self-hosted instance. 

---

# Final conclusion

Now that the swap behavior is actually understood, the project overview is:

- **single-user, self-hostable Go daemon**
- **Cobra CLI**
- **SQLite**
- **protocol-heavy `internal/vt` package**
- **explicit `register/single` and `register/swap` behavior**
- **watch scheduler on top of that**
- **optional thin web UI**
- **studentdata-centered verification model**
- **parser-first, state-machine-first implementation**

That is the real shape of the project now.

The next useful thing is to turn this into a concrete starter skeleton with the actual package/file list and what each file should contain in v1.

---

# you asked

also if you needed an example of what it looks like when errors are returned from the shockabosorber endpoint i tihnk:

      --data-urlencode "_pers_id=$PERS_ID" \
      --data-urlencode "_pers_id_proof=$PERS_ID_PROOF" \
      --data-urlencode "_pers_real_id=$PERS_ID"
  )

  echo "$RESP" | tee "status_$i.json" | jq .

  BODY=$(echo "$RESP" | jq -r '.body // empty')
  if [[ "$BODY" != "WAIT" ]]; then
    break
  fi

  sleep 2
done
status poll 1
{
  "body": "PROCESSED",
  "code": 200,
  "data": {
    "id": "144505",
    "ticketResponse": {
      "reg_success": {
        "60058": {
          "reg_entry": "60058|AAEC 2104||N|3|UG",
          "grade_mode": "N",
          "credits": 3,
          "level": "UG",
          "message": "This class has been added to your schedule"
        }
      },
      "reg_errors": {
        "60900": {
          "message": "Failed to remove this class from your schedule"
        }
      },
      "reg_non-course_errors": [
        "Registration hours cannot be less than required minimum hours."
      ]
    }
  }
}

---

# chatgpt response

Yes. That example is extremely useful.

And it changes one important assumption:

## No, swap is not safely modelable as “atomic”

That response shows a **partial success** outcome:

- `reg_success["60058"]` says the add succeeded
- `reg_errors["60900"]` says the drop failed
- `reg_non-course_errors` includes a global Banner-side rule failure: `"Registration hours cannot be less than required minimum hours."`

So from the tool’s point of view, this is **not** an all-or-nothing transaction. It is one registration attempt that can produce:

- per-CRN successes
- per-CRN failures
- global non-course errors

all in the same terminal status payload.

That means if you keep calling this “atomic swap” in the implementation model, you will write the wrong code. The request shape may be the UI’s swap flow, but the observed result proves the outcome can still be **partially applied**.

## What this means for the architecture

You should update the registrar design around this fact.

### 1. Registration result is not boolean
Do **not** model `register` as:

- success
- failure

That is wrong now.

It needs to be something more like:

- queued
- processed with full success
- processed with partial success
- processed with full failure
- ambiguous / timed out

And the payload parser needs to preserve:
- `reg_success`
- `reg_errors`
- `reg_non-course_errors`

because those are separate signal channels.

### 2. Swap needs a reconciliation phase
After **every** swap attempt, the app must fetch fresh `studentdata` and classify the final state. Your updated doc already centers `studentdata` as the authoritative confirmation step, and this example reinforces that. 

For swap, the reconciliation logic should explicitly detect:

- **SwapSucceeded**  
  add CRN present, drop CRN absent

- **AddOnlyPartial**  
  add CRN present, drop CRN still present

- **DropOnlyPartial**  
  add CRN absent, drop CRN absent or old missing unexpectedly

- **NoChange**  
  add absent, drop still present

- **Ambiguous**  
  state doesn’t fit neatly, or studentdata fetch failed

Your pasted example is clearly **AddOnlyPartial**.

### 3. “Swap” deserves its own result type
I would not use the same result model for:
- single add
- single drop
- swap

Single add can reasonably end in:
- added
- not added
- ambiguous

Swap needs richer structure, something like:

```go id="a6r06w"
type SwapOutcome struct {
    AddedCRN        string
    DroppedCRN      string
    AddSucceeded    bool
    DropSucceeded   bool
    CourseErrors    map[string]string
    GlobalErrors    []string
    FinalState      SwapFinalState
}
```

Because no, a swap is not just “two fields on an add.”

### 4. The watch/attempt log has to store structured response data
Your `attempts` table should not just keep a single status string like `success` or `failed`.

It should store:
- phase
- raw response JSON
- parsed add success map
- parsed per-CRN errors
- parsed non-course errors
- reconciled final state

Otherwise later debugging will suck.

## What this means for the docs

Your updated reference should add a section under `shockabsorber status` or swap behavior saying something like:

> A terminal `PROCESSED` response may contain mixed outcomes in `ticketResponse`, including `reg_success`, `reg_errors`, and `reg_non-course_errors` simultaneously. Therefore, swap behavior is not guaranteed to be all-or-nothing from the client’s perspective. A client must reconcile against fresh `studentdata` after every swap attempt.

That is now an empirical fact, not theory.

## What this means for testing

This should become a first-class fixture.

Add something like:

- `shockabsorber_register_swap_partial.json`

And specifically test parser behavior for:
- add success + drop failure
- add failure + drop success if you ever observe it
- mixed per-CRN and non-course errors

Your updated doc already emphasizes fixtures and careful integration testing, and this is exactly the sort of case that belongs there. 

## What this means for the product behavior

The UI/CLI should not say:

- “swap failed”

if the add succeeded and the drop failed.

That is misleading.

It should say something like:

- “Added 60058, but failed to remove 60900”
- “Banner returned a global error: Registration hours cannot be less than required minimum hours”

And then show the actual final schedule state after reconciliation.

That is the only honest UX.

## What I would change in the project design right now

I would make these explicit:

### `internal/register/swap.go`
Responsible for:
- building swap replay URL
- submitting request
- polling status
- parsing mixed results
- reconciling with `studentdata`

### `internal/register/confirm.go`
Responsible for:
- fetching fresh `studentdata`
- classifying final add/drop state
- producing canonical outcome enums

### `internal/vt/shockabsorber.go`
Needs types that can represent:

```go id="nzmxq7"
type ShockabsorberStatusResponse struct {
    Body string `json:"body"`
    Code int    `json:"code"`
    Data struct {
        ID             string `json:"id"`
        TicketResponse struct {
            RegSuccess        map[string]RegSuccess `json:"reg_success"`
            RegErrors         map[string]RegError   `json:"reg_errors"`
            RegNonCourseErrors []string             `json:"reg_non-course_errors"`
        } `json:"ticketResponse"`
    } `json:"data"`
}
```

Because that shape is now real.

## Bottom line

Yes, that response is valuable.

More than that, it forces an architectural correction:

**swap is not safely modeled as an atomic success/failure operation. It is a multi-outcome registration flow that requires post-hoc reconciliation against `studentdata`.**

That should directly affect:
- your docs
- your parser types
- your attempt/result model
- your CLI/web UX
- your test fixtures
- your state machine

That is a good find.

If you want, the next useful step is for me to rewrite the project overview one more time with this new “partial swap outcome” reality baked into the core architecture.

---

# you asked

yes

---

# chatgpt response

Good. With the partial-swap outcome reality included, the project overview changes in one important way:

**registration is not a yes/no operation, and swap is not a guaranteed all-or-nothing transaction.**

Your updated reference already established the core protocol details, especially around `studentdata`, `time_ticket`, `url_replay`, `default` cart usage, and the real swap replay shape with both CRNs in the inner replay URL.   
Your live testing then proved two extra things:

- if you omit the second CRN from the replay URL, the “swap” silently becomes add-only 
- even with the correct swap request shape, Banner can still return mixed outcomes, so the client must reconcile final state instead of assuming atomicity

That should now be baked into the architecture from the start.

# What the project is

**A single-user Go daemon that can run locally or on a self-hosted VPS, with a Cobra CLI, SQLite persistence, VT-specific protocol code, and an optional small web UI.**

Not SaaS.  
Not multi-user.  
Not “backend plus worker plus auth system.”

Same codebase, two deployment modes:

- local machine
- self-hosted single-user instance

That is still the right product shape, and your reference continues to support that direction from both a security and operational perspective. 

# The core architectural truth

The app is not “a bot that clicks register.”

It is a **stateful protocol client** built around three VT layers:

- `fose` for public seat polling
- `sisproxy` for authenticated state and cart operations
- `shockabsorber` for queued registration submission and status polling 

And the source of truth is:

- **always `studentdata`**

That one endpoint gives you identity, registration windows, current schedule, cart state, history, and confirmation after writes. 

# The core concepts

## 1. `studentdata` is authoritative
Use it for:

- credential validation
- session health
- registration window checks
- current registration state
- confirmation after every register attempt

Do not let the app become dependent on local guesses when VT will tell you the truth. 

## 2. Parsing is first-class
You are dealing with:

- JSONP wrappers
- pipe-delimited records
- VT’s weird timestamp format
- time ticket construction
- double-encoded replay URLs for swaps 

So `internal/vt` is not just a thin HTTP wrapper. It is a parsing and request-construction layer.

## 3. Registration is a workflow, not one request
The real flow is:

1. poll `fose`
2. run `preflight`
3. stage cart
4. submit `shockabsorber register`
5. poll `status`
6. confirm via `studentdata` 

Each step can fail independently. The code should reflect that.

## 4. Swap is a separate operation
No, swap should not be treated as “add with one extra optional field.”

A real swap requires:

- `cart_add(..., crn_drop=DROP_CRN)`
- replay URL whose inner `crn` is `ADD_CRN,DROP_CRN`
- correct double-encoding in the outer request 

That earns a dedicated code path.

## 5. Swap outcomes can be partial
This is the new architectural correction.

A swap attempt can produce:
- add success
- drop failure
- global non-course errors

all in the same terminal response.

So the client model must support:
- full success
- partial success
- full failure
- ambiguous outcome

No binary `success/failure` model is good enough anymore.

# The right mental model

The daemon has four real jobs:

## Session manager
Responsible for:
- storing credentials locally
- validating them
- checking registration windows
- optionally keeping the session alive
- surfacing session expiry/problems

## Watch scheduler
Responsible for:
- polling public section status
- deciding when a watch is due
- moving a watch into an attempt flow when a seat opens

## Registrar
Responsible for:
- preflight
- cart staging
- time ticket building
- replay URL construction
- register submission
- status polling
- reconciliation via `studentdata`

## Result reconciler
Responsible for:
- taking shockabsorber result payloads plus fresh `studentdata`
- classifying the actual final state
- producing a canonical outcome the rest of the app can use

That last piece is now important enough to name explicitly.

# The repo structure

This is the structure I would use now:

```text
vtwatch/
  go.mod
  go.sum
  README.md

  cmd/
    vtwatch/
      main.go

  internal/
    cli/
      root.go
      run.go
      watch.go
      session.go
      doctor.go

    config/
      config.go

    store/
      sqlite.go
      migrations.go
      sessions.go
      watches.go
      attempts.go

    vt/
      fose.go
      sisproxy.go
      shockabsorber.go
      jsonp.go
      timeticket.go
      replay.go
      cartparse.go
      studentdata.go
      types.go

    session/
      service.go

    watch/
      service.go
      scheduler.go

    register/
      service.go
      single.go
      swap.go
      confirm.go
      outcome.go

    web/
      server.go
      handlers.go

    notify/
      notify.go

  migrations/
    001_init.sql

  deploy/
    Dockerfile
    docker-compose.yml
    vtwatch.service

  scripts/
  testdata/
```

## Why this is right

### `internal/vt/`
This is the VT protocol layer:
- request builders
- response types
- parsing
- time ticket logic
- replay URL construction

### `internal/register/`
This is business behavior:
- guarded single add
- guarded swap
- result classification
- post-registration reconciliation

### `internal/watch/`
This is scheduling and seat monitoring, not VT protocol logic.

### `internal/session/`
This now clearly deserves its own package because session validity and ticket readiness are first-class concerns. 

### `internal/web/`
Still optional, but useful for self-hosting. Thin admin UI only.

# The data model

Keep it small.

## `sessions`
Fields like:
- `authtoken`
- `pers_id`
- `pers_id_proof`
- `captured_at`
- `last_validated_at`
- `status`
- maybe a cached summary of current registration windows

## `watches`
Fields like:
- `id`
- `term`
- `mode` (`add` or `swap`)
- `add_crn`
- optional `drop_crn`
- `active`
- `last_seen_stat`
- `next_poll_at`
- `last_attempt_at`

## `attempts`
This table matters more now.

Store:
- `watch_id`
- `phase`
- `outcome_kind`
- raw response JSON
- parsed success map
- parsed per-CRN error map
- parsed non-course errors
- reconciled final state
- timestamps

Because no, a single boolean `success` column is not enough anymore.

# The result model

This is the biggest conceptual upgrade.

## Single add outcome
Possible classified states:
- `Added`
- `NotAdded`
- `AlreadyRegistered`
- `Ambiguous`

## Swap outcome
Possible classified states:
- `SwapSucceeded`
- `AddOnlyPartial`
- `DropOnlyPartial`
- `NoChange`
- `Ambiguous`

And you should preserve:
- course-level success entries
- course-level error entries
- global non-course errors

So the app can say what actually happened instead of lying.

# The core flows

## Connect session
1. capture creds via bookmarklet or console snippet
2. store locally
3. call `studentdata`
4. validate credentials and tickets
5. expose session state to CLI/UI 

## Create add watch
1. user provides term + CRN
2. watch stored
3. scheduler polls `fose`
4. when `stat=A`, attempt add flow

## Create swap watch
1. user provides term + add CRN + drop CRN
2. app verifies drop CRN is currently registered
3. watch stored
4. scheduler polls `fose` for add target
5. when open, attempt swap flow

## Single add attempt
1. fresh `studentdata`
2. refuse if already registered
3. `preflight`
4. `cart_add`
5. build single replay URL
6. `register`
7. `status`
8. `studentdata` confirmation
9. classify outcome

## Swap attempt
1. fresh `studentdata`
2. verify add CRN absent, drop CRN present
3. `preflight`
4. `cart_add(..., crn_drop=DROP_CRN)`
5. build swap replay URL with both CRNs
6. `register`
7. `status`
8. `studentdata` confirmation
9. classify full success vs partial vs no change

That last classification is now mandatory.

# The CLI shape

Still use Cobra.

Command tree should probably look like:

```bash
vtwatch run
vtwatch doctor

vtwatch session import
vtwatch session validate
vtwatch session show

vtwatch watch add --term 202606 --crn 60058
vtwatch watch swap --term 202606 --add 60058 --drop 60900
vtwatch watch list
vtwatch watch pause <id>
vtwatch watch remove <id>

vtwatch serve
```

`watch add` and `watch swap` should remain separate commands. They are different operations now.

# The web UI

Keep it small and boring.

It should do:
- session import/health
- watch management
- attempt history
- current status
- ticket window visibility

It should not be a giant SPA-first project. The daemon is the product. The web UI is just an admin surface.

# The testing strategy

Your updated reference already provides the right order, and now there is one more category you need to emphasize: mixed-outcome swap results. 

## Unit tests
Test:
- JSONP stripping
- VT timestamp normalization
- time ticket building
- replay URL builder
- cart parsing
- studentdata parsing
- shockabsorber status parsing

## Fixtures
You now definitely want fixtures for:
- `cart_add_with_swap.json`
- `shockabsorber_register_wait.json`
- `shockabsorber_register_swap_success.json`
- `shockabsorber_register_swap_partial.json`
- `preflight_success.json`
- `preflight_blocked.json`
- `studentdata_after_add.json`
- `studentdata_after_swap_partial.json`

## Live tests
Use disposable CRNs only.
Especially for swap.

# The implementation order

This is the order I would use now.

## Phase 1
- repo bootstrap
- Cobra root command
- SQLite
- config

## Phase 2
- `jsonp.go`
- `timeticket.go`
- `fose.go`
- `studentdata.go`

## Phase 3
- `cart_read`
- `cart_add`
- `cart_remove`
- `preflight`

## Phase 4
- single add register path
- `status` polling
- confirmation and classification

## Phase 5
- swap replay builder
- swap register path
- partial outcome parsing
- reconciliation logic

## Phase 6
- watch scheduler
- daemon loop
- rate limiting
- backoff
- attempt history

## Phase 7
- small web UI
- deployment polish
- self-hosting docs

Do not start with the scheduler. Start with the VT protocol and parsing layer.

# The non-negotiable rules

These should be hard-coded into the design.

## Rule 1
`studentdata` is authoritative. 

## Rule 2
Never blindly retry register after an ambiguous result. 

## Rule 3
Single-CRN register on an already-registered CRN is dangerous and should be refused unless explicitly doing a drop. 

## Rule 4
Swap requests must verify both CRNs are present in the replay URL. 

## Rule 5
All cart operations use `default`. No option exposed for this. 

## Rule 6
Registration outcomes are classified after reconciliation, not just copied from shockabsorber’s immediate response.

## Rule 7
Credentials never leave the user’s machine or self-hosted instance. 

# Final conclusion

With the new partial-swap reality baked in, the project is now best understood as:

**a single-user Go daemon with a Cobra CLI, SQLite persistence, a VT protocol layer, a cautious registrar with reconciliation logic, a watch scheduler on top, and an optional thin web UI.**

The biggest design correction is this:

**swap is not atomic in the way your client code can safely assume. It is a multi-step, multi-outcome operation that must be reconciled against `studentdata` after every attempt.**

That should drive the whole implementation.

The next useful step is to turn this into a concrete starter skeleton with exact files and the v1 responsibilities for each file.

---

# you asked

help me get sstarted with phase 1

---

# chatgpt response

I’m going to make Phase 1 concrete: exact repo skeleton, the minimum files to create, and the first commands to run. No architecture fluff unless it changes what you should type.

Yes. For **Phase 1**, the goal should be narrow:

## Phase 1 target

By the end of Phase 1, you should have:

- a Go module
- one runnable `vtwatch` binary
- Cobra wired in with a root command and a `run` command
- config loading from a local file path
- SQLite opening successfully
- one migration table created
- zero VT API logic yet

That matches the repo shape we settled on, and it lines up with Go’s guidance to keep a command repo simple, use `internal/` for non-exported supporting packages, and use `cmd/` when you have commands plus internal packages. Cobra is a good fit here because it is built around commands, arguments, and flags, and the `cobra-cli` generator is meant to bootstrap a barebones CLI from within a Go module.  ([Go](https://go.dev/doc/modules/layout))
## What to build right now

Use this exact Phase 1 scope:

```text
vtwatch/
  go.mod
  go.sum
  README.md

  cmd/
    vtwatch/
      main.go

  internal/
    cli/
      root.go
      run.go
    config/
      config.go
    store/
      sqlite.go
      migrate.go

  migrations/
    001_init.sql
```

Do not add `vt/`, `watch/`, `register/`, `web/`, `notify/`, or `session/` yet. Those are real packages, but not Phase 1 packages. Right now you need a command, config, and persistence skeleton, nothing more. ([Go](https://go.dev/doc/modules/layout))
## Step 1: create the repo and module

From macOS zsh:

```zsh
mkdir -p ~/code/vtwatch
cd ~/code/vtwatch
go mod init github.com/YOUR_GITHUB_USERNAME/vtwatch
mkdir -p cmd/vtwatch internal/cli internal/config internal/store migrations
```

If you want to use Cobra’s generator, install it and initialize from within the module. Cobra’s own docs say `cobra-cli` is installed with `go install github.com/spf13/cobra-cli@latest`, and that `cobra-cli init` is run from inside a Go module. ([GitHub](https://github.com/spf13/cobra/blob/main/README.md))
## Step 2: use Cobra, but keep it thin

You have two valid choices.

### Option A, the one I recommend
Install Cobra generator, use it once, then simplify the generated code.

```zsh
go install github.com/spf13/cobra-cli@latest
cobra-cli init
cobra-cli add run
```

That gives you the bootstrap quickly. Cobra documents `cobra-cli init` for the initial app and `cobra-cli add` for new commands. ([GitHub](https://github.com/spf13/cobra-cli/blob/main/README.md))
### Option B
Write the CLI files yourself.

I actually think Option B is better for understanding the project, but Option A is fine if you want momentum.

## Step 3: add the minimal dependencies

You need:
- Cobra
- SQLite driver

I would use modernc SQLite first because it avoids CGO pain on Apple Silicon during early development.

```zsh
go get github.com/spf13/cobra@latest
go get modernc.org/sqlite
```

Cobra is the official library dependency. Its README explicitly describes it as a library for modern CLI apps and shows the normal import path. ([GitHub](https://github.com/spf13/cobra/blob/main/README.md))
## Step 4: put in the first files

### `cmd/vtwatch/main.go`

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/YOUR_GITHUB_USERNAME/vtwatch/internal/cli"
)

func main() {
	ctx := context.Background()
	if err := cli.Execute(ctx); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
```

### `internal/cli/root.go`

```go
package cli

import (
	"context"

	"github.com/spf13/cobra"
)

func Execute(ctx context.Context) error {
	return newRootCmd().ExecuteContext(ctx)
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vtwatch",
		Short: "Single-user VT registration watcher",
		Long:  "vtwatch is a single-user daemon/CLI for monitoring seats and attempting registration.",
	}

	cmd.AddCommand(newRunCmd())
	return cmd
}
```

### `internal/cli/run.go`

```go
package cli

import (
	"context"
	"fmt"

	"github.com/YOUR_GITHUB_USERNAME/vtwatch/internal/config"
	"github.com/YOUR_GITHUB_USERNAME/vtwatch/internal/store"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var cfgPath string

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Start the daemon loop",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			return run(ctx, cfgPath)
		},
	}

	cmd.Flags().StringVar(&cfgPath, "config", "", "Path to config file")
	return cmd
}

func run(ctx context.Context, cfgPath string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	db, err := store.Open(ctx, cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := store.Migrate(ctx, db); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	fmt.Println("vtwatch booted successfully")
	fmt.Printf("database: %s\n", cfg.DatabasePath)
	return nil
}
```

## Step 5: config loading

For Phase 1, keep config dead simple. No Viper.

### `internal/config/config.go`

```go
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	DatabasePath string `json:"database_path"`
}

func Load(path string) (Config, error) {
	if path == "" {
		path = defaultConfigPath()
	}

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		cfg := defaultConfig()
		if err := writeDefault(path, cfg); err != nil {
			return Config{}, err
		}
		return cfg, nil
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	if cfg.DatabasePath == "" {
		cfg.DatabasePath = defaultConfig().DatabasePath
	}

	return cfg, nil
}

func defaultConfig() Config {
	base := defaultDataDir()
	return Config{
		DatabasePath: filepath.Join(base, "vtwatch.db"),
	}
}

func defaultConfigPath() string {
	return filepath.Join(defaultDataDir(), "config.json")
}

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".config", "vtwatch")
}

func writeDefault(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, b, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
```

Why this is enough:
- it gives you a stable config path
- it writes a default file on first run
- it avoids pulling in Viper too early
- it keeps Phase 1 small

That is the right move for now.

## Step 6: SQLite open + migration

### `internal/store/sqlite.go`

```go
package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func Open(ctx context.Context, dbPath string) (*sql.DB, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("empty database path")
	}

	dsn := filepath.Clean(dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql open: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return db, nil
}
```

### `internal/store/migrate.go`

```go
package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
)

func Migrate(ctx context.Context, db *sql.DB) error {
	b, err := os.ReadFile("migrations/001_init.sql")
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}

	if _, err := db.ExecContext(ctx, string(b)); err != nil {
		return fmt.Errorf("apply migration: %w", err)
	}

	return nil
}
```

### `migrations/001_init.sql`

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  authtoken TEXT NOT NULL,
  pers_id TEXT NOT NULL,
  pers_id_proof TEXT NOT NULL,
  captured_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_validated_at DATETIME,
  status TEXT NOT NULL DEFAULT 'unknown'
);

CREATE TABLE IF NOT EXISTS watches (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  term TEXT NOT NULL,
  mode TEXT NOT NULL,
  add_crn TEXT NOT NULL,
  drop_crn TEXT,
  active INTEGER NOT NULL DEFAULT 1,
  last_seen_stat TEXT,
  next_poll_at DATETIME,
  last_attempt_at DATETIME,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS attempts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  watch_id INTEGER NOT NULL,
  phase TEXT NOT NULL,
  outcome_kind TEXT NOT NULL,
  raw_response TEXT,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (watch_id) REFERENCES watches(id)
);
```

That schema is intentionally small. It is enough to start without forcing you to design the whole app prematurely.

## Step 7: make sure the DB directory exists

Your config loader creates the config directory, but the DB path may still need its parent directory created before opening SQLite. Fix that in `run.go` or `store.Open`.

Add this to `store.Open` before `sql.Open`:

```go
if err := os.MkdirAll(filepath.Dir(dsn), 0o755); err != nil {
	return nil, fmt.Errorf("create db dir: %w", err)
}
```

And add `os` to imports.

That avoids a dumb first-run failure.

## Step 8: run it

```zsh
go run ./cmd/vtwatch run
```

What you want:
- no panic
- config file created
- DB file created
- migration applied
- “vtwatch booted successfully” printed

At that point, Phase 1 is real.

## Step 9: add a minimal README

Create `README.md` with:

```md
# vtwatch

Single-user VT registration watcher and self-hostable daemon.

## Current status

Phase 1 complete:
- Go module
- Cobra CLI
- config loading
- SQLite boot and migration

## Run

```sh
go run ./cmd/vtwatch run
```
```

## Step 10: initialize git

```zsh
git init
git add .
git commit -m "phase 1: bootstrap cobra cli, config, and sqlite"
```

## What not to do yet

Do not start Phase 2 by adding all of these at once:
- `fose`
- `studentdata`
- bookmarklet import
- scheduler loop
- web UI
- session keepalive
- swap logic

That would be you getting impatient and making the codebase sloppy.

Phase 1 ends when the project boots cleanly and has stable config + persistence plumbing.

## My actual recommendation

Start by getting exactly these three commands working:

```zsh
go run ./cmd/vtwatch
go run ./cmd/vtwatch run
go test ./...
```

If those are clean, then move to Phase 2.

If you want, the next step is for me to give you the **exact Phase 1 file tree with finished code** in one pasteable block so you can drop it straight into the repo.

---

# you asked

you need to explain everything at a high level - what is going on, how it all connects, the explanation behind the directory structure and various files, etc

---

# chatgpt response

Absolutely. Let’s step back and make the whole thing coherent.

Right now, the main risk is not “you do not know enough Go.” The main risk is that you start creating files and packages without a clean mental model of what the program actually is.

So here is the high-level picture.

---

# What this project actually is

This project is **not** a website, and it is **not** a SaaS backend.

It is best thought of as a **single-user automation daemon**.

That means:

- one user runs it
- it stores that one user’s credentials and watches
- it runs in the background
- it periodically checks course availability
- when conditions are met, it attempts the VT registration flow on that user’s behalf
- it records what happened
- it exposes control through a CLI, and maybe later a small web UI

That model fits the API you documented because VT’s flow is fundamentally user-specific. `studentdata` returns one student’s identity, current registrations, cart, time tickets, and other registration state, so the tool naturally centers around **one authenticated session at a time**. 

So the app is basically:

> “A long-running personal registration assistant with a CLI front end and a local database.”

That is the core idea.

---

# The main moving pieces

There are five important concepts in the system.

## 1. The user session
This is the credential state the app needs in order to do anything authenticated.

Your reference shows that the important pieces are:

- `authtoken`
- `_pers_id`
- `_pers_id_proof`

Those come from the logged-in VT browser session and are needed for `sisproxy` and `shockabsorber` operations. 

So conceptually, the tool needs a place to store:

- the credentials
- when they were captured
- whether they still seem valid
- what registration windows are available

That is why there is a **session** concept in the architecture.

---

## 2. The watch
A watch is just a user-defined rule like:

- “watch CRN 60058 in term 202606”
- or “if 60058 opens, add it”
- or “if 60058 opens, swap out 60900 for it”

A watch is not an HTTP request.  
It is a persistent object in your app.

That object needs to store:
- term
- target CRN
- optional drop CRN
- whether it is active
- what the last seen seat status was
- when to check next

That is why there is a **watch manager** and a `watches` table.

---

## 3. The VT protocol layer
This is the code that knows how to speak VT’s API correctly.

That includes:
- calling `fose`
- calling `sisproxy`
- calling `shockabsorber`
- stripping JSONP
- parsing weird pipe-delimited response formats
- constructing `time_ticket`
- building the correct `url_replay`
- handling the double-encoding for swaps 

This is a big deal because this project is **not just business logic**. A lot of the difficulty is protocol correctness.

That is why we isolate VT-specific behavior into its own package layer.

---

## 4. The registrar
This is the orchestration layer that performs a real registration attempt.

It is higher-level than the VT client layer.

The VT client layer knows how to send requests.  
The registrar knows **when** and **why** to send them and in what order.

For example, the registrar knows the correct sequence is:

1. check `studentdata`
2. run `preflight`
3. `cart_add`
4. `shockabsorber register`
5. poll `status`
6. confirm final state with `studentdata` 

And for swaps, it knows there is a different path:
- stage cart with `crn_drop`
- build replay URL with both CRNs
- classify partial outcomes if add succeeds but drop fails

That is why the registrar deserves its own package. It is the behavioral brain of the app.

---

## 5. The daemon loop
This is the thing that makes the app “alive.”

Without it, you just have a collection of commands and HTTP helpers.

The daemon loop:
- loads the active watches
- figures out which ones are due to be checked
- polls `fose`
- decides whether a registration attempt should start
- applies rate limiting
- records outcomes
- sleeps and repeats

That is the “worker” concept, except now it is local and single-user rather than some cloud background worker.

So the full program is basically:

```text id="x4u1c0"
config + database + credentials
           ↓
      daemon loop
           ↓
      watch scheduler
           ↓
    VT protocol calls
           ↓
 registration attempts
           ↓
   confirmation + storage
```

---

# Why the directory structure exists

Now let’s connect that to the repo structure.

The point of the directory structure is **not** to look professional.

The point is to separate different kinds of code so you do not end up with one giant pile of functions.

The structure we landed on is:

```text id="g4djlwm"
vtwatch/
  go.mod
  go.sum
  README.md

  cmd/
    vtwatch/
      main.go

  internal/
    cli/
    config/
    store/
    vt/
    session/
    watch/
    register/
    web/
    notify/

  migrations/
  deploy/
  scripts/
  testdata/
```

Here is what each part means.

---

## `go.mod` and `go.sum`

These define the Go module and dependency lock data.

High level:
- `go.mod` says “this repo is a Go module, here are its dependencies”
- `go.sum` records checksums for reproducible dependency resolution

You need them because Go tooling is built around modules.

---

## `cmd/vtwatch/main.go`

This is the **entrypoint** for the executable.

Its job should be tiny.

It should do almost nothing except:
- create the root context
- call the CLI package
- exit with the right status code

Why?

Because `main.go` is not the place for application logic.  
It is just the door into the program.

If you put real logic in `main.go`, the codebase gets messy fast.

So this file is basically:

> “start the app”

and nothing more.

---

## `internal/cli/`

This is where Cobra command wiring lives.

This package is for:
- defining commands
- parsing flags
- help text
- mapping CLI commands to application behavior

Examples:
- `vtwatch run`
- `vtwatch session import`
- `vtwatch watch add`
- `vtwatch doctor`

This package should **not** contain VT API logic or business rules.

Its job is translation:

> “The user typed this command, so call this service function.”

That separation matters because later you may also have a web UI. Both the CLI and the web UI should call the same underlying services, not duplicate logic.

---

## `internal/config/`

This package is for configuration.

High level responsibilities:
- decide where the config file lives
- load config from disk
- supply defaults
- validate config

Examples of config:
- SQLite database path
- polling intervals
- maybe whether the web UI is enabled
- maybe host/port for self-hosted mode

Why separate this?

Because config is cross-cutting. Lots of packages need settings, but none of them should own how config files are loaded.

---

## `internal/store/`

This is the persistence layer.

It is responsible for:
- opening SQLite
- applying migrations
- storing sessions
- storing watches
- storing attempts

Think of this package as:

> “everything related to durable local state”

The rest of the program should not need to know SQL details.  
It should ask the store package for higher-level operations.

This is important because the daemon needs memory across restarts:
- what watches existed
- what credentials were stored
- what the last known state was
- what attempts happened recently

That is why SQLite is here. One user, one app instance, one local DB. That is the right level of complexity.

---

## `internal/vt/`

This is the VT protocol package.

It exists because the hardest part of the project is not generic app structure. It is the weirdness of the VT API.

This package is responsible for:
- endpoint-specific request builders
- response parsing
- JSONP stripping
- time ticket construction
- replay URL building
- cart line parsing
- studentdata parsing
- shockabsorber response parsing

This package is where the raw API knowledge from your technical reference becomes code. 

This is also why I would keep this package fairly dense. It has the most domain-specific complexity.

You can think of it as:

> “everything needed to correctly talk to VT”

---

## `internal/session/`

This package is for session lifecycle management.

This is higher-level than `store/` and higher-level than `vt/`.

It is responsible for:
- storing and retrieving current credentials
- validating them
- maybe refreshing or re-checking them through `studentdata`
- exposing session health to the rest of the app

Why is this separate?

Because session handling is now clearly its own subsystem:
- it has security concerns
- it has expiry concerns
- it has registration window readiness concerns
- it interacts with both storage and VT APIs

So this package sits between those layers.

---

## `internal/watch/`

This package manages watches and scheduling.

Responsibilities:
- create/remove/pause watches
- compute when a watch should be checked again
- poll public course availability
- detect state transitions like `F -> A`
- queue registration attempts when appropriate

This package should not know how to build `time_ticket`.  
It should not know how to parse `shockabsorber` status.  
That belongs elsewhere.

Its job is:

> “track user intent over time and decide when to try something”

---

## `internal/register/`

This is where the real registration workflows live.

This package is responsible for:
- single add attempts
- swap attempts
- confirmation logic
- final outcome classification
- safe retry policy
- destructive-action guards

This package consumes:
- VT protocol helpers
- session state
- store state

And produces:
- attempt results
- final classification
- updated watch state

This package is where the app’s caution lives.

And it needs to be cautious because your real testing showed that a swap can be partially applied:
- add succeeds
- drop fails
- global error also exists

So the register package cannot just return `true` or `false`. It needs to reconcile real final state. That is one of the most important architectural conclusions from your testing.

---

## `internal/web/`

This is optional at first, but useful if you self-host on a VPS.

Its job is to expose a tiny web interface for:
- session health
- watch list
- recent attempts
- maybe adding/removing watches

It should be thin.

It should not contain business logic.  
It should call the same services the CLI calls.

You should think of it as:

> “an admin dashboard for one user”

not “the main application.”

---

## `internal/notify/`

This package is for notifications.

Examples:
- print to console
- maybe desktop notification
- maybe webhook or email later

This is not essential to Phase 1, but it is a real concern eventually, so it has a natural home.

---

## `migrations/`

This directory holds SQL schema migrations.

High level purpose:
- version your database schema
- avoid embedding huge SQL blobs in random files
- make database evolution explicit

Even if you only have one migration at first, this is still useful because it gives you a predictable place for schema changes.

---

## `deploy/`

This is for deployment artifacts.

Examples:
- Dockerfile
- docker-compose.yml
- systemd unit
- launchd plist later if you want

Why separate this?

Because deployment is real project content, but it is not application code.

This directory answers:
> “How do I run this thing as a daemon in the real world?”

---

## `scripts/`

For one-off helper scripts:
- dev bootstrap
- install service
- clean DB
- run test fixtures

These are supportive tools, not core app code.

---

## `testdata/`

This is extremely important for this project.

Because the VT API has:
- weird response wrappers
- weird cart formats
- weird swap behavior
- partially successful outcomes

you want real fixture files captured from testing, with secrets redacted.

This directory exists so you can write deterministic tests against real-looking data instead of hitting the network constantly.

That is not optional for a project like this. It is how you avoid breaking parsing and replay logic later.

---

# How the pieces connect

Now let’s connect the packages as a system.

## The CLI path

When the user runs:

```bash id="3k7h3t"
vtwatch run
```

the flow is:

1. `main.go` starts
2. `internal/cli` parses the command
3. `internal/config` loads config
4. `internal/store` opens SQLite and runs migrations
5. `internal/session` loads/validates session state
6. `internal/watch` loads active watches
7. daemon loop starts
8. `internal/vt/fose` is called to poll seat availability
9. if needed, `internal/register` runs a registration attempt
10. `internal/store` records what happened

That is the full internal chain.

## The registration path

When a watch triggers:

1. watch layer sees seat open
2. register layer asks session layer for current credentials
3. register layer asks VT layer for:
   - `studentdata`
   - `preflight`
   - `cart_add`
   - `register`
   - `status`
4. register layer reconciles final state using fresh `studentdata`
5. store layer saves the attempt and updated state

That is why the layers exist. They break the process into understandable responsibilities.

---

# Why Phase 1 is narrow

Phase 1 is **not** about VT yet.

That may feel boring, but it is correct.

Phase 1 exists to build the chassis:
- executable
- config
- DB
- migration
- command wiring

Why do this first?

Because later packages need stable ground to stand on.

If you skip this and jump straight into `studentdata` or `shockabsorber`, you will end up rewriting everything once you need:
- stored credentials
- persistent watches
- attempt history
- config-driven file paths
- long-running execution

So Phase 1 is not fluff. It is the skeleton the real logic will attach to.

---

# What the first files are trying to accomplish

At a high level, the first few files do this:

## `main.go`
Just starts the app.

## `cli/root.go`
Defines the root command and connects subcommands.

## `cli/run.go`
Implements the first real “boot the app” path.

## `config/config.go`
Answers:
- where is config stored?
- where is the DB stored?
- what are the defaults?

## `store/sqlite.go`
Answers:
- can we actually open the DB?

## `store/migrate.go`
Answers:
- can we initialize schema safely on startup?

## `migrations/001_init.sql`
Defines the first durable state model:
- sessions
- watches
- attempts

That is the basic boot path.

---

# The reason Cobra is used

You asked about Cobra earlier, so here is the high-level reason again.

This app naturally wants commands like:
- `run`
- `session import`
- `watch add`
- `watch swap`
- `doctor`

That is not a one-flag toy CLI. It is a command tree.

Cobra is useful because it gives you:
- command hierarchy
- flags
- help text
- tab completion later
- consistent structure

But the important point is:

> Cobra is just the control surface.

It is not the app itself.

The app logic should remain in your internal packages.

---

# The reason SQLite is used

Because this is not multi-user infrastructure.

You need a local store for:
- credentials
- watches
- attempts
- maybe cached session metadata

SQLite is enough because:
- one process
- one user
- low concurrency
- minimal ops burden
- portable between local and VPS

Using Postgres here would just be fake sophistication.

---

# The reason this is a daemon-first project

Because the whole point is background behavior.

If the user has to sit there and manually run commands every few seconds, the product is pointless.

So even though the CLI is how you control the app, the app itself is fundamentally:

> a long-running background process

That is why the structure centers around `run`, persistent state, and schedulers rather than a static request-response interface.

---

# The big conceptual correction from your testing

This is the one thing I most want you to internalize:

## The registration system is not neat
It is not cleanly atomic from your client’s point of view.

Your testing proved that:
- malformed swap replay becomes add-only
- correctly formed swap can still partially succeed

So the architecture should assume:
- mixed outcomes are normal
- final state must be reconciled
- shockabsorber responses are informative but not final truth
- `studentdata` is the final truth

That should affect how you think about almost every package.

---

# So what is Phase 1 really doing for you?

Phase 1 gives you:

- a real binary
- a predictable config path
- a persistent DB
- a migration system
- command structure
- a place for later work to live

In other words:

> it turns the project from “a bunch of notes and curl experiments” into “an actual application skeleton.”

That is why it matters.

---

# The final mental model

If I had to compress the whole project into one sentence, it would be:

> **vtwatch is a single-user daemon that stores a VT session, tracks watches, speaks VT’s registration protocol carefully, reconciles actual results with `studentdata`, and exposes control through a CLI and optional small web UI.**

That is the system you are building.

And the directory structure is just a way of keeping those responsibilities separate enough that you can reason about them.

If you want, the next step is I walk through **Phase 1 file by file** and explain exactly what each file should contain and why, before you start writing it.

---

# you asked

yes

---

# chatgpt response

Good. I’ll walk through **Phase 1 file by file**, but I’m going to do it in the right order:

1. what Phase 1 is trying to accomplish  
2. what each file is for  
3. how the files connect  
4. what should and should not be inside each one

That is the part that usually gets skipped, and then people end up with a repo full of files they do not really understand.

---

# First: what Phase 1 actually is

Phase 1 is **not** “start implementing VT.”

Phase 1 is:

> **turn the project into a real application skeleton that can boot, load config, open persistence, and run a command cleanly.**

That means Phase 1 is about **application infrastructure**, not VT protocol logic yet.

By the end of Phase 1, you want to be able to run:

```bash
vtwatch run
```

and have the program:

- start successfully
- load a config file
- open SQLite
- apply its initial schema
- print that it booted successfully

No `fose`.  
No `studentdata`.  
No `shockabsorber`.  
No watch scheduler yet.

That sounds boring, but it is correct. Until the app can boot cleanly and own its local state, all the VT code is just experiments taped together.

---

# The Phase 1 directory structure

This is the version I want you to understand:

```text
vtwatch/
  go.mod
  go.sum
  README.md

  cmd/
    vtwatch/
      main.go

  internal/
    cli/
      root.go
      run.go
    config/
      config.go
    store/
      sqlite.go
      migrate.go

  migrations/
    001_init.sql
```

This is intentionally small.

The structure is built around a simple principle:

- `cmd/` is the executable entrypoint
- `internal/` is the real app code
- `migrations/` is the database schema
- root files are module metadata and documentation

That is it.

---

# Root-level files

## `go.mod`

This declares the project as a Go module.

High-level purpose:
- gives the project an identity
- tells Go what dependencies it uses
- allows imports between your own packages
- makes builds reproducible

Conceptually, this file answers:

> “What is this codebase called, and what libraries does it depend on?”

Without `go.mod`, this is not really a modern Go project.

---

## `go.sum`

This is dependency checksum data.

You do not really “design” this file. Go manages it.

Its purpose is:
- lock dependency content integrity
- make installs and builds more reproducible

Mentally, treat it as:
- necessary project metadata
- not application logic

Do not overthink it.

---

## `README.md`

This should explain:
- what the project is
- what Phase 1 currently does
- how to run it
- what is not implemented yet

The reason this matters early is simple:

You already have a lot of API knowledge and testing notes. If the repo has no clean README, the project feels like a pile of experiments instead of an application.

So `README.md` is not fluff. It is the first layer of project coherence.

---

# `cmd/` and the executable entrypoint

## Why `cmd/` exists

In Go, `cmd/` is commonly used when a repo contains one or more runnable commands. The point is to separate “the executable entrypoint” from “the reusable internal code.”

That distinction matters.

You do **not** want the whole app living in `main.go`.

You want:
- a tiny executable entrypoint
- real logic elsewhere

That is why `cmd/vtwatch/` exists.

---

## `cmd/vtwatch/main.go`

This file should be tiny.

Its job is:

- create the app context
- call into the CLI layer
- exit cleanly if there is an error

Conceptually:

> `main.go` is the front door of the program, not the building itself.

What should be inside:
- `package main`
- imports
- `func main()`
- a call to the CLI package

What should **not** be inside:
- config loading logic
- database setup logic
- SQL
- VT API logic
- watch scheduling logic

The reason is simple: `main.go` should stay boring forever.

If `main.go` starts growing, it is a sign your architecture is leaking.

---

# `internal/`

`internal/` is where the app’s real code lives.

In Go, `internal` has a real meaning. Packages inside it are not meant to be imported from outside the module subtree. So it is a natural place for application-private code.

At a high level, `internal/` means:

> “this is the implementation of the app, not a public library API.”

For Phase 1, there are only three packages there:

- `cli`
- `config`
- `store`

That is enough.

---

# `internal/cli/`

This package exists because the user interacts with the app through commands.

The CLI layer is the **control surface** of the app.

It is responsible for:
- defining commands
- parsing flags
- mapping user intent to application behavior

It is **not** responsible for actually implementing the app’s domain logic.

Think of it as the translation layer between:
- “the user typed this”
and
- “the program should now do this”

That is why it gets its own package.

---

## `internal/cli/root.go`

This file defines the root command.

High-level purpose:
- define what `vtwatch` is
- attach subcommands
- expose one clean `Execute()` entrypoint to `main.go`

Conceptually, this file answers:

> “What is the command-line application itself?”

It is the root of the command tree.

What belongs here:
- root Cobra command definition
- short/long description
- subcommand registration
- `Execute()` function

What does **not** belong here:
- database code
- config file parsing
- VT logic
- business behavior

This file should just define the command hierarchy.

---

## `internal/cli/run.go`

This file defines the `run` command.

This is the first real command because `run` is the daemon boot path.

When the user runs:

```bash
vtwatch run
```

this file should be what gets triggered.

Conceptually, `run.go` answers:

> “What does it mean to start the app?”

In Phase 1, “start the app” means:
- load config
- open DB
- apply migrations
- confirm boot success

That is enough.

This file is important because it is the first place where the CLI starts invoking the actual app systems.

What belongs here:
- the `run` Cobra command
- flag definitions for `run`
- the first call chain into config + store

What does **not** belong here:
- raw SQL
- VT request builders
- schedule loop implementation
- watch logic

This file should orchestrate, not implement low-level details.

So mentally:

- `root.go` says the app exists
- `run.go` says what happens when you start it

---

# `internal/config/`

This package exists because configuration is a distinct concern.

The app needs to know things like:
- where the config file lives
- where the SQLite database should live
- what defaults to use if the user has not configured anything yet

That is not CLI logic.
That is not store logic.
That is not VT logic.

So it gets its own package.

---

## `internal/config/config.go`

This file should do all of Phase 1 config work.

Conceptually, it answers:

> “How does the app discover and load its runtime settings?”

In Phase 1, keep it very simple.

Likely responsibilities:
- define a `Config` struct
- choose a default config path
- choose a default data directory
- load config from JSON or similar
- create a default config file if missing

What belongs here:
- config struct definitions
- file path defaults
- config file parsing
- default generation

What does **not** belong here:
- environment-specific hacks everywhere
- VT session data
- watch definitions
- DB access
- Cobra flag handling

This package should be boring and deterministic.

The important architectural reason it exists is that **many other packages will depend on configuration**, but none of them should have to know how the config file itself is loaded.

So:

- CLI asks config package to load config
- config package returns a typed struct
- the rest of the app uses that struct

That is the clean boundary.

---

# `internal/store/`

This package is the persistence layer.

The app needs durable state for:
- credentials later
- watches later
- attempts later

Phase 1 does not implement all that behavior yet, but it should establish the persistence foundation.

This package exists so the rest of the app does not need to know:
- how SQLite is opened
- where schema setup happens
- how migrations are applied

Conceptually, this package answers:

> “How does the application own durable local state?”

---

## `internal/store/sqlite.go`

This file is responsible for opening the SQLite database connection.

That sounds trivial, but it is not just one line.

It should handle:
- path validation
- directory creation if needed
- opening the DB
- pinging it to ensure it actually works

At a high level, this file answers:

> “Can the app reliably talk to its database?”

This should be separate from migration logic because:
- opening a DB and
- defining its schema

are related, but not the same responsibility.

What belongs here:
- open connection function
- driver import
- connection sanity checking

What does **not** belong here:
- schema SQL
- insert/update business logic
- session storage logic yet
- watch repository logic yet

In Phase 1, this file is just “database boot plumbing.”

---

## `internal/store/migrate.go`

This file is responsible for applying migrations.

Conceptually, it answers:

> “Now that the database is open, does it have the schema we expect?”

This is a separate concern from opening the DB.

Why separate it?
Because one day you may:
- change migration behavior
- add migration tracking
- support multiple migration files
- add transactional migration handling

And you do not want all of that mixed into DB-open code.

In Phase 1, this file can be very small:
- read `migrations/001_init.sql`
- execute it

That is fine.

What belongs here:
- migration file reading
- migration execution

What does **not** belong here:
- application data inserts
- command-line logic
- VT parsing
- watch scheduling

This file is strictly about schema bootstrapping.

---

# `migrations/`

This directory is where the SQL schema lives.

The reason it is a separate directory is important:

> your database schema is part of the application, but it is not Go code

Schema changes are their own kind of change.  
They deserve their own place.

This becomes especially useful later when:
- you add fields to watches
- you add attempt outcome storage
- you add session metadata
- you refine the schema after learning more from testing

---

## `migrations/001_init.sql`

This is the initial schema.

In Phase 1, it should create the minimum tables you know you will need:
- `sessions`
- `watches`
- `attempts`
- maybe a small migration tracking table

Why these tables so early?

Because they represent the core entities the app will eventually manage:

### `sessions`
for credential and session state

### `watches`
for persistent watch definitions

### `attempts`
for a durable history of what the app tried and what happened

Even if Phase 1 does not fully use those tables yet, defining them early gives the rest of the architecture a clear center of gravity.

That is important. You want the program skeleton to already reflect the real concepts of the app.

---

# How these files connect at runtime

This is the most important part.

When you run:

```bash
vtwatch run
```

here is what should happen conceptually:

## Step 1: `main.go`
The operating system launches the program.

`main.go` starts and says:
- create context
- hand control to the CLI package

## Step 2: `cli/root.go`
The CLI layer identifies that the user invoked the `run` subcommand.

It hands off to `run.go`.

## Step 3: `cli/run.go`
The `run` command says:
- load configuration
- open the DB
- migrate the DB
- report success

## Step 4: `config/config.go`
The config package decides:
- where config lives
- whether a config file already exists
- whether to generate defaults
- what the database path is

Then it returns a typed config object.

## Step 5: `store/sqlite.go`
The store layer opens the SQLite database file.

## Step 6: `store/migrate.go`
The store layer applies the initial schema.

## Step 7: control returns to `run.go`
The app prints that it booted successfully.

That is the entire Phase 1 boot path.

Notice what is **not** happening yet:
- no session validation
- no `studentdata`
- no polling
- no registration
- no watch processing

That is intentional.

---

# Why this is the correct first slice

Because it gives you a stable place to put later work.

Once Phase 1 is done, Phase 2 can safely say:
- load session data from the DB
- store credentials
- add VT client code
- read/write structured app state

Without Phase 1, all of that would be floating.

So the deeper reason behind this structure is:

> build the shell of the application first, so later protocol code has a real home.

---

# What should not exist yet

This matters just as much.

At the end of Phase 1, you should **not** have:

- `internal/vt/`
- `internal/register/`
- `internal/watch/`
- `internal/web/`
- bookmarklet import logic
- watch polling loop
- `studentdata` parsing
- `shockabsorber` types
- full repository abstractions everywhere

Why?

Because that would be architecture based on impatience instead of need.

You already know the project will eventually need those packages. That does **not** mean you need to create all of them before the app even boots.

A good Go project grows in layers:
- boot
- state
- protocol
- workflows
- scheduler
- UI

That is the right order.

---

# The deeper point behind the structure

If I strip away all the file names, the Phase 1 structure is really expressing this separation:

## User interface layer
`cmd/` and `internal/cli/`

This is how a human tells the program what to do.

## Configuration layer
`internal/config/`

This is how the program knows where things are and how it should run.

## Persistence layer
`internal/store/` and `migrations/`

This is how the program remembers things over time.

That is the whole foundation.

Later, you will add:

## External protocol layer
`internal/vt/`

How the program talks to VT.

## Workflow layer
`internal/register/`, `internal/watch/`, `internal/session/`

How the program turns low-level protocol calls into useful behavior.

That layering is the real reason the directory structure makes sense.

---

# So what should you take away from this?

Here is the clean mental model:

## `main.go`
Starts the executable.

## `cli/`
Defines commands and routes them into the app.

## `config/`
Loads runtime settings and defaults.

## `store/`
Owns the database connection and schema setup.

## `migrations/`
Defines durable app state.

And together, Phase 1 answers:

> “Can vtwatch start as a real program with persistent local state?”

That is the whole point.

---

# The best way to approach writing it

Do not try to write all files “from memory.”

Build it in this order:

1. `go.mod`
2. `cmd/vtwatch/main.go`
3. `internal/cli/root.go`
4. `internal/cli/run.go`
5. `internal/config/config.go`
6. `internal/store/sqlite.go`
7. `internal/store/migrate.go`
8. `migrations/001_init.sql`
9. `README.md`

At each step, ask:
- what responsibility does this file own?
- what should it *not* own?

That is how you avoid architecture drift.

---

# Final summary

Phase 1 is the app skeleton.

The structure exists so responsibilities are separated clearly:

- **entrypoint** in `cmd/`
- **command control layer** in `internal/cli`
- **config loading** in `internal/config`
- **database bootstrapping** in `internal/store`
- **schema definition** in `migrations/`

Those files connect in a simple boot chain:

```text
main.go
  → cli
    → config
      → store open
        → migrations
          → app boot success
```

That is the first coherent version of the project.

Next, the right move is to go **file by file and draft the actual contents** for Phase 1 in a way that matches this architecture.

---

# you asked

yes - keep it simple and explain evrything and all design decisions

---

# chatgpt response

Good. We’ll keep Phase 1 small and deliberate.

The goal is **not** to start talking to VT yet. The goal is to create the smallest real application skeleton that matches the system you are actually building: a single-user daemon with persistent local state, a CLI entrypoint, and room for the VT protocol layer to be added later. That matches the architecture we settled on, where `studentdata` will eventually be the source of truth, `default` is the real cart, and registration will be a cautious multi-step flow rather than a single “do the thing” call. 

# What Phase 1 should accomplish

When you finish Phase 1, you should be able to run:

```zsh
go run ./cmd/vtwatch run
```

and the program should:

- start cleanly
- load a config file
- create a data directory
- open a SQLite database
- apply the initial schema
- print that it booted successfully

That is enough. No VT API code yet.

# The file structure

Use this:

```text
vtwatch/
  go.mod
  go.sum
  README.md

  cmd/
    vtwatch/
      main.go

  internal/
    cli/
      root.go
      run.go
    config/
      config.go
    store/
      sqlite.go
      migrate.go

  migrations/
    001_init.sql
```

## Why this structure

This structure is intentionally small.

- `cmd/vtwatch/main.go` is the executable entrypoint
- `internal/cli` is the command-line control surface
- `internal/config` loads runtime settings
- `internal/store` owns persistence bootstrapping
- `migrations` defines the DB schema

That is the minimum clean separation that still feels like a real app.

You do **not** need `internal/vt`, `internal/watch`, `internal/register`, `internal/session`, or `internal/web` yet. Those are later phases.

---

# Step 1: create the repo

From zsh on macOS:

```zsh
mkdir -p ~/code/vtwatch
cd ~/code/vtwatch

go mod init vtwatch

mkdir -p cmd/vtwatch internal/cli internal/config internal/store migrations
touch README.md
```

## Why `go mod init vtwatch`

You can absolutely use a GitHub module path later, but for local development `vtwatch` is fine and keeps imports simple.

You can change it later if needed. Right now, clarity beats ceremony.

---

# Step 2: add dependencies

We need:

- Cobra for the CLI
- SQLite driver for local persistence

```zsh
go get github.com/spf13/cobra@latest
go get modernc.org/sqlite
```

## Why Cobra

Because the app naturally wants commands like:

- `vtwatch run`
- `vtwatch session import`
- `vtwatch watch add`
- `vtwatch doctor`

That is a command tree, not a one-flag utility.

## Why `modernc.org/sqlite`

Because it avoids CGO, which means fewer build headaches on Apple Silicon while you are getting started.

You are not making a performance-critical database engine. You just need local SQLite that works cleanly.

---

# Step 3: write the files

## `cmd/vtwatch/main.go`

```go
package main

import (
	"context"
	"log"
	"os"

	"vtwatch/internal/cli"
)

func main() {
	ctx := context.Background()

	if err := cli.Execute(ctx); err != nil {
		log.Printf("vtwatch: %v\n", err)
		os.Exit(1)
	}
}
```

## What this file does

This is the executable entrypoint.

It does exactly three things:

1. creates a root context
2. hands control to the CLI layer
3. exits with a non-zero code on error

## Why it is tiny

Because `main.go` should never become the place where your app logic lives.

If `main.go` starts loading config, opening the DB, talking to VT, or doing scheduling, you are already losing the architecture.

---

## `internal/cli/root.go`

```go
package cli

import (
	"context"

	"github.com/spf13/cobra"
)

func Execute(ctx context.Context) error {
	return newRootCmd().ExecuteContext(ctx)
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vtwatch",
		Short: "Single-user VT registration watcher",
		Long:  "vtwatch is a single-user daemon and CLI for monitoring seats and attempting registration.",
	}

	cmd.AddCommand(newRunCmd())

	return cmd
}
```

## What this file does

This defines the root command.

Its job is:

- define what `vtwatch` is
- attach subcommands
- expose one clean `Execute()` function to `main.go`

## Why `Execute` lives here

Because `main.go` should not know about Cobra internals. It should just say “CLI package, run the app.”

That keeps the executable boundary clean.

---

## `internal/cli/run.go`

```go
package cli

import (
	"fmt"

	"vtwatch/internal/config"
	"vtwatch/internal/store"

	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Start vtwatch",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			db, err := store.Open(cmd.Context(), cfg.DatabasePath)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer db.Close()

			if err := store.Migrate(cmd.Context(), db); err != nil {
				return fmt.Errorf("migrate database: %w", err)
			}

			fmt.Println("vtwatch booted successfully")
			fmt.Printf("config: %s\n", cfg.ConfigPath)
			fmt.Printf("database: %s\n", cfg.DatabasePath)

			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", "Path to config file")
	return cmd
}
```

## What this file does

This file defines the `run` command, which is the first real boot path.

When you run:

```zsh
vtwatch run
```

this file is what gets executed.

## Why this command exists in Phase 1

Because the app is fundamentally a daemon-first tool. `run` is the future place where the scheduler and registration loop will live.

Right now it only boots config and storage, but that is exactly what it should do at this stage.

## Why the command only orchestrates

Notice what it does **not** do:

- no SQL here
- no config parsing here
- no VT requests here

It just connects the pieces:

1. load config
2. open DB
3. migrate DB

That is the right role for CLI code.

---

## `internal/config/config.go`

```go
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	ConfigPath   string `json:"-"`
	DatabasePath string `json:"database_path"`
}

func Load(path string) (Config, error) {
	if path == "" {
		path = defaultConfigPath()
	}

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		cfg := defaultConfig(path)
		if err := writeDefaultConfig(path, cfg); err != nil {
			return Config{}, err
		}
		return cfg, nil
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	cfg.ConfigPath = path

	if cfg.DatabasePath == "" {
		cfg.DatabasePath = defaultDatabasePath()
	}

	return cfg, nil
}

func defaultConfig(path string) Config {
	return Config{
		ConfigPath:   path,
		DatabasePath: defaultDatabasePath(),
	}
}

func defaultConfigPath() string {
	return filepath.Join(defaultDataDir(), "config.json")
}

func defaultDatabasePath() string {
	return filepath.Join(defaultDataDir(), "vtwatch.db")
}

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}

	return filepath.Join(home, ".config", "vtwatch")
}

func writeDefaultConfig(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	contents, err := json.MarshalIndent(struct {
		DatabasePath string `json:"database_path"`
	}{
		DatabasePath: cfg.DatabasePath,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, contents, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}
```

## What this file does

This is the config package.

Its responsibilities are:

- decide where the config file lives by default
- load the config file if it exists
- create a default config file if it does not
- return a typed `Config` struct

## Why the config is JSON

Because Phase 1 should be simple.

JSON is fine here:
- easy to read
- easy to write
- stdlib support
- no extra dependency

No Viper. No YAML. No environment-variable framework.

## Why config owns the default paths

Because path decisions are configuration decisions, not CLI decisions and not database decisions.

Later, more parts of the app will need settings. This package is the one place that should know how to load them.

## Why config writes a default file automatically

Because it improves first-run UX a lot.

Instead of failing with “missing config,” the app creates a usable default. That is the kind of boring convenience you want early.

---

## `internal/store/sqlite.go`

```go
package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

func Open(ctx context.Context, dbPath string) (*sql.DB, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("database path is empty")
	}

	dbPath = filepath.Clean(dbPath)

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create database dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("sql open: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return db, nil
}
```

## What this file does

This file opens the SQLite database.

Its responsibilities are:

- validate the DB path
- ensure the parent directory exists
- open the DB connection
- verify it actually works with `PingContext`

## Why this is separate from migrations

Because:

- opening a DB
- and applying schema

are two separate responsibilities.

One is about connectivity.  
The other is about structure.

Keeping them separate now will save you pain later.

## Why use `database/sql`

Because you are not doing anything fancy yet. You just need the standard DB interface and one driver.

Simple beats clever.

---

## `internal/store/migrate.go`

```go
package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
)

func Migrate(ctx context.Context, db *sql.DB) error {
	sqlBytes, err := os.ReadFile("migrations/001_init.sql")
	if err != nil {
		return fmt.Errorf("read migration file: %w", err)
	}

	if _, err := db.ExecContext(ctx, string(sqlBytes)); err != nil {
		return fmt.Errorf("apply migration: %w", err)
	}

	return nil
}
```

## What this file does

This file applies the initial schema.

That is all.

## Why it is this simple for now

Because Phase 1 does not need a migration framework yet.

You do not need:
- migration versioning
- rollback support
- multiple migration files
- checksums
- a migration library

You need one schema file to run on boot. That is enough.

## Why this still matters

Because this is what turns the DB from “a file” into “the app’s persistent state.”

Without migration logic, you are still just opening an empty container.

---

## `migrations/001_init.sql`

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  authtoken TEXT NOT NULL,
  pers_id TEXT NOT NULL,
  pers_id_proof TEXT NOT NULL,
  captured_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_validated_at DATETIME,
  status TEXT NOT NULL DEFAULT 'unknown'
);

CREATE TABLE IF NOT EXISTS watches (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  term TEXT NOT NULL,
  mode TEXT NOT NULL,
  add_crn TEXT NOT NULL,
  drop_crn TEXT,
  active INTEGER NOT NULL DEFAULT 1,
  last_seen_stat TEXT,
  next_poll_at DATETIME,
  last_attempt_at DATETIME,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS attempts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  watch_id INTEGER NOT NULL,
  phase TEXT NOT NULL,
  outcome_kind TEXT NOT NULL,
  raw_response TEXT,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (watch_id) REFERENCES watches(id)
);
```

## What this file does

This creates the initial schema.

## Why these tables already exist in Phase 1

Because they reflect the real core concepts of the app:

- `sessions` for credential/session state
- `watches` for persistent user intent
- `attempts` for history of what the app tried

Even though Phase 1 does not fully use them yet, creating them now gives the rest of the project a clean data model to build around.

That is better than pretending persistence is generic and figuring it out later.

## Why `attempts` already has `phase`, `outcome_kind`, and `raw_response`

Because your testing already proved registration outcomes are not simple booleans. You can get mixed success/error states from shockabsorber, especially with swaps, and you already know the app will need durable attempt history and reconciliation. 

So the initial schema should already respect that reality.

---

## `README.md`

Start simple:

```md
# vtwatch

vtwatch is a single-user VT registration watcher and daemon.

## Phase 1

Current status:
- Go module initialized
- Cobra CLI bootstrapped
- config loading implemented
- SQLite boot and initial migration implemented

## Run

```sh
go run ./cmd/vtwatch run
```
```

## Why this file matters now

Because it forces the project to have a clear identity.

That sounds trivial, but it is not. You already have a lot of protocol knowledge and testing notes. The README is where the repo stops feeling like scratch work and starts feeling like an actual application.

---

# How all the files connect

Now the important part: how this actually works as a system.

When you run:

```zsh
go run ./cmd/vtwatch run
```

this is the flow:

## 1. `main.go`
The executable starts and hands control to the CLI package.

## 2. `cli/root.go`
Cobra recognizes the root command and sees that you invoked the `run` subcommand.

## 3. `cli/run.go`
The `run` command starts the app boot sequence:
- load config
- open DB
- migrate schema
- print success

## 4. `config/config.go`
The config package decides where the config file should live, creates it if missing, and returns the `Config` struct.

## 5. `store/sqlite.go`
The store package ensures the DB directory exists, opens SQLite, and verifies the connection works.

## 6. `store/migrate.go`
The store package reads `migrations/001_init.sql` and applies the schema.

## 7. `run.go`
The command prints that boot succeeded.

That is the whole app for Phase 1.

No scheduler yet. No VT API yet. Just a clean boot path.

---

# Why this is the right first slice

Because it gives later phases a stable place to live.

When you get to Phase 2 and beyond, you will need somewhere to put:
- stored credentials
- watch definitions
- attempt history
- config-driven behavior
- long-running execution

If you skip this and jump straight into VT protocol code, you will wind up rewriting everything once the app needs persistence and real boot behavior.

So Phase 1 is the chassis.

---

# What you should do next in the terminal

Once the files exist, run:

```zsh
go run ./cmd/vtwatch run
```

You should expect:
- a config file created under `~/.config/vtwatch/config.json`
- a DB file created under `~/.config/vtwatch/vtwatch.db`
- the migration applied
- boot success printed

Then verify the files exist:

```zsh
ls -la ~/.config/vtwatch
```

You should see something like:
- `config.json`
- `vtwatch.db`

---

# What not to add yet

Do not add these yet:

- `internal/vt`
- `internal/register`
- `internal/watch`
- `internal/session`
- `internal/web`
- bookmarklet logic
- actual SQL repository functions
- `doctor` command
- `session import` command

You know they are coming, but creating them now just creates noise.

Phase 1 should stay small enough that you understand every file completely.

---

# The design decisions in one sentence each

## Why one binary
Because this is one single-user daemon app, not a multi-process distributed system.

## Why Cobra
Because the app naturally wants a command tree.

## Why SQLite
Because this is local or self-hosted single-user state, not multi-tenant infrastructure.

## Why config package
Because path and runtime settings are a distinct concern.

## Why store package
Because persistence boot logic should not be mixed into commands.

## Why migrations directory
Because schema is part of the app, but it is not Go code.

## Why no VT code yet
Because the app needs a clean boot and persistence skeleton before protocol logic has a sane place to live.

---

# The exact order I’d build it in

1. `go.mod`
2. `cmd/vtwatch/main.go`
3. `internal/cli/root.go`
4. `internal/cli/run.go`
5. `internal/config/config.go`
6. `internal/store/sqlite.go`
7. `internal/store/migrate.go`
8. `migrations/001_init.sql`
9. `README.md`

That order is deliberate:
- entrypoint first
- command surface second
- config third
- persistence fourth
- schema last

---

# Bottom line

Phase 1 is about building the smallest correct shell of the application.

The directory structure reflects real responsibilities:

- `main.go` starts the app
- `cli/` defines how the user invokes it
- `config/` decides where things live and how settings are loaded
- `store/` opens and initializes persistence
- `migrations/` defines the durable state model

That is all Phase 1 needs to do.

The next step is to actually write these files and get `go run ./cmd/vtwatch run` working cleanly.

---

# chatgpt response

Good. The Cobra guide you uploaded is useful, but we should **not** copy it literally. It is showing the common Cobra pattern:

- bare `main.go`
- a `cmd/` package that defines commands
- one file per command 

That part is right.

But for **this** project, I still want the executable entrypoint in `cmd/vtwatch/main.go` and the Cobra code in `internal/cli/`, not a top-level `cmd` package at the repo root. The reason is simple: this is not just “a CLI app.” It is a real application with config, storage, later VT protocol code, and a daemon loop. So I want the command surface separated from the rest of the app code from day one.

So the final Phase 1 shape stays:

```text id="pzmf2w"
vtwatch/
  go.mod
  go.sum
  README.md

  cmd/
    vtwatch/
      main.go

  internal/
    cli/
      root.go
      run.go
    config/
      config.go
    store/
      sqlite.go
      migrate.go

  migrations/
    001_init.sql
```

The Cobra guide supports the general idea that:
- `main.go` should be tiny
- commands should be separated
- `RunE` is the right pattern when command execution can fail 

We are just organizing it slightly differently because your project is larger than a toy CLI.

---

# The big picture of Phase 1

Phase 1 is the minimum skeleton of the application.

It is doing three things:

## 1. Giving the app a front door
That is the CLI.

The user needs some way to say:
- start the app
- later import credentials
- later add a watch
- later run diagnostics

That front door is Cobra.

## 2. Giving the app a memory
That is SQLite.

This app needs durable state because it is not a stateless command. It will eventually need to remember:
- the current VT session
- the user’s watches
- attempt history
- last known watch state

So even before VT integration exists, the storage layer belongs in the skeleton.

## 3. Giving the app a place to live
That is config plus the filesystem layout.

The app needs a stable place for:
- config file
- SQLite file

That is Phase 1.

So another way to say it is:

> Phase 1 creates the shell of a daemon app before we add VT-specific behavior.

---

# Why the repo is organized this way

Let’s go through the structure at a high level first, then file by file.

## `cmd/`
This is for **runnable entrypoints**.

Right now there is only one:
- `vtwatch`

That is why we have:

```text id="jno3fr"
cmd/vtwatch/main.go
```

This is the executable the user actually runs.

## `internal/`
This is for the app’s implementation.

We are using `internal` because this is app-private code, not a reusable library. That is a clean fit for Go.

Inside `internal/`, each package represents a distinct responsibility:

- `cli` = command definitions
- `config` = runtime settings
- `store` = persistence bootstrapping

## `migrations/`
This is where SQL schema lives.

It is separate because schema is part of the app, but not Go code.

---

# File by file

Now the important part.

## `go.mod`

This file tells Go:

- this project is a module
- here is its module path
- here are its dependencies

For Phase 1, the important dependencies are:
- Cobra
- SQLite driver

### Why it matters
Without `go.mod`, the project is not a proper modern Go module. You would be fighting the toolchain immediately.

### Design decision
Keep the module simple. Do not overcomplicate the module path right now if you have not even booted the app yet.

---

## `go.sum`

This is dependency checksum metadata.

### Why it exists
It makes dependency resolution reproducible and safer.

### Design decision
Do not think about this as an application file. It is Go tooling metadata.

---

## `README.md`

This file explains:
- what the project is
- what is implemented right now
- how to run it

### Why it matters so early
Because otherwise the repo just looks like experiments and random files.

### Design decision
Keep the README honest. At Phase 1, the README should say:
- the app boots
- config loads
- DB opens
- migrations run
- VT integration is not implemented yet

No hype.

---

## `cmd/vtwatch/main.go`

This file is the executable entrypoint.

Its only job is:
- start the process
- call the CLI layer
- exit properly if there is an error

### Why keep it tiny
Because `main.go` should never become your logic dump.

If you put config loading, DB boot, scheduler code, or VT logic in `main.go`, then every architectural boundary starts collapsing.

### Design decision
`main.go` should be boring forever.

That is a feature, not a limitation.

---

## `internal/cli/root.go`

This file defines the root Cobra command.

At a high level, it answers:

> “What is the `vtwatch` command-line application?”

It should define:
- `Use`
- a short description
- maybe a long description
- which subcommands exist

### Why this file exists separately
Because the root command is the command tree anchor. It is not the same thing as a specific action like `run`.

### Design decision
Keep root command setup isolated so the subcommand files stay focused.

This also lines up with the Cobra guide’s general command organization advice, even though we are putting commands in `internal/cli` instead of a top-level `cmd` package. 

---

## `internal/cli/run.go`

This file defines the `run` command.

This is the first real command because the app is daemon-first. The future core behavior of the app will ultimately happen under `vtwatch run`.

At Phase 1, `run` does only this:

1. load config
2. open SQLite
3. run migrations
4. print success

### Why `run` is the first command
Because this app is fundamentally meant to stay alive and do background work.

Even though Phase 1 does not yet implement the background loop, `run` is still the right first command because it represents the real lifecycle of the app.

### Why use `RunE`
Because booting the app can fail:
- config can fail to load
- DB can fail to open
- migration can fail

The Cobra guide explicitly recommends `RunE` when a command may return an error to the caller. That is exactly the case here. 

### Design decision
The `run` command should orchestrate app startup, not implement low-level details. So it calls config and store packages, rather than doing everything inline.

---

## `internal/config/config.go`

This file owns runtime settings.

For Phase 1, its responsibilities are:

- decide where the config file lives
- decide where the database file lives
- create a default config file if missing
- load and parse config
- return a typed `Config` struct

### Why config deserves its own package
Because “where files live” and “how settings are loaded” is its own problem.

It should not be mixed into:
- CLI code
- DB code
- VT code

### Why keep config simple
Because you do not need Viper or environment-variable plumbing yet.

The Cobra guide shows Viper integration because it is demonstrating capabilities, not because every project needs Viper on day one. 

For this app, that would be premature.

### Design decision
Use a plain JSON config file with sensible defaults. That is enough.

Also, store app files under a stable per-user directory. For macOS and simple local development, `~/.config/vtwatch` is fine.

---

## `internal/store/sqlite.go`

This file is responsible for opening the SQLite database.

Its responsibilities are:

- validate the DB path
- create the DB directory if needed
- open the connection
- ping it to make sure it is real

### Why separate opening the DB from migrations
Because:
- “can we connect?”
and
- “does schema exist?”
are different concerns.

That separation keeps the storage layer clean.

### Why SQLite
Because this is a single-user daemon, not multi-user infrastructure.

You need durable local state, not a networked DB.

### Design decision
Use a simple SQLite driver that avoids CGO pain during early development. That is why `modernc.org/sqlite` is the practical choice here.

---

## `internal/store/migrate.go`

This file is responsible for applying schema.

Its job is:
- read the SQL migration file
- execute it

### Why this is a separate file
Because schema initialization is not the same responsibility as opening a DB connection.

### Why it is simple in Phase 1
Because you only need one migration right now.

You do not need:
- a migration framework
- migration version bookkeeping logic
- rollback support

Not yet.

### Design decision
A minimal migration runner is enough until the schema starts changing.

---

## `migrations/001_init.sql`

This file defines the initial schema.

In Phase 1, it should create the tables that reflect the real future app concepts:

- `sessions`
- `watches`
- `attempts`

### Why these tables already exist
Because they are the actual persistent entities the app revolves around:

#### `sessions`
stores VT credentials and validation state

#### `watches`
stores the things the user wants monitored

#### `attempts`
stores what the app tried and what happened

Even though Phase 1 is not populating these tables fully yet, the schema should already reflect the app’s real mental model.

That prevents you from building fake generic infrastructure and then redesigning the DB later.

### Design decision
Create the minimal real schema now, not a placeholder schema with meaningless names.

---

# How the pieces connect

This is the full Phase 1 runtime flow.

When you run:

```zsh id="mmtkfm"
go run ./cmd/vtwatch run
```

here is what happens:

## Step 1
`cmd/vtwatch/main.go` starts the process and calls the CLI package.

## Step 2
`internal/cli/root.go` builds the root Cobra command and recognizes the `run` subcommand.

## Step 3
`internal/cli/run.go` starts the boot sequence.

## Step 4
`internal/config/config.go` loads or creates config and returns the resolved paths.

## Step 5
`internal/store/sqlite.go` opens the SQLite database.

## Step 6
`internal/store/migrate.go` reads and applies `migrations/001_init.sql`.

## Step 7
Control returns to `run.go`, which prints that boot succeeded.

That is the full app for Phase 1.

No VT API.  
No session import.  
No watch scheduler.  
No registration logic.

Just a clean boot path.

---

# Why not add more now

Because adding more now would be fake progress.

You already know later phases will need:
- VT client code
- watch logic
- session handling
- registrar logic
- maybe a web UI

That does **not** mean they belong in Phase 1.

Phase 1 should stay small enough that:
- you understand every file
- every package has a clear reason to exist
- the boot path is obvious

That is how you avoid cargo-cult architecture.

---

# The exact files we should write in Phase 1

These are the files that matter, in order:

1. `go.mod`
2. `cmd/vtwatch/main.go`
3. `internal/cli/root.go`
4. `internal/cli/run.go`
5. `internal/config/config.go`
6. `internal/store/sqlite.go`
7. `internal/store/migrate.go`
8. `migrations/001_init.sql`
9. `README.md`

That order matters because it mirrors the runtime boot chain.

---

# The key design decisions, plainly

## Why `cmd/vtwatch/main.go` instead of root-level `main.go`
Because I want runnable entrypoints clearly separated from the rest of the app implementation.

## Why `internal/cli` instead of top-level `cmd` package for Cobra commands
Because this repo is not just “a Cobra app.” It is a real application with multiple internal subsystems. I want CLI code to be one implementation package among others, not the defining package of the whole repo.

## Why no Viper yet
Because simple JSON config is enough for Phase 1 and anything more is premature.

## Why SQLite now
Because persistent local state is central to the app model, even before VT integration exists.

## Why migrations now
Because the app already has clear durable entities, and schema should reflect them from the start.

## Why no VT code yet
Because the app needs a clean boot skeleton before protocol code has a sane place to live.

---

# What success looks like for Phase 1

When Phase 1 is done, you should be able to:

## 1. Run the app
```zsh id="dyc5mh"
go run ./cmd/vtwatch run
```

## 2. See config and DB files created
```zsh id="t9d4jq"
ls -la ~/.config/vtwatch
```

Expected:
- `config.json`
- `vtwatch.db`

## 3. Re-run without errors
The second run should also succeed cleanly.

That means the skeleton is stable.

---

# Bottom line

Phase 1 is just the clean, minimal shell of the application.

Each file exists for one reason:

- `main.go` starts the executable
- `cli/root.go` defines the root command
- `cli/run.go` defines the boot action
- `config.go` owns runtime settings and default paths
- `sqlite.go` opens persistence
- `migrate.go` initializes schema
- `001_init.sql` defines the first real durable entities
- `README.md` gives the repo a clear identity

That is the right simple starting point.

The next step is to actually write those files.