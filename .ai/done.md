# Done

Completed features and fixes, logged by date.

## 2026-08-02

- **UX Phase 4 + space-key fix verified.** Prompt highlight (UXR-006), inline invalid-status error, filter chip (UXR-007) all confirmed live via PTY. Fixed a pre-existing bug where the space key was dropped from the filter prompt (bubbletea sends `KeySpace` for ` `, not `KeyRunes`) — multi-token filters now typeable.
- **UX Phase 5 (UXR-008) verified.** `enter` on a feed row expands request detail (request_id, matched, timestamp, headers, query, body); `enter` again collapses. Selection derived from viewport position (no list widget). Expansion keyed by `RequestID`, survives ticks, auto-clears on ring eviction/filter. `esc`/`c` also collapse. Security fix: all log-derived feed text is now sanitized (control chars/ESC stripped) — a request to `/\x1b[2J` could previously inject ANSI into the TUI alternate screen. 95 tests, race + vet clean.
- **UX Phase 6 (UXR-009/010/011) verified.** Two-column dashboard redesign at width ≥ 100 × height ≥ 14: title row with `[ Pause Stream ]` button, SERVER INFO + METRICS chips, fixed 40/60 panels (REGISTERED ENDPOINTS table Method|Endpoint|Stub|Req|p95 with its own scrollable viewport via shift+↑/↓ /`K`/`J`; LIVE REQUEST STREAM table Time|Method|Path|Stub|Status), `RoundedBorder`. Byte-identical legacy fallback below the threshold; all keys/behaviour preserved. `e` hides the left panel (stream takes full width). UXR-008 enter-expansion, UXR-007 chip relocation, UXR-002 Req data all retained. Security: `ep.Path`/`ep.ID` also sanitized. 42 new tests (137 total), race + vet clean. QA initially flagged a 0-endpoint wide-layout panic that was **confirmed a false positive** (inferred from inspection, never reproduced — a 420-case race-enabled sweep and a live shrinks-to-zero test both pass; the 191/198 panic only occurs in an unnatural loop of hundreds of fresh models).

## 2026-08-03

- **WS-017b verified — admin API extracted to `mock.NewAdminHandler`.** The REST admin API (`/_admin/endpoints`) moved from inline closures in `main.go` (zero tests) to `mock.NewAdminHandler(registry)` in `src/mock/admin.go` (commit `505e12c`). Wire format preserved byte-for-byte — including the POST 201 no-Content-Type quirk (sniffed `text/plain; charset=utf-8`, stakeholder-approved Q1), `http.Error` nosniff error paths, DELETE 204 empty, empty list `[]\n`, order-of-checks invariants (`PATCH /_admin/endpoints/` ⇒ 400 not 405; invalid JSON ⇒ 400 before dup/unknown checks), and no `Allow` header on 405. Admin stays unlogged via mux precedence (locked by `TestAdminHandler_MuxPrecedence_AdminNotLogged`). Ride-along race fix: `Encode(&ep)` pointer-encode on POST/PUT (latent `rrIndex` race under `-race`). QA verified 16/16 ACs with raw `DumpResponse` wire dumps; no bugs. 250 tests, race + vet clean.

## 2026-08-04

