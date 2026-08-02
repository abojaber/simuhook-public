# Progress

## Current Session

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
