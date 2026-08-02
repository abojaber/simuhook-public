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
- **Keys:** `q`/`ctrl+c` quit, `r` refresh, `/`/`f` filter prompt, `enter` apply (or expand row — see UXR-008), `esc` cancel/clear, `c` clear, `e` toggle endpoints panel, `L` pause/resume live-follow, `?` help; scrolling via viewport DefaultKeyMap (`↑/k`, `↓/j`, `pgup/pgdn`, `g/G`).
- **Feed row expansion (UXR-008):** `enter` (non-prompt) expands the row at the top visible line. Selection is **derived from the viewport position** — `selectedIdx = feedLineToEntry[view.YOffset]` — so no cursor keys or list widget are needed (satisfies the "viewport only" contract). Expansion keyed by `LogEntry.RequestID`, survives ticks, auto-clears when the entry leaves the ring or stops matching the filter. Detail lines (request_id, matched, RFC3339Nano timestamp, headers, query, body) map to the same entry index, muted with `│` gutter, truncated to width via local `truncPlain` (lipgloss v1.1.0 has no `Truncate`). `esc`/`c` also collapse; collapsing resumes live-follow. `syncFeed` uses relative-delta correction so an expanded row's offset survives ticks; `WindowSizeMsg` re-pins the expanded summary line.
- **Feed security (UXR-008):** ALL log-derived text rendered in the feed (path, webhook, header keys/values, query keys/values, body) passes through `sanitize()` which maps control chars `<0x20` and DEL to space — request content is attacker-controlled and could otherwise inject ANSI escape sequences into the TUI alternate screen. The selected-row highlight is built **segment-wise** (each segment carries its own `Background`) because wrapping a string containing an inner `\x1b[0m` in a `Background()` style loses the background after the reset.
- **Startup order:** `net.Listen` → registry/watcher/load → TUI decision → logger + log redirect → banner (non-TUI only) → mux/server → `dashboard.Run` (blocks) → `srv.Shutdown`.

### WS-017 — Single request handler (`[Could]`, done)
- **Problem:** three handler families existed — main.go's inline closure (shipped, 0% tested), `mock.Server` (dead code, referenced only by `server_test.go`, 2 real bugs), and 11 self-referential mini-handlers inside `mock_test.go` that tested their own code. The shipped matching path had no tests.
- **Decision (Option C):** extract to `mock.NewHandler(HandlerConfig) http.Handler` as the single source of truth; `main` keeps listener/mux/signals/shutdown/TUI. `mock.Server`/`NewServer`/`Start`/`Stop`/`MarshalEndpoints` and `server.go` deleted; the 11 self-testing tests deleted.
- **`HandlerConfig{Registry, Observers []func(LogEntry), Now}`:** observers are called synchronously after the response is written; `stats.RecordRequest` binds as a method value, `logger.Log` needs a 1-line closure. `Registry` is required (nil panics in `NewHandler`). `Now` is a test seam (defaults to `time.Now`).
- **`FormatRequestLine(LogEntry)`** renders the WS-016 one-line summary; it re-localizes the UTC `LogEntry.Timestamp` so the displayed clock stays LOCAL.
- **Locked semantics (shipped main.go behavior wins over dead-code bugs):** response-header templates receive `headers, query` (was `nil, nil` → empty renders); template data built BEFORE the delay sleep (`{{.Now}}` = request-receipt time); unmatched 404 returns explicit `Content-Type: application/json`; `LatencyMs` includes `resp.Delay`; `r.Body` nil-guarded; `PickResponse`'s pointer is never written through.
- **TUI log-dir fix:** `os.MkdirAll(*logDir)` before opening `server.log` in TUI mode (pre-existing WS-010b bug — `NewLogger` creates the dir lazily, so a fresh `--log-dir` crashed TUI startup).
- **Coverage:** mock 74.4% → 81.4% (dead code removed, shipped path now covered for the first time); `go vet` clean (4 pre-existing findings lived in the deleted self-testing tests).

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

- **WS-017b** (`[Could]`, backlog): extract the admin REST API (`/_admin/endpoints`) from main.go into `mock.NewAdminHandler(registry) http.Handler` with unit tests (deferred from WS-017 by stakeholder).
- **WS-011** (`[Could]`, backlog): webhook forwarding / proxy mode.
- **WS-012** (`[Could]`, backlog): WebSocket / SSE endpoint simulation.
