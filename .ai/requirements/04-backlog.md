# Phase 04 — Backlog

> Future ideas. Could-have and won't-have features.

---

## WS-010 — Interactive TUI dashboard

**Status:** `ready` | **Priority:** `[Could]`

**User Story:**
> As a developer, I want an interactive terminal dashboard showing live request counts, response times, and recent logs so that I can monitor the simulator in real time during demos.

**Decisions (stakeholder sign-off):**
1. **Framework:** `bubbletea` (first external UI dependency — approved, overrides stdlib-first for UI)
2. **Data:** extend `StatsTracker` to support response-time metrics (currently only hit counts + 50-entry ring)
3. **Default:** the TUI **replaces** the current per-request line output (WS-016). Startup banner still prints before entering the TUI.
4. **Controls:** `q` quit, `r` refresh, filtering, scrolling log feed.

**Design review sign-off (2026-08-02):**
1. ✅ `--tui auto|on|off` approved (`auto` = TUI on TTY, WS-016 line fallback on non-TTY)
2. ✅ **p95 via fixed logarithmic histogram** (BA decision; ~19% error over ALL requests; exact Count/Sum/Min/Max). Rejected: exact p95 over sliding 4096-window.
3. ✅ Add `--recent N` flag (default 50) for filter history
4. ✅ **No breaking change:** per-request stdout line output is RETAINED (active when TUI is off or stdout is non-TTY). Logs remain source of truth.
5. ✅ Open backlog item for duplicate `mock.Server` handler (see WS-017)

**Sub-stories:**

### WS-010a — StatsTracker latency metrics

**Status:** `done` | **Commit:** `65ea750`

**User Story:**
> As a developer, I want the dashboard to show response-time stats so that I can spot slow endpoints at a glance.

**Acceptance Criteria:**
```
Given a tracker with recorded requests of varying latency
When  Snapshot() is called
Then  the snapshot includes per-endpoint and overall latency stats:
      count, sum, min, max, and p95
Given no requests have been recorded
When  Snapshot() is called
Then  latency stats are zero-valued (no division by zero, no NaN)
Given maxLogs (50) ring buffer wraps
When  latency is aggregated
Then  latency stats reflect ALL recorded requests, not just the last 50
```

**Design notes (Architect):**
- `LatencyStats{Count, Sum, Min, Max, P95}` + `Avg()` (single guarded division)
- Exact accumulators + 92-bucket log histogram (4 sub-buckets/octave, ~19% p95 error, clamped to [Min,Max])
- `StatsSnapshot` gains: `TotalRequests`, `Latency`, `EndpointLatency`, `UnmatchedCount`
- `TotalLogs` keeps ring-capped meaning (no breaking change); `TotalRequests` is monotonic lifetime count
- Unmatched requests: in overall `Latency` + `UnmatchedCount`, excluded from `EndpointLatency`
- `Notify()` — non-blocking coalescing signal channel (buffered 1) for TUI refresh

### WS-010b — bubbletea TUI replacing line output

**Status:** `done` | **Commit:** `ef101ab`

**User Story:**
> As a developer, I want the simulator to launch a full-screen dashboard on start so that I can monitor activity without the old one-line-per-request log.

**Acceptance Criteria:**
```
Given the server starts with stubs loaded
When  the server begins listening
Then  the startup banner prints once, then the TUI enters full-screen alternate-buffer mode
And   the TUI shows: uptime, total requests, per-endpoint hit counts, latency summary, recent log feed
Given requests arrive while the TUI is active
When  a request is processed
Then  the dashboard updates live (counts increment, recent feed scrolls)
Given the user presses q
When  the key is handled
Then  the TUI exits cleanly, restores the terminal, and the server shuts down
Given the user presses r
When  the key is handled
Then  the view refreshes immediately
```

