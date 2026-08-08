# UXR-018 — Unify feed selection: cursor navigation with auto-scroll in overflow mode

Requirement file owner: BA. Status: `verified` (stakeholder confirmed scope 2026-08-07; design
approved by stakeholder 2026-08-07; implemented by Developer; QA verified all AC1–AC10 PASS,
366 tests green, live PTY smoke test — see `.ai/done.md`). Phase: bugfix / UX unification.

## 0. Origin

Stakeholder report (2026-08-07): "there is an issue when the request streaming list is to many
and exceed the line number of panel. and when click j or down arrow, it will scroll the list
instead of select items. And below list will be not accessable/nor selectable."

Reproduced and root-caused by the BA from the code (see §1). The stakeholder confirmed the
desired behavior in three explicit decisions (2026-08-07):

1. **Cursor + auto-scroll:** `j`/`↓`/`k`/`↑` move a highlight cursor **entry-by-entry** through
   the feed, with the viewport auto-scrolling to keep the cursor visible.
2. **No implicit live-follow from cursor-at-bottom (may change after testing):** moving the
   cursor to the newest (bottom) entry must **not** silently resume live-follow; the user will
   evaluate and possibly adjust after testing the build.
3. **Unify the two modes:** one selection model everywhere — the old
   "selection = top visible line" derivation in overflow mode is replaced by the explicit
   cursor index used in fits mode.

## 1. Root cause

The dashboard has **two different selection mechanisms** (UXR-008 + UXR-014), split by
`feedFits()`:

- **Fits mode** (rendered feed lines ≤ viewport height): selection is the explicit `sel` index
  into `feedEntries`; `j`/`k`/`↑`/`↓` move it directly (`model.go` nav-key case, UXR-014). This
  works.
- **Overflow mode** (feed lines > viewport height — the stakeholder's report): `j`/`↓` **falls
  through to `m.view.Update`** and scrolls the viewport (`model.go:511–515` → fallthrough at
  `model.go:545`). The "selection" is then **derived** from the scroll position:
  `selectedIdxFromOffset()` returns the entry at the **top visible line**
  (`feedLineToEntry[view.YOffset]`, `model.go:598–611`).

Consequences in overflow mode, matching the report exactly:

- Pressing `j`/`↓` **scrolls content** rather than moving a selection cursor → "it will scroll
  the list instead of select items."
- The highlight is always pinned to the **top visible line**; an entry anywhere below the top
  line can never be highlighted directly, and entries below the visible area can only be
  reached by scrolling them to the top line → "below list will be not accessable/nor
  selectable."

The two-mode split is also confusing: cursor movement and scroll movement are conflated in
overflow mode but decoupled in fits mode.

## 2. Impact

**Medium-High.** The overflow case is the normal state for a busy server (more recent requests
than fit in the panel). Today, users cannot move a highlight through the visible feed, cannot
select an entry in the lower half of the panel without scrolling it to the top, and the two
navigation modes behave differently for the same keys. This blocks efficient use of `enter`
(expand) and `i` (inspect) on mid/bottom entries under load.

## 3. Acceptance criteria

**AC1 — Cursor reaches every entry in overflow mode:** Given a feed with more entries than fit
in the viewport, when the user presses `j`/`↓` repeatedly, then the selection highlight moves
entry-by-entry through the feed to every entry, and the viewport auto-scrolls as needed to keep
the selected entry's summary line visible.

**AC2 — Cursor moves up:** Given an overflowing feed with the selection below the first entry,
when the user presses `k`/`↑` repeatedly, then the selection moves entry-by-entry toward the
first entry, and the viewport auto-scrolls to keep the selected entry visible.

**AC3 — Entry-by-entry, not line-by-line:** Given an expanded entry whose detail block occupies
multiple rendered lines, when the user presses `j`/`↓`, then the selection moves to the **next
entry** (skipping the detail lines), not to the next rendered line.

**AC4 — Highlight follows the cursor, not the top line:** Given the selection is moved with
`j`/`↓` to an entry in the middle or lower portion of the visible area, then the highlight is
rendered on that entry's summary line at its actual position (the top visible line is **not**
automatically selected).

