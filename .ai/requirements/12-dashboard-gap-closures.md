# DASH-002..007 — DASH-001 gap-analysis closures (D2-D7)

Requirement file owner: BA. Phase: bugfix/cleanup (post-Phase-7). Status: see per-story.

> All seven decisions from the DASH-001 (TUI dashboard) gap analysis were **agreed by the
> stakeholder 2026-08-05** ("agree on your recommendations (D2-D7)"). D1 (UXR-014 selection
> bug) lives in `09-feed-selection-bug.md` (design approved 2026-08-05, verified 2026-08-06).
> This file holds D2-D7 as ready stories. D3/D5 are doc changes executed by the BA on
> 2026-08-05; D6 is backlogged (`.ai/backlog/observer-panic-policy.md`); D2/D4/D7 are handed
> to the Developer. Sequencing constraint: **D4 depends on D1** (its "selected row" is only
> well-defined after the UXR-014 `sel` mechanism lands); **D7 runs last** (model.go/render.go
> are touched by D1/D2/D4 — gofmt after, not before).
>
> **Update 2026-08-06:** D2/D4/D7 implemented by the Developer and **QA-verified** (all ACs
> PASS, 0 bugs; 319/319 tests, race + vet + gofmt clean; see `.ai/done.md` 2026-08-06).

---

## DASH-002 — Feed timestamps rendered in local time `[Must]`

**Status:** `verified` (Developer 2026-08-06; QA AC1-AC5 PASS, 0 bugs) | **Corresponds to:** decision D2

**User Story:**
> As a developer watching the TUI dashboard, I want the request-stream timestamps to use the
> same local clock as the WS-016 per-request line output, so that the live view and the
> logged line output agree without me doing timezone math.

**Context:**
- `mock.FormatRequestLine` (WS-016, `handler.go:186-189`) parses the UTC `LogEntry.Timestamp`
  and renders it with `t.Local()` — the line output shows **local** time.
- The dashboard renders the same timestamps **without** `.Local()`:
  - `streamRow` — `render.go:392-396` (`t.Format("15:04:05")`)
  - `summaryLine` — `render.go:588-592` (same)
- Result on any non-UTC host: the TUI shows UTC while the line output shows local — a
  visible mismatch. (UTC hosts are indistinguishable — hence the no-regression AC below.)

**Design intent (approved):** one-line change per function — `t.Local().Format("15:04:05")`,
mirroring `FormatRequestLine`. Parse-failure fallback (`"??:??:??"`) stays byte-identical.

**Acceptance Criteria:**
```
AC1 — Given a host whose local timezone is not UTC (e.g. TZ=America/New_York)
     When  the dashboard renders a stream row in the wide layout
     Then  the Time column shows the entry's timestamp in local time, matching the
           WS-016 FormatRequestLine output for the same entry

AC2 — Given a host whose local timezone is not UTC
     When  the dashboard renders a summary line in the vertical layout
     Then  the timestamp shows local time (same expectation as AC1)

AC3 — Given a UTC host (TZ=UTC)
     When  either row type renders
     Then  the output is byte-identical to the pre-fix output (no regression)

AC4 — Given an entry whose Timestamp fails RFC3339Nano parsing
     When  either row type renders
     Then  the existing "??:??:??" fallback is shown unchanged

AC5 — Tests must set an explicit fixed location (e.g. time.FixedZone / TZ env via
     t.Setenv) rather than relying on the CI host's local timezone
```

**Non-blocking follow-up (QA, 2026-08-06):** AC3 (UTC host byte-identical) and AC4
(parse-failure fallback) have **no dedicated unit test** — verified by reasoning + PTY.
Two small tests recommended: pin `TZ=UTC` and compare pre/post-fix bytes; render a row with
an unparseable `Timestamp` and assert `??:??:??`.

---

## DASH-003 — Correct WS-010 ADR live-update description (doc drift) `[Must]`

**Status:** `done` (BA, 2026-08-05) | **Corresponds to:** decision D3

**User Story:**
> As a maintainer, I want the architecture ADR to describe the live-update mechanism that
> actually shipped, so future work isn't misled by a non-existent pump design.

**Context:** the WS-010 ADR in `.ai/architecture.md` claimed `StatsTracker.Notify()`
(buffered(1), coalescing) signals a pump goroutine that calls `Program.Send` at ≤10 Hz. The
implementation is a **fixed 1 s `tea.Tick` poll** (`model.go` `interval: time.Second`,
`refreshCmd`), no pump, no `Program.Send`. `Notify()` is defined but **not consumed** by the
dashboard.

**Work performed (BA):**
- `.ai/architecture.md` — WS-010 "Live updates" bullet rewritten to "fixed 1s poll"; notes
  `Notify()` is retained unused as a seam.
- `.ai/requirements/04-backlog.md` — WS-010 design-notes bullet corrected the same way
  (marked historical).

**Acceptance Criteria:**
```
AC1 — architecture.md WS-010 ADR states the fixed 1 s tea.Tick poll and explicitly
      says there is no pump goroutine and Program.Send is never used for refresh

AC2 — The ADR notes StatsTracker.Notify() exists but is not consumed by the dashboard
      (retained as a seam)

AC3 — No remaining "pump goroutine" / "≤10 Hz" claim in .ai/architecture.md or
      .ai/requirements/04-backlog.md
```

---

## DASH-004 — `i` inspector works standalone on the selected row `[Should]`

