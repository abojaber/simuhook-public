# Dashboard Redesign — Phase 6 (Two-column layout)

**Status:** `verified` | **Owner:** BA | **Date:** 2026-08-03 | **Contract:** `.ai/ui-ux-dashboard.md` + WS-010 ADR

> Redesign of the TUI dashboard to a two-column layout per stakeholder wireframe. The wireframe is a **draft reference only** — where it conflicts with an implemented feature, a signed-off requirement, or a project standard, the implemented behavior wins (stakeholder decision 2026-08-03).
>
> **Design approved by stakeholder 2026-08-03** (all sign-off items S1–S4 ✅). Threshold: **`wideMinWidth = 100`, `wideMinHeight = 14`**.

## 0. Stakeholder decisions (2026-08-03)

| # | Decision | Outcome |
|---|----------|---------|
| D1 | Column split | **Fixed 40/60 split** (endpoints : stream). Left panel gets its own `bubbles/viewport` so it scrolls independently. Below the width threshold, fall back to the current vertical layout (UXR-005 auto-collapse preserved). |
| D2 | Stream columns | Wireframe is an example. **Stream table = Time \| Method \| Path \| Stub \| Status** (latency lives only in the endpoints p95 column). **UXR-008 `enter` detail expansion MUST keep working** — detail renders below the selected table row. |
| D3 | Status-class filter chips | **Ignored.** No `All / 2xx / 4xx / 5xx` chips. Existing `/` text prompt (ep:/method:/status:) is the only filter mechanism. No filter.go changes. |
| D4 | Status indicator | **Removed.** No `[Status: Active]`. Only the `[ Pause Stream ]` control in the title row. |
| D5 | Endpoints table | **Keep `Req` column** (preserves signed-off UXR-002). Table = Method \| Endpoint \| Stub \| Req \| p95. |
| D6 | Boxes | **lipgloss `RoundedBorder`.** Wireframe is a draft; ignore it on any conflict with implementing features or standards. |

---

## 1. Stories

### UXR-009 — Two-column dashboard layout `[Should]` — PHASE 6

**Story:** As a developer, I want the dashboard split into two fixed side-by-side panels with SERVER INFO and METRICS rows up top, so that I can monitor endpoint stats and the live request stream without scrolling.

**Status:** `verified` | **Phase:** 6 | **Implemented:** `544db99` | **QA:** verified 2026-08-03 (16/16 ACs PASS, no bugs)

**AC:**
```
Given a terminal with width >= the layout threshold (Architect to compute the
      minimum width where all table columns fit; provisional 90)
When  the dashboard renders
Then  the top shows, in order:
        1. Title row: "Simuhook Dashboard" + the Pause Stream control (UXR-010)
        2. SERVER INFO row: [Address] [Stubs <dir> (N)] [Uptime] [Req] chips
        3. Log dir + Admin URL on a muted line (dropped when width shrinks)
        4. METRICS & LATENCY row: [Avg] [Min] [Max] [p95] chips
And   below the info rows the screen splits at a FIXED 40/60 ratio:
        left  = REGISTERED ENDPOINTS panel
        right = LIVE REQUEST STREAM panel
Given more endpoints than the left panel's height allows
When  the panel renders
Then  it scrolls independently (own viewport) without moving the stream panel
Given a terminal narrower than the threshold
When  the dashboard renders
Then  it falls back to the current vertical layout (title + config + stats +
      endpoints list + feed) exactly as today, including UXR-005 auto-collapse
And   every existing key binding keeps working in BOTH layouts:
      / f (prompt), L (pause), e (collapse left panel in new layout; collapse
      endpoints in fallback), esc, c, r, ?, enter (expand), scroll keys
```

**Notes:**
- New layout is the primary one at width ≥ threshold; the vertical layout becomes the narrow fallback. Both layouts are supported in the same `View()` via a width branch.
- `e` in the new layout toggles the LEFT panel (stream takes full width); in the fallback it keeps today's endpoints-list collapse.
- UXR-005 short-height auto-collapse still applies: height < 20 → auto-collapse the left panel (new layout) / endpoints list (fallback).
- Model additions: second `viewport.Model` for the left panel (`endpointsView`), and layout-aware height math replacing `headerHeight()`.
- **Test impact (explicit):** existing tests that assert the vertical layout must construct models with width < threshold. This is a test-only change to width constants; no behavior assertions change. All 95 tests stay green.

---

### UXR-010 — Pause Stream button `[Should]` — PHASE 6

**Story:** As a developer, I want a visible Pause Stream control in the title bar so that I can pause/resume live-follow with one key and see the state at a glance.

**Status:** `verified` | **Phase:** 6 | **Implemented:** `544db99` | **QA:** verified 2026-08-03 (16/16 ACs PASS, no bugs)

**AC:**
```
Given live-follow is on (the default)
When  the dashboard renders
Then  the title row shows a bordered [ Pause Stream ] button
When  I press L
Then  the button becomes [ Resume Stream ]; live-follow pauses; the footer
      shows the ⏸ paused indicator; new requests do not move the view
When  I press L again, or scroll to the bottom (↓ / j / g / G / pgdn)
Then  live-follow resumes; the button returns to [ Pause Stream ]
And   there is NO status indicator anywhere (no Status: Active/Paused)
And   L still does nothing while the filter prompt is active
```

**Notes:**
- The button is a visual affordance for the existing `L` key. **No mouse support** — the UI stays keyboard-first per the WS-010 contract.
- Removes the `Status:` concept entirely; only the pause control remains.

---

### UXR-011 — Stream + endpoints table format `[Should]` — PHASE 6