- **Header/query cap for UXR-008 `enter` expansion (backlog item `detaillines-header-cap`).** `detailLines` request headers and query params were the one remaining unbounded-map render path — the UXR-013 inspector already capped headers at 30 (renamed shared const `maxInspectHeaders` → `maxRenderedHeaders`) but `detailLines` was intentionally left untouched during WS-018 to keep it byte-identical while not inspecting. Both blocks now cap at `maxRenderedHeaders` (30) with a `(N more headers)` / `(N more query)` marker line, appended as plain text so the existing end-of-function mute/gutter/truncation still applies uniformly — output is byte-identical below the cap. Scope: `detailLines` only; `inspectLines`' query block stays uncapped by explicit decision (deferred, not a regression — same exposure existed before this fix). 4 new tests (over-cap + at-cap for both headers and query); dashboard coverage 94.5%, full suite + race + vet clean.
- **`--max-log-body` / `LOG_MAX_BODY` capture cap (backlog item `max-log-body-cap`).** New opt-in knob (default `0` = unlimited, unchanged behavior) caps `LogEntry.Body`/`ResponseBody` at N bytes, rune-safe (a multi-byte UTF-8 sequence straddling the cap is dropped whole rather than split). Enforced in `mock.Handler` at capture time, strictly **after** the real response has been written and echo/template rendering has used the full, uncapped bytes — the cap only bounds what reaches the logger, the stats ring, and the dashboard, never the wire response. `HandlerConfig.MaxBodyBytes` threads through from a new `--max-log-body` flag / `LOG_MAX_BODY` env var (validated non-negative, `log.Fatalf` on invalid, matching existing `--recent`/`LOG_DIR` conventions). Live-verified end-to-end via a temp echo stub: full body on the wire, 5-byte-capped `body`/`response_body` in the JSON log line. 7 new tests (truncation, rune-boundary safety, zero-is-unlimited, unit table for `truncateRuneSafe`); full suite + race + vet clean. `contracts.md` and `architecture.md` updated (architecture.md already speced this exact flag/env/default during the WS-018 risk review — implementation matched verbatim).
- **`response_variant` index in the log line (backlog item `response-variant-index`).** `PickResponse(ep) *Response` changed to `PickResponse(ep) (*Response, int)` — the second value is the 0-based index into a weighted/round-robin `Responses` slice, or `-1` for a singular `Response` (no variants configured). `LogEntry.ResponseVariant *int` (pointer, `json:",omitempty"`) surfaces it: nil/omitted when not applicable, present (even `0`) otherwise — the only way to distinguish those two states under `omitempty`. `inspectLines` (UXR-013) shows a `variant: N` line in the RESPONSE section when set. 5 existing `PickResponse` tests updated for the new signature (2 strengthened with explicit index assertions) + 4 new tests (nil-for-singular incl. JSON-omitted check, round-robin index sequence, inspector line presence/absence); full suite + race + vet clean.
- **`g`/`G` viewport bindings (backlog item `viewport-g-g-bindings`).** Confirmed via bubbles v1.0.0 source that `viewport.DefaultKeyMap` only binds `PageDown/PageUp/HalfPageUp/HalfPageDown/Up/Down/Left/Right` — `g`/`G` were never actually bound despite being documented, a pre-existing doc/binding mismatch (also found a second, related doc bug: `architecture.md`'s `Keys` bullet had claimed `g/G` came from the viewport `DefaultKeyMap` itself). Implemented as explicit `KeyMsg` handling in `model.Update`: `g` → `GotoTop` + pause live-follow; `G` → `GotoBottom` + resume live-follow. Pure navigation — does not collapse an active row expansion, consistent with existing pgup/pgdn/arrow-key scrolling. One new help line (`g/G  jump to top / bottom of feed`), verified exactly 63 cells like every other help line. 5 new tests (top/bottom navigation + liveFollow state, expansion preserved across both keys, prompt-mode swallows `g`/`G` as filter text, help width). Live PTY-verified with a real terminal emulator (`pyte`, since bubbletea's line-diffed alt-screen redraws defeat naive ANSI stripping): `[Resume Stream]`/`⏸ live` after `g`, `[Pause Stream]`/`▶ live` after `G`, help overlay renders correctly. Both `architecture.md` doc bugs fixed. Full suite + race + vet clean.

All four WS-018/pre-existing backlog items (`detaillines-header-cap`, `max-log-body-cap`, `response-variant-index`, `viewport-g-g-bindings`) are now closed; `.ai/backlog/` is empty.

## 2026-08-05

- **Admin REST API removed from the product (stakeholder decision, ADM-002).** The
  `/_admin/endpoints` CRUD surface (WS-017b, verified `505e12c`) is deleted — endpoint
  management is file-based only via the YAML stubs directory + hot-reload watcher. Rationale:
  hot-reload already covers live create/edit/delete; removal also eliminates the admin-specific
  risks found in BA analysis (no input validation → unreachable/orphan endpoints, watcher
  clobbering, unauthenticated CRUD surface, base64 body surprises). Moved to backlog at low
  priority as WS-014/ADM-001 with the full wire contract retained in `07-admin-handler.md`
  as the reintroduction spec. Requirements/docs updated: `04-backlog.md`, `07-admin-handler.md`,
  `10-admin-and-dashboard-features.md`, `contracts.md`, `architecture.md`.
- **BA analysis of ADM-001 (Gap A) agreed by stakeholder → solution: remove, don't patch.**
  Rather than adding input validation to the admin handler, the stakeholder chose to remove the
  admin API entirely. Remaining gaps (B: watcher clobbering, C: auth, D: base64, E: PUT
  id-forcing) are moot while the feature is in backlog; reintroduction contract carries them.
  - Gap B (watcher clobbering): **resolved by removal** — single registry writer (watcher), no
    interaction possible.
  - Gap C (auth): **resolved 2026-08-05 (stakeholder) — no authentication required.** Dev-only
    tool; unauthenticated binding accepted posture. Struck from the reintroduction contract.
