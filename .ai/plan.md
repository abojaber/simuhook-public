# Plan

## Milestones

- [x] **MVP** — mock HTTP server: YAML stubs, hot-reload watcher, request matching,
      response templating/echo/delay/variants, JSON-lines logging, WS-016 line output.
- [x] **V1 / post-MVP** — WS-017 single request handler; WS-017b admin handler extraction
      (later removed); WS-010 interactive TUI dashboard + UXR-001..014 UX phases 1-7.
- [x] **Admin API removal (ADM-002 decision)** — stakeholder decision 2026-08-05: endpoint
      management is file-based only. Requirements/docs updated. **Code deletion pending.**
- [x] **DASH-001 gap analysis closures (D1-D7)** — all signed off 2026-08-05; D1/D2/D4/D7
      implemented + QA-verified 2026-08-06 (commit `4d3c0d1`); D3/D5 docs done; D6 backlogged.

## Active Priority

1. **ADM-002 — delete the admin REST API code** (`src/mock/admin.go`, `admin_test.go`,
   mux mounts, banner/dashboard admin lines). Decision made; only the code removal remains.
2. **D2 test-gap follow-ups (non-blocking)** — two small unit tests: DASH-002 AC3
   (TZ=UTC byte-identical) and AC4 (`??:??:??` parse-failure fallback).

## Backlog

- **WS-011** — webhook forwarding / proxy mode (`[Could]`, draft).
- **WS-012** — WebSocket / SSE endpoint simulation (`[Could]`, draft).
- **WS-014 / ADM-001** — admin REST API reintroduction (`[Could]`, low; contract retained in
  `07-admin-handler.md`; requires input validation + watcher-ownership semantics).
- **DASH-006** — observer panic policy (`.ai/backlog/observer-panic-policy.md`; awaiting
  policy confirmation).
- **UXR-012** — endpoint-select filter (pending UX follow-up).
- **WS-013** — built-in webhook templates (`[Won't]`, rejected).
- **TGR-04** — main.go integration tests (`[Won't]`, draft).
