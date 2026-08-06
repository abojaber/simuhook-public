# Feature-level stories — Admin REST API + TUI dashboard

Requirement file owner: BA. Status: `done` (both features implemented and verified).
Phase: post-MVP.

> This file captures the two headline features at the **feature level** (what a user can
> do), as distinct from the implementation stories already on record (`WS-017b` for the
> admin-handler extraction, `WS-010` + `UXR-001..014` for the dashboard internals).
> The wire contracts and implementation details live in `contracts.md`, `.ai/architecture.md`,
> `07-admin-handler.md`, and `05-ux-vnext.md`; those files remain the source of truth for
> engineering. This file is the product-facing view.

---

## ADM-001 — Runtime endpoint management via admin REST API

**Status:** `removed` | **Priority:** `[Could]` (backlog) | **Implemented:** inline since v0, extracted to `mock.NewAdminHandler` in `505e12c` | **QA:** verified 2026-08-03 (16/16 ACs) | **Removed by stakeholder decision 2026-08-05**

> **REMOVED from the product — moved to backlog (low priority).** Stakeholder decision
> 2026-08-05: "remove all admin endpoints for now… all APIs can be handled manually in the
> stubs folder." The YAML hot-reload watcher already provides live create/edit/delete, so
> the admin API is redundant for the dev workflow. This also eliminates the admin-specific
> risks found during BA analysis (no input validation, watcher clobbering, unauthenticated
> CRUD surface). Reintroduction is a backlog item — see consolidated entry in `04-backlog.md`.

**User Story (historical — feature as it existed):**
> As a developer running the simulator, I want to list, create, update, and delete
> registered endpoints through an HTTP admin API so that I can change the mock surface
> at runtime without editing YAML files or restarting the server.

**Context:**
- Discovery (01-discovery.md) originally fixed the control plane as "CLI first **with** an admin
  REST API for runtime management (`/_admin/endpoints`)" — superseded by the 2026-08-05 removal
  decision.
- The stale WS-014 entry (`[Won't]` "stakeholder said CLI-only") is corrected and consolidated
  with this story in `04-backlog.md`.
- Wire contract was locked byte-for-byte (see `07-admin-handler.md` — kept for the record and
  as the contract for any reintroduction).

**Acceptance Criteria (historical — defined the shipped behavior; retained as the
reintroduction contract):**
```
Given the server is running
When  I GET /_admin/endpoints
Then  I receive a 200 JSON array of all registered endpoints
      (empty list is [], never null)

Given a valid endpoint JSON body
When  I POST /_admin/endpoints
Then  I receive 201 with the created endpoint, and the endpoint starts serving
      requests immediately

Given an endpoint id that already exists
When  I POST /_admin/endpoints
Then  I receive 409 and no duplicate endpoint is created

Given an existing endpoint id
When  I GET /_admin/endpoints/{id}
Then  I receive 200 with that endpoint's JSON
Given an unknown endpoint id
When  I GET /_admin/endpoints/{id}
Then  I receive 404

Given an existing endpoint id and a full replacement body
When  I PUT /_admin/endpoints/{id}
Then  I receive 200 with the updated endpoint JSON, the endpoint is replaced,
      and live requests use the new configuration immediately

Given a DELETE request for an existing endpoint id
When  I DELETE /_admin/endpoints/{id}
Then  I receive 204 and the endpoint stops serving requests

Given a malformed body or a missing/unknown id
When  I call POST/PUT/GET/DELETE with it
Then  I receive a 4xx error describing the problem (400 bad request, 404 unknown id,
      405 wrong method)

Given admin API traffic
When  it hits /_admin/endpoints
Then  it is NOT written to the request logs nor counted in dashboard stats
      (admin traffic is control-plane, not simulated traffic)
```

**Notes for the Developer/QA (historical):**
- Reuses `mock.Registry` (same store the YAML watcher feeds) — admin-created endpoints
  behave identically to file-loaded ones.
- Admin-created endpoints are **not** written back to YAML; they live for the process
  lifetime. (Persistence of runtime edits is a separate story — see `WS-014` backlog note.)
- Full wire contract incl. preserved quirks (POST 201 Content-Type, error format, 405 without
  `Allow`, base64 `[]byte` fields) in `07-admin-handler.md`.

---

## DASH-001 — Interactive TUI dashboard

