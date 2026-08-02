# UX v-next — Implementation Plan

**Status:** `draft` | **Owner:** BA | **Contract:** `.ai/ui-ux-dashboard.md` | **Date:** 2026-08-02

> This is the implementation plan for the v-next UX features described in the design contract. Each feature is a user story with a unique ID, MoSCoW tag, and acceptance criteria. Implementation is phased so each phase is independently shippable and testable.

---

## 0. Scope decision (resolve contract inconsistency)

The design contract lists `enter` → expand feed row in §4.1 (key bindings) **and** in §8 (Out of Scope). **Resolved:** `enter` row expansion requires a selection cursor on the viewport, which the contract explicitly ruled out ("bubbles v1.0.0 — viewport only — no list/textinput"). Recommendation: **defer row expansion to a later phase** and keep `enter` = apply-filter only. This keeps the plan aligned with the ratified viewport-only decision.

**Recommended MVP of the UX plan (this document):** features UXR-001 through UXR-006. Row expansion (UXR-007) is a stretch phase pending a design decision on viewport selection.

---

## 1. Stories

### UXR-001 — Status-code tinting in feed `[Should]`

**Status:** `ready` | **Phase:** 1

**Story:** As a developer, I want status codes tinted by class so that I can scan the feed for errors at a glance.

**AC:**
```
Given a feed with entries returning 200, 302, 404, and 500
When  the feed renders
Then  each status segment is styled: 2xx green, 3xx cyan, 4xx yellow, 5xx red, others muted
And   only the status segment is colored (not the whole line)
```

**Notes:** pure render change. `tint(status int) string` helper in `render.go` wrapping just the status in lipgloss. No model changes. 1xx/other → muted.

---

### UXR-002 — Per-endpoint request counts + p95 `[Should]`

**Status:** `ready` | **Phase:** 1

**Story:** As a developer, I want each endpoint row in the panel to show its request count and p95 latency so that I can spot hot/expensive endpoints without switching views.

**AC:**
```
Given an endpoint with N matched requests and recorded latency
When  the endpoints panel renders
Then  the row shows "requests: N  p95: X.Xms" aligned right
And   endpoints with zero requests show "requests: 0" (no p95 or "—")
And   the values update on tick without reordering rows
```

**Notes:** data already available via `Snapshot().EndpointLatency map[string]LatencyStats` + `HitCounts map[string]uint64` (WS-010a). Zero mock changes. Endpoint row width grows; keep tabular alignment.

---

### UXR-003 — Live-follow pause/resume (`L`) `[Should]`

**Story:** As a developer, I want to freeze the feed while inspecting a request so that new traffic does not yank the view.

**AC:**
```
Given the feed is following live
When  I press L
Then  live-follow pauses; footer shows "⏸ live" (or "⏸ paused"); new requests do not scroll the view
When  I press L again
Then  live-follow resumes; view re-anchors to the bottom; footer shows "▶ live"
And   L does nothing while the filter prompt is active
And   pressing g (GotoBottom) while paused resumes live-follow
```

**Notes:** new `liveFollow bool` (default true) on Model. `syncFeed()` skips `GotoBottom` when paused. `g`/`G` (viewport scroll keys) re-enable live-follow — needs a hook: check `view.AtBottom()` after viewport updates. Footer swaps `▶ live` / `⏸ paused`.

---

### UXR-004 — Collapsible endpoints panel (`e`) `[Should]`

**Story:** As a developer, I want to collapse the endpoints panel so that short terminals have more room for the feed.

**AC:**
```
Given the endpoints panel is open
When  I press e
Then  the panel collapses to a single "Endpoints: (9) ▾" line; feed gains the freed rows
When  I press e again
Then  the panel expands back to the full list
And   e does nothing while the filter prompt is active
```

**Notes:** new `showEndpoints bool` (default true). `headerHeight()` must subtract the panel rows when collapsed; `syncFeed()`/`WindowSizeMsg` re-size the viewport accordingly.

---

### UXR-005 — Auto-collapse + layout adaptation `[Should]`

**Story:** As a developer, I want the dashboard to adapt to short/narrow terminals so that the feed never gets squeezed to zero rows.

**AC:**
```
Given a terminal with height < 20 rows and endpoints registered
When  the dashboard renders
Then  the endpoints panel collapses automatically (still expandable via e)
Given a terminal width < 80 cols
When  the dashboard renders
Then  the config line collapses to one line (admin URL dropped first)
Given a terminal width < 60 cols
When  the dashboard renders
Then  feed lines truncate from the right, dropping the latency segment first
```

**Notes:** auto-collapse is a render-time/init decision based on `m.height`. Config collapse: hide admin URL below 80 cols. Feed truncation: build feed lines then truncate to width (lipgloss `Truncate`), latency segment dropped first — reorder segments so latency is last.

---

### UXR-006 — Prompt UX polish: token highlight + inline validation `[Could]`

**Story:** As a developer, I want live filter feedback while typing so that I can correct mistakes before pressing enter.

**AC:**
```
Given the prompt is active and I type "ep:ping status:2"
When  the prompt bar renders
Then  recognized prefixes (ep:, method:, status:) are highlighted; bare text stays plain
Given the prompt contains an invalid status value, e.g. "status:abc"
When  I press enter
Then  I stay in the prompt and see an inline error "✗ invalid status: abc"
Given the prompt is valid
When  I press enter
Then  the filter applies as today
```

