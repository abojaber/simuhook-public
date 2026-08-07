# UXR-016 — Response section in the `enter` request detail

- **ID:** UXR-016
- **Status:** `verified` (QA sign-off 2026-08-07; 11/11 ACs PASS, `-race` clean)
- **Priority:** `[Should]`
- **Design status:** approved (stakeholder 2026-08-07; Architect proposal accepted as-is)
- **Date:** 2026-08-07

## User Story

As a developer debugging my integration,
I want the `enter` row-expansion (stream feed) to show the response alongside the request detail,
so that I can see status, response headers, and response body without pressing `i`.

## Context

Today the feed has two detail surfaces with different content:

| Surface | Content |
|---|---|
| `enter` expansion (`detailLines`, render.go:633–716) | `request_id`, `matched`, `timestamp`, request `headers` (capped 30), `query` (capped 30), `request body` (single-line, yellow 220). **No response.** |
| `i` inspector (`inspectLines`, render.go:803–928) | REQUEST section (header, `request:`/`endpoint:`/`request_id:`/`matched:`/`timestamp:`/`latency:`, headers, query, `request body:` multi-line via `formatBody`) + RESPONSE section (bold-240 + tint(status) header, `variant:` when set, response headers capped 30, `response body:` multi-line via `formatBody`) |

Stakeholder request 2026-08-07: "add response to the stream (when click a request and see the
request details, it should show a response with it)".

**Stakeholder decisions 2026-08-07 (proposal agreed as-is):**
1. **Mirror the inspector's RESPONSE section** in `detailLines`: status-tinted header,
   response headers (capped at `maxRenderedHeaders`), and response body.
2. **Response body is multi-line** (`formatBody`, pretty-printed JSON / capped plain),
   matching the inspector — not a single flattened line.
3. **The `i` inspector stays** as-is (full-detail view, `latency:` + `endpoint:` etc.);
   this story only augments the `enter` expansion. No removal, no key changes.
4. **Unmatched requests still show a RESPONSE section** — status `404` (the server's
   unmatched fallback), empty headers/body render as `(none)`/`(empty)` placeholders.

## Out of scope

- No changes to `inspectLines`, key bindings, the inspector trigger, or the TUI layout.
- No new flags, env vars, or dependencies.
- Mode-2 (WS-016 line output) and the startup banner remain frozen.
- `detailLines` request-side content stays byte-identical except where noted (the
  response section is appended after `request body:`).

## Design intent

- Reuse the existing UXR-015 primitives — `appendBodyBlock`, `renderStyledLine`,
  `splitKV`, `highlightJSON`, `kindBodyResp`/`kindJSONResp`/`kindBlockHead`/`kindMuted`,
  `styleSection` + `tint(status)`, `styleMuted` — so the response section in
  `detailLines` renders identically to the inspector's RESPONSE section.
- `detailLines` gains a RESPONSE section built exactly like `inspectLines`' RESPONSE
  block: header line (`styleSection` bold-240 + `tint(e.ResponseStatus)` with the same
  width-overflow degradation to muted `RESPONSE <code>`), optional `variant:` line when
  `e.ResponseVariant != nil`, response headers block (capped 30, `(N more headers)`
  marker), and `appendBodyBlock(&respLines, "response body:", e.ResponseBody, false)`.
- Ordering invariant (UXR-015): sanitize → build plain → `truncPlain` (plain only) →
  style segments → muted gutter; the RESPONSE header keeps its pre-existing
  styled-string truncation exception.
- No returned line contains `\n` (AC14 line-to-entry invariant preserved).
- The expansion gets taller; expansion/viewport math already derives line counts
  dynamically (`entryToLine`/`feedLineToEntry`, `expandedSummaryLine`), so no changes
  there — but the response block is bounded (capped headers, `formatBody` line/byte
  caps) so an expanded row cannot unboundedly inflate the feed.

## Acceptance Criteria

```
AC1 — Given a matched log entry
      When  the row is expanded with enter
      Then  the detail shows a RESPONSE section after "request body:" with a header
            line "RESPONSE" in bold 240 + status tint (same segment-wise style as the
            inspector), e.g. "RESPONSE 200"

AC2 — Given a matched entry whose response body is valid JSON
      When  the enter detail renders the response body
      Then  it is multi-line, pretty-printed via formatBody, and JSON-syntax-highlighted
            (same kindJSONResp path as the inspector: keys cyan, strings default,
            numbers yellow, bool/null magenta, punct muted)

AC3 — Given a matched entry whose response body is non-JSON plain text
      When  the enter detail renders the response body
      Then  it is multi-line via formatBody and tinted green (76) — identical to the
            inspector's response body

AC4 — Given a matched entry with response headers
      When  the enter detail renders
      Then  response headers appear under a "headers:" block header, keys cyan /
            values default, capped at maxRenderedHeaders with a "(N more headers)"
            marker when exceeded

AC5 — Given a matched entry with no response headers and/or an empty response body
      When  the enter detail renders
      Then  "(none)" / "(empty)" placeholders render muted, same as the inspector

AC6 — Given an unmatched log entry
      When  the enter detail renders
      Then  a RESPONSE section still renders with status 404 (tinted), and headers /
            body render as "(none)"/"(empty)" placeholders

AC7 — Given a matched entry whose response used weighted/round-robin variants
      When  the enter detail renders
      Then  a "variant: N" fixed line appears under the RESPONSE header (only when
            e.ResponseVariant != nil), matching the inspector

AC8 — Given the enter expansion of any entry
      When  it renders
      Then  no returned line contains \n (line-to-entry mapping invariant holds) and
            every line still passes sanitize-before-style (no ANSI injection through
            the new response lines)

AC9 — Given a narrow viewport
      When  the RESPONSE header line overflows
      Then  the same width-degradation path as the inspector applies (falls back to
            muted "RESPONSE <code>" without tint)

AC10 — Given a log entry
      When  the enter detail renders the request side
      Then  the request-side lines (request_id, matched, timestamp, headers, query,
            request body) are byte-identical to today except that the response section
            is appended after "request body:"

AC11 — Tests: new unit tests cover the response section in detailLines (status-tinted
      header, JSON highlight vs plain tint, capped response headers + marker, variant
      line, unmatched-404 case, empty placeholders, no-newline invariant, width
      degradation); existing detailLines tests that assert the full line set are
      updated for the added section
```

## Traceability

- Decision to add response to `enter` detail: stakeholder 2026-08-07 (this file).
- Reuses: UXR-015 primitives (`.ai/requirements/14-detail-coloring.md`),
  inspector RESPONSE block design (UXR-013, `.ai/requirements/08-dashboard-inspect.md`).
