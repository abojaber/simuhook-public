# Phase 04 — Backlog

> Future ideas. Could-have and won't-have features.

---

## WS-010 — Interactive TUI dashboard

**Status:** `draft` | **Priority:** `[Could]`

**User Story:**
> As a developer, I want an interactive terminal dashboard showing live request counts, response times, and recent logs so that I can monitor the simulator in real time during demos.

> **Note:** A simpler CLI dashboard (WS-016) was already implemented — boxed startup banner + per-request log lines. This story covers a **full interactive TUI** with real-time screen updates, charts, hit-rate visualization, and keyboard navigation.

Leveraging `bubbletea` or similar Go TUI framework. The existing `StatsTracker` and ring buffer (50 entries) can be reused as the data source.

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