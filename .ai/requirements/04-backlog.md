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

**Out of scope:** WS-011 forwarding (separate story), multi-screen navigation, charts beyond ASCII bars, SSE/web UI.

---

## WS-017 — Duplicate mock.Server handler (Architect finding B4)

**Status:** `draft` | **Priority:** `[Could]`

**Story:**
> As a developer, I want a single source of truth for the request-handling path so that mock.Server and the main.go inline handler do not drift.

**Context (Architect finding):**
- `mock.NewServer`/`mock.Server.handler` is referenced only from `server_test.go`. main.go never calls it.
- `mock.Server.handler` has no `RecordRequest`/`Logger` calls and differs from the live inline handler in main.go (no logging, no stats, different template-data handling).
- Two divergent implementations of the core matching path are a latent correctness risk — `server_test.go` can pass while the shipped handler differs.

**Options:** unify the two handlers into `mock.Server` (preferred) or delete `mock.Server` and move tests to the real handler. Decision deferred.

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