**AC5 — Auto-scroll keeps the cursor visible:** Given the selection is moved past the bottom
edge of the viewport, then the viewport scrolls so the selected entry's summary line becomes
visible; symmetrically for the top edge. The cursor is never left off-screen by a navigation
key.

**AC6 — enter/i operate on the cursor in both modes:** Given the cursor is on any entry, when
the user presses `enter` (expand) or `i` (inspect), then that entry is expanded/inspected —
identically in fits and overflow modes.

**AC7 — One selection model (unification):** Given either a feed that fits or one that
overflows, navigation, highlighting, and `enter`/`i` operate on the same explicit cursor index;
the overflow-mode "selection = top visible line" derivation is removed.

**AC8 — No implicit live-follow from cursor movement (decision 2):** Given live-follow is
paused (e.g. `g` or `L`), when the user moves the cursor to the newest (bottom) entry with
`j`/`↓`, then live-follow must **stay paused** — cursor movement alone must not resume
live-follow. Live-follow resumes only via the explicit `L` key (and existing explicit
`G`-at-bottom semantics if retained — see §5 Q2). New entries arriving while the cursor sits on
the newest entry must not drag the viewport to follow.

**AC9 — g/G with the cursor:** Given overflow mode, when the user presses `g`, then the cursor
jumps to the first entry and the viewport shows the top; when the user presses `G`, then the
cursor jumps to the last entry and the viewport shows the bottom (live-follow semantics per
§5 Q2).

**AC10 — No regressions:** Existing behavior must keep working: `enter` expand/collapse, `i`
inspect (standalone + on expanded row, DASH-004), `esc`/`c` clear filter + collapse, `L`
live-follow toggle, `e` endpoints panel toggle, `/`/`f` filter prompt, `q`/`ctrl+c` quit, sticky
bottom while live-follow is on, ring-eviction auto-clear of expansion, `sel` survival across
ticks/filters (existing renderFeed clamp). The full 343-test suite (dashboard 234, mock 109)
stays green with `-race`.

## 4. Design decision (approved by stakeholder 2026-08-07)

Produced by the Architect subagent (2026-08-07); **approved by stakeholder**. Scope:
`src/dashboard/model.go` (+ tests + docs). `render.go` needs **zero** changes. No new
dependencies, no flag changes, no list/textinput widget — the "viewport only, no list widget"
contract (WS-010) is preserved; the cursor remains a plain `int`.

**1. Selection unification (decision 3):**
- `selectedIdx()` becomes `return clampSel(m.sel, len(m.feedEntries))` — no mode branch. All six
  existing call sites (render.go:171, render.go:241, model.go:338/433/452) keep working.
- `selectedIdxFromOffset()` (model.go:594–611) **deleted**; its four call sites replaced
  (g/G → unconditional jumps, fallthrough re-sync → deleted, renderFeed → clamp-only).
- `feedFits()` retained **only** as the syncFeed live-anchor guard (§3.3); removed from all
  selection/key paths. Doc comment updated (no longer a selection-path concept).
- Fallthrough (model.go:542–558) stripped to a bare `m.view.Update` forward — the sel re-sync
  and the "resume live-follow on genuine bottom-ward movement" guard are both deleted.
  Deleting the guard is the literal implementation of decision 2: `liveFollow` flips to `true`
  only at `L`-resume, `G`, and the three collapse paths (enter-collapse, esc, c,
  collapseExpansion).