**Design notes (Architect):**
- New package `src/dashboard/` (`dashboard.go`, `model.go`, `render.go`, `filter.go` + tests). Dependency direction `main → dashboard → mock`
- `StatsSource` interface (`Snapshot() mock.StatsSnapshot`) for testability
- **Live updates: coalesced signal + pull.** `StatsTracker.Notify()` (non-blocking, coalescing) → pump goroutine → `Program.Send` rate-limited to 10 Hz → model re-reads `Snapshot()`. Rationale: bubbletea `Program.Send` is unbuffered and blocks (verified in source); this keeps render latency OFF the request path.
- Startup: `net.Listen` first (fail fast before TUI), banner → primary buffer (altscreen swaps to alternate buffer, banner reappears on quit), `log.SetOutput(<log-dir>/server.log)` to stop watcher log spam corrupting the screen
- Signals: **bubbletea owns SIGINT/SIGTERM** — main.go's `signal.Notify` is removed. `q`/Ctrl+C/SIGTERM all → `p.Run()` returns → `&http.Server{}.Shutdown(ctx)`. Requires replacing `http.ListenAndServe` with explicit server.
- `--tui auto|on|off` (default auto): TTY→TUI; non-TTY→WS-016 line output (kept, no breaking change)
- `--recent N` (default 50): ring buffer capacity flag

### WS-010c — Log feed filtering and scrolling

**Status:** `done` | **Commit:** `1bc7aeb`

**User Story:**
> As a developer, I want to filter and scroll the recent-log feed so that I can inspect a specific endpoint's traffic during a demo.

**Acceptance Criteria:**
```
Given the dashboard is active
When  I filter by endpoint, method, or status
Then  only matching recent entries are shown
Given the feed exceeds the visible area
When  I scroll (keys: arrows / j,k)
Then  I can view earlier entries while live updates continue
Given the filter is cleared
When  I press the clear-filter key
Then  all recent entries are shown again
```

**Design notes (Architect):**
- `Filter{Endpoint, Method, Status, Text}` — pure type, `ParseFilter` (e.g. `ep:orders method:post status:5xx /path`), `Match(LogEntry) bool`
- Filtering at render time only (never persisted); ring stays filter-unaware
- Sticky-bottom viewport: `syncFeed()` checks `AtBottom()` before `SetContent`, re-`GotoBottom()` if following live, else preserve YOffset
- Key map: `q`/`ctrl+c` quit, `r` refresh, `/` filter prompt, `enter` apply, `esc` cancel/clear, `c` clear, `↑/k ↓/j` scroll, `pgup/pgdn` page, `g/G` top/bottom, `?` help
- Prompt-mode swallows command keys (via `updateNormal`/`updatePrompt` split)
- Status bar: `showing 3 of 50 recent (filtered)`; footer `▼ live` vs `⏸ scrolled`
- `viewport.DefaultKeyMap` also binds `d,u,f,b,h,l,space` — reserve those in future key assignments

**QA sign-off (2026-08-02, commit `d12cf8a`):** All 20 ACs verified. QA flagged two live-TTY observations (help overlay not toggling off; `q` appearing inside filter prompt). Investigation: both traced to PTY stream-capture artifacts and bubbletea's key-batching, NOT logic bugs — the model-level tests `TestModelToggleHelp` and `TestModelPromptQuit` pass, and live PTY runs exit cleanly. Intended behavior locked in by regression tests:
- standalone `q` press while prompting → cancels prompt (not quit)
- `q` inside batched filter text (e.g. `ep:queue`) → preserved as text
- `esc` after typed text → cancels prompt entirely

**Out of scope:** WS-011 forwarding (separate story), multi-screen navigation, charts beyond ASCII bars, SSE/web UI.

---

## WS-017 — Single source of truth for the request-handling path (Architect finding B4)

**Status:** `done` | **Priority:** `[Could]` | **Implemented:** commit `WS-017` (handler.go), `QA: verified`

**Story:**
> As a developer, I want a single source of truth for the request-handling path so that the shipped handler and any test doubles do not drift.

**Context (Architect finding, 2026-08-02):**
- Three handler families exist: (1) the anonymous closure in `main.go` — **shipped, 0% tested**; (2) `mock.Server` — **dead code** (referenced only by `server_test.go`), 63% covered, with 2 real bugs; (3) 11 ad-hoc mini-handlers written inline inside `mock_test.go` that re-implement match→respond and **test themselves** (they also produce all 4 `go vet` findings).
- `mock.Server.handler` has no `RecordRequest`/`Logger` calls and differs from the live inline handler (no logging, no stats, different template-data handling).
- Verified bugs in the dead `mock.Server` code: response-header `{{Header}}`/`{{Query}}` render empty (`nil, nil` passed), 404 unmatched returns `text/plain` instead of `application/json`, template clock rendered *after* the delay sleep, no admin API, hard `Close()` instead of graceful `Shutdown`.
- Reported 74.4% mock coverage is inflated by dead code while the shipped path sits at 0%.

