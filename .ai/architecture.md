# Architecture

## Overview

Simuhook is a mock HTTP server written in Go. Users define endpoints as YAML files in a `stubs/` directory. The server watches the directory, matches incoming requests against the stubs, and returns configured responses. Stub changes are hot-reloaded live without restart. Every request is logged as a JSON line, and an optional full-screen TUI dashboard shows live stats.

## Commands

- `simuhook` — single command. Flags:
  - `--addr :8080` — listen address (env `PORT`)
  - `--stubs ../stubs` — stubs directory (env `STUBS_DIR`)
  - `--log-dir logs` — log directory (env `LOG_DIR`)
  - `--tui auto|on|off` — dashboard mode. `auto` (default): TUI on a TTY, line output otherwise.
  - `--recent N` (default 50) — recent-log ring buffer size (also the TUI feed depth)
  - `--max-log-body N` (default 0 = unlimited, env `LOG_MAX_BODY`) — caps `body`/`response_body` length in `LogEntry` at capture time (rune-safe truncation); never affects the actual HTTP response bytes or echo/template rendering
  - `--help`, `--version` per CLI conventions

## Components

```
src/
├── main.go               # Entry point: flags, banner, HTTP server, shutdown
├── dashboard/            # bubbletea TUI dashboard
│   ├── dashboard.go      # Run() — program entry (alternate screen)
│   ├── model.go          # tea.Model, StatsSource, tick/prompt/key handling
│   ├── render.go         # header/feed/footer/help rendering (lipgloss)
│   ├── filter.go         # Filter, ParseFilter, Matches (pure, tea-free)
│   └── *_test.go
└── mock/
    ├── types.go          # Endpoint, Response structs
    ├── registry.go       # Store, match, CRUD endpoints (thread-safe)
    ├── admin.go          # NewAdminHandler: REST admin API (/_admin/endpoints) (WS-017b)
    ├── handler.go        # Handler: single request-handling source of truth (WS-017)
    ├── stub.go           # YAML stub file parser
    ├── logger.go         # LogEntry, JSON Lines logging
    ├── stats.go          # StatsTracker: counts, ring, latency stats, Notify
    ├── templating.go     # Body/header template execution
    └── watcher.go        # File watcher (hot-reload)

stubs/                    # YAML stub definitions
├── health.yaml
├── ping.yaml
├── order-placed.yaml
└── ...
```

Dependency direction: `main → dashboard → mock`. Nothing in `mock` imports `dashboard` or `main`.

## API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/_admin/endpoints` | List all endpoints |
| POST | `/_admin/endpoints` | Create endpoint (JSON body) |
| GET | `/_admin/endpoints/{id}` | Get one endpoint |
| PUT | `/_admin/endpoints/{id}` | Update endpoint |
| DELETE | `/_admin/endpoints/{id}` | Remove endpoint |

All other paths are the mock surface: requests are matched against stubs and answered with the configured response. Matched and unmatched requests are both logged.

## Data Flow

1. YAML stubs loaded from `stubs/` at startup; watcher hot-reloads changes live
2. HTTP server binds early (`net.Listen` before any UI output — fail fast on port conflicts)
3. Incoming requests matched against stubs (method, path, headers, query, body)
4. Matched requests get configured response (status, headers, body, optional delay, echo/template modes)
5. Every request logged as a JSON line to `logs/<webhook-id>.log`; unmatched go to `logs/_unmatched.log`
6. `StatsTracker.RecordRequest` updates counts + latency aggregations; the dashboard polls via `Snapshot()`

## Key Decisions

