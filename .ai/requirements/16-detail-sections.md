# UXR-017 — Request/response visual separation in detail views

- **ID:** UXR-017
- **Status:** `verified` (QA sign-off 2026-08-07; 11/11 ACs PASS, `-race` clean)
- **Priority:** `[Should]`
- **Design status:** approved (stakeholder 2026-08-07; Architect proposal accepted as-is)
- **Date:** 2026-08-07

## User Story

As a developer debugging my integration,
I want the request and response sections of the detail views to be visually separated,
so that I can tell at a glance where the request ends and the response begins.

## Context

After UXR-016, both the `enter` row expansion (`detailLines`, render.go:639–766) and the
`i` inspector (`inspectLines`, render.go:803–928) render a REQUEST section followed by a
RESPONSE section. They currently share:

- No visual divider between the sections (RESPONSE header appears directly after the
  request body).
- The terminal's default background for all lines — only the selected feed row uses a
  background band (`styleSelBG`, 236).
- The **same cyan** key color (39) for request header/query keys, response header keys,
  and response JSON keys (`styleKey` / `styleJSONKey`) — keys on both sides look alike.

Stakeholder request 2026-08-07: "add separated line between request side and response
side. give them different background color and change the response keys color so it will
not be cyan like request."

**Stakeholder decisions 2026-08-07 (defaults confirmed as-is):**
1. **Separator line:** a muted full-width `─` rule between the request section and the
   RESPONSE header, rendered with the detail gutter (`"  │ "` + `─` runes padded to the
   content width). Applies to BOTH surfaces (enter detail + inspector).
2. **Section background bands:**
   - REQUEST section: dark gray `236` (same color family as the selection band).
   - RESPONSE section: dark navy `17` (stakeholder amendment 2026-08-07 — originally
     dark green `22`, replaced to avoid the green-on-green clash with the green-76
     response text and to give a distinct hue from the gray request band).
   - Bands are **full-width** (padded to the viewport width, like the selected-row band),
     so sections read as clean blocks with no ragged right edge.
3. **Response key color:** response header/query keys and response JSON keys change from
   cyan `39` to **magenta `213`**. Request keys stay cyan. JSON string values stay default.
4. **Scope:** both `detailLines` (enter) and `inspectLines` (i) — they must stay visually
   consistent. The inspector also gains the separator line.
5. **RESPONSE header line:** keeps its bold-240 + status tint styling and sits at the top
   of the response band (after the separator), on the response background.

## Out of scope

- No changes to key bindings, the inspector trigger, TUI layout/geometry, flags, env vars,
  or dependencies.
- Mode-2 (WS-016 line output) and the startup banner remain frozen.
- The selected-row band (`styleSelBG`/`styleSelMarker`) and the feed summary rows are
  unchanged.
- `highlightJSON` tokenizer colors: only the key color changes (39 → 213) for response
  JSON; numbers/bools/punct unchanged. Request JSON keys stay cyan.

## Design intent

- New package-level styles:
  - `styleSectionReq = lipgloss.NewStyle().Background(lipgloss.Color("236"))`
  - `styleSectionResp = lipgloss.NewStyle().Background(lipgloss.Color("17"))`
  - `styleKeyResp = lipgloss.NewStyle().Foreground(lipgloss.Color("213"))` (response
    header/query keys)
  - `styleJSONKeyResp = lipgloss.NewStyle().Foreground(lipgloss.Color("213"))` (response
    JSON object keys)
- Every line in the REQUEST section carries the request background; every line in the
  RESPONSE section carries the response background. Because `renderStyledLine` renders a
  single style per segment, the background must be applied as a *line-level* wrapper:
  render the styled content, then pad with background-colored spaces to the full viewport
  width and re-apply/keep the background. The UXR-015 ordering invariant
  (sanitize → plain → truncPlain → splitKV → style → gutter) still holds for the
  *content*; the background band + full-width padding is applied after segment styling.
- Full-width padding must be width-aware (`lipgloss.Width`), never raw `len()`, because
  the content already contains ANSI. Padding runs to `m.view.Width` (the viewport width),
  not `contentW`, so the band edge is flush with the panel.