**Notes:** `renderPromptBar()` tokenizes the prompt (reuse `ParseFilter`/token rules in `filter.go` — keep it tea-free). `enter` handler: if parse fails, keep prompting + set `promptErr`. `esc`/backspace clears the error. Pure render + small model state.

---

### UXR-007 — Filter chip in endpoints panel header `[Could]`

**Story:** As a developer, I want the active filter visible in the endpoints panel so that I know the panel is filtered without looking at the footer.

**AC:**
```
Given a filter is active (e.g. ep:ping)
When  the endpoints panel renders
Then  its heading shows a chip like "Endpoints: (9) [ep:ping]"
And   pressing c or esc clears both the chip and the footer filter text
```

**Note:** contract says the panel itself is filtered — per-endpoint rows show only matching endpoints. This is a slightly bigger semantic change; mark as stretch. (If deferred, chip shows unfiltered count but still indicates the active filter.)

---

### UXR-008 — Feed row expansion (`enter`) `[Could]` — DEFERRED

**Story:** As a developer, I want to press enter on a feed row to see its headers/body/query detail.

**AC:** (from contract §9)

**Blocked on:** design decision for viewport row selection (the contract banned list/textinput widgets). Needs a lightweight cursor-overlay approach on the viewport content. Do NOT schedule until UXR-001..006 land and the stakeholder approves the selection approach.

---

## 2. Phasing

| Phase | Stories | Scope | Risk |
|-------|---------|-------|------|
| **1** | UXR-001, UXR-002 | Status tinting + per-endpoint stats (render-only, no layout change) | low |
| **2** | UXR-003, UXR-004 | `L` pause + `e` collapse (Model state + viewport resize) | medium — viewport re-sizing is the fiddly part |
| **3** | UXR-005 | Layout adaptation (auto-collapse, width truncation) | medium — interacts with phase 2 sizes |
| **4** | UXR-006, UXR-007 | Prompt polish + filter chip | low |
| **5** | UXR-008 | Row expansion | deferred — needs design decision |

**Dependency:** phase 3's `headerHeight()`/viewport logic must incorporate phase 2's collapse state — design `headerHeight()` once, in phase 2, to be aware of both.

---

## 3. Files touched

| File | Change |
|------|--------|
| `src/dashboard/model.go` | new fields: `liveFollow`, `showEndpoints`, `promptErr`; key cases `L`/`e`/`g`-hook; `headerHeight()` aware of collapse + auto-collapse; `WindowSizeMsg` re-size |
| `src/dashboard/render.go` | `tint()` helper; per-endpoint counts/p95; footer `▶ live`/`⏸ paused`; config collapse; feed truncation; prompt token highlighting + error |
| `src/dashboard/filter.go` | expose token/prefix rules for highlighting (stay tea-free) |
| `src/dashboard/dashboard_test.go` | new tests per story (see §4) |
| `src/main.go` | none expected (pure dashboard changes) |
| `.ai/architecture.md` | WS-010 ADR additions for new keys/features |
| `.ai/requirements/04-backlog.md` or this file | story status tracking |

---

## 4. Test plan

Per `.ai/rules.md` testing conventions (create test plans from ACs, cover happy path + edge cases, verify each AC explicitly):

- **UXR-001:** `TestRenderStatusTint` — 200/302/404/500/other each render with expected ANSI; only the status substring is wrapped (assert no color on path/label).
- **UXR-002:** `TestRenderEndpointStats` — endpoint with hits shows `requests: N  p95: X.Xms`; zero-hit endpoint shows `requests: 0`; assert no reorder between ticks (IDs still sorted).
- **UXR-003:** `TestModelLiveFollowPause` — `L` toggles `liveFollow`; paused `syncFeed()` preserves offset while content updates; `L` again re-anchors; prompt swallows `L`; `g`/`G` resumes.
- **UXR-004:** `TestModelCollapseEndpoints` — `e` toggles `showEndpoints`; `headerHeight()` shrinks/grows; viewport height adjusts; prompt swallows `e`.
- **UXR-005:** `TestLayoutAdaptation` — short height auto-collapses panel; narrow width collapses config + truncates feed; viewport never gets ≤0 height.
- **UXR-006:** `TestRenderPromptHighlight` — `ep:ping status:2` highlights prefixes; `TestModelPromptInvalidStatus` — `status:abc` + enter stays in prompt with error, valid applies.
- **UXR-007:** `TestRenderFilterChip` — chip appears with active filter, clears on `c`/`esc`.
- **Regression:** all 26 existing dashboard tests + full `go test -race -count=1 ./...` green at each phase boundary.

---

## 5. Delivery & sign-off

1. Each phase ends with: code + tests + full suite green + a live PTY smoke (dashboard renders, keys work, `q` exits rc=0 — reuse the Python `pty` harness pattern).
2. Docs updated at the end: `.ai/architecture.md` (WS-010 ADR: new keys `L`/`e`, layout rules), this file status flip per phase.
3. This plan and UXR-001..007 go through stakeholder sign-off before phase 1 implementation.

---

## 6. Sign-off requests

| # | Decision | Recommended |
|---|----------|-------------|
| 1 | Scope = UXR-001..006 for the first delivery; UXR-007 stretch; UXR-008 deferred | ✅ |
| 2 | `enter` stays apply-filter-only (no row expansion yet) | ✅ |
| 3 | Deliver phase-by-phase with full-suite green at each boundary | ✅ |
