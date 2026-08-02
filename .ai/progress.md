# Progress

## Current Session

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
