# UXR-012 & UXR-013 — Endpoint-select filter + request/response inspector

Requirement file owner: BA. Status: UXR-013 `done` (superseded by WS-018/DASH-004), UXR-012 `backlog`. Phase: ux-vnext.

## 0. Stakeholder feature requests (2026-08-03)

> - select one of registered endpoints → filter live request stream
> - select one item (request) of live request stream → see request and response in a nice format

Two features, tabled by the stakeholder in one breath. Drafted here as UXR-012 and UXR-013
with a shared data-model dependency.

## 1. Stories

### UXR-012 — Select a registered endpoint to filter the live request stream `[Should]`

**As a** Simuhook operator watching the dashboard,
**I want** to pick one registered endpoint from the endpoints panel and have the live
request stream filter down to just that endpoint's requests,
**so that** I can watch a single integration without typing an `ep:` filter every time.

**Acceptance criteria:**
- Given the dashboard is running with ≥1 registered endpoint,
  When I select (highlight + confirm) a row in the REGISTERED ENDPOINTS panel,
  Then the LIVE REQUEST STREAM shows only requests that matched that endpoint, and the
  selection is visually highlighted.
- Given an endpoint is selected via the panel, When requests arrive, Then the stream /
  detail and all derived views (filter chip) reflect the `ep` predicate (equivalent to the
  existing `ep:` filter behaviour — reuse `Filter`/`Matches` machinery, no second code path).
- Given an endpoint is the active filter, When I select a different endpoint,
  Then the filter switches to it (one active endpoint at a time).
- Given an active endpoint filter, When I clear the selection (esc / dedicated key),
  Then the stream shows all endpoints again and any prior filter text is restored/unchanged.
- Given my chosen keymap, When no history remains after the filter is applied AND entries are
  scrubbed, the clear behavior does not leak state between reports.

**Non-goals (explicit):** no mouse support (keyboard-only TUI, consistent with existing).
No multi-endpoint filtering (single endpoint). No change to the typed `ep:` prompt filter
— panel selection is an alternative way to set the same predicate.

**Interaction questions (BA open, need stakeholder answer):**
1. The endpoints panel currently has NO selection interaction (viewport scrolls only via
   `shift+↑/↓`). What key marks an endpoint as the active filter — `enter` (currently
   unused in that panel) or `space`? And which key clears it (`esc`)?
2. Is this "replace the typed filter's `ep`" or "AND-intersect"? I recommend **set the `ep`
   predicate** — i.e. same value the prompt filter would produce, replacing any prior `ep`.
3. Should an active endpoint filter show a visible indicator in the endpoints panel
   (e.g. `▸` marker / highlighted row)? I recommend yes.

---

### UXR-013 — Select a request and inspect full request + response in a nice format `[Should]` — **done, superseded by WS-018 + DASH-004**

**As a** Simuhook operator on the dashboard,
**I want** to select one request in the live stream and see its full request AND its full
response (status, headers, body) formatted readably,
**so that** I can debug my integration without curling the admin API or grepping log JSON.

**Acceptance criteria:**
- Given a request row in the live stream is selected, When I trigger the inspect view,
  Then it shows the request line (method, path, matched, latency, timestamp) alongside a
  request section (headers, query, body) AND a response section (status, headers, body).
- Given a body that is valid JSON, When rendered, THEN the body is pretty-printed (indented)
  — syntactically-valid JSON never rendered as a single mangled line.
- Given a response, WHEN body is not JSON, THEN it is displayed as plain text with the
  sanitize() feed-security applied (no ANSI/control injection).
- Given the inspection view is on screen, When I dismiss it, THEN the stream returns to the
  live-follow state it was in before (position preserved, like UXR-008 expansion today).
- Existing UXR-008 `enter`, expansion must continue to work UNCHANGED when there is no
  selected-request inspect is not active.

**Data-model dependency (REQUIRED, needs sign-off):** `mock.LogEntry` today captures the
request (headers/body/query) plus `ResponseStatus` and `LatencyMs` only — it does NOT capture
the response headers or body. This story requires:
- adding `ResponseHeaders map[string]string` and `ResponseBody string` to `LogEntry`,
- populating them in the single request handler (`src/mock/handler.go`) at response-write time,
- extend the JSON log line (additive, backward-compatible for existing log consumers),
- dashboard `detailLines` gains a response section fed from the new fields.