- **Requirements review (DASH-001 gap analysis) concluded — all seven decisions agreed
  by stakeholder 2026-08-05 (D1-D7):** D1 in implementation; D2/D4/D7 handed to the
  Developer; D3/D5 executed by the BA (docs); D6 backlogged. See
  `.ai/requirements/12-dashboard-gap-closures.md` for the stories/ACs.

## 2026-08-05 (cont.)

- **Agent models switched to `opencode/deepseek-v4-flash-free`** for all four agents (ba,
  developer, qa, architect) in `opencode.json`. Requires an opencode restart to take effect.
- **DASH-001 gap analysis — all stakeholder decisions agreed (D1-D7, sign-off 2026-08-05):**
  - D1 (UXR-014 selection bug): **fix now** — implementation in progress with Developer
    (`sel` field added to `model.go`; design in `09-feed-selection-bug.md` §4).
  - D2 (timezone): **fix to local time** — dashboard `streamRow`/`summaryLine` must match the
    WS-016 line output (`t.Local()`, one line each in `render.go:395,591`). Story DASH-002.
  - D3 (Notify() doc drift): **doc corrected (BA, 2026-08-05)** — architecture.md WS-010 ADR
    now states "fixed 1s poll" instead of the non-existent pump. Story DASH-003.
  - D4 (`i` inspect): **make `i` standalone** — remove the `expandedID == ""` guard in
    `model.go:427-430` so `i` works directly on the selected row; render path keyed off the
    selected entry (render.go:977). Depends on D1's selection mechanism. Story DASH-004.
  - D5 (max-log-body + inspector): **doc note added** to DASH-001 (BA, 2026-08-05). Story DASH-005.
  - D6 (observer panic policy): **backlog item** created — `.ai/backlog/observer-panic-policy.md`. Story DASH-006.
  - D7 (gofmt drift): **`gofmt -w` pass** on the 5 flagged files (dashboard_test.go, model.go,
    render.go, main.go — admin.go dies in ADM-002 anyway). Sequenced AFTER D1/D2/D4 since
    model.go/render.go are in-flight. Story DASH-007.

## 2026-08-06

- **DASH-001 gap closures D1/D2/D4/D7 implemented and QA-verified (319/319 tests, race + vet
  + gofmt clean, PTY-verified).** Developer completed the batch in order (D1 → D2 → D4 → D7):
  - **D1 / UXR-014 (feed selection stuck on first entry) — fixed + verified.** Explicit `sel`
    index for the fits-on-screen case: `feedFits()`, `clampSel()`, `selectedIdx()` (fits →
    clamped `sel`; overflow → `selectedIdxFromOffset()` ≡ old `feedLineToEntry[YOffset]`),
    nav keys `j/k/↑/↓` move `sel` in fits mode, `g`/`G` set `sel=0`/`sel=last` (+pause/resume),
    `renderFeed()` re-syncs `sel` after every rebuild. **Bug B fixed (AC4):** live-follow
    resumes only when `YOffset != before && AtBottom()` — a key that can't move the viewport
    no longer silently resumes. Overflow path byte-identical (pre-existing tests unchanged,
    only gofmt-aligned). 7 new fits-mode tests. QA: AC1-AC6 PASS, PTY-verified in wide and
    narrow layouts.
  - **D2 / DASH-002 (feed timestamps local time) — done + verified.** `t.Local()` added to
    `streamRow` (render.go:395) and `summaryLine` (render.go:591); `??:??:??` fallback
    untouched. New `pinLocalZone` test helper (fixed zone, no CI-host dependence) +
    `TestSummaryLineRendersLocalTime`; `TestRenderStreamTableColumns` pinned to a non-UTC
    zone. PTY cross-check (TZ=America/New_York): TUI clock `23:54:16` == WS-016 line-output
    clock for the same UTC timestamp. Non-blocking coverage gaps flagged by QA: no unit test
    pins TZ=UTC (AC3) or exercises the parse-failure fallback (AC4) — behavior verified by
    reasoning/PTY, tests recommended as follow-ups.
  - **D4 / DASH-004 (standalone `i` inspector) — done + verified.** `expandedID == ""` guard
    dropped; standalone `i` adopts the selected row's `RequestID` as `expandedID`, reusing the
    whole expanded-inspect machinery (render, pinning, dismiss/live-follow semantics — AC1/4/5
    hold by construction). Empty feed / blank RequestID → safe no-op (AC2). Expanded-mode
    behavior unchanged. 3 new tests incl. overflow-row and empty-feed cases. QA: AC1-AC6 PASS,
    PTY-verified in both layouts.
  - **D7 / DASH-007 (gofmt drift) — done + verified.** `gofmt -w` on all 5 flagged files;
    `gofmt -l src/` empty. main.go (comment alignment) and admin.go (EOF newline) diffs are
    gofmt-only — zero behavioral change.
  - **QA verdict: 0 bugs.** Suite 319 passed / 0 failed (dashboard 193, mock 126), `-race`
    clean ×3, `go vet` clean, `go build` OK. Stories marked `verified` in
    `09-feed-selection-bug.md` and `12-dashboard-gap-closures.md`.
  - **Tooling note:** the Developer subagent returned an empty result on the first invocation
    (same tooling failure the Architect hit during the D1 design phase — see
    `09-feed-selection-bug.md` §4); a single retry completed all four stories cleanly.
  - **Remaining open:** ADM-002 (admin.go deletion, decision made, code removal still pending
    in the tree); DASH-003/005/006 docs done (BA); D6 backlog item awaits policy confirmation.
    Non-blocking D2 AC3/AC4 test gaps tracked as follow-ups.

