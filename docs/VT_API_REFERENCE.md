# VT Registration API — Technical Reference

Empirically verified reference for Virginia Tech's registration API, derived from reverse engineering `fose.js` and `sam_core.js` plus live request testing. Use this as the single source of truth when building tools against this API.

**Last verified:** April 2026, against the classes.vt.edu production API.

---

## 1. Base Facts

- **Host:** `https://classes.vt.edu`
- **Base path:** `/api/`
- **Three distinct endpoints**, differentiated by `page` query param:
  - `page=fose` — Public course search (no auth needed for basic data)
  - `page=sisproxy` — Authenticated student data, cart, preflight, registration
  - `page=shockabsorber` — Registration queue / Banner bridge
- **SIS type:** `foseConfig.sis === "banner"` (not peoplesoft — this matters for how credentials flow)
- **Auth mode:** `sam.auth.mode === "token"` (VT uses token auth, not cookie-only)
- **HTTP version:** HTTP/2 supported and used by the browser; HTTP/1.1 or HTTP/2 both work from clients

## 2. Authentication

### 2.1 What VT actually accepts

**Confirmed empirically:** all sisproxy and shockabsorber operations work with `authtoken` alone. No `IDMSESSID` cookie is required. The reverse-engineered PDF that claimed `IDMSESSID + _pers` was mandatory was wrong for VT's token-auth configuration.

### 2.2 The credentials SeatHawk stores

| Credential | Source (browser JS) | Stability | Purpose |
|---|---|---|---|
| `authtoken` | `sam.auth.token` | Session-stable, expires on logout or after ~24h | Authenticates most calls |
| `_pers_id` | `studentdata.pers.id` | Session-stable | Proves identity to shockabsorber |
| `_pers_id_proof` | `studentdata.pers.idProof` | May rotate between calls (safe to refresh) | Signed proof for shockabsorber |

The user only needs to capture `authtoken` from the logged-in browser session. SeatHawk calls `studentdata` during import and stores `_pers_id` plus `_pers_id_proof` from that response. This avoids relying on copied proof values that may already be stale.

### 2.3 Where credentials go in requests

- `authtoken` → goes in the **query string** for sisproxy calls, and in the **POST form body** for shockabsorber calls
- `_pers_id` + `_pers_id_proof` → go in the **POST form body** for shockabsorber register/status calls
- **Do NOT include authtoken in `url_replay`** — that's the PeopleSoft path, which VT doesn't use

## 3. The JSONP Wrapper

Sisproxy responses are wrapped in a JSONP function call. Strip the wrapper before parsing as JSON.

| Endpoint action | Wrapper function | Example response |
|---|---|---|
| `cart_read`, `cart_add`, `cart_remove` | `setCart(...)` | `setCart({"cart":[...]})` |
| `studentdata` | `setRecord(...)` | `setRecord({"pers":{...},"reg_tickets":[...],...})` |
| `preflight` | `preflight(...)` | `preflight({"reg_course_errors":{},"reg_non-course_errors":[]})` |
| `get_time_tickets` | `setTimeTickets(...)` | `setTimeTickets({"tickets":[...]})` |

Fose and shockabsorber return plain JSON (no wrapper).

**Stripping pattern:** match and strip `functionName(...)` optionally followed by `;` or whitespace. A regex like `^[a-zA-Z_]+\(([\s\S]*)\)[;\s]*$` captures the inner JSON.

## 4. Endpoint: `fose` (Public Search)

### 4.1 Search by CRN

```
POST https://classes.vt.edu/api/?page=fose&route=search
Content-Type: application/json

Body (JSON):
{
  "other": {"srcdb": "202606"},
  "criteria": [{"field": "crn", "value": "60058"}]
}
```

**Response (plain JSON):**
```json
{
  "srcdb": "202606",
  "count": 1,
  "results": [
    {
      "crn": "60058",
      "code": "AAEC 2104",
      "title": "Hunger Issues",
      "stat": "A",
      "no": "60058",
      "total": "30",
      "schd": "L",
      "instr": "...",
      "hours_html": "3 Credit Hours",
      "srcdb": "202606"
    }
  ]
}
```

### 4.2 Status codes (`stat` field)

| Code | Meaning |
|---|---|
| `A` | Open, seats available |
| `F` | Full, no seats, no waitlist |
| `W` | Full, waitlist open |
| `C` | Canceled |

### 4.3 Other fose routes (reference)

The fose endpoint also supports these routes; none are strictly required for the monitor+register flow but listed for completeness:

- `route=details` — Full course details (use with `_pers` for meeting times)
- `route=cache-search` — Cached variant of search, used for cart batch fetching
- `route=promoted` — Promoted/featured content for UI banners
- `route=load-filter-data` — Per-term filter options
- `route=arranger-cart-sections` — Schedule Builder helper
- `route=plan-arranger-*` — Schedule Builder planning routes

### 4.4 No auth required

Search returns public data. Safe to poll without credentials. Be polite with frequency (30+ seconds between calls per CRN).

## 5. Endpoint: `sisproxy` (Authenticated)

All sisproxy calls are `GET` with parameters in the query string. Include `authtoken` as a query parameter.

### 5.1 `studentdata` — Fetch full student record

**This is the single most valuable endpoint.** One call returns everything needed to build a tool: identity, pers credentials, registration tickets, current cart, registered sections, history.

```
GET https://classes.vt.edu/api/?page=sisproxy&action=studentdata&authtoken=<TOKEN>
```

