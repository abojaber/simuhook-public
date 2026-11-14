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
- [x] **Richer templates (TPL-001/002)** — 27 gofakeit data generators + hand-rolled
      JSONPath extraction; D9–D16 approved, implemented + QA-verified 2026-08-16.
- [x] **Transparent proxy (PROXY-001/002)** — `--proxy /path=https://dest` forwarding
      fallback with full-fidelity logging (`raw_headers`, `upstream_request`,
      `upstream_response`); D1–D4 signed off 2026-08-19, implemented + committed 2026-08-21.
- [x] **Raw listener-level tee (PROXY-003)** — captures raw request+response bytes at
      `net.Conn` level before Go's parser rejects malformed requests; `>>>`/`<<<` markers
      in `<log-dir>/_raw.log`; D1–D6 locked 2026-08-19, revised 2026-08-21 (D5=request+response),
      design approved 2026-08-21, implemented + committed 2026-08-21.
- [x] **Proxy TUI view (PROXY-004)** — dedicated proxy view in the TUI dashboard showing
      structured proxy log entries (filtered from StatsTracker where webhook == "_proxy")
      plus raw bytes from the tee ring buffer; key `2` switches to proxy view, `1` returns
      to main dashboard; implemented 2026-08-21.
- [x] **Proxy QA verification pass (PROXY-002/003/004)** — subagent QA verified full-fidelity
      logging, raw tee, and proxy TUI view 2026-10-19 (562 tests PASS under `-race`, live
      end-to-end smoke + scripted-pty TUI probe). PROXY-001 held at `in-progress` — see Active
      Priority #4 (Bug 1).

## Active Priority

1. **Publish the GitHub release** — `gh auth login` then
   `gh release create v0.1.0 --title "Simuhook v0.1.0" --notes-file dist/RELEASE_NOTES.md
   dist/simuhook-* dist/CHECKSUMS.txt` (binaries + notes + checksums ready in `dist/`).
2. **Close Forgejo #9** — the requirements index it asked for is delivered (`.ai/requirements/00-index.md`).
3. **Fix PROXY-001 Bug 1 (High) — collapsed multi-value response headers.** `handler.go` replays
   the upstream response from `firstValues(resp.Header)` via `w.Header().Set`; must write the full
   multi-value `resp.Header` with `Add`/direct assignment. Add a caller-visible multi-value
   regression test. Blocks PROXY-001 sign-off. (Bug 2 — response-side hop-by-hop stripping — is a
   cheap same-block follow-up.) Report: `.ai/qa/20-proxy-forwarding-report.md`.
4. **Create missing Forgejo issues** for backlog items (no CLI available in-env — manual):
   WS-014/ADM-001, UXR-012, and (when scoped) WS-012.
5. **Non-blocking QA test gaps (small, low-effort):**
   - D2 AC3/AC4: pin TZ=UTC byte-identical + `??:??:??` parse-failure tests (`12-dashboard-gap-closures.md`).
   - ADM-003: banner unit test, wide info-line assertion, admin-paths-fall-through test (`13-admin-url-residue.md`).
   - Proxy coverage gaps: PROXY-004 AC2 field asserts, response-side hop-by-hop test,
     `TestRawTeeListener_NoProxy_NoWrap` main.go-guard test.
6. **DASH-006 policy confirmation** — observer panic policy (`.ai/backlog/observer-panic-policy.md`):
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