### WS-010 — Interactive TUI dashboard (`[Could]`, done)
- **Framework:** bubbletea v1.3.10 + lipgloss v1.1.0 + bubbles v1.0.0 (viewport only — no list/textinput). First external UI dependency; approved to override stdlib-first for UI only.
- **`--tui auto|on|off`**: `auto` detects a TTY (`golang.org/x/term.IsTerminal`); non-TTY falls back to the WS-016 per-request line output. **No breaking change:** line output and startup banner remain when TUI is disabled; `--tui on` forces the TUI even on a non-TTY.
- **Live updates — coalesced signal + pull:** `StatsTracker.Notify()` (buffered(1), non-blocking, coalescing) signals a pump goroutine which calls `Program.Send` at ≤10 Hz; the model re-reads `Snapshot()` on each tick. `Program.Send` is unbuffered and blocks, so it is NEVER called on the request path.
- **StatsTracker latency:** exact `Count/Sum/Min/Max` accumulators over ALL requests plus a fixed 92-bucket logarithmic histogram (4 sub-buckets × 23 octaves, 0.01 ms → ~84 s) for an approximate p95 (interpolated, clamped to `[Min,Max]`). `LatencyStats.Avg()` is the single division and is guarded against `Count==0` (no NaN/div-zero).
- **Signal ownership:** in TUI mode bubbletea owns SIGINT/SIGTERM (its `handleSignals` registers `signal.Notify(sig, SIGINT, SIGTERM)`; SIGINT → `InterruptMsg`, SIGTERM → `QuitMsg`). `main.go` therefore does NOT register `signal.Notify` in TUI mode. After `p.Run()` returns (q/Ctrl+C/SIGTERM), `srv.Shutdown(ctx)` drains with a 5 s timeout. Non-TUI mode keeps the explicit `signal.Notify` + `<-done` path.
- **Log redirection:** when TUI is active, `log.SetOutput(<log-dir>/server.log)` so watcher/stub reload messages can't corrupt the alternate screen. Request logs still go to their own files via `logger.Log`.
- **Filtering:** `Filter{Endpoint, Method, Status, Text}` with `ParseFilter` (`ep:`/`method:`/`status:` prefixes + bare text) is pure and tea-free. Applied at render time only; the ring buffer is never mutated. Prompt mode (`/` or `f`) swallows all keys except backspace/enter/esc/ctrl+c; `q` cancels the prompt before quitting. **Space key:** bubbletea v1.3.10 delivers ` ` as `KeyMsg{Type: KeySpace}` (not `KeyRunes`); the prompt handler must accept both, or multi-token filters (`ep:ping status:200`) are impossible to type (fixed + `TestModelPromptSpaceTyping`).
- **Sticky-bottom feed:** `syncFeed()` checks `viewport.AtBottom()` before `SetContent`, re-anchoring to bottom only if the user was following live; manual scroll offsets survive live updates.
- **Status tinting (UXR-001):** `tint(status)` wraps only the status segment in the feed — 2xx green `"76"`, 3xx cyan `"39"`, 4xx yellow `"220"`, 5xx red `"196"`, other muted `"240"`. Pure render.
- **Per-endpoint stats (UXR-002):** endpoints panel rows append `requests: N  p95: X.Xms` from `Snapshot().HitCounts`/`EndpointLatency`; ID padded to 20 chars for column alignment; zero-hit endpoints show `requests: 0`; row order stays sorted by ID.
- **Keys:** `q`/`ctrl+c` quit, `r` refresh, `/`/`f` filter prompt, `enter` apply (or expand row — see UXR-008), `esc` cancel/clear, `c` clear, `e` toggle endpoints panel, `L` pause/resume live-follow, `?` help; scrolling via viewport DefaultKeyMap (`↑/k`, `↓/j`, `pgup/pgdn`); `g`/`G` jump to top/bottom of the stream feed (explicit `KeyMsg` handling — see the `g`/`G` bullet below, bubbles' `DefaultKeyMap` doesn't bind them).
- **Feed row expansion (UXR-008):** `enter` (non-prompt) expands the row at the top visible line. Selection is **derived from the viewport position** — `selectedIdx = feedLineToEntry[view.YOffset]` — so no cursor keys or list widget are needed (satisfies the "viewport only" contract). Expansion keyed by `LogEntry.RequestID`, survives ticks, auto-clears when the entry leaves the ring or stops matching the filter. Detail lines (request_id, matched, RFC3339Nano timestamp, headers, query, body) map to the same entry index, muted with `│` gutter, truncated to width via local `truncPlain` (lipgloss v1.1.0 has no `Truncate`). `esc`/`c` also collapse; collapsing resumes live-follow. `syncFeed` uses relative-delta correction so an expanded row's offset survives ticks; `WindowSizeMsg` re-pins the expanded summary line.
- **Feed security (UXR-008):** ALL log-derived text rendered in the feed (path, webhook, header keys/values, query keys/values, body) passes through `sanitize()` which maps control chars `<0x20` and DEL to space — request content is attacker-controlled and could otherwise inject ANSI escape sequences into the TUI alternate screen. The selected-row highlight is built **segment-wise** (each segment carries its own `Background`) because wrapping a string containing an inner `\x1b[0m` in a `Background()` style loses the background after the reset.
- **Two-column layout (UXR-009/010/011, Phase 6, `544db99`):** at `width ≥ 100 && height ≥ 14` the dashboard renders the redesigned layout — title row with `[ Pause Stream ]` button (UXR-010), SERVER INFO chips, muted log-dir/admin line, METRICS & LATENCY chips, then two `RoundedBorder` panels at a fixed 40/60 split (`leftTotal = width*2/5`; `rightTotal = width-leftTotal`): left = REGISTERED ENDPOINTS table (Method|Endpoint|Stub|Req|p95) with its OWN `endpointsView` viewport scrolled by `shift+↑/↓` (aliased `K`/`J`), right = LIVE REQUEST STREAM table (Time|Method|Path|Stub|Status). Below the threshold it falls back byte-identical to the legacy vertical layout. All size math lives in one pure `geometry()` (single source of truth) + `applyGeometry()`/`syncEndpoints()`; `headerHeight()` is unchanged. **No focus state** — existing keys always drive the stream viewport. `e` hides the left panel (stream full width); `compactMs`/`compactCount` render Req/p95; `fitBlock` + per-line `truncPlain` guarantee every panel block is exactly its budgeted size (never `MaxHeight` — it clips the border). Filter chip relocates from the endpoints title to the stream title when the left panel is hidden (UXR-007). Colour 39 in wide chrome is reserved for the chip only. `g`/`G` are NOT bound in bubbles v1.0.0's `DefaultKeyMap` (only `PageDown/PageUp/HalfPageUp/HalfPageDown/Up/Down/Left/Right` are) — implemented separately as explicit `KeyMsg` handling in `model.Update` (backlog item `viewport-g-g-bindings`, resolved): `g` → `GotoTop` + pause live-follow, `G` → `GotoBottom` + resume live-follow; pure navigation, does not collapse an active row expansion. Both `ep.Path` and `ep.ID` are sanitized (unauthenticated admin API is an injection vector).
- **Startup order:** `net.Listen` → registry/watcher/load → TUI decision → logger + log redirect → banner (non-TUI only) → mux/server → `dashboard.Run` (blocks) → `srv.Shutdown`.

