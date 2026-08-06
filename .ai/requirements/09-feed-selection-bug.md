# UXR-014 — Feed selection stuck on the first entry when content fits the viewport

Requirement file owner: BA. Status: `verified` (implemented by Developer 2026-08-06; QA verified all AC1-AC6 PASS, 0 bugs — see `.ai/done.md` 2026-08-06). Phase: bugfix.

## 0. Origin

Stakeholder report (2026-08-05): "i cannot navigate 'live request stream' items." Investigated
and reproduced by the BA — root cause confirmed via unit tests (`TestReproSelectionStuckOnFew`,
`TestReproAnyKeyResumes`, both since removed — reproduction steps below are sufficient to
recreate them) and live PTY sessions against the built binary (WezTerm + Ghostty, both wide
and narrow layouts).

## 1. Root cause

UXR-008 derives the "selected" (highlighted, `enter`-expandable, `i`-inspectable) entry purely
from viewport scroll position: `selectedIdx = feedLineToEntry[view.YOffset]`. This is correct
and elegant **when the feed has more entries than fit in the viewport** — scrolling brings a
different entry to the top line, so scroll position and selection are equivalent.

It breaks down whenever `total rendered lines <= viewport height` (i.e. **the feed has few
entries — the common case for light traffic, a fresh server, or any test with a handful of
requests**): `viewport.Model.maxYOffset()` is `max(0, len(lines)-Height)`, which is `0` in this
case. `YOffset` is permanently pinned to `0`, `AtBottom()` **and** the (unexported) all-visible
condition are simultaneously and permanently true, and:

- **Bug A (primary):** `j`/`k`/arrow keys/`pgup`/`pgdn` are forwarded to `m.view.Update` but can
  never move `YOffset` away from `0` — the topmost (and only ever) "selected" entry is always
  the first one. The user cannot select, expand (`enter`), or inspect (`i`) any entry but the
  very first, no matter how many keys they press. This matches the report exactly: "i can not
  select any item but the first top one."
- **Bug B (side effect, same root cause):** `handleKeyMsg`'s fallthrough —
  ```go
  m.view, cmd = m.view.Update(msg)
  if m.view.AtBottom() { m.liveFollow = true }
  ```
  — re-checks `AtBottom()` unconditionally after *every* unhandled key. Since `AtBottom()` is
  trivially always `true` when content fits, **any key silently resumes live-follow**, even
  immediately after explicitly pausing with `g`. Matches the report: "actually any key may
  Resume."

Confirmed via reproduction:
```go
// 2 entries, 80x24 (plenty tall for 2 rows). Press j/down repeatedly.
// Result: YOffset=0, selectedIdx=0 after every key — never changes.

// Press 'g' (pause) then 'j' (ordinary nav key).
// Result: liveFollow flips back to true after 'j' alone — no scrolling involved.
```

## 2. Impact

**High.** This is not an edge case — it is the default state for:
- a freshly started server before traffic ramps up,
- any manual/exploratory testing with a handful of requests,
- low-volume integrations.

Any of these leaves `enter` (expand) and `i` (inspect) — the dashboard's core debugging
features — usable **only on the single oldest visible entry**, with no way to reach anything
else. Combined with Bug B, `g` (pause-stream) is also unreliable in this same common state.

## 3. Acceptance criteria

**AC1 — Selection reachability:** Given a feed with N entries where N is small enough that all
N fit within the viewport height, when the user presses the down-navigation keys enough times,
then the selection must be able to reach every one of the N entries (not just the first).

**AC2 — Selection reachability (up):** Given the selection has been moved away from the first
entry per AC1, when the user presses the up-navigation keys enough times, then the selection
must be able to return to the first entry.

**AC3 — No regression on the existing (overflowing) case:** Given a feed with more entries than
fit in the viewport (the case UXR-008 already handles correctly), all existing scroll/selection
behavior (sticky-bottom live-follow, `enter` expand/collapse, `i` inspect, `g`/`G`) must remain
byte-identical — this bug's fix must not touch that path's behavior or tests.