**Status:** `verified` (Developer 2026-08-06, after D1; QA AC1-AC6 PASS, 0 bugs) | **Corresponds to:** decision D4

**User Story:**
> As a developer using the dashboard, I want to press `i` on any selected stream row without
> expanding it first, so that inspecting a request is one keystroke instead of
> enter-then-i.

**Context (current behavior):**
- `i` is a no-op unless a row is already expanded: guard `if m.expandedID == "" { return m, nil }`
  at `model.go:427-430`.
- The inspect detail renders **inline** keyed on `expandedID`: `buildFeed` at `render.go:977`
  (`if e.RequestID != "" && e.RequestID == m.expandedID` → `m.inspectLines(e)`).
- So "make `i` standalone" is **not** a one-line guard drop — the render path and the
  open-inspect positioning (`expandedSummaryLine()`, `model.go:451-453`) both key off
  `expandedID`. When opening standalone, the developer must target the *selected* entry
  (D1's `sel`/`selectedIdx()` mechanism) instead.

**Design intent (approved shape — Developer may refine):**
- Drop the `expandedID == ""` guard so `i` opens inspect mode on the currently selected row.
- When the target row isn't expanded, the inspect detail must still render for it (e.g.
  introduce an `inspectID` distinct from `expandedID`, or set `expandedID` to the selected
  row's RequestID as an implementation detail — Developer's choice, behavior is what's locked).
- Dismissal (`i` again / `esc` / `c`), position-restore, and live-follow semantics stay as
  today. Empty feed → `i` is a safe no-op (no panic).
- Sequencing: implement **after** D1 (UXR-014) so "selected row" is defined in both the
  fits-on-screen (`sel`) and overflowing (`YOffset`) cases. Do not start before D1's
  `sel`/`selectedIdx()` machinery is merged.

**Acceptance Criteria:**
```
AC1 — Given a stream row is selected but NOT expanded
     When  I press i
     Then  the inspector opens for THAT row (request headers/query/body + response
           detail) without any prior enter press

AC2 — Given the feed is empty (or no row is selected)
     When  I press i
     Then  nothing happens — no panic, no crash, state unchanged

AC3 — Given a row IS expanded
     When  I press i
     Then  behavior is unchanged from today (inspect toggles on the expanded row,
           i again dismisses, esc/c collapse as before)

AC4 — Given inspect is open on a row that is not the first
     When  it renders
     Then  the viewport pins to that row's summary line so the detail is visible
           (same positioning guarantee as today's expanded case)

AC5 — Given inspect is open
     When  I dismiss it (i / esc / c)
     Then  the prior scroll position and live-follow state are restored exactly as
           in today's expanded-inspect flow

AC6 — No list/textinput widget is introduced (viewport-only contract maintained;
     consistent with UXR-008 and the D1 fix)
```

---

## DASH-005 — DASH-001 doc note: `--max-log-body` vs inspector `[Must]`

**Status:** `done` (BA, 2026-08-05) | **Corresponds to:** decision D5

**User Story:**
> As a maintainer, I want the DASH-001 notes to document that the inspector shows the
> capped body when `--max-log-body` is set, so nobody files a bug for a documented
> interaction.

**Work performed (BA):** added a "`--max-log-body` vs the inspector (doc note, DASH-005)"
note to the DASH-001 Notes-for-Developer section in `10-admin-and-dashboard-features.md`.

**Acceptance Criteria:**
```
AC1 — 10-admin-and-dashboard-features.md (DASH-001 notes) documents that the capture cap
      bounds body/response_body in the logger, stats ring, and inspector, and never the
      wire response or echo/template rendering
```

---

## DASH-006 — Observer panic policy `[Could]`

**Status:** `backlog` | **Forgejo:** [#8](https://forgejo.abojaber.com/Projects/simuhook/issues/8) | **Corresponds to:** decision D6

**User Story:**
> As an operator, I want request handling to be robust against an observer panicking, so
> that a bug in a logging/stats observer can never kill a connection or starve later
> observers.

**Context:** observers run synchronously after the response write (`handler.go:172-174`,
WS-017). No guard today. Decision D6: **backlog item**, not implemented now.

**Location:** `.ai/backlog/observer-panic-policy.md` (full spec, candidate approaches, ACs,
steps, status checklist).

---

## DASH-007 — gofmt drift pass on flagged files `[Must]`

**Status:** `verified` (Developer 2026-08-06, ran last; QA AC1-AC3 PASS) | **Corresponds to:** decision D7

**User Story:**
> As a maintainer, I want `gofmt -l src/` clean so formatting drift doesn't accumulate and
> reviewers aren't distracted.

**Context:** `gofmt -l src/` flags 5 files: `src/dashboard/dashboard_test.go`,
`src/dashboard/model.go`, `src/dashboard/render.go`, `src/main.go`, `src/mock/admin.go`
(`admin.go` also dies in ADM-002 — if it is deleted first, the pass covers the other four).
model.go and render.go are in-flight for D1/D2/D4, so **run this after those land and before
commit** — never concurrently.

**Acceptance Criteria:**
```
AC1 — Given D1/D2/D4 have landed, when gofmt -w is run on the flagged files, then
      gofmt -l src/ returns nothing (clean)

AC2 — gofmt -w introduces no behavioral changes: the full test suite passes after the
      pass (go test -v -race ./src/...)

AC3 — If src/mock/admin.go is deleted by ADM-002 before this task runs, the pass covers
      the remaining flagged files only (admin.go not resurrected)
```