**Status:** `done` | **Priority:** `[Should]` | **Implemented:** `WS-010` series + `UXR-001..011` (UXR-012/013/014 in progress) | **QA:** verified (see `05-ux-vnext.md` per-story status) | **Contract:** `.ai/ui-ux-dashboard.md`, `06-dashboard-redesign.md`

**User Story:**
> As a developer running the simulator, I want a live full-screen dashboard showing the
> server state, per-endpoint stats, and the recent request stream — filterable and
> inspectable — so that I can monitor and debug my mocks without leaving the terminal or
> grepping log files.

**Context:**
- Default behavior: `--tui auto` launches the TUI on a TTY and falls back to per-request
  line output (WS-016) on non-TTY. `--tui on`/`--tui off` force either side.
- The dashboard is the primary monitoring surface; the per-request line output and JSON log
  files remain as the non-TTY fallback and the durable source of truth.

**Acceptance Criteria:**
```
Given I start the simulator on a terminal (TTY)
When  the server begins listening
Then  the startup banner prints once, then a full-screen dashboard takes over
      showing: uptime, total requests, per-endpoint hit counts, latency summary,
      and a live recent-request feed

Given a terminal at least 100 columns wide and 14 rows tall
When  the dashboard renders
Then  it shows the wide layout: title row with [ Pause Stream ] control, SERVER INFO
      and METRICS chips, and two side-by-side panels:
      left  = REGISTERED ENDPOINTS table (Method | Endpoint | Stub | Req | p95)
      right = LIVE REQUEST STREAM table (Time | Method | Path | Stub | Status)
Given a narrower or shorter terminal
When  the dashboard renders
Then  it falls back to the vertical layout and auto-collapses the endpoints panel
      so the feed is never squeezed to zero rows

Given requests arrive while the dashboard is active
When  a request is processed
Then  counts increment, the stream scrolls, and status codes are tinted by class
      (2xx green, 3xx cyan, 4xx yellow, 5xx red) for at-a-glance scanning

Given I want to inspect a single request
When  I move the selection to a stream row and press enter (expand) or i (inspect)
Then  I see that request's detail: headers, query, body, and (for inspect) the full
      response with status, headers, and pretty-printed JSON body

Given the feed is busy or I want to pause
When  I press L (or click-free: it is keyboard-only by design)
Then  live-follow pauses so the view does not yank, and resumes when I press L again
      or scroll to the bottom

Given I want to narrow the view
When  I press / or f and type a filter (ep:<id>, method:<verb>, status:<code>, bare text)
Then  the feed shows only matching entries, with the active filter indicated
      (chip in the stream/endpoints header and in the footer)

Given I press q or Ctrl+C
When  the key is handled
Then  the TUI exits cleanly, the terminal is restored, and the server shuts down
      gracefully (draining in-flight requests)
```

**Keys (complete):** `q`/`ctrl+c` quit · `r` refresh · `/`/`f` filter prompt ·
`enter` apply / expand row · `esc` cancel/clear · `c` clear · `e` toggle endpoints panel ·
`L` pause/resume live-follow · `?` help · scroll `↑/k ↓/j pgup/pgdn` · `g`/`G` top/bottom ·
`shift+↑/↓` (`K`/`J`) scroll endpoints panel (wide layout).

**Notes for the Developer/QA:**
- Dashboard is pure-read: it consumes `StatsSnapshot`; it never mutates the registry or logs.
- All log-derived text rendered in the TUI is sanitized (no ANSI/control injection) —
  this is a security requirement (UXR-008/UXR-011), not just polish.
- Feature toggles: `--recent N` (ring buffer / feed depth, default 50), `--max-log-body N`
  (capture cap, default 0 = unlimited).
- **`--max-log-body` vs the inspector (doc note, DASH-005):** the cap is applied at
  *capture* time in `mock.Handler`, strictly after the real response has been written —
  it only bounds what reaches the logger, the stats ring, and the dashboard (including the
  UXR-013 request/response inspector's `body`/`response_body` display). It NEVER affects the
  wire response bytes or echo/template rendering. So with a cap set, the inspector shows the
  capped body: a documented interaction, not a bug.
- Pending follow-ups already tracked: UXR-012 (endpoint-select filter), UXR-013 (request/
  response inspector — implemented as WS-018), UXR-014 (selection bug fix, `ready`).