**Response** (after stripping `setRecord(...)` wrapper):
```json
{
  "releaseversion": "2.17",
  "pers": {
    "fn": "Brennan",
    "id": "zKNYy3aq24T9f5K25HFUJw==",
    "idProof": "HMPWV|+ISfljc4PhUjnftCnNTgjw==",
    "clas": "10",
    "levl": "",
    "inst": false,
    "cur": [...],
    "roles": []
  },
  "cart": [
    "202606|default|60058|3||||AAEC 2104||N|||E|||||",
    ...
  ],
  "reg": {
    "202606": ["60900|CS 3304||N|3|UG|joHHuW"],
    "202609": [...]
  },
  "hist": {...},
  "waitlist": {...},
  "bl": {},
  "wl": {},
  "override": {},
  "reg_tickets": [
    {
      "term": "202606",
      "start_date": "2026-03-17T07:00:00:000000000-04:00",
      "end_date": "2026-06-23T23:59:00:000000000-04:00",
      "ticket": "2026-03-17T07:00:00:000000000-04:00|202606||4J3hVN/Bqjc1yqKpDrW6Yw==|MaVNpknJF4zGJACjMSVeoQ",
      "pinreq": false,
      "actions": ["add","modify","drop"]
    },
    {
      "term": "202609",
      "ticket": null,
      "actions": ["regopt"]
    }
  ],
  "plan": [...]
}
```

### 5.2 `cart_read` — Read current cart

```
GET https://classes.vt.edu/api/?page=sisproxy&action=cart_read&authtoken=<TOKEN>
```

**Response** (after stripping `setCart(...)` wrapper):
```json
{
  "cart": [
    "202606|default|60058|3||||AAEC 2104||N|||E|||||"
  ]
}
```

Each cart entry is a pipe-delimited string (see §5.7 for field positions).

### 5.3 `cart_add` — Add section to cart

```
GET https://classes.vt.edu/api/?page=sisproxy
  &action=cart_add
  &term_code=202606
  &cart_name=default
  &crn=60058
  &hours=3
  &gmod=N
  &reg_info=E
  &authtoken=<TOKEN>
```

**Critical:** `cart_name` is `default`, **not `PRIMARY`**. The reverse-engineered doc was wrong about this. Both names work for cart_add, but shockabsorber only processes the `default` cart.

**Parameters:**

| Param | Required | Description | Example |
|---|---|---|---|
| `action` | Yes | Must be `cart_add` | `cart_add` |
| `term_code` | Yes | Term code | `202606` |
| `cart_name` | Yes | Cart identifier — **always use `default`** | `default` |
| `crn` | Yes | Section CRN | `60058` |
| `hours` | Yes | Credit hours (from section data) | `3` |
| `gmod` | Yes | Grade mode: `N`=A-F, `P`=Pass/Fail, `A`=Audit | `N` |
| `reg_info` | Yes | Registration info flag | `E` |
| `authtoken` | Yes | Your authtoken | `...` |
| `crn_drop` | No | CRN to drop atomically (for swap; see §6.5) | `60900` |
| `wait_list_okay` | No | `Y` to join waitlist if full | `Y` |
| `prmsn_nbr` | No | Permission number for restricted sections | |
| `options` | No | URL-encoded JSON: `{regorder, regalt1, regalt2, regalt3}` | `%7B%22regorder%22%3A1%7D` |

**Response:** `setCart({"cart": [...]})` with the updated cart contents.

### 5.4 `cart_remove` — Remove section from cart

```
GET https://classes.vt.edu/api/?page=sisproxy
  &action=cart_remove
  &term_code=202606
  &cart_name=default
  &crn=60058
  &authtoken=<TOKEN>
```

**Response:** `setCart(...)` with the updated cart.

### 5.5 `preflight` — Validate before registering

Runs Banner-side validation for a set of CRNs. Returns errors that would block registration (prereqs, conflicts, restrictions).

```
GET https://classes.vt.edu/api/?page=sisproxy
  &action=preflight
  &term_code=202606
  &cart_name=default
  &crn_list=60058
  &authtoken=<TOKEN>
```

`crn_list` can be a single CRN or comma-separated list (e.g., `60058,60059`).

**Response** (after stripping `preflight(...)` wrapper):
```json
{
  "reg_course_errors": {
    "60058": "||"
  },
  "reg_non-course_errors": []
}
```

**Interpreting errors:**
- `reg_course_errors` is keyed by CRN. Value is a pipe-delimited error string. Empty pipes `||` means a soft state (often "registration not currently open" or ticket inactive). Real errors contain text.
- `reg_non-course_errors` is a list of global errors like `"Another registration is in progress for this ID and TERM"`.
- To parse: split on `|` then filter empty strings. For each non-empty part, split on `\n` to get individual error lines.

### 5.6 `get_time_tickets` — Fetch registration tickets (alternative path)

This endpoint exists as the "slow path" for fetching tickets when `reg_tickets` isn't already in the user record. For VT, `studentdata` already returns `reg_tickets`, so this endpoint usually isn't needed by external tools.

```
GET https://classes.vt.edu/api/?page=sisproxy&action=get_time_tickets&term_code=202606&authtoken=<TOKEN>
```

**Response wrapper:** `setTimeTickets(...)`

The response shape differs from `reg_tickets` inside studentdata. Prefer `studentdata` for ticket information.

### 5.7 Cart Entry Format (pipe-delimited, 18 fields)

When cart entries are returned, they're pipe-delimited strings. Field positions:

| Index | Field | Example |
|---|---|---|
| 0 | `term` | `202606` |
| 1 | `cartID` | `default` |
| 2 | `crn` | `60058` |
| 3 | `hours` | `3` |
| 4 | `meetInfo` | (usually empty) |
| 5 | `resourceURL` | (usually empty) |
| 6 | `addPath` | (usually empty) |
| 7 | `course` | `AAEC 2104` |
| 8 | `equivalencies` | (comma-separated) |
| 9 | `gradeMode` | `N` |
| 10 | `institution` | (usually empty) |
| 11 | `enrollCRN` | (usually empty) |
| 12 | `regInfo` | `E` |
| 13 | `permissionNumber` | (usually empty) |
| 14 | `swapCrn` | (usually empty) |
| 15 | `status` | (usually empty) |
| 16 | `waitListOkay` | `Y` or empty |
| 17 | `options` / `dropCrn` | JSON options OR drop CRN (dual-use) |