### Decision (signed off by stakeholder 2026-08-02)

**Option C — extract to `mock.NewHandler(HandlerConfig) http.Handler`** as the single source of truth.

- `src/mock/handler.go` NEW: `Handler{registry, observers []func(LogEntry), now}`; `HandlerConfig{Registry *Registry (required, nil panics), Observers []func(LogEntry), Now func() time.Time (test seam)}`; `NewHandler(cfg)`; `ServeHTTP`; `FormatRequestLine(LogEntry) string` (WS-016 line, local time).
- Observer seam (`[]func(LogEntry)` called synchronously after the response is written) mirrors the `dashboard.StatsSource` precedent and keeps `mock` TUI-agnostic. `stats.RecordRequest` is assignable directly as a method value; `logger.Log` needs a 1-line closure.
- DELETE `src/mock/server.go` + `src/mock/server_test.go` (2 tests relocate to `registry_test.go`, 2 rewritten against the real handler).
- DELETE the 11 self-referential `TestServer_*` mini-handlers from `mock_test.go`.
- `main.go` keeps: flags, env, `net.Listen` fail-fast, mux, admin routes, watcher, banner, TUI, `srv.Shutdown`. Lines 225–331 replaced by ~12 lines wiring `mock.NewHandler` with observers.
- No lifecycle changes: WS-010 startup order and bubbletea signal ownership untouched.

### Locked semantics (all resolve toward shipped main.go behavior)

1. Response-header templates get `headers, query` passed (fixes the empty-render bug).
2. Template data built **before** the delay sleep — `{{.Now}}` = request-receipt time.
3. 404 unmatched: explicit `Content-Type: application/json`.
4. Request ID generated first, used in templates and `LogEntry`.
5. `LatencyMs` includes `resp.Delay` (unchanged, documented).
6. Keep the `r.Body != nil` guard.
7. Never write through the `*Response` returned by `PickResponse`; use a local status var.
8. **WS-016 line stays LOCAL time** (`FormatRequestLine` parses and `.Local()`s the UTC `LogEntry.Timestamp`). *Stakeholder approved 2026-08-02.*
9. **Response-header `{{Header}}`/`{{Query}}` begin resolving real values.** *Stakeholder approved 2026-08-02 — acknowledged as an output change for stubs using them.*

### Out of scope (deferred to WS-017b)

- `mock.NewAdminHandler(registry) http.Handler` extraction from `main.go:159-222` + tests (GET/POST/PUT/DELETE, 400/404/405). *Stakeholder approved deferral 2026-08-02.*
- `logger.Log` dropped-error reporting; observer panic policy; latency-includes-delay goroutine resource risk.

### Test plan (~24 tests, all vs. the shipped handler)

Rewritten (from server_test.go): round-robin variants (status + body), nil-headers no-panic. Relocated (registry_test.go): `List` JSON round-trip + `created_at`. New: matched/unmatched observer entries, latency includes delay, logger+stats integration (the test whose absence allowed the drift), template-header funcs, pre-delay timestamp, 404 content-type, nil body, empty-body no-write, multi-value query, concurrency under `-race`, `FormatRequestLine`, nil-registry panic. Timing asserts are lower-bound only; round-robin tests never `t.Parallel()`.

### Verification gates

- Step 3 manual smoke: `go run . --tui off` + curl matched/unmatched, then `--tui on` (counts increment, `q` exits cleanly).
- `go vet ./...` is clean only from step 5 onward (4 pre-existing findings live in deleted tests).
- `rg -n 'NewServer\|mock\.Server\|MarshalEndpoints' src/` ⇒ no hits after step 4.

### QA findings during verification (2026-08-02)