- `renderFeed` (model.go:684–691): clamp-only — `m.sel = clampSel(m.sel, len(m.feedEntries))`.
  No scroll at tick time (deliberate — see §3.3; cursor-follow at tick time would violate
  AC8's "no drag").

**2. Cursor movement + auto-scroll (decision 1):**
- `j`/`↓`/`k`/`↑` (non-prompt): `m.sel = clampSel(m.sel ± 1, len(m.feedEntries))`,
  `m.selTouched = true`, `m.ensureSelectedVisible()`, `return m, nil` — never reach the
  viewport fallthrough (this is what makes AC8 hold).
- New `ensureSelectedVisible()`: scrolls so `feedEntryToLine[sel]` stays within
  `[YOffset+margin, YOffset+Height-1-margin]` with `margin = 1` (0 when `Height < 3`).
  Top-edge trigger `line < y+margin` → `SetYOffset(line-margin)`; bottom-edge trigger
  `line > y+h-1-margin` → `SetYOffset(line-h+1+margin)`. `SetYOffset` clamps internally
  (bubbles v1.0.0), so fits mode = no-op (maxYOffset 0) — one code path for both modes (AC7).
  Trailing `-1` in feedLineToEntry is never targeted (feedEntryToLine only holds real entries);
  bottom-margin math lands exactly on maxYOffset for the last entry.
- AC3 (entry-by-entry): `sel` indexes entries; `feedEntryToLine[sel]` is the summary line;
  detail lines map to the same entry index and are structurally skipped.
- `g`/`G` unconditional (AC9): `g` → `sel = clampSel(0, n)`, `selTouched = true`, `GotoTop`,
  `liveFollow = false`. `G` → `sel = clampSel(n-1, n)`, `selTouched = false`, `GotoBottom`,
  `liveFollow = true` (explicit action — story §5 Q2 resolution).
- **Page keys (stakeholder-approved addition):** `pgup`/`pgdn`/`half-page`/`b`/`space`/`u`/`d`
  intercepted (non-prompt) and converted to cursor jumps of `view.Height-1` lines (half that
  for `u`/`ctrl+u`/half-page bindings), clamped + `ensureSelectedVisible()`. Required because
  the deleted fallthrough re-sync would otherwise let these scroll the view while the cursor
  drifts off-screen. `f` stays the filter-prompt key; `h`/`l`/`left`/`right` fall through
  (horizontal scroll is a no-op on vertical content).

**3. Live-follow semantics (decision 2 / AC8):**
- `liveFollow` changes only via `L`, `G`, and the three collapse paths. j/k/↑/↓/page keys/g
  never touch it (all return early).
- `syncFeed` live branch gains a live-attach guard: `if !m.selTouched && !m.feedFits() {
  m.sel = clampSel(len(m.feedEntries)-1, len(m.feedEntries)) }` — while passively
  live-following, the cursor rides the newest entry; once the user moves the cursor
  (`selTouched = true`), ticks do **not** yank it.
- New field `m.selTouched bool` (default false in newModel). New tiny helper
  `attachCursorToNewest()` (sets `sel = last`, `selTouched = false`; does NOT touch
  liveFollow/viewport — callers do), folded into `L`-resume and the collapse paths.
- AC8 "no drag" trade-off (documented): with follow paused and the cursor on the newest entry,
  new arrivals let that entry sink below the view — intended; the next nav key re-anchors.
  AC5 guarantees visibility only for navigation keys.
- `!m.feedFits()` guard keeps fits-mode tick-survival byte-identical
  (`TestModelFitsSelection_SurvivesTickAndFilter` unchanged) — this is the one remaining
  `feedFits()` use.

**4. Interactions (no change needed, AC6 + AC10):** `enter` (expand/collapse), `i` (inspect,
DASH-004 standalone-on-selected), `esc`/`c`, filter-apply, prompt mode (`&& !m.prompting`
guards keep the prompt handler intact), ring eviction / `expandedID` auto-clear, `r` refresh —
all operate on `selectedIdx()` which now returns the unified cursor. Optional polish (1 line
each): `ensureSelectedVisible()` after WindowSizeMsg re-pin and after filter-apply/esc/c
(these are explicit user actions, so cannot violate AC8).

**5. Edge cases:** empty feed / no-match filter → `clampSel(_, 0) == -1`, nav keys no-op,
`ensureSelectedVisible` no-ops; stale `sel` across ticks/filters → renderFeed clamp; cursor
off-screen after ring eviction → accepted (next nav key re-anchors); resize → optional re-pin;
narrow layout → same path (`m.view.Height` is layout-correct); tiny viewport (h<3) → margin 0;
sticky-bottom vs cursor-follow never fight (different frames: key-press vs tick ordering).

**6. Test plan:** 7 existing tests updated/rewritten (they assert overflow-mode offset-derived
selection or scroll-resumes-follow — `TestModelExpandPinsSummaryToTop`,
`TestModelOverflow_NoSilentLiveFollowResume`, `TestModelLiveFollowResumeOnScrollToBottom`,
`TestRenderPauseButtonResumesOnScrollBottom`, `TestModelLiveFollowPausePreservesOffset`,
`TestModelInspectDismissWithLivePaused`, `TestFeedSelectedIdxBounds` comment); ~17 new tests
per AC1–AC9 + mechanism (see §6.2 in the Architect's full proposal in the session record):
`TestUnifiedCursor_ReachesEveryEntryOverflow`, `_ReturnsToFirstUp`, `_SkipsDetailLines`,
`_HighlightNotTopLine`, `_AutoScrollTopAndBottom`, `_EnterInspectBothModes`,
`_OneModelNoOffsetDerivation`, `_NoImplicitLiveFollowResume`, `_NewEntriesNoDragWhilePaused`,
`_GAndG`, `TestEnsureSelectedVisible_*` (6 unit), `_LiveAttachNoYank`,
`_ResumeAndCollapseAttachToNewest`, `_PageKeysJumpCursor`, `_RingEvictionAndFilterClamp`,
`_EmptyAndNoMatch`, `_ResizeKeepsCursorVisible`.

**7. Docs to update with this story:** `architecture.md` — UXR-008 bullet (selection-derived-
from-viewport text now wrong), WS-010 keys bullet (scrolling keys now move the cursor), the
"viewport only, no list/textinput" contract (confirmed intact — make explicit), new UXR-018
ADR, UXR-014 superseding note; `.ai/requirements/17-feed-cursor-navigation.md` §4/§5 (this);
`.ai/tui-dashboard-reference.md` keys/selection contract.

## 5. Open questions for the Architect (resolved 2026-08-07)

1. **Auto-scroll margin:** **1 line**, clamped to 0 for viewport heights < 3. Confirmed.
2. **`G` and live-follow:** **`G` is an explicit action** — jump cursor to newest, `GotoBottom`,
   `liveFollow = true`, `selTouched = false` (as today). `g` jumps to first, pauses. Only plain
   `j`/`↓`/`k`/`↑` movement is exempt from resuming. Confirmed by stakeholder as part of the
   design approval.
3. **Ring eviction / filter vs sticky-bottom:** no fight — cursor-follow runs only on key
   presses; sticky-bottom runs on ticks when `atBottom && liveFollow` and executes after
   `renderFeed`; renderFeed is clamp-only and the live-attach guard requires
   `!selTouched && !feedFits()`. Confirmed.
4. **Endpoints panel:** **no** — rows aren't individually selectable/expandable (carried over
   from UXR-014 §4 Q3); `shift+↑/↓`/`K`/`J` viewport scrolling untouched. Confirmed.

## 6. Status

- 2026-08-07: reported by stakeholder; root-caused by BA (see §1); three scope decisions
  confirmed by stakeholder (see §0). Story handed to Architect for a design proposal.
- 2026-08-07: Architect produced the design (§4, full proposal in session record); **approved
  by stakeholder** (including the page-key cursor-jump addition). Status → `ready`, handoff to
  Developer.
- 2026-08-07: implemented by Developer (model.go only — unified `selectedIdx()`, deleted
  `selectedIdxFromOffset()`, `ensureSelectedVisible()`, page-key intercepts, `selTouched` +
  live-attach, g/G unconditional; `render.go` zero changes). 23 new tests + 7 rewritten; full
  suite 366 tests (dashboard 257, mock 109) green with `-race`, vet + gofmt clean.
- 2026-08-07: QA verified **AC1–AC10 all PASS** with no defects, no coverage gaps, no design
  deviations; live PTY smoke test confirmed AC8 (no implicit resume), AC9 (g/G), AC10
  (enter/i/esc/L under genuine overflow). Status → `verified`.
- 2026-08-08: **implemented by Developer** (`src/dashboard/model.go` + tests; `render.go`
  untouched). 8 existing tests rewritten/updated, 23 new tests added (`TestUnifiedCursor_*`
  per AC1–AC9 + mechanism, `TestEnsureSelectedVisible_*` units). Full suite 366 tests
  (dashboard 257, mock 109) green with `-race`; `go vet` clean; `gofmt` clean. Status →
  `implemented`, handoff to QA (verify each AC explicitly).