### 5.8 Registered Section Format (pipe-delimited, in `reg.<term>`)

| Index | Field | Example |
|---|---|---|
| 0 | `crn` | `60900` |
| 1 | `code` (course) | `CS 3304` |
| 2 | (reserved) | |
| 3 | `gradeMode` | `N` |
| 4 | `hours` | `3` |
| 5 | `career` | `UG` |
| 6 | (misc identifier) | `joHHuW` |

### 5.9 Waitlist Entry Format (in `waitlist.<term>`)

| Index | Field |
|---|---|
| 0 | `crn` |
| 1 | `career` |
| 2 | `position` (numeric) |

### 5.10 History Entry Format (in `hist.<term>`)

Pipe-delimited strings where index 0 is the course code. Used to detect previously-taken courses.

### 5.11 Plan Entry Format (in `plan[]`)

| Index | Field |
|---|---|
| 0 | `plan` (plan ID, usually `default`) |
| 1 | `subject` (e.g., `CS`) |
| 2 | `number` (e.g., `2114`) |
| 3 | `inTerm` (planned term) |
| 4 | `crn` |
| 5 | (reserved) |
| 6 | `dateAdded` |
| 7 | `surrogateId` |
| 8 | `topic` |
| 9 | `hours` |

## 6. Endpoint: `shockabsorber` (Registration Queue)

This is the queue/buffer layer between the frontend and Banner. Actual Banner registration happens here. Construct these requests carefully — this is where VT changes academic records.

### 6.1 The `time_ticket` parameter (the hardest part)

`time_ticket` is a pipe-delimited string with **7 fields**. The first 5 come from the server (via `reg_tickets[i].ticket` in studentdata). The last 2 are constructed client-side.

**Format:**
```
<start_iso>|<term>|<career_or_empty>|<banner_hash>|<signed_hash>|<timestamp_ms>|<pers_id>
```

**Example (fully assembled):**
```
2026-03-17T07:00:00:000000000-04:00|202606||4J3hVN/Bqjc1yqKpDrW6Yw==|MaVNpknJF4zGJACjMSVeoQ|1773745200000|zKNYy3aq24T9f5K25HFUJw==
```

**Construction algorithm:**

1. Find the `reg_tickets` entry where `.term == your_target_term`
2. If `.ticket == null` or `.actions` does not contain `"add"` (or `"drop"`/`"modify"` for those operations), **you have no active registration window for that operation** — abort
3. Take the `.ticket` field (the 5-field pipe-delimited string from the server)
4. Parse `.start_date` into a Unix timestamp in milliseconds
5. Append `|<timestamp_ms>|<pers_id>` to the server's ticket string

**The VT datetime format is non-standard:**
```
2026-03-17T07:00:00:000000000-04:00
```

Note the `:` before the fractional seconds where ISO 8601 uses `.`. Before parsing with a standard datetime library, replace the fifth `:` with `.`:

```
2026-03-17T07:00:00:000000000-04:00
                   ^
                   replace this colon with a period
```

A regex like `(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}):(\d+[-+]\d{2}:\d{2})` captures the non-standard form; replace with `$1.$2` to make it parseable as ISO 8601.

### 6.2 `register` — Submit registration

**This is the actual Banner registration call.** After cart_add, call this to commit.

```
POST https://classes.vt.edu/api/?page=shockabsorber
  &time_ticket=<URL-ENCODED TIME_TICKET>
  &action=register
  &cart_name=default
  &url_replay=<URL-ENCODED RELATIVE REPLAY URL>

Content-Type: application/x-www-form-urlencoded

Form body:
  authtoken=<TOKEN>
  _pers_id=<PERS_ID>
  _pers_id_proof=<PERS_ID_PROOF>
  _pers_real_id=<PERS_ID>
```

**The `url_replay` parameter** is the relative sisproxy URL shockabsorber will replay against Banner.

For a **single-CRN add or drop**:
```
api/?page=sisproxy&action=register&term_code=202606&crn=60058&wait_crn=&swap_crn=
```

For an **atomic add/drop (swap)**, see §6.5 — the `crn` parameter takes a comma-separated list.