**Pending (explicit):** no re-render of the stream while inspect is open (no live metrics
inside the detail). No body size cap discussion yet — see open question.

**Open questions (flagged, need stakeholder answer):**
1. "Nice format": beyond pretty-printed JSON + sections, do you want colour-coded status /
   a request-vs-response split visually (I lean yes), and is a short-overview + full toggle
   worth the extra key, or is body-fits-on-screen enough? (I recommend: sections with muted
   head, JSON body pretty-printed and colour-tinted, no layout panels change — reuse the
   existing feed/detail rendering area.)
2. Response body size cap — I recommend truncating very large response bodies (e.g. limit
   rendered response body length) to keep the TUI responsive; do you accept a cap?

## 3. Phasing

- **UXR-013** (data-model + renderer) first: bigger blast radius (logger/handler/contracts),
  contains the format-“nice” core requirement.
- **UXR-012** second: smaller, pure-dashboard, no data change — also reuses the same
  feed-render surface QA exercises in UXR-013.

## 4. Files touched (expected)

- `src/mock/types.go` / `src/mock/logger.go` — `LogEntry` fields (UXR-013 data).
- `src/mock/handler.go` — capture response headers/body into `LogEntry`.
- `src/dashboard/model.go` — selection state (UXR-012), inspect state (UXR-013).
- `src/dashboard/render.go` — endpoint-row highlight + inspector panel render (JSON pretty print).
- `src/dashboard/filter.go` — no code needed? panel selection feeds the same `Filter`.
- `contracts.md` — log line schema gain (pending sign-off).
- tests in each touched package.

## 5. Non-goals (explicit)

- Mouse support / hover (keep DOM keyboard-only, consistent with the codebase).
- Multi-endpoint selection in UXR-012.
- Editing/mutating an endpoint from the panel for UXR-012.
- Response-body size / performance deep dive beyond a display cap for UXR-013.
- Anything touching the network annotation (no render of auth).

## 6. Sign-off requests (stakeholder :admin)

| # | Decision | Answer |
|---|----------|--------|
| 1 | UXR-012 selection key: `enter` to set active endpoint filter, `esc` to clear | ✅ |
| 2 | UXR-012 set `ep` predicate (replace prior), not multi-select | ✅ |
| 3 | UXR-012 active highlight visible in endpoints panel (`▸` marker + highlighted row) | ✅ |
| 4 | UXR-013 ADD responseHeaders+responseBody to LogEntry + log line (additive, backward-compatible) | ✅ |
| 5 | UXR-013 "nice format" = muted section headers + pretty-printed (indented) JSON body + status tint, same rendering area | ✅ |
| 6 | UXR-013 body render cap (truncate large bodies) | ✅ |
| 7 | Phasing: UXR-013 (data+render) first, then UXR-012 | ✅ |

## 7. Status

- 2026-08-03: stakeholder signed off all 7 decisions (`yes to all`).
- 2026-08-03: WS-018 design (Architect) reviewed; Backlog items spinned out to `.ai/backlog/` (max-log-body-cap, response-variant-index, detaillines-header-cap, viewport-g-g-bindings) — commit `2973eb3`.
- UXR-013 / **WS-018 design APPROVED → handed to Developer** (`[Should]`). UXR-012 waits for phase 2.
- **UXR-013 CLOSED → superseded.** Implemented as **WS-018** (inspector data model:
  `LogEntry.ResponseHeaders`/`ResponseBody`, additive/backward-compatible) + **DASH-004**
  (standalone `i` inspector on the selected row), both QA-verified (see `done.md` 2026-08-06
  and `12-dashboard-gap-closures.md`). The "nice format" requirements (pretty-printed JSON,
  status tint, section split, body cap) were later delivered by UXR-015/016/017 (colorized
  detail, RESPONSE section, background bands). No further work on UXR-013.
- **UXR-012 MOVED to backlog.** Endpoint-select filter — decisions 1–3 signed off
  (`enter` sets, `esc` clears, `ep` predicate replace, `▸` marker), but never implemented;
  parked as a `backlog` `[Should]` pending a UX follow-up stakeholder request. Not in
  current active scope.