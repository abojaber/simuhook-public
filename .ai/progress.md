# Progress

## Current Session

### 2026-08-16 — Richer templates (TPL-001/002): faker generators + JSONPath — implemented + verified

- **Stories TPL-001/002 (`[Should]`, verified).** Realistic data generators (27 curated
  `gofakeit/v7` funcs: `{{Name}}`, `{{Email}}`, `{{Price}}`, `{{ULID}}`, `{{UUID}}`, …)
  + hand-rolled strict JSONPath extraction (`{{JSONPath "$.user.name"}}`) from the JSON
  request body, parsed exactly once per request into `TemplateData.parsedBody`.
- **Design (Architect).** Full 632-line contract in `.ai/designs/19-richer-templates-design.md`,
  verified against the actual gofakeit v7.15.0 source — surfaced 3 overrides to the story
  table (all stakeholder-approved D9–D16): `ULID` does not exist → `gofakeit.ID()`;
  `Sentence`/`Paragraph` deprecated with ignored args; `Price` requires 2 args → zero-arg
  wrapper. Also locked: `$` on non-JSON body → `""`, canonical re-marshal for objects/
  arrays, `FormatFloat('g')` numbers, strict key grammar, pin v7.15.0.
- **Implementation (BA direct).** The Developer subagent failed to deliver twice (empty
  results); stakeholder directed "fix the gofakeit dep" → the durable fix is the import.
  The BA executed all 4 phases in-session: A dependency (promoted to direct require),
  B `jsonpath.go` + ~30 table tests, C `templating.go` (`fakerFuncMap`,
  `buildTemplateFuncMap`, `parsedBody`) + 13 templating + 2 handler tests, D docs
  (README, contracts.md, architecture.md ADR) + `stubs/template-showcase.yaml` demo stub.
- **QA (BA direct, report in `.ai/qa/19-richer-templates-report.md`).** All ACs PASS —
  live curl round-trip extracted `cust-42`/`Ada Lovelace` via JSONPath, faker determinism
  confirmed (3 distinct names/emails/ULIDs), non-JSON body → `"id":""` with 200 (D12).
  Full suite **415 → 448 tests** green under `-race`; `go vet` + `gofmt` clean. No
  defects/gaps/deviations. Status → `verified`.
- **Process note.** All three subagent channels delivered truncated/empty results this
  session; the BA completed implementation and verification directly against the real
  build. Lesson (echo of the 2026-08-07 QA printf study): when delegation fails to
  deliver, verify the artifact on disk (`ls` + read), then execute in-session and
  record the evidence.

### 2026-08-07 — Detail coloring, response section, visual separation (UXR-015/016/017) + v0.1.0 release prep

