# Progress

## Current Session

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