**AC4 — Live-follow only resumes on genuine bottom-ward movement:** Given live-follow is
explicitly paused (`g`, or scrolling away from the bottom), when the user presses a key that
does **not** move the viewport (e.g. it's already pinned, or the key is unbound), then
live-follow must **stay paused** — it must only resume when a key **actually scrolls the
viewport to the bottom**.

**AC5 — Interaction with expansion/inspect:** Selecting a non-first entry (per AC1) and then
pressing `enter`/`i` must expand/inspect **that** entry, not the first one. Collapsing must
return to the correct prior scroll/selection state (consistent with existing UXR-008 semantics).

**AC6 — No new list widget:** Consistent with the existing "viewport only, no
list/textinput" architectural contract *if reasonably achievable* — flagged to the Architect as
an open question below since this bug's existence suggests the constraint may need to flex.

## 4. Design decision (approved by stakeholder 2026-08-05)

The Architect subagent was invoked three times and returned empty results each time (tooling
failure), so the BA produced the design with full code context and it was approved by the
stakeholder. Recorded here as the signed-off design for the Developer.

**Approach: explicit `sel` selection index, scoped to the fits-on-screen case.**

The overflowing case stays **byte-identical** (AC3) — selection keeps deriving from
`view.YOffset`, scroll keys keep falling through to the viewport. Only the broken
fits-on-screen case gets a new mechanism.

**Rejected alternative (padded content):** appending blank padding lines to the content so the
logical height exceeds the viewport breaks bottom-anchored live-follow (`GotoBottom` would pin
a blank line at top, hiding the newest entry); prepending padding breaks `g`/top-anchoring.
Rejected in favour of an explicit `sel` int, which leaves viewport geometry untouched.

**Exact changes in `src/dashboard/model.go`:**

1. New field `sel int` — selected entry index into `feedEntries` (default 0).
2. New helper `feedFits() bool` — `len(feedLineToEntry) <= m.view.Height`.
3. `selectedIdx()`: fits → return clamped `m.sel`; overflow → unchanged
   (`feedLineToEntry[view.YOffset]`).
4. `renderFeed()`: after `SetContent`, overflow → `sel = feedLineToEntry[view.YOffset]`;
   fits → clamp `sel` into `[0, len(entries)-1]` (selection survives ticks/filters, never
   snaps back to 0).
5. Nav keys `j`/`k`/`↑`/`↓` (non-prompt): fits → `sel = clamp(sel ± 1)`; overflow → fall
   through to `m.view.Update` as today, then sync `sel` from the new `YOffset`.
6. **Bug B fix (AC4):** in the overflow fallthrough, capture `before := view.YOffset`; only
   set `liveFollow = true` when `after != before && view.AtBottom()`.
7. `g`/`G`: fits → `g` sets `sel = 0` (+ pause), `G` sets `sel = len(entries)-1` (+ resume);
   overflow → unchanged `GotoTop`/`GotoBottom` + sync `sel`.
8. `enter`/`i`/`esc`/`c`/collapse: no logic change — they already operate on `selectedIdx()`,
   which now returns the moved `sel` in fits mode (AC5).

**`render.go` needs no changes** — `buildFeed(sel)` already takes `sel` as a parameter, so the
highlight renders correctly in both layouts.

**Endpoints panel (open question #3):** `endpointsView` (shift+↑/↓, `K`/`J`) has the same
latent limitation when few endpoints fit, but no live impact today (rows aren't individually
selectable/expandable). **Not fixed now; deferred to backlog** if rows ever become interactive.

## 5. Open questions for the Architect (resolved 2026-08-05)

1. How should "selection" move independently of `AtBottom()`/`AtTop()` becoming trivially true
   when content is short, while still satisfying (or consciously revising) the "viewport only"
   contract? Candidate directions to evaluate (not prescriptive):
   - Decouple the scrolling engine's logical `Height` from the panel's rendered/bordered height
     (pad the *rendered output* with blank lines outside the scrollable region, as
     `renderStreamPanel` already does for the opposite case — fewer content lines than `feedH`).
   - An explicit `selectedOffset` int distinct from `view.YOffset`, reconciled with scroll
     position only when content actually overflows (bigger departure from UXR-008's design).
   - Something else the Architect judges better once inspecting `render.go`/`model.go` directly.
2. Confirm the Bug B fix (`if m.view.YOffset != beforeOffset && m.view.AtBottom()`) is the right
   surgical shape, or propose an alternative.
3. Does the endpoints panel (`endpointsView`, `shift+↑/↓`/`K`/`J`) have the same latent issue
   when there are few endpoints? (Likely no live impact today since endpoints aren't
   individually selectable/expandable, but worth a one-line confirmation either way.)

**Resolutions:** Q1 → explicit `sel` index (see §4); padded-content direction evaluated and
rejected (breaks bottom/top-anchored live-follow). Q2 → yes, the `beforeOffset` guard shape is
correct (see §4 #6). Q3 → yes, latent but no live impact; deferred to backlog.

## 6. Status

- 2026-08-05: reported by stakeholder, reproduced and root-caused by BA (see reproduction
  snippets above). Handed to Architect for a design proposal before implementation.
- 2026-08-05: Architect subagent returned empty results three times (tooling failure); BA
  produced the design (§4) with full code context. **Design approved by stakeholder**
  (via chat). Story status → `ready`, handoff to Developer.
- 2026-08-06: implemented by Developer (see §4 steps 1-8; `sel` field + `feedFits`/
  `clampSel`/`selectedIdx`/`selectedIdxFromOffset`, nav-key + `g`/`G` handling, Bug B
  guard). QA verified **AC1-AC6 all PASS** (319/319 suite, race + vet + gofmt clean,
  PTY-verified wide & narrow). Status → `verified`.