1. **TUI startup fails when `--log-dir` is a fresh path** — pre-existing (introduced WS-010b, present on `e32fb72`), NOT a WS-017 regression. `mock.NewLogger` creates the dir lazily on first `Log()`, but the TUI branch opened `<log-dir>/server.log` up front. **Fixed in WS-017:** `os.MkdirAll(*logDir, 0o755)` before opening. Re-verified: fresh dir auto-created, `server.log` written, HTTP 200, `q` exits rc=0.
2. "No shutdown message on SIGINT" — **capture artifact**, not a bug. `shutting down...` is printed (verified under real PTY).
3. "`q` doesn't exit" in synthetic PTY harness — **harness artifact** (zombie-reaping confusion in `os.kill(pid,0)`; dashboard package byte-identical to signed-off WS-010c; `waitpid` confirms clean rc=0 exit).

### Implementation steps

1. Add `handler.go` (+ nothing else). Green: `go build ./... && go vet ./... && go test -race -count=1 ./...`
2. Add `handler_test.go` (tests 1–3 + 6–25). Green: `go test -race -count=1 ./mock/`
3. Rewire `main.go` (delete 225–331, insert observer wiring). Manual smoke as above.
4. Delete `server.go` + `server_test.go`; relocate 2 tests. `rg` clean.
5. Delete 11 self-referential tests from `mock_test.go`; prune imports. `go vet ./...` = 0 findings.
6. *(WS-017b, deferred)* `mock.NewAdminHandler` extraction.
7. Docs: `.ai/architecture.md` — remove Open Items WS-017, add ADR block, `server.go` → `handler.go` in tree, record locked semantics.
8. Backlog: WS-017 `ready → done`, record chosen option + commit; open WS-017b item.

---

## WS-017b — Extract admin API into `mock.NewAdminHandler` (deferred from WS-017)

**Status:** `draft` | **Priority:** `[Could]`

**Story:**
> As a developer, I want the admin REST API handler extracted into a testable `mock.NewAdminHandler(registry) http.Handler` so that the control-plane routes are covered by unit tests rather than only by manual curl smoke tests.

**Context:**
- Deferred from WS-017 (stakeholder approved deferral 2026-08-02) to keep WS-017 focused on the request-handling path.
- Admin routes currently live inline in `main.go:159-222` (`/_admin/endpoints` + `/_admin/endpoints/`), untested by unit tests.