**Story:** As a developer, I want the live stream and the endpoint list rendered as columnar tables so that I can scan requests and endpoints quickly.

**Status:** `verified` | **Phase:** 6 | **Implemented:** `544db99` | **QA:** verified 2026-08-03 (16/16 ACs PASS, no bugs)

**AC:**
```
Given recent logs exist and width >= the layout threshold
When  the right panel renders
Then  each entry is a table row: Time | Method | Path | Stub | Status, with the
      status tinted by class (UXR-001 colors preserved byte-for-byte)
And   the Path column truncates with "…" when it overflows; all other columns
      are fixed-width
Given an entry with no matching stub
When  it renders
Then  the Stub column shows "unmatched" (or the webhook label if non-empty)
Given endpoints are registered
When  the left panel renders
Then  it shows a REGISTERED ENDPOINTS table: Method | Endpoint | Stub | Req | p95
      (Req = HitCounts[ID], p95 = EndpointLatency[ID].P95; zero-hit endpoints
      show Req 0 and p95 —)
Given a feed row is at the top visible line
When  I press enter
Then  the row expands below it showing request_id, matched, timestamp, headers,
      query, body (UXR-008 behavior, detail content, and auto-clear preserved)
And   every piece of log-derived text in BOTH tables (paths, stub labels,
      header/query keys+values, bodies) passes through sanitize() — no ANSI
      injection into the TUI
```

**Notes:**
- Latency per stream row is dropped (it lives in the endpoints p95 column).
- The `feedLineToEntry`/`feedEntryToLine`/`expandedID`/`selectedIdx` machinery is line-based and carries over unchanged; only the summary line's rendering changes from a loose line to a table row.
- Stub column width: pad/truncate to a fixed width so rows align.

---

## 1b. Design approval (2026-08-03) — decisions locked

| # | Decision | Outcome |
|---|----------|---------|
| S1 | Width threshold | **`wideMinWidth = 100`** (40/60 split + 5 endpoint columns force 98; 100 = 2 cells headroom). Below 100 → byte-identical vertical fallback. `wideMinHeight = 14`. |
| S2 | Boxes | `RoundedBorder` on the two panels only; chips/button are single-line `[ ... ]` pills (bordered chips cost 6 rows). |
| S3 | Left-panel scroll | `shift+↑`/`shift+↓` aliased `K`/`J`; **no focus state** — existing keys always drive the stream viewport. |
| S4 | Asserted literals | Panel titles `REGISTERED ENDPOINTS (N)` / `LIVE REQUEST STREAM`; column headers `TIME METHOD PATH STUB STATUS` / `METHOD ENDPOINT STUB REQ P95`; chips `[ Address :8080 ]` etc. |
| S5 | UXR-007 chip | Chip on endpoints panel title when visible; relocates to stream title when hidden. |
| S6 | Security | `sanitize()` on `ep.Path`/`ep.ID` too (unauthenticated admin API = injection vector). |
| S7 | `g`/`G` | Not bound in bubbles v1.0.0 — correct the ADR/help docs; backlog the binding. UXR-010 AC lists `g`/`G` as resume triggers → rely on the `AtBottom()` path only. |
| S8 | UXR-002 wording | Literal `requests: N  p95: Xms` → `REQ`/`P95` column headers in wide layout; values byte-preserved. |
| S9 | Left-panel scroll keys in help | Help gains one line (63 cells) documenting `shift+↑/↓ K/J`; `e` line kept byte-identical ("collapse endpoints"). |
| S10 | Existing tests | 13 functions, 15 numeric literals (`newModel(...,120,30)` → `80,30`; `TestWindowSizeMsg` 120→80) — width-only, zero assertion changes. |

## 2. Phasing


| Phase | Stories | Scope | Risk |
|-------|---------|-------|------|
| **6** | UXR-009, UXR-010, UXR-011 | Two-column layout + Pause control + table formats | medium-high — second viewport, width-threshold branching, height math, test-width refactor |

**Dependency:** none — pure `src/dashboard/` work. No `mock/` changes (all data already in `StatsSnapshot`).

## 3. Files touched (expected)

| File | Change |
|------|--------|
| `src/dashboard/model.go` | `endpointsView viewport.Model`; layout-aware height math; `e` handling per layout; narrow fallback branch |
| `src/dashboard/render.go` | `renderServerInfo`, `renderMetrics`, `renderEndpointsTable`, `renderStreamTable`; RoundedBorder panels; summary line → table row |
| `src/dashboard/dashboard_test.go` | new tests per story; existing vertical-layout tests set width < threshold |
| `.ai/architecture.md` | WS-010 ADR additions for new layout + threshold |
| `.ai/requirements/06-dashboard-redesign.md` | this file, status flips |

## 4. Non-goals (explicit)

- No mouse support.
- No quick-filter chips / status-class matching (D3 ignored).
- No Status indicator (D4 removed).
- No backend/mock changes.
- Admin CRUD API extraction (WS-017b) remains in backlog — unchanged.

## 5. Sign-off requests

| # | Decision | Recommended |
|---|----------|-------------|
| 1 | Fixed 40/60 split with independent left-panel viewport; vertical layout becomes narrow fallback | ✅ |
| 2 | Stream table Time/Method/Path/Stub/Status; latency only in endpoints p95; enter-detail preserved | ✅ |
| 3 | No chips, no Status indicator; Pause Stream button = L visual affordance | ✅ |
| 4 | Keep Req column in endpoints table (UXR-002 preserved) | ✅ |
| 5 | lipgloss RoundedBorder; wireframe is draft-only | ✅ |
