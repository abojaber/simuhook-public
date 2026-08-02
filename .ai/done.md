# Done

Completed features and fixes, logged by date.

## 2026-08-02

- **UX Phase 4 + space-key fix verified.** Prompt highlight (UXR-006), inline invalid-status error, filter chip (UXR-007) all confirmed live via PTY. Fixed a pre-existing bug where the space key was dropped from the filter prompt (bubbletea sends `KeySpace` for ` `, not `KeyRunes`) — multi-token filters now typeable.
- **UX Phase 5 (UXR-008) verified.** `enter` on a feed row expands request detail (request_id, matched, timestamp, headers, query, body); `enter` again collapses. Selection derived from viewport position (no list widget). Expansion keyed by `RequestID`, survives ticks, auto-clears on ring eviction/filter. `esc`/`c` also collapse. Security fix: all log-derived feed text is now sanitized (control chars/ESC stripped) — a request to `/\x1b[2J` could previously inject ANSI into the TUI alternate screen. 95 tests, race + vet clean.
