# Plan

## Milestones

- [x] **MVP** — mock HTTP server: YAML stubs, hot-reload watcher, request matching,
      response templating/echo/delay/variants, JSON-lines logging, WS-016 line output.
- [x] **V1 / post-MVP** — WS-017 single request handler; WS-017b admin handler extraction
      (later removed); WS-010 interactive TUI dashboard + UXR-001..014 UX phases 1-7.
- [x] **Admin API removal (ADM-002/ADM-003)** — stakeholder decision 2026-08-05; code
      deletion executed 2026-08-06; QA verified 2026-08-07 (302/302, 0 blockers).
- [x] **DASH-001 gap analysis closures (D1-D7)** — all signed off 2026-08-05; D1/D2/D4/D7
      implemented + QA-verified 2026-08-06 (commit `4d3c0d1`); D3/D5 docs done; D6 backlogged.
- [x] **Detail-view UX pass (UXR-015/016/017)** — colorized request detail, response
      section in the `enter` expansion, request/response visual separation; all verified
      2026-08-07 (commit `4c42c19`).
- [x] **v0.1.0 release** — `--version` flag, README rewrite, tag pushed, release binaries
      built; GitHub release creation pending auth (blocked step, see below).

## Active Priority

1. **Publish the GitHub release** — `gh auth login` then
   `gh release create v0.1.0 --title "Simuhook v0.1.0" --notes-file dist/RELEASE_NOTES.md
   dist/simuhook-* dist/CHECKSUMS.txt` (binaries + notes + checksums ready in `dist/`).
2. **Non-blocking QA test gaps (small, low-effort):**
   - D2 AC3/AC4: pin TZ=UTC byte-identical + `??:??:??` parse-failure tests (`12-dashboard-gap-closures.md`).
   - ADM-003: banner unit test, wide info-line assertion, admin-paths-fall-through test
     (`13-admin-url-residue.md`).
3. **DASH-006 policy confirmation** — observer panic policy (`.ai/backlog/observer-panic-policy.md`):
   confirm direction 1 (per-observer recover) when picked up.

## Backlog

- **WS-012** — WebSocket / SSE endpoint simulation (`[Could]`, draft).
- **WS-014 / ADM-001** — admin REST API reintroduction (`[Could]`, low; contract retained in
  `07-admin-handler.md`; requires input validation + watcher-ownership semantics).
- **DASH-006** — observer panic policy (`.ai/backlog/observer-panic-policy.md`; awaiting
  policy confirmation).
- **UXR-012** — endpoint-select filter (pending UX follow-up).
- **WS-013** — built-in webhook templates (`[Won't]`, rejected).
- **TGR-04** — main.go integration tests (`[Won't]`, draft).