### WS-017 — Single request handler (`[Could]`, done)
- **Problem:** three handler families existed — main.go's inline closure (shipped, 0% tested), `mock.Server` (dead code, referenced only by `server_test.go`, 2 real bugs), and 11 self-referential mini-handlers inside `mock_test.go` that tested their own code. The shipped matching path had no tests.
- **Decision (Option C):** extract to `mock.NewHandler(HandlerConfig) http.Handler` as the single source of truth; `main` keeps listener/mux/signals/shutdown/TUI. `mock.Server`/`NewServer`/`Start`/`Stop`/`MarshalEndpoints` and `server.go` deleted; the 11 self-testing tests deleted.
- **`HandlerConfig{Registry, Observers []func(LogEntry), Now}`:** observers are called synchronously after the response is written; `stats.RecordRequest` binds as a method value, `logger.Log` needs a 1-line closure. `Registry` is required (nil panics in `NewHandler`). `Now` is a test seam (defaults to `time.Now`).
- **`FormatRequestLine(LogEntry)`** renders the WS-016 one-line summary; it re-localizes the UTC `LogEntry.Timestamp` so the displayed clock stays LOCAL.
- **Locked semantics (shipped main.go behavior wins over dead-code bugs):** response-header templates receive `headers, query` (was `nil, nil` → empty renders); template data built BEFORE the delay sleep (`{{.Now}}` = request-receipt time); unmatched 404 returns explicit `Content-Type: application/json`; `LatencyMs` includes `resp.Delay`; `r.Body` nil-guarded; `PickResponse`'s pointer is never written through.
- **TUI log-dir fix:** `os.MkdirAll(*logDir)` before opening `server.log` in TUI mode (pre-existing WS-010b bug — `NewLogger` creates the dir lazily, so a fresh `--log-dir` crashed TUI startup).
- **Coverage:** mock 74.4% → 81.4% (dead code removed, shipped path now covered for the first time); `go vet` clean (4 pre-existing findings lived in the deleted self-testing tests).

