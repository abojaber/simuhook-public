# Requirements Index

> Single reference for all story IDs across the 21 requirements files.
>
> **Last updated:** 2026-10-19 | **Total stories:** 65 | **Forgejo:** https://forgejo.abojaber.com/Projects/simuhook

---

## Summary by Status

> Recounted from table rows 2026-10-19 (the previous summary was stale — it summed to 61, not 65).

| Status | Count |
|--------|-------|
| done | 28 |
| verified | 26 |
| in-progress | 1 |
| backlog | 3 |
| draft | 3 |
| wont-do | 2 |
| removed | 2 |

## Summary by Prefix

| Prefix | Domain | Count |
|--------|--------|-------|
| WS | Webhook Stories (core features) | 21 |
| UXR | User Experience / Dashboard UX | 18 |
| MAT | Smarter Matching | 6 |
| TPL | Richer Templates | 2 |
| PROXY | Proxy / Forwarding | 4 |
| ADM | Admin API (removed) | 3 |
| DASH | Dashboard features | 7 |
| TGR | Test Gap Remediation | 4 |

---

## WS — Webhook Stories

| ID | File | Status | Priority | Description |
|----|------|--------|----------|-------------|
| WS-001 | `02-mvp.md` | done | [Must] | Define webhook endpoints via YAML stubs |
| WS-002 | `02-mvp.md` | done | [Must] | Log incoming requests to file (JSON Lines) |
| WS-003 | `02-mvp.md` | done | [Must] | Custom responses with configurable delay |
| WS-004 | `02-mvp.md` | done | [Must] | CLI serve command with graceful shutdown |
| WS-005 | `02-mvp.md` | done | [Must] | Dynamic request matching (path, method, headers, payload, query) |
| WS-006 | `03-v1.md` | done | [Should] | Dynamic payload templating (Go template variables in response body/headers) |
| WS-007 | `03-v1.md` | done | [Should] | Response variability (weighted random / round-robin multiple responses) |
| WS-008 | `03-v1.md` | wont-do | [Won't] | Webhook replay from logs (rejected — out of scope, undefined target) |
| WS-009 | `03-v1.md` | wont-do | [Won't] | Request match conditions (superseded by existing header/query matching) |
| WS-010 | `04-backlog.md` | done | [Could] | Interactive TUI dashboard (umbrella; sub-stories WS-010a/b/c + UXR-001..014) |
| WS-010a | `04-backlog.md` | done | — | StatsTracker latency metrics (count/sum/min/max/p95 via log histogram) |
| WS-010b | `04-backlog.md` | done | — | bubbletea TUI replacing line output (full-screen alternate-buffer dashboard) |
| WS-010c | `04-backlog.md` | done | — | Log feed filtering and scrolling (Filter type, sticky-bottom viewport, key map) |
| WS-012 | `04-backlog.md` | draft | [Could] | WebSocket / SSE endpoint simulation |
| WS-013 | `04-backlog.md` | draft | [Won't] | Built-in webhook templates (Stripe, GitHub, Slack) — rejected, custom-only |
| WS-014 | `04-backlog.md` | backlog | [Could] | REST API to manage config at runtime (consolidated with ADM-001; removed 2026-08-05) — ⚠️ needs Forgejo issue (see ADM-001) |
| WS-015 | `02-mvp.md` | done | [Must] | Echo mode: return request payload as response |
| WS-016 | `03-v1.md` | done | [Should] | CLI dashboard with startup banner and per-request log line |
| WS-017 | `04-backlog.md` | done | [Could] | Single source of truth for request-handling path (extract `mock.NewHandler`) |
| WS-017b | `04-backlog.md` | done→removed | [Could] | Extract admin API into `mock.NewAdminHandler` (implemented 2026-08-03, removed 2026-08-05) |
| WS-018 | `08-dashboard-inspect.md` | done | [Should] | Request/response inspector data model (LogEntry gains ResponseHeaders/ResponseBody) |

## UXR — User Experience

| ID | File | Status | Priority | Description |
|----|------|--------|----------|-------------|
| UXR-001 | `05-ux-vnext.md` | done | [Should] | Status-code tinting in feed (2xx green, 3xx cyan, 4xx yellow, 5xx red) |
| UXR-002 | `05-ux-vnext.md` | done | [Should] | Per-endpoint request counts + p95 latency in endpoints panel |
| UXR-003 | `05-ux-vnext.md` | done | [Should] | Live-follow pause/resume via `L` key |
| UXR-004 | `05-ux-vnext.md` | done | [Should] | Collapsible endpoints panel via `e` key |
| UXR-005 | `05-ux-vnext.md` | done | [Should] | Auto-collapse + layout adaptation for short/narrow terminals |
| UXR-006 | `05-ux-vnext.md` | verified | [Could] | Prompt UX polish: token highlight + inline validation |
| UXR-007 | `05-ux-vnext.md` | verified | [Could] | Filter chip in endpoints panel header |
| UXR-008 | `05-ux-vnext.md` | verified | [Could] | Feed row expansion via `enter` (request detail inline) |
| UXR-009 | `06-dashboard-redesign.md` | verified | [Should] | Two-column dashboard layout (40/60 endpoints/stream split) |
| UXR-010 | `06-dashboard-redesign.md` | verified | [Should] | Pause Stream button in title bar (visual `L` affordance) |
| UXR-011 | `06-dashboard-redesign.md` | verified | [Should] | Stream + endpoints table format (columnar Time/Method/Path/Stub/Status) |
| UXR-012 | `08-dashboard-inspect.md` | backlog | [Should] | Select a registered endpoint to filter the live request stream (decisions signed off, never implemented) |
| UXR-013 | `08-dashboard-inspect.md` | done | [Should] | Select a request and inspect full request + response (superseded by WS-018 + DASH-004) |
| UXR-014 | `09-feed-selection-bug.md` | verified | [Should] | Feed selection stuck on first entry when content fits viewport (superseded by UXR-018) |
| UXR-015 | `14-detail-coloring.md` | verified | [Should] | Colorized request detail (keys/values + request/response body + JSON syntax highlighting) |
| UXR-016 | `15-detail-response.md` | verified | [Should] | Response section in the `enter` request detail |
| UXR-017 | `16-detail-sections.md` | verified | [Should] | Request/response visual separation (separator line, background bands, response key color) |
| UXR-018 | `17-feed-cursor-navigation.md` | verified | [Should] | Unify feed selection: cursor navigation with auto-scroll in overflow mode |

## MAT — Smarter Matching

| ID | File | Status | Priority | Description |
|----|------|--------|----------|-------------|
| MAT-001 | `18-smarter-matching.md` | verified | [Should] | Path glob matching (`**` multi-segment, `*` single segment, `?` single char) |
| MAT-002 | `18-smarter-matching.md` | verified | [Should] | Path regex matching (Go stdlib RE2) |
| MAT-003 | `18-smarter-matching.md` | verified | [Should] | Partial JSON request-body matching (`payload_contains`) |
| MAT-004 | `18-smarter-matching.md` | verified | [Should] | Specificity ordering for overlapping stubs (exact > glob > regex; fewer wildcards wins) |
| MAT-005 | `18-smarter-matching.md` | verified | [Should] | Schema validation, compile-at-load, and TUI display for pattern-based stubs |
| MAT-006 | `18-smarter-matching.md` | verified | [Should] | Documentation and sample stubs for new matching capabilities |

## TPL — Richer Templates

| ID | File | Status | Priority | Description |
|----|------|--------|----------|-------------|
| TPL-001 | `19-richer-templates.md` | verified | [Should] | Realistic data generator functions (27 curated gofakeit funcs) |
| TPL-002 | `19-richer-templates.md` | verified | [Should] | Hand-rolled JSONPath selector (`{{JSONPath "$.user.name"}}` extracts from request body) |

## PROXY — Proxy / Forwarding

| ID | File | Status | Priority | Description |
|----|------|--------|----------|-------------|
| PROXY-001 | `20-proxy-forwarding.md` | in-progress | [Should] | Transparent forwarding fallback (`--proxy /path=https://destination`, longest-prefix wins) — ⚠️ QA blocked: Bug 1 (High) fails AC2 |
| PROXY-002 | `20-proxy-forwarding.md` | verified | [Should] | Full-fidelity logging (raw_headers, upstream_request, upstream_response) |
| PROXY-003 | `20-proxy-forwarding.md` | verified | [Should] | Raw listener-level tee for malformed requests (`RawTeeListener`, `_raw.log`) |
| PROXY-004 | `20-proxy-forwarding.md` | verified | [Should] | Proxy TUI view with structured log entries (dedicated proxy view, key `2`) |

## ADM — Admin API (removed)

| ID | File | Status | Priority | Description |
|----|------|--------|----------|-------------|
| ADM-001 | `10-admin-and-dashboard-features.md` | removed | [Could] | Runtime endpoint management via admin REST API (removed by stakeholder 2026-08-05) |
| ADM-002 | `11-admin-removal.md` | done | — | Remove the admin REST API code from the product (code deletion) |
| ADM-003 | `13-admin-url-residue.md` | verified | — | Stale admin API URL cleanup in TUI + banner |

## DASH — Dashboard

| ID | File | Status | Priority | Description |
|----|------|--------|----------|-------------|
| DASH-001 | `10-admin-and-dashboard-features.md` | done | [Should] | Interactive TUI dashboard (umbrella; implemented via WS-010 + UXR-001..011) |
| DASH-002 | `12-dashboard-gap-closures.md` | verified | [Must] | Feed timestamps rendered in local time |
| DASH-003 | `12-dashboard-gap-closures.md` | done | [Must] | Correct WS-010 ADR live-update description (doc drift — fixed 1s poll) |
| DASH-004 | `12-dashboard-gap-closures.md` | verified | [Should] | `i` inspector works standalone on the selected row |
| DASH-005 | `12-dashboard-gap-closures.md` | done | [Must] | `--max-log-body` vs inspector doc note |
| DASH-006 | `12-dashboard-gap-closures.md` | backlog | [Could] | Observer panic policy — [Forgejo: #8] |
| DASH-007 | `12-dashboard-gap-closures.md` | verified | [Must] | gofmt drift pass on flagged files |

## TGR — Test Gap Remediation

| ID | File | Status | Priority | Description |
|----|------|--------|----------|-------------|
| TGR-01 | `04-backlog.md` | done | [Should] | StatsTracker unit tests (ring buffer, hit counters, snapshot logic) |
| TGR-02 | `04-backlog.md` | done | [Should] | Server and templating edge-case test coverage |
| TGR-03 | `04-backlog.md` | done | [Could] | Stub parsing and registry concurrency test coverage |
| TGR-04 | `04-backlog.md` | draft | [Won't] | main.go integration tests (CLI flags, env vars, startup output) |

---

## Forgejo Issues (active backlog + new work)

| Forgejo | Title | Labels | Status |
|---------|-------|--------|--------|
| [#3](https://forgejo.abojaber.com/Projects/simuhook/issues/3) | TLS support for HTTPS webhook testing | `long-term`, `priority:low` | open |
| [#4](https://forgejo.abojaber.com/Projects/simuhook/issues/4) | Request recording/replay — capture real traffic to YAML stubs | `long-term`, `priority:low` | open |
| [#5](https://forgejo.abojaber.com/Projects/simuhook/issues/5) | Multi-server orchestration — run multiple instances | `long-term`, `priority:low` | open |
| [#6](https://forgejo.abojaber.com/Projects/simuhook/issues/6) | Observability hooks — async webhook callback delivery | `feature`, `priority:medium` | open |
| [#7](https://forgejo.abojaber.com/Projects/simuhook/issues/7) | Observability hooks — synchronous webhook callback (initial) | `feature`, `priority:high` | open |
| [#8](https://forgejo.abojaber.com/Projects/simuhook/issues/8) | Observer panic policy — recover-per-observer | `task`, `backlog`, `priority:low` | open |
| [#9](https://forgejo.abojaber.com/Projects/simuhook/issues/9) | Requirements index — story ID to file mapping | `task`, `priority:high` | open (✅ delivered — `00-index.md` created; issue can be closed) |

**⚠️ Convention gap (2026-10-19):** three `backlog` stories have no Forgejo issue, violating
the backlog convention (`.ai/rules.md`: every backlog item must have a corresponding issue).
No Forgejo CLI is available in this environment to create them — the stakeholder must add:
- **WS-014 / ADM-001** — admin REST API reintroduction (suggested labels: `feature`, `backlog`, `priority:low`).
- **UXR-012** — endpoint-select filter (suggested labels: `feature`, `backlog`, `priority:medium`).
- **WS-012** — WebSocket / SSE endpoint simulation (still `draft`; promote to backlog + issue when scoped).