**Scope:**
- Extract to `mock.NewAdminHandler(registry) http.Handler`; mount on both `/_admin/endpoints` and `/_admin/endpoints/`.
- Preserve contract byte-for-byte: `POST` → 201 + created entity, `DELETE` → 204, empty-id → 400, invalid JSON → 400, missing id → 404, wrong method → 405.
- Tests: GET list, POST create, GET one, PUT update, DELETE remove, 400 invalid JSON, 400 empty id, 404 missing id, 405 wrong method.
- Retires most of TGR-04's admin gap (currently `[Won't]`).

**Dependencies:** WS-017 must land first (handler.go provides the observer/registry seams and deletes the old `MarshalEndpoints`).

---

## WS-011 — Webhook forwarding / proxy mode

**Status:** `draft` | **Priority:** `[Could]`

**User Story:**
> As a developer, I want the simulator to forward incoming requests to a real upstream service and log both the request and the upstream response so that I can use it as an inspection proxy.

---

## WS-012 — WebSocket / SSE endpoint simulation

**Status:** `draft` | **Priority:** `[Could]`

**User Story:**
> As a developer, I want to simulate WebSocket or SSE endpoints so that I can test real-time integrations.

---

## WS-013 — Built-in webhook templates (Stripe, GitHub, Slack)

**Status:** `draft` | **Priority:** `[Won't]`

**User Story:**
> As a developer, I want pre-built webhook templates for popular services.

*Rejected — stakeholder explicitly requested custom/dynamic webhooks only. No service-specific presets.*

---

## WS-014 — REST API to manage config at runtime

**Status:** `draft` | **Priority:** `[Won't]`

**User Story:**
> As a developer, I want a REST API to CRUD webhook endpoints at runtime.

*Rejected — stakeholder explicitly said CLI-only. Config is file-based.

---

# Test Gap Remediation

> Technical debt: untested paths identified during audit. Prioritized by risk.

---

## TGR-01 — StatsTracker unit tests

**Status:** `done` | **Priority:** `[Should]`

**Story:**
> As a developer, I want the `StatsTracker` to have full unit test coverage so that I can trust the ring buffer, hit counters, and snapshot logic are correct.

**Coverage gaps:**
- `NewStatsTracker(maxLogs)` — constructor sets correct initial state
- `RecordRequest(entry)` — basic record increments hit count for the endpoint ID
- `RecordRequest` with empty webhook — hit count is NOT incremented
- `RecordRequest` ring buffer wrap — recording >maxLogs entries overwrites oldest correctly
- `Snapshot()` before wrap — returns entries in insertion order, `TotalLogs` matches
- `Snapshot()` after wrap — returns entries in chronological order (oldest first), `TotalLogs` = maxLogs
- `Snapshot()` with empty buffer — returns empty slice, `TotalLogs` = 0
- `Uptime()` — returns positive duration
- Concurrent `RecordRequest` + `Snapshot` — no data race under `-race`

**Estimated tests:** 8 new test functions

**Implemented:** `src/mock/stats_test.go` — 10 test functions covering all gaps (plus a snapshot copy-not-alias check). Full suite: 61 tests, all pass under `go test -race`.

---

## TGR-02 — Server and templating edge cases

**Status:** `done` | **Priority:** `[Should]`

**Story:**
> As a developer, I want edge cases in the server handler and template engine covered by tests so that nil pointers and fallback paths don't cause runtime panics.

**Coverage gaps:**
- `MarshalEndpoints()` — returns valid JSON representing all registered endpoints
- `writeResponse()` with `nil` response headers map — does not panic
- `handler()` with zero-weight response variant in `responses[]` — treated as round-robin (no crash)
- `randomInt(max, min)` where `max <= min` — returns `min` (edge case logic)
- `ExecuteBodyTemplate()` with invalid Go template syntax (e.g. `{{.`) — falls back to original string without error
- `ExecuteBodyTemplate()` with `{{.Body}}` containing template-like characters — renders as literal
- `BuildTemplateData()` with nil/empty body — `.Body` is empty string, no panic

**Estimated tests:** 7 new test functions

**Implemented:** `src/mock/server_test.go` + `src/mock/templating_test.go` — 12 test functions covering all gaps (plus CreatedAt serialization and round-robin body alternation). Full suite: 73 tests, all pass under `go test -race`.

---

## TGR-03 — Stub parsing and registry concurrency

**Status:** `done` | **Priority:** `[Could]`

**Story:**
> As a developer, I want confidence that YAML edge cases are handled and the registry is safe under concurrent access.

**Coverage gaps:**
- `ParseStubFile` with `.yml` extension (not just `.yaml`) — accepted without error
- `ParseStubFile` with extra unknown YAML fields — ignored silently, no reject
- `Register.Add` concurrent with `Match` — no data race under `-race`
- `Register.Update`/`Remove` concurrent with `Match` — no data race under `-race`

**Estimated tests:** 4 new test functions

**Implemented:** `src/mock/registry_test.go` (3 concurrency tests) + 2 stub tests in `src/mock/stub_test.go`. Full suite: 78 tests, all pass under `go test -race`.

---

## TGR-04 — main.go integration tests

**Status:** `draft` | **Priority:** `[Won't]`

**Story:**
> As a developer, I want integration-level tests for the entry point so that CLI flags, env vars, admin API, and startup output are verified.

**Coverage gaps:**
- Admin REST API — `GET /_admin/endpoints`, `POST /_admin/endpoints`, `GET /_admin/endpoints/{id}`, `PUT /_admin/endpoints/{id}`, `DELETE /_admin/endpoints/{id}` via `httptest`
- Admin REST API — `POST` with invalid JSON returns 400
- Admin REST API — `DELETE` on non-existent ID returns 404
- Env var overrides — `PORT`, `STUBS_DIR`, `LOG_DIR` take precedence over defaults
- Startup banner — renders correctly with 0, 1, and many endpoints
- Per-request line format — matched returns `(endpoint-id)`, unmatched returns `(unmatched)`
- Graceful shutdown — SIGINT triggers clean shutdown message

**Estimated tests:** 8-10 new test functions (requires `httptest.Server` + `main.go` refactor for testability)*