## 2026-08-07

- **ADM-002 executed + ADM-003 verified — admin API fully removed from the codebase.**
  - **ADM-002 (code deletion):** `src/mock/admin.go` + `src/mock/admin_test.go` deleted
    (749 lines); admin mux mounts + banner line removed from `src/main.go`; stale admin URL
    stripped from the dashboard (`renderInfoDetail` → `Log dir: %s` only, `renderHeader`
    config line collapsed — the `<80`/`>=80` split is gone). No unused imports.
  - **ADM-003 (stale admin URL residue, severity Low):** fixed by the same pass — banner,
    wide info line, narrow config line all admin-free. `TestRenderConfigNarrow` rewritten as
    a no-admin regression guard; one stale "admin API" comment updated
    (`TestModelHeaderEndpointRefresh`); BA also fixed the `headerHeight` comment
    (`model.go`, "address/stubs/log-dir" without admin) and `architecture.md` ("muted
    log-dir line").
  - **QA verified 2026-08-07:** ADM-003 AC1-AC4 PASS + all ADM-002 scope checks; **302/302
    tests** (dashboard 280, mock 22), `-race` clean, `go vet` clean, `gofmt -l` empty,
    `rg "_admin|AdminEndpointsPath|NewAdminHandler|adminHandler" src/` → **zero hits**,
    PTY-verified wide 120×24 + narrow 80×24 + `--tui off` banner. Wire: `/_admin/endpoints`
    now 404s and logs as unmatched like any unknown path. 0 blockers.
  - **Non-blocking coverage gaps (QA, tracked in `13-admin-url-residue.md`):** no unit test
    for the banner (no `main_test.go`), no direct wide-info-line unit assertion, and no
    automated "admin path falls through to unmatched 404" test.
  - **TUI developer reference added** (`.ai/tui-dashboard-reference.md`, commit `7009e27`) —
    the code-verified contract of the shipped dashboard; `ui-ux-dashboard.md` banner now
    marks it as the v-next draft and defers to the reference.
  - **Layout audit → ADM-003:** the audit that found the residue also confirmed the current
    layout contract (header/footer/content two-column, help overlay, filter prompt, row
    expansion, inspector — all documented in the reference).
  - **UXR-015 — colorized request detail (`[Should]`, verified 2026-08-07).** The `enter`
    detail expansion and `i` inspector were uniformly muted gray; now labels are cyan-bold,
    header/query keys cyan, values default, request bodies yellow, response bodies green,
    and JSON payloads are syntax-highlighted (keys 39, strings default, numbers 220,
    booleans/null 213, punctuation 240). New pure helpers in `render_detail.go`:
    `highlightJSON` (character-scanner tokenizer, NOT regex — RE2 lacks lookahead for
    key-vs-value), `splitKV` (first-colon split, label+rest == line byte-for-byte preserving
    %-14s alignment), `isJSONBody` (mirrors `formatBody`'s JSON path exactly). All surfaces
    share one `renderStyledLine` assembler. Stakeholder decisions 2026-08-07: (1) NEW
    cyan-bold `REQUEST` section header added to the inspector (only structural line-count
    change), (2) inspector request body routed through `formatBody` (multi-line, previously
    a single blob while the response was pretty-printed), (3) label column widened
    `%-12s` → `%-14s`, (4) truncated JSON lines render **unstyled** (accepted + documented
    in ADR/reference). Ordering invariant holds on every line: sanitize → build plain →
    `truncPlain` (plain only) → splitKV → style segments → muted gutter; no ANSI ever hits
    `truncPlain`, no returned line contains `\n` (AC14 mapping invariant). RESPONSE header
    unchanged (bold-240 + status tint + degradation path). QA: **16/16 ACs PASS**, 321 tests
    (dashboard 212, mock 109), `-race` clean, `go vet` clean, `gofmt` clean. ADR + TUI
    reference updated. Non-blocking: `curls_manual_test.sh` untracked at repo root (manual
    test artifact — don't commit).
  - **UXR-016 — response section in the `enter` request detail (`[Should]`, verified
    2026-08-07).** The `enter` row expansion showed request detail only (status/response
    headers/body required `i`). Now `detailLines` appends a RESPONSE section after
    `request body:` built **byte-for-byte identical** to the inspector's block: pre-styled
    `styleSection` bold-240 + `tint(status)` header (gutter inside the section style;
    styled-width-overflow degradation to muted `RESPONSE <code>`; the pre-existing
    styled-string truncation exception — now at two call sites), optional `variant: N`
    when `ResponseVariant != nil`, response headers capped at `maxRenderedHeaders` with a
    muted `(N more headers)` marker, and a multi-line response body via `appendBodyBlock`
    (JSON pretty-printed + syntax-highlighted via `kindJSONResp`, or plain text green 76).
    Request-side lines byte-identical (AC10). **No refactor (explicit):** the RESPONSE
    block is deliberately duplicated between `detailLines` and `inspectLines` — the
    inspector stays untouched per story scope; byte-parity is locked by tests. Unmatched
    entries render `RESPONSE 404` + placeholders; expansion stays bounded (30 headers, 40
    body lines / 4 KiB plain, 256 KiB JSON → worst-case ~72 lines). QA: **11/11 ACs PASS**,
    333 tests (dashboard 224, mock 109), `-race` clean, `go vet` clean, `gofmt` clean.
    ADR + TUI reference updated. Low-severity notes: stale line-number comment in
    `render.go:713` (cosmetic); `curls_manual_test.sh` still untracked at repo root.
  - **UXR-017 — request/response visual separation (`[Should]`, verified 2026-08-07).**
    The request and RESPONSE sections of `detailLines` (enter) and `inspectLines` (i)
    were visually indistinguishable — one gray wall. Now: (1) a muted full-width `─`
    separator closes the request section before the RESPONSE header (kindMuted, gutter
    included, one content line, request band); (2) full-width background bands — REQUEST
    dark gray `236` (selection-band family), RESPONSE dark navy `17` (stakeholder color
    amendment 2026-08-07 — replaced the original dark green `22` to avoid the
    green-on-green clash with the green-76 text and to give a distinct hue from the gray
    request band), padded flush to `m.view.Width`; (3) response keys turn magenta `213` via a new
    `kindKVResp` lineKind (`styleKeyResp`) and a `keyStyle` param on `highlightJSON`
    (`styleJSONKeyResp`), while request keys stay cyan and JSON string
    values/numbers/bools/punct are untouched. Core mechanism: `band()` — a reset-safe
    post-pass that re-asserts the background after every `\x1b[0m` and at line start,
    then pads with background-active spaces. A naive `Background().Render(line)` would
    lose the band at the first inner reset (the UXR-008 segment-wise lesson); the ANSI
    surgery is safe because `sanitize()` strips every user ESC before styling, so the
    only `\x1b` bytes in a rendered line are our own styles'. Width measured via
    `lipgloss.Width`, never `len()`. The pre-styled RESPONSE header (bold-240 + tint)
    and its width-overflow degradation are banded for free; the styled-string truncation
    exception is unchanged. UXR-016 no-refactor duplication preserved — both assembly
    sites changed identically; `model.go` untouched (mapping already dynamic). Selection
    band/feed rows byte-identical to HEAD (AC6). Line counts +1 (empty 9→10, mapped
    12→13, lineToEntry 15→16). QA: **11/11 ACs PASS**, 343 tests (dashboard 234, mock
    124), `-race` clean, vet/gofmt clean. Docs: ADR + TUI reference updated. Notes:
    `styleSectionReq`/`styleSectionResp` declared but unused (package-level vars, legal);
    dev report said "8 TestHighlightJSON_*" — actually 7 (cosmetic).