- The separator line is a `styledLine` of kind `kindMuted` whose plain content is
  `detailGutter`-equivalent runes — specifically a `─` run repeated to fill the content
  width — rendered muted (foreground 240) on the request background, then padded to the
  viewport width. It counts as one detail line for `feedLineToEntry`/`entryToLine` (the
  mapping is already dynamic).
- The RESPONSE header (pre-styled `styleSection` bold-240 + `tint(status)`, gutter inside
  the section style) must now also carry the response background (`22`) and be padded
  full-width. Its width-overflow degradation path (muted `RESPONSE <code>`) also gains
  the response background.
- `highlightJSON` gains a way to color keys by section (request vs response): either a
  parameter or a second function / style seam. The tokenizer logic itself is unchanged —
  only which key style is applied differs. `appendBodyBlock` gains the section
  (request/response) so body lines get the right background + right JSON key color.
- Invariants preserved: no returned line contains `\n`; sanitize-before-style; plain
  truncation before styling; the styled-string truncation exception for the RESPONSE
  header stays (and now also applies to the full-width padding step, which must be
  width-safe).
- Expansion line counts grow by exactly 1 (the separator) for entries with a response
  section. Existing hardcoded-count tests updated accordingly.

## Acceptance Criteria

```
AC1 — Given an expanded feed row (enter) or an open inspector (i) for any entry
      When  the detail renders
      Then  a separator line renders between the request section and the RESPONSE
            header: muted foreground, "─" runes spanning the content width, with the
            detail gutter; it is a single content line (no \n)

AC2 — Given the same view
      When  the detail renders
      Then  every REQUEST-section line has the dark-gray (236) background band and
            every RESPONSE-section line (including the RESPONSE header, variant,
            response headers, and response body) has the dark-green (22) background
            band; bands are padded to the full viewport width (flush right edge)

AC3 — Given the same view
      When  the detail renders response headers or query
      Then  their keys render in magenta (213), not cyan (39); request header/query
            keys remain cyan (39)

AC4 — Given a response body that is valid JSON
      When  the detail renders it
      Then  response JSON object keys render in magenta (213) while request JSON keys
            stay cyan (39); string values stay default foreground; numbers/bools/punct
            colors unchanged

AC5 — Given a non-JSON response body
      When  the detail renders it
      Then  it is still tinted green (76) as before, on the response background band

AC6 — Given the selected-row band and feed summary rows
      When  the feed renders
      Then  they are unchanged (selection band, marker, and summary styling untouched)

AC7 — Given a narrow viewport
      When  the RESPONSE header overflows
      Then  the existing degradation path applies (muted "RESPONSE <code>"), now with
            the response background and full-width padding, and all rendered lines
            still fit within the viewport width

AC8 — Given any expanded/inspected entry
      When  detailLines or inspectLines renders
      Then  no returned line contains \n (feedLineToEntry/entryToLine invariant holds);
            sanitize-before-style still holds for all content; the full-width padding
            is width-aware (no line exceeds the viewport width)

AC9 — Given the enter detail of any entry
      When  it renders
      Then  the request-side content lines are unchanged from UXR-016 except for the
            background band and the new separator; the RESPONSE section content is
            unchanged except background + key color + separator

AC10 — Given the inspector of any entry
      When  it renders
      Then  it gains the same separator, background bands, and response-key colors as
            the enter detail (visual consistency); the REQUEST section header remains
            cyan-bold

AC11 — Tests: new unit tests cover the separator line (presence, position, no-\n),
      background bands (per-section color on every line, full-width padding via
      lipgloss.Width <= viewport width), response-key color for headers/query/JSON
      keys (magenta 213), request keys still cyan, green-76 plain body on response
      background, narrow-viewport degradation with response background, and the
      feedLineToEntry/entryToLine mapping after the added separator; existing
      hardcoded-count tests updated (detail line counts grow by 1)
```

## Traceability

- Request/response visual separation: stakeholder 2026-08-07 (this file).
- Builds on: UXR-015 primitives (`.ai/requirements/14-detail-coloring.md`), UXR-016
  RESPONSE section in `detailLines` (`.ai/requirements/15-detail-response.md`).