- **UXR-015 — colorized request detail (`[Should]`, verified).** `detailLines` (enter
  expansion) and `inspectLines` (i inspector) were uniformly muted gray. Now labels are
  cyan-bold, header/query keys cyan, values default, request bodies yellow 220, response
  bodies green 76, and JSON payloads are syntax-highlighted (keys cyan 39, strings default,
  numbers yellow 220, booleans/null magenta 213, punctuation muted 240). New pure helpers in
  `src/dashboard/render_detail.go`: `highlightJSON` (character-scanner tokenizer — NOT
  regex, RE2 lacks lookahead for key-vs-value), `splitKV` (first-colon split preserving
  %-14s alignment), `isJSONBody` (mirrors `formatBody`'s JSON path exactly), shared
  `renderStyledLine` assembler. Stakeholder decisions: (1) NEW cyan-bold `REQUEST` section
  header in the inspector; (2) inspector request body routed through `formatBody`
  (multi-line, was a single blob); (3) label column `%-12s` → `%-14s`; (4) truncated JSON
  lines render unstyled (documented). QA 16/16 ACs.
- **UXR-016 — response section in the `enter` expansion (`[Should]`, verified).** The
  `enter` row detail showed request only; status/response headers/body required `i`. Now
  `detailLines` appends a RESPONSE section after `request body:` built byte-for-byte
  identical to the inspector's block: pre-styled `styleSection` bold-240 + `tint(status)`
  header (gutter inside style, width-degradation to muted `RESPONSE <code>`, the pre-existing
  styled-string truncation exception — now two call sites), optional `variant: N`,
  response headers capped at `maxRenderedHeaders` with a muted `(N more headers)` marker,
  multi-line response body via `appendBodyBlock` (JSON highlighted or plain green).
  Request-side lines byte-identical (AC10). No-refactor decision: RESPONSE block explicitly
  duplicated between `detailLines`/`inspectLines`; parity locked by tests. Unmatched → `RESPONSE
  404` + placeholders. QA 11/11 ACs.
- **UXR-017 — request/response visual separation (`[Should]`, verified).** Both detail
  surfaces were one gray wall. Now: (1) muted full-width `─` separator closes the request
  section; (2) full-width background bands — REQUEST dark gray `236` (selection-band
  family), RESPONSE dark navy `17` (stakeholder color amendment 2026-08-07, replaced
  original dark green `22` to avoid green-on-green with the green-76 text and to give a
  distinct hue); (3) response keys magenta `213` (`kindKVResp` + `styleKeyResp`;
  `highlightJSON` gains a `keyStyle` param → `styleJSONKeyResp`). Core mechanism: `band()`
  — a reset-safe post-pass re-asserting `\x1b[48;5;<bg>m` after every `\x1b[0m` and
  padding with background-active spaces to `m.view.Width`. A naive `Background().Render()`
  would lose the band at the first inner reset (UXR-008 segment-wise lesson); the ANSI
  surgery is safe because `sanitize()` strips every user ESC before styling. Selection
  band/feed rows byte-identical (AC6). QA 11/11 ACs.
- **QA totals:** 343 tests (dashboard 234, mock 109), `-race` clean, `go vet` + `gofmt`
  clean, no defects across all three stories.
- **Release prep (v0.1.0):**
  - **`--version` flag added** (`src/main.go`): `var version = "dev"`, injected via
    `-ldflags "-X main.version=v0.1.0"`. Verified both dev (`simuhook dev`) and release
    (`simuhook v0.1.0`) builds. Closes the long-standing CLI-convention gap
    ("Support `--help` and `--version` on every command").
  - **README rewritten:** admin API section removed (ADM-002/ADM-003 follow-through),
    CLI flags documented, architecture tree updated to current layout (dashboard/
    render_detail.go, mock/handler.go, no server.go/admin.go), log sample updated with
    response fields.
  - **Committed `4c42c19`** (11 files, +2468/−144): the UXR-015/016/017 batch + version +
    README + docs (ADRs, TUI reference, requirements `14/15/16`).
  - **Tagged `v0.1.0`** and pushed to origin (commit + tag).
  - **Release binaries built** in `dist/` (all `-X main.version=v0.1.0`):
    `simuhook-darwin-arm64/amd64`, `simuhook-linux-arm64/amd64` + `CHECKSUMS.txt` +
    `RELEASE_NOTES.md`.
  - **⏳ Blocked: GitHub release creation** — `gh` installed via brew but unauthenticated
    (no token). One command remains: `gh auth login` then
    `gh release create v0.1.0 --title "Simuhook v0.1.0" --notes-file dist/RELEASE_NOTES.md dist/simuhook-* dist/CHECKSUMS.txt`.
  - `curls_manual_test.sh` remains untracked at repo root (manual-test artifact, QA
    flagged — do not commit).
- **Process notes:** the Developer subagent returned an empty result on the first UXR-015
  invocation (same tooling failure seen 2026-08-06); one resume completed it cleanly.
  QA's UXR-016 pass confirmed the 4 predicted test breaks and found 0 defects.

### 2026-08-07 — Admin API fully removed (ADM-002/ADM-003) + TUI reference doc

- **ADM-002 executed — admin code deleted from the tree.** `src/mock/admin.go` +
  `admin_test.go` deleted (749 lines), admin mux mounts + banner line removed from
  `src/main.go`, dashboard admin URL stripped (`renderInfoDetail` → `Log dir: …` only,
  `renderHeader` collapsed, no more `<80`/`>=80` split). `/_admin/endpoints` now 404s and
  logs as unmatched like any unknown path.
- **ADM-003 (stale admin URL residue, Low) fixed + verified.** All three surfaces (wide info
  line, narrow config line, banner) admin-free. `TestRenderConfigNarrow` rewritten as a
  no-admin regression guard. QA: AC1-AC4 PASS, 302/302 tests, race + vet + gofmt clean,
  `rg` zero hits in `src/`, PTY-verified. 0 blockers. (One Low comment residue at
  `model.go:695` fixed by BA; three non-blocking test-gap follow-ups tracked in
  `13-admin-url-residue.md`.)
- **TUI developer reference committed** (`7009e27`): `.ai/tui-dashboard-reference.md` —
  code-verified contract of the shipped dashboard (layouts, chrome, panels, states,
  selection model, keys, code map). `ui-ux-dashboard.md` now banners itself as the v-next
  draft deferring to the reference.

### 2026-08-06 — DASH-001 gap closures D1/D2/D4/D7 implemented + verified

- **All four code closures from the 2026-08-05 sign-off landed and passed QA (0 bugs).**
  Committed `4d3c0d1`.
  - **UXR-014 (D1): feed selection stuck on first entry when content fits.** Explicit `sel`
    index for the fits-on-screen case (`feedFits()`, `clampSel()`, `selectedIdx()` split into
    fits→`sel` / overflow→`selectedIdxFromOffset()`), `j/k/↑/↓` move `sel` in fits mode,
    `g`/`G` set `sel=0`/`sel=last` (+ pause/resume), `renderFeed()` re-syncs `sel` after every
    rebuild. **Bug B fixed (AC4):** live-follow resumes only on `YOffset != before &&
    AtBottom()`. Overflow path byte-identical (pre-existing tests untouched, gofmt-only).
  - **DASH-002 (D2): feed timestamps local time.** `t.Local()` in `streamRow` +
    `summaryLine` (render.go:395/591), matching WS-016 line output; PTY cross-checked with
    TZ=America/New_York (TUI clock == line-output clock). New `pinLocalZone` helper.
  - **DASH-004 (D4): standalone `i` inspector.** Guard dropped; `i` adopts the selected row's
    RequestID as `expandedID`, reusing the full expanded-inspect machinery; empty feed =
    safe no-op.
  - **DASH-007 (D7): gofmt pass** on 5 flagged files (`gofmt -l src/` empty; main.go/admin.go
    diffs format-only).
- **QA:** 319/319 tests (dashboard 193, mock 126), `-race` clean ×3, `go vet` clean,
  `go build` OK, PTY-verified wide + narrow. No bugs. Non-blocking coverage gaps flagged:
  DASH-002 AC3 (TZ=UTC) and AC4 (parse-failure fallback) lack dedicated unit tests.
- **Process note:** the Developer subagent returned an empty result on first invocation
  (same tooling failure the Architect hit during the D1 design phase); a single retry
  completed all four stories cleanly.
- Stories marked `verified` in `09-feed-selection-bug.md` and `12-dashboard-gap-closures.md`.

### 2026-08-05 — Admin API removed + DASH-001 gap analysis signed off

- **Admin REST API removed from the product (stakeholder decision, ADM-002).** Endpoint
  management is file-based only (YAML + hot-reload watcher). Requirements/docs updated
  (`01-discovery`, `04-backlog`, `07-admin-handler`, `10-admin-and-dashboard-features`,
  `11-admin-removal`, `contracts`, `architecture`). **Code deletion still pending.**
- **DASH-001 (TUI dashboard) gap analysis — all 7 decisions agreed:**
  - D1 UXR-014 selection bug → fix now; D2 timestamps → local time; D3 Notify() doc drift →
    correct docs; D4 `i` standalone; D5 max-log-body doc note; D6 observer panic policy →
    backlog; D7 gofmt drift → pass.
  - BA executed D3 (`architecture.md` WS-010 ADR + `04-backlog.md` design note now state
    "fixed 1s poll", no pump) and D5 (DASH-001 doc note) same day; D6 backlog item created
    (`.ai/backlog/observer-panic-policy.md`).
- **Agent models switched** to `opencode/deepseek-v4-flash-free` for all four agents
  (opencode.json).

### 2026-08-04 — WS-018 backlog items closed

- **Four pre-existing backlog items implemented + verified** (full suite + race + vet clean,
  `-race` clean):
  - `detaillines-header-cap` — UXR-008 `enter` expansion headers/query capped at
    `maxRenderedHeaders` (30) with `(N more ...)` marker lines (byte-identical below cap).
  - `max-log-body-cap` — new `--max-log-body` / `LOG_MAX_BODY` capture cap (default 0 =
    unlimited), rune-safe truncation at capture time, never affects wire response.
  - `response-variant-index` — `PickResponse` returns the 0-based variant index;
    `LogEntry.ResponseVariant *int` surfaces it in logs + inspector.
  - `viewport-g-g-bindings` — `g`/`G` were never bound in bubbles v1.0.0's `DefaultKeyMap`;
    implemented as explicit `KeyMsg` handling (g → top + pause, G → bottom + resume).
    Two architecture.md doc bugs fixed along the way.
- `.ai/backlog/` empty again after these closed.

### 2026-08-03 — UX Phase 6: two-column dashboard redesign (UXR-009/010/011)

- **Implemented and verified.** At width ≥ 100 × height ≥ 14 the dashboard renders the redesigned layout: title row with `[ Pause Stream ]` button, SERVER INFO chips, muted log-dir/admin line, METRICS chips, and two `RoundedBorder` panels at fixed 40/60 — left REGISTERED ENDPOINTS table (Method|Endpoint|Stub|Req|p95, own scrollable viewport via shift+↑/↓/K/J), right LIVE REQUEST STREAM table (Time|Method|Path|Stub|Status). Below the threshold it falls back byte-identical to the legacy vertical layout. `e` hides the left panel. UXR-008 enter-expansion, UXR-007 chip relocation, UXR-002 Req data, UXR-010 pause all preserved. `ep.Path`/`ep.ID` now also sanitized (unauthenticated admin API = injection vector).
- **Process note:** QA's first pass returned NOT VERIFIED on a suspected 0-endpoint wide-layout panic that was never actually reproduced. The BA ran a 420-combination race-enabled sweep (7 widths × 5 heights × 4 endpoint counts × 3 log counts, incl. all 0-endpoint wide combos) — **zero panics**; a live "endpoints drop to zero at runtime" test also passes. Confirmed a QA false positive (the 191/198 panic only appears in an unnatural loop creating hundreds of fresh models). Added `TestLayoutWideZeroEndpoints` to permanently lock the case; QA re-verified 16/16 ACs. Lesson: when a QA defect conflicts with an empirical sweep, reproduce before sending back to the developer.
- 42 new tests (137 total), dashboard coverage 94.8%, race + vet clean. Committed `544db99`.

### 2026-08-02 — UXR-008 feed row expansion (Phase 5) done

- **UXR-008 (Phase 5) implemented and verified.** `enter` on a feed row (top visible line, selection derived from `view.YOffset` — no list widget) expands request detail: request_id, matched, full RFC3339Nano timestamp, headers, query, body. `enter` collapses; `esc`/`c` also collapse and resume live-follow.
- Design blocker resolved: the contract's "viewport only — no list/textinput" rule is satisfied by deriving selection from scroll position. Design signed off by stakeholder (6 items ✅), implemented by Developer, verified by QA (9/9 ACs PASS, no bugs).
- **Security fix surfaced by design work:** all log-derived feed text is now sanitized (control chars/ESC/DEL → space) — a request to `/\x1b[2J` could previously inject ANSI escape sequences into the TUI alternate screen. Verified no raw path in feed rendering.
- Two pre-existing latent bugs found & fixed along the way: trailing-`\n` off-by-one in viewport line counting; highlight built segment-wise so the status tint's inner reset can't wipe the selected-row background.
- `enter` now also auto-clears when the expanded entry leaves the ring or stops matching the filter (keyed by `LogEntry.RequestID`).
- 29 new tests (95 total), full suite + race + vet green. Committed `678bd96`.

### 2026-08-02 — UXR-006/007 verified; space-key prompt bug fixed

- **Found & fixed a real bug (from WS-010c):** bubbletea v1.3.10 delivers a space keypress as `KeyMsg{Type: KeySpace}`, not `KeyRunes`. The prompt handler only handled `KeyRunes`/`KeyBackspace`, so **spaces were silently dropped from the filter prompt** — multi-token filters like `ep:ping status:200` were impossible to type.
- Fix: added `case msg.Type == tea.KeySpace || s == " ":` branch in `model.go` prompt mode to append `" "`.
- Added `TestModelPromptSpaceTyping` (types `ep:ping status:200` via key messages incl. `KeySpace`, asserts prompt + applied filter) and `TestModelPromptSlashOpens` (regression guard for `/` entering prompt mode).
- Live PTY verification (reusing the proven `/tmp/qa-ux67.py` harness): prompt now renders `filter: ep:ping status:2▌` (space preserved), highlight + invalid-status error + chip + clean exit all confirmed.
- Dashboard coverage holds at 95.3%; full suite + vet + build green.

## History

### 2026-07-14 — Migrated to unified .ai/ pattern

- Extracted project-specific content from custom agents (.opencode/agents/) into AGENTS.md + .ai/rules.md + .ai/architecture.md
- Created .ai/ from cli-tool template: plan.md, progress.md, done.md, architecture.md, rules.md
- Copied 5 requirements files from .opencode/requirements/ to .ai/requirements/ (content preserved)
- Ran sync.sh — 5 shared agents installed (ba, developer, qa, architect, workspace-historian)
- Created root opencode.json with new instructions schema
- Old .opencode/ preserved for backward compatibility (zero data loss)