**Critical details:**
- `url_replay` is **relative** (starts with `api/`), not absolute
- `url_replay` points to sisproxy **`action=register`** (NOT `cart_add`)
- `url_replay` does **NOT** contain `authtoken` (that's the peoplesoft path, VT is banner)
- Empty `wait_crn=` and `swap_crn=` are still included as parameters

**Response (plain JSON):**
```json
{"body":"WAIT","code":200,"data":{"id":"144260"}}
```

**Observed response body values:**
- `"WAIT"` — Queued, poll status for completion
- `"OK"` — Completed successfully (may come back immediately for simple cases)
- Error bodies (specific strings and structures need further empirical confirmation)

**`data.id`** is the queue job ID. Use it plus the cart_name to poll status.

**Response code `500` from shockabsorber** typically means your request is malformed (bad cart_name, bad url_replay, missing credentials, etc.). See §8.5.

### 6.3 `status` — Poll registration status

Poll this until the registration reaches a terminal state.

```
POST https://classes.vt.edu/api/?page=shockabsorber
  &time_ticket=<URL-ENCODED TIME_TICKET>
  &action=status
  &cart_name=default

Form body:
  authtoken=<TOKEN>
  _pers_id=<PERS_ID>
  _pers_id_proof=<PERS_ID_PROOF>
  _pers_real_id=<PERS_ID>
```

Response shapes follow the same `{body, code, data}` pattern as register. A `"WAIT"` body means continue polling; other values indicate terminal states. The full error vocabulary needs empirical confirmation from real failure cases.

**Polling cadence:** every 1-2 seconds is reasonable. Give up after ~30 polls (~60 seconds) if no terminal state is reached.

### 6.4 Add vs. Drop vs. Swap (all use `register`)

VT's registration system uses **the same `register` action** for add, drop, and swap. The system determines intent from your current registration state plus what's in your cart **AND** from the contents of the `crn` parameter inside the `url_replay` URL.

- **Add:** Single CRN in `url_replay` → `crn=<ADD_CRN>`. Banner adds it.
- **Drop:** Single CRN in `url_replay` → `crn=<DROP_CRN>`. If the CRN is currently in your registrations, Banner drops it.
- **Swap (atomic add/drop):** Both CRNs in `url_replay` → `crn=<ADD_CRN>,<DROP_CRN>` (comma-separated). Banner processes them as one transaction.

The cart_add `crn_drop` parameter alone is **not sufficient** to make a swap atomic. It stages the intent in the cart, but the actual atomic transaction is driven by what shockabsorber replays. See §6.5 for the full mechanism.

**Dangerous implication:** If a user is already registered for the target CRN and a tool mistakenly fires register again with that CRN alone, **it will drop them**. Always check `reg.<term>` from studentdata before attempting registration. Never call register on a single CRN the user is already registered for (unless they explicitly requested a drop).

### 6.5 Atomic swap (drop-and-add) — the full mechanism

**Empirically confirmed from a real browser-issued atomic swap request.** The mechanism is more involved than the cart_add `crn_drop` parameter alone.

Both pieces are required for an atomic transaction:

**Step 1: Stage the swap in the cart**
```
GET .../api/?page=sisproxy
  &action=cart_add
  &term_code=202606
  &cart_name=default
  &crn=60058           # the CRN to add
  &crn_drop=60900      # the CRN to drop (currently registered)
  &hours=3
  &gmod=N
  &reg_info=E
  &authtoken=<TOKEN>
```

**Step 2: Build the inner replay URL with BOTH CRNs comma-separated**

The inner `url_replay` URL must put both CRNs in the `crn` parameter, separated by a comma. Before being placed into the outer shockabsorber URL, the comma is URL-encoded once as `%2C`:

```
api/?page=sisproxy&action=register&term_code=202606&crn=60058,60900&wait_crn=&swap_crn=
```

After the first round of URL-encoding to embed in the outer URL:

```
api/?page=sisproxy&action=register&term_code=202606&crn=60058%2C60900&wait_crn=&swap_crn=
```

**Step 3: Encode the entire inner URL again for the outer shockabsorber call**

When the inner URL becomes the value of the outer `url_replay` parameter, it gets URL-encoded a second time. The previously-encoded `%2C` becomes `%252C`:

```
url_replay=api%2F%3Fpage%3Dsisproxy%26action%3Dregister%26term_code%3D202606%26crn%3D60058%252C60900%26wait_crn%3D%26swap_crn%3D
```

The `%252C` in the final URL is correct — it's `%2C` (the comma) that has been URL-encoded once more for the outer URL.

**Step 4: POST to shockabsorber as normal**
```
POST https://classes.vt.edu/api/?page=shockabsorber
  &time_ticket=<TIME_TICKET>
  &action=register
  &cart_name=default
  &url_replay=<DOUBLY-ENCODED INNER URL>

Form body:
  authtoken=<TOKEN>
  _pers_id=<PERS_ID>
  _pers_id_proof=<PERS_ID_PROOF>
  _pers_real_id=<PERS_ID>
```

**Pseudocode for building the swap request:**

```
# Build inner replay URL with comma-separated CRNs
inner_crns = ADD_CRN + "," + DROP_CRN
inner_url = "api/?page=sisproxy&action=register&term_code=" + TERM
          + "&crn=" + inner_crns
          + "&wait_crn=&swap_crn="

# URL-encode once for embedding in outer URL
url_replay_encoded = urlencode(inner_url)

# POST to shockabsorber with url_replay as a query parameter
# (most HTTP libraries will handle the second round of encoding automatically
#  when serializing query parameters)
```

**The bug to avoid:** if you only put one CRN in the inner replay URL and rely on `crn_drop` in the cart entry to handle the drop, **Banner will only process the add and leave the other course registered**. The cart_add `crn_drop` parameter is necessary but not sufficient. The atomic transaction requires both CRNs in the replay URL.

**Edge case:** if the drop CRN is a prerequisite for something else you're registered for, Banner may reject the transaction even if the add would otherwise succeed. Preflight does not always catch this.

## 7. Complete Registration Flow

The end-to-end sequence for registering when a seat opens:

```
1. fose search            → confirm seat is still Open (stat=A)
2. sisproxy preflight     → validate eligibility, no conflicts
3. sisproxy cart_add      → place CRN in cart (with optional crn_drop for swap)
4. shockabsorber register → submit to Banner via the queue
                            (with comma-separated CRNs in url_replay for swaps)
5. shockabsorber status   → poll until terminal state
6. sisproxy studentdata   → confirm registered in reg.<term>
```

Each step can fail independently. A state machine must handle:

- Seat disappears between steps 1-3 (preflight OK, cart_add fails, or shockabsorber rejects)
- Preflight blockers (prereqs not met, time conflict, already registered for equivalent, etc.)
- Token expiration at any point
- Network failures (retry vs. abort decision)
- Registration window closes mid-flight
- Shockabsorber returns a `WAIT` that never transitions (queue stuck)

**The confirmation step matters:** a missed or timed-out status response does not mean the registration failed. Always verify against `studentdata` before marking a watch as failed, because the registration may have succeeded even if the status call errored.

## 8. Request Quirks and Gotchas

### 8.1 HTTP method conventions

| Endpoint | Method | Params location |
|---|---|---|
| `fose` all routes | POST | JSON body |
| `sisproxy` all actions | GET | Query string |
| `shockabsorber register` | POST | Query string + form body |
| `shockabsorber status` | POST | Query string + form body |

### 8.2 URL encoding

All values in `url_replay` and `time_ticket` must be URL-encoded when placed in the outer shockabsorber URL. The `time_ticket` string contains `+`, `/`, `=`, and `|` characters that all need encoding. The `_pers_id` and `_pers_id_proof` values also contain `+`, `/`, `=`, and `|` — encode them whenever they appear in URLs (less of a concern when placed in a form body, since standard form-urlencoded serializers handle this).

**Double-encoding for swap CRN lists:** the comma separator inside `url_replay`'s `crn` parameter gets encoded twice — once when the inner URL is built (`,` → `%2C`) and once when that inner URL is placed in the outer query string (`%2C` → `%252C`). See §6.5. This is correct behavior, not a bug; most HTTP client libraries handle the outer encoding automatically when you pass `url_replay` as a query parameter value.

### 8.3 The `cart_name` trap

- Browser UI uses `default` for the primary cart
- Some old reverse-engineered docs say `PRIMARY`
- `cart_add` accepts both names, so you might write items to one cart and try to register from another
- **Always use `default`** for consistency with the UI flow
- The constant `cartID` in the browser code is also `default` for the primary cart

### 8.4 Credential rotation

- `authtoken` typically stable for ~24h or until logout (`sam.config.sessionTimeout = 86400` seconds)
- `_pers_id` stable across session
- `_pers_id_proof` may rotate between requests; fetch fresh before critical calls if getting auth errors
- Time tickets are tied to specific registration windows; expire when `end_date` passes

**Keep-alive:** the browser pings `cart_read&role=keepalive` every 5 minutes to extend the session. A long-running tool may want to do the same:

```
GET https://classes.vt.edu/api/?page=sisproxy&action=cart_read&role=keepalive&pers_id=<ID>&pers_id_proof=<PROOF>
```

The response includes `session.maxExtend` (seconds until expiry). If under 120s, the UI warns "session expiring soon."

### 8.5 Debugging shockabsorber HTTP 500

If shockabsorber returns `{"body":"Internal Server Error","code":500,"data":{}}`, common causes in rough order of likelihood:

1. `cart_name` is wrong (used `PRIMARY` instead of `default`)
2. `url_replay` points to `cart_add` instead of `register`
3. `url_replay` is an absolute URL instead of relative
4. `url_replay` contains `authtoken` when it shouldn't (VT is banner, not peoplesoft)
5. Missing `_pers_id`, `_pers_id_proof`, or `_pers_real_id` in form body
6. Time ticket malformed, expired, or for wrong term
7. Nothing in the `default` cart for the target term
8. `actions` in reg_ticket does not include the intended operation (add/drop/modify)

When debugging, capture the exact request the browser sends (DevTools → Network → Copy as cURL) and diff against your request byte for byte.

### 8.6 The "swap that only adds" bug

If you fire a swap request and Banner only adds the new CRN without dropping the old one, the bug is almost certainly in the `url_replay` `crn` parameter. The browser sends both CRNs comma-separated (see §6.5). If you only put one CRN there and relied on the cart's `crn_drop` field to handle the rest, Banner ignores the drop.

### 8.7 Impersonation (admins only, mostly irrelevant)

The browser code has special handling for `pers.realuser` which only applies when an admin is impersonating another student. Most users will never encounter this. If it appears, `_pers_real_id` differs from `_pers_id` and `url_replay` gets an additional `realuser` parameter. For non-admin users, `_pers_real_id` equals `_pers_id`.

### 8.8 Cold-start behavior

Some endpoints (notably `preflight` and the first call after a session has been idle) can intermittently return errors on the first try and succeed on immediate retry. If you see a pattern of "fails, then works on retry," don't assume your request is wrong — it might be a Banner cold-start issue. Be careful retrying `shockabsorber register` though: see §11.4.

## 9. Reference Data

### 9.1 Term codes (`srcdb` / `term_code`)

Format: `YYYYMM` where MM is `01` (Spring), `06` (Summer), `09` (Fall), `12` (Winter).

Examples: `202601` = Spring 2026, `202606` = Summer 2026, `202609` = Fall 2026, `202612` = Winter 2026-2027.

### 9.2 Grade modes (`gmod`)

| Code | Meaning |
|---|---|
| `N` | A-F standard letter grade |
| `P` | Pass/Fail |
| `A` | Audit |
| `E` | Equivalent Credit |
| `Z` | Committee Action |

### 9.3 Registration actions (in `reg_tickets[i].actions`)

| Action | Meaning |
|---|---|
| `add` | Can add sections |
| `modify` | Can modify existing registrations (includes swaps) |
| `drop` | Can drop sections |
| `regopt` | Registration options period (view only, cannot register) |

An empty `actions` array means the ticket exists but the window has not yet opened. For a swap, you need both `add` and `drop` (or `modify`) in the actions list.

### 9.4 Campus codes (`camp`, for fose search filters)

| Value | Campus |
|---|---|
| `0` | Blacksburg (main) |
| `2` | Western |
| `4` | National Capital Region |
| `6` | Central |
| `7` | Hampton Roads Center |
| `8` | Capital |
| `10` | Virtual |
| `14` | VTCSOM |
| `SA` | Study Abroad |

### 9.5 Course modality (`crs_modality`)

| Code | Meaning |
|---|---|
| `A` | Face-to-face |
| `H` | Hybrid |
| `N` | Online synchronous |
| `O` | Online asynchronous |

### 9.6 Schedule type codes (`schd`)

| Code | Type |
|---|---|
| `L` | Lecture |
| `B` | Lab |
| `R` | Research |
| `I` | Independent Study |
| `C` | Recitation |
| `VL` | Virtual Campus Lecture |
| `VB` | Virtual Campus Lab |

### 9.7 Section note types (from `notes` array on sections)

| Value | Meaning |
|---|---|
| `cart` | Section is in your cart |
| `registered` | You are registered for this section |
| `waitlist` | You are on the waitlist |
| `full` | Section is full |
| `full-with-waitlist` | Full but waitlist is open |
| `canceled` | Section is canceled |
| `taken` | Previously taken this course |
| `taken-equiv` | Taken an equivalent |
| `warn` | Warning (restriction conflict) |
| `soft-warn` | Advisory warning |

### 9.8 Student classification codes (`clas` in `pers`)

Numeric-string values. Observed: `10`, corresponds to undergraduate level within Banner's internal classification. Not usually needed by external tools but visible in the record.

## 10. Bookmarklet: Capturing Authtoken

A bookmarklet extracts the VT authtoken from the user's logged-in browser session. This is run once during initial "connect" flow. SeatHawk derives the remaining stored credentials by calling `studentdata` during import.

### 10.1 Readable source

```javascript
javascript:(async()=>{
  try {
    const authtoken = typeof sam !== 'undefined' && sam.auth ? sam.auth.token : '';
    if (!authtoken) {
      throw new Error('VT authtoken not found; make sure you are logged in on classes.vt.edu');
    }
    const creds = {
      captured_at: new Date().toISOString(),
      authtoken
    };
    await navigator.clipboard.writeText(JSON.stringify(creds, null, 2));
    alert('VT authtoken copied to clipboard. Paste into SeatHawk.');
  } catch (e) {
    alert('Capture failed: ' + e.message);
  }
})();
```

### 10.2 Stable JS paths

- `sam.auth.token` → the authtoken string
- `sam.record.getProperty('pers').id` → `_pers_id`
- `sam.record.getProperty('pers').idProof` → `_pers_id_proof`
- `foseCart.timeTickets.termTickets[term]` → cached term ticket object (structured)
- `foseCart.timeTickets.mustGetTimeTicketParam(term)` → fully-assembled 7-field time_ticket string
- `sam.user.id()` → alias for `_pers_id`
- `sam.user.getProperty('reg_windows')` or `sam.user.getProperty('reg_tickets')` → raw ticket data as in studentdata

SeatHawk should not need to copy `_pers_id`, `_pers_id_proof`, or `reg_tickets` from the page. It can refresh all of those by calling `studentdata` with the captured authtoken whenever needed.

### 10.3 Copy-paste bookmarklet

Save this as the bookmark URL:

```javascript
javascript:(async()=>{try{const t=typeof sam!='undefined'&&sam.auth?sam.auth.token:'';if(!t)throw new Error('VT authtoken not found; open the logged-in registration page and try again after it finishes loading');const j=JSON.stringify({captured_at:new Date().toISOString(),authtoken:t},null,2);if(typeof copy=='function'){copy(j);alert('VT authtoken copied to clipboard. Paste into SeatHawk.');return}try{await navigator.clipboard.writeText(j);alert('VT authtoken copied to clipboard. Paste into SeatHawk.')}catch(_){prompt('Copy this SeatHawk session JSON:',j)}}catch(e){alert('Capture failed: '+e.message)}})();
```

This bookmarklet intentionally does not call `studentdata`. It reads the same `sam.auth.token` value that works in the browser console, then SeatHawk calls `studentdata` during `session import`. That keeps the browser-side capture path small and avoids copying stale `idProof` values.

## 11. Security and Operational Notes

### 11.1 What the credentials can do

Anyone with `authtoken` can fetch `studentdata`, derive `_pers_id` and `_pers_id_proof`, and then:
- Read the full student record (registration history, current schedule, plans)
- Add/remove from the student's cart
- **Register or drop any course during an active registration window**
- Access waitlist, permissions, and override data

Treat credentials with the same care as a VT password. Never log them. Never transmit to any server other than classes.vt.edu.

### 11.2 Storage

Store credentials locally only. On Unix-like systems, use file permissions `0600` (owner read/write only). On Windows, use ACLs or OS-level credential storage (Credential Manager).

For a local-only tool, plain file storage with proper permissions is acceptable. For anything with broader access, use the OS keychain (macOS Keychain, Linux Secret Service, Windows Credential Manager).

### 11.3 Rate limiting

Be polite. Suggested baseline limits:
- fose search: one call per CRN per 30+ seconds
- sisproxy calls: bundle when possible, don't loop faster than 1/sec
- shockabsorber register: never more than once per CRN per registration attempt (not a retry hammer)

Use a token bucket or similar rate limiter to enforce these globally across your tool, not per-CRN. 50 CRNs being watched should not mean 50 req/sec to VT.

### 11.4 Error recovery discipline

If a registration call times out or returns ambiguously, **do not retry blindly**. Read `studentdata` to see the actual registration state, then decide. Double-registering or accidental drops happen when a retry fires after a successful-but-timed-out request.

The safe retry protocol:
1. Registration call times out / returns unexpected response
2. Fetch `studentdata`
3. If `reg.<term>` now contains the target CRN: registration succeeded, stop
4. If cart is empty for that CRN and registration state unchanged: registration failed cleanly, safe to try again (respecting rate limits)
5. If state is ambiguous: alert the user and do not retry automatically

### 11.5 Out-of-window behavior

Check `reg_tickets` before every registration attempt:
- `.ticket == null` → no ticket issued yet, skip entirely
- `.actions` doesn't contain the needed action (`"add"`, `"drop"`, or `"modify"`) → cannot perform it, skip
- Current time before `.start_date` → window not open yet, skip
- Current time after `.end_date` → window closed, mark the watch expired

### 11.6 Confirm before destructive actions

Because add/drop/swap all go through the same `register` action, the client-side guard against accidental drops is critical:

1. Before any register call, fetch studentdata fresh
2. If the target CRN is already in `reg.<target_term>` and you're sending it as a single CRN in url_replay, **you will drop it** — refuse unless the user explicitly requested a drop
3. For swaps, the user must explicitly specify the drop_crn; never infer it
4. Verify the swap request actually includes both CRNs in url_replay (a missed comma will silently turn a swap into an add)
5. Consider requiring a `--confirm` or similar explicit flag for swap operations

### 11.7 Acceptable use considerations

VT's Acceptable Use Policy (Policy 7000) defines credential sharing with another person as misuse. A local tool that uses the user's own credentials on their own machine to automate their own browsing is closer to "scripting a browser" than "sharing credentials." A hosted service where users give credentials to a third party is closer to the misuse definition.

A tool that auto-registers occupies a policy gray area. Mitigations:
- Local-only execution (credentials never leave the user's machine)
- Respectful rate limiting (don't look like a bot)
- Clear disclaimers to users about risk
- No hosted/service version

## 12. Implementation Priority

When building a client, implement in this order:

1. **JSONP stripper** — needed by every sisproxy call
2. **VT timestamp parser** — needed for time_ticket construction
3. **`fose search`** — no auth, safest to test against real server
4. **`sisproxy studentdata`** — unlocks everything else
5. **`sisproxy cart_read`** — simple auth test, read-only
6. **`sisproxy cart_add` / `cart_remove`** — write operations, test with a known-failing CRN first
7. **`sisproxy preflight`** — safe validation call
8. **Time ticket builder** — pure function, unit-testable
9. **`shockabsorber register` (single CRN)** — the dangerous one, test first with a deliberately failing CRN to see error shapes, then with a disposable test CRN and immediate drop
10. **`shockabsorber register` (swap with comma-separated CRNs)** — even more dangerous because mistakes can drop a class you wanted to keep; test with two disposable CRNs only
11. **`shockabsorber status`** — poll logic

Mock everything above the network layer in tests. Only hit real Banner during careful integration testing with CRNs you're willing to briefly register for and immediately drop.

## 13. Testing Strategy

### 13.1 Safe tests (no state changes)

- `fose search` with any CRN
- `sisproxy studentdata`
- `sisproxy cart_read`
- `sisproxy preflight` (read-only validation)

### 13.2 Reversible tests (state changes, cleanable)

- `sisproxy cart_add` followed immediately by `sisproxy cart_remove`
- Repeated calls to verify idempotency (cart_add for a CRN already in cart should succeed or return a sensible error)

### 13.3 Dangerous tests (actually register you)

Only run these with:
- A CRN you're willing to be briefly registered for
- A plan to immediately drop it after the test
- Confirmation afterward via `studentdata` that the state matches expectations

For swap testing, you need two disposable CRNs and you need to be already registered for one of them. Mistakes here can leave you registered for the wrong section or no section at all. Best practice: register manually for one disposable CRN via the UI, then test swapping it for another disposable CRN, then drop whatever you ended up with.

Never test with popular/required courses.

### 13.4 Ideal test fixture data

Capture real responses (with credentials redacted) and store as fixtures:
- `studentdata_response_sample.json`
- `fose_search_open_section.json`
- `fose_search_full_section.json`
- `preflight_success.json`
- `preflight_blocked.json`
- `cart_add_success.json`
- `cart_add_with_swap.json` — the cart entry shape when crn_drop is specified
- `shockabsorber_register_wait.json`
- `shockabsorber_register_success.json`
- `shockabsorber_register_swap_success.json`
- `shockabsorber_register_error_*.json` (as you encounter them)

These are gold for unit testing without hitting the network.

---

## Appendix A: Quick Reference Table

| Operation | Method | Endpoint | Auth |
|---|---|---|---|
| Search by CRN | POST | `/api/?page=fose&route=search` | None |
| Get student record | GET | `/api/?page=sisproxy&action=studentdata&authtoken=X` | authtoken |
| Read cart | GET | `/api/?page=sisproxy&action=cart_read&authtoken=X` | authtoken |
| Add to cart | GET | `/api/?page=sisproxy&action=cart_add&...&authtoken=X` | authtoken |
| Remove from cart | GET | `/api/?page=sisproxy&action=cart_remove&...&authtoken=X` | authtoken |
| Preflight | GET | `/api/?page=sisproxy&action=preflight&...&authtoken=X` | authtoken |
| Keepalive | GET | `/api/?page=sisproxy&action=cart_read&role=keepalive&pers_id=X&pers_id_proof=Y` | pers ids |
| Register (single) | POST | `/api/?page=shockabsorber&action=register&...` (`crn=X` in replay) | full (form body) |
| Register (swap) | POST | `/api/?page=shockabsorber&action=register&...` (`crn=X,Y` in replay) | full (form body) |
| Status | POST | `/api/?page=shockabsorber&action=status&...` | full (form body) |

## Appendix B: Sample `time_ticket` Construction (Pseudocode)

```
function buildTimeTicket(regTicket, persId):
    if regTicket.ticket == null:
        throw "no active registration window"
    if "add" not in regTicket.actions and "drop" not in regTicket.actions:
        throw "registration window not active"

    startMillis = parseVTTimestamp(regTicket.start_date)

    return regTicket.ticket + "|" + startMillis + "|" + persId


function parseVTTimestamp(vtTimestamp):
    # vtTimestamp looks like: "2026-03-17T07:00:00:000000000-04:00"
    # standard parsers need:   "2026-03-17T07:00:00.000000000-04:00"
    normalized = regexReplace(
        vtTimestamp,
        /(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}):(\d+[-+]\d{2}:\d{2})/,
        "$1.$2"
    )
    return parseISO8601(normalized).toUnixMillis()
```

## Appendix C: Full Example Request Trace (Successful Single-CRN Registration)

For reference, here is the complete sequence for registering CRN 60058 in term 202606 (no swap, just an add):

**Step 1: Confirm seat is open**
```
POST https://classes.vt.edu/api/?page=fose&route=search
Body: {"other":{"srcdb":"202606"},"criteria":[{"field":"crn","value":"60058"}]}

Response: {"results":[{"crn":"60058","stat":"A",...}]}
```

**Step 2: Preflight (optional but recommended)**
```
GET https://classes.vt.edu/api/?page=sisproxy&action=preflight&term_code=202606&cart_name=default&crn_list=60058&authtoken=TOKEN

Response: preflight({"reg_course_errors":{"60058":"||"},"reg_non-course_errors":[]})
```

**Step 3: Add to cart**
```
GET https://classes.vt.edu/api/?page=sisproxy&action=cart_add&term_code=202606&cart_name=default&crn=60058&hours=3&gmod=N&reg_info=E&authtoken=TOKEN

Response: setCart({"cart":["202606|default|60058|3||||AAEC 2104||N|||E|||||"]})
```

**Step 4: Register via shockabsorber (single CRN)**
```
POST https://classes.vt.edu/api/?page=shockabsorber
  &time_ticket=2026-03-17T07%3A00%3A00%3A000000000-04%3A00%7C202606%7C%7C4J3hVN%2FBqjc1yqKpDrW6Yw%3D%3D%7CMaVNpknJF4zGJACjMSVeoQ%7C1773745200000%7CzKNYy3aq24T9f5K25HFUJw%3D%3D
  &action=register
  &cart_name=default
  &url_replay=api%2F%3Fpage%3Dsisproxy%26action%3Dregister%26term_code%3D202606%26crn%3D60058%26wait_crn%3D%26swap_crn%3D

Form body:
  authtoken=TOKEN
  _pers_id=zKNYy3aq24T9f5K25HFUJw==
  _pers_id_proof=HMPWV|+ISfljc4PhUjnftCnNTgjw==
  _pers_real_id=zKNYy3aq24T9f5K25HFUJw==

Response: {"body":"WAIT","code":200,"data":{"id":"144260"}}
```

**Step 5: Poll status until terminal**
```
POST https://classes.vt.edu/api/?page=shockabsorber&time_ticket=...&action=status&cart_name=default
(form body with same credentials)

Response: {"body":"WAIT",...} or {"body":"OK",...} when done
```

**Step 6: Verify via studentdata**
```
GET https://classes.vt.edu/api/?page=sisproxy&action=studentdata&authtoken=TOKEN

Response: setRecord({..., "reg":{"202606":["60058|AAEC 2104|...",...]}})
```

If CRN 60058 now appears in `reg.202606`, the registration succeeded.

## Appendix D: Full Example Request Trace (Atomic Swap)

Here is the complete sequence for an atomic swap: drop CRN 60900 (currently registered) and add CRN 60058, in term 202606.

**Step 1: Confirm new section is open**
```
POST https://classes.vt.edu/api/?page=fose&route=search
Body: {"other":{"srcdb":"202606"},"criteria":[{"field":"crn","value":"60058"}]}

Response: {"results":[{"crn":"60058","stat":"A",...}]}
```

**Step 2: Preflight both CRNs**
```
GET https://classes.vt.edu/api/?page=sisproxy&action=preflight&term_code=202606&cart_name=default&crn_list=60058,60900&authtoken=TOKEN
```

**Step 3: cart_add the new CRN with crn_drop pointing at the old one**
```
GET https://classes.vt.edu/api/?page=sisproxy
  &action=cart_add
  &term_code=202606
  &cart_name=default
  &crn=60058
  &crn_drop=60900
  &hours=3
  &gmod=N
  &reg_info=E
  &authtoken=TOKEN

Response: setCart({"cart":["202606|default|60058|3||||AAEC 2104||N|||E|||||{...}"]})
```

The cart entry now contains drop intent in field 17. **But this alone is not enough to make the swap atomic** — see step 4.

**Step 4: Register via shockabsorber with BOTH CRNs in url_replay**

The inner replay URL must include both CRNs separated by a comma:

```
inner: api/?page=sisproxy&action=register&term_code=202606&crn=60058,60900&wait_crn=&swap_crn=
```

After URL-encoding to embed in the outer URL, the comma becomes `%2C`:

```
api/?page=sisproxy&action=register&term_code=202606&crn=60058%2C60900&wait_crn=&swap_crn=
```

After the second round of encoding (when this becomes the value of `url_replay`), the `%2C` becomes `%252C`:

```
POST https://classes.vt.edu/api/?page=shockabsorber
  &time_ticket=<TIME_TICKET>
  &action=register
  &cart_name=default
  &url_replay=api%2F%3Fpage%3Dsisproxy%26action%3Dregister%26term_code%3D202606%26crn%3D60058%252C60900%26wait_crn%3D%26swap_crn%3D

Form body:
  authtoken=TOKEN
  _pers_id=zKNYy3aq24T9f5K25HFUJw==
  _pers_id_proof=HMPWV|+ISfljc4PhUjnftCnNTgjw==
  _pers_real_id=zKNYy3aq24T9f5K25HFUJw==

Response: {"body":"WAIT","code":200,"data":{"id":"..."}}
```

The `%252C` in the final URL is correct double-encoding — it represents `%2C` (the once-encoded comma) being encoded a second time for the outer URL.

**Step 5: Poll status until terminal** (same as single-CRN case)

**Step 6: Verify via studentdata**

After success, `reg.202606` should contain `60058` (newly added) and no longer contain `60900` (dropped). If the swap silently became an add-only, `reg.202606` will contain both CRNs, which is the symptom of putting only one CRN in the replay URL (see §8.6).