### WS-017b — Single admin handler (`[Could]`, done)
- **Problem:** the admin REST API lived inline in `main.go:161-224` with zero unit tests — the same gap WS-017 closed for the request path.
- **Decision:** extract to `mock.NewAdminHandler(registry *Registry) http.Handler` in `src/mock/admin.go`; `main.go` keeps only the two mounts (`mux.Handle(AdminEndpointsPath, admin)` + `mux.Handle(AdminEndpointsPath+"/", admin)`). Concrete type `adminHandler` unexported; **no config struct** (admin has no observers, no clock — contrast WS-017's `HandlerConfig`). Nil registry panics in the constructor (WS-017 convention). Exported `AdminEndpointsPath = "/_admin/endpoints"` removes the literal duplication between the mux mounts and the id-parsing `TrimPrefix`.
- **One handler, two mounts:** the handler distinguishes collection from item with `r.URL.Path == AdminEndpointsPath`. Rejected an internal `ServeMux` (double routing, inherits mux path-cleaning/redirect semantics, prefix-bound 404s) and split constructors (double API surface) — there are exactly two routes.
- **Admin stays unlogged** by mux precedence: both admin patterns beat `"/"`, so admin traffic never reaches `mock.NewHandler` ⇒ no `logger.Log`, no `stats.RecordRequest`, no TUI row. Locked by `TestAdminHandler_MuxPrecedence_AdminNotLogged`, which composes the same mux inside package `mock`.
- **Locked wire semantics (byte-for-byte from the shipped main.go):** success paths set `Content-Type: application/json` **except POST 201, which sets none** (sniffed to `text/plain; charset=utf-8` on the wire — preserved quirk, approved by stakeholder 2026-08-03, Q1); `json.Encoder` supplies the trailing `"\n"`; every error goes through `http.Error` ⇒ `text/plain; charset=utf-8` + `X-Content-Type-Options: nosniff` + `msg + "\n"` (**405 included**); `DELETE` → 204 empty body, no `Content-Type`; empty `List()` encodes `[]\n` (never `null`).
- **Order-of-checks invariants are observable and must not be tidied:** empty-id 400 precedes the item method switch (`PATCH /_admin/endpoints/` ⇒ **400**, not 405); JSON decode precedes `Add`/`Update` (invalid JSON ⇒ **400** even for a duplicate or unknown id).
- **`registry.Update` sets `ep.ID = id`,** so a `PUT` response reports the **path** id and no second map key is created; `PUT` is a full replace (absent fields become zero; `rrIndex` resets). Load-bearing coupling — documented in `admin.go`.
- **Race fix (byte-identical):** POST/PUT now `Encode(&ep)` instead of `Encode(ep)`. The value copy read the unexported `rrIndex` non-atomically while `PickResponse` atomically incremented it through the registry's pointer — reproducible `-race` warning. Pointer-encode produces identical JSON and matches GET-list/GET-one.
- **Known quirks preserved (not fixed):** `created_at` is client-supplied/zero on POST (only the YAML loader stamps `time.Now()`); `List()` order is map-iteration order; `[]byte` fields (`payload`, `response.body`) are base64 on the wire; no `Allow` header on 405; the admin API is unauthenticated (standing risk — `ep.Path`/`ep.ID` sanitized in the TUI because of it).

### Stubs and matching
- YAML stubs support: method, path, query match, header match, payload match, response status/body/headers, `delay`, `echo`, `template` modes, and multiple response variants with weights (round-robin on equal weight).
- Hot reload via fsnotify; every reload is logged.

### Internal packages / conventions
- `net/http` (stdlib) — no external HTTP framework
- Interfaces for testability (`StatsSource` for the dashboard; `http.RoundTripper` when mocking HTTP)
- Graceful shutdown on SIGINT/SIGTERM
- Idempotency support via `X-Idempotency-Key` header
- All decisions traceable to a user story or requirement (see `.ai/requirements/`)

### Deliverables
- Runnable Go binary: `simuhook`
- REST admin API for CRUD on endpoints
- Live TUI dashboard with counts, latency stats, filterable recent-log feed
- JSON Lines delivery log with status, latency, headers, body, query

## Open Items

- **WS-011** (`[Could]`, backlog): webhook forwarding / proxy mode.
- **WS-012** (`[Could]`, backlog): WebSocket / SSE endpoint simulation.
