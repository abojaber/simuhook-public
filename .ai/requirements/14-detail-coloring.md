# UXR-015 — Colorized request detail (keys/values + request/response body + JSON syntax highlighting)

Requirement file owner: BA. Phase: post-Phase-7 polish (TUI). Status: see per-story.

> Stakeholder request 2026-08-07: "when show request detail, give the font nice coloring. so i
> can know the key,value and know the request body and rsponse payload." Scope confirmed the
> same day — **Option B** (include JSON syntax highlighting for bodies). Applies to **both**
> surfaces: the `enter` inline detail (`detailLines`) and the `i` inspector
> (`inspectLines`). Mode-2 (WS-016 line output / banner) is frozen and out of scope.
>
> **Stakeholder decisions 2026-08-07 (on the Architect's design proposal):**
> 1. **Add a REQUEST section header** to the inspector (cyan-bold, mirrors the RESPONSE
>    header without the status tint) — the only structural line-count change.
> 2. **Route the inspector request body through `formatBody`** (multi-line pretty-printed
>    JSON / plain), mirroring the response body, so it gets the same highlighting.
> 3. **Widen the label column from `%-12s` to `%-14s`** in both surfaces, so the
>    `request body:` / `response body:` labels align with the fixed fields.
> 4. **Truncated JSON lines render unstyled (default foreground)** — accepted; **document
>    this** in the architecture ADR and TUI reference.

---

## UXR-015 — Colorized request detail `[Should]`

**Status:** `verified` (QA sign-off 2026-08-07; 16/16 ACs PASS, `-race` clean) | **Design status:** approved

**User Story:**
> As a developer debugging a webhook integration, I want the request-detail views to colorize
> keys vs values and visually separate the request body from the response payload, so that I
> can read a captured request at a glance without parsing a wall of identical gray text.

**Context (current behavior — all confirmed in `render.go` @ 2026-08-07):**
- `detailLines` (`render.go:625-699`, `enter` expansion): every line is built as a plain
  string, truncated, then wrapped wholesale in `styleMuted` at `render.go:694-696`
  (`styleMuted.Render("  │ " + truncated)`). Labels, header/query keys, values, and the body
  are indistinguishable gray.
- `inspectLines` (`render.go:786-933`, `i` inspector): REQUEST-section lines get the same
  wholesale muted treatment (`render.go:919-922`); RESPONSE-section lines likewise
  (`render.go:927-930`). Only the RESPONSE section header carries color (bold `240` label +
  `tint(status)`, `render.go:860-864`).
- Body rendering differs by surface:
  - `detailLines` — body is a **single line**: `sanitize(e.Body)` (newlines become spaces),
    label `body:` at `render.go:683-688`.
  - `inspectLines` — body goes through `formatBody` (`render.go:707-727`): pretty-prints valid
    JSON (≤ `maxInspectJSONBytes`), else plain text rune-safe truncated
    (`plainBody`/`capLines`). Labels `body:` at `render.go:850-855` (request) and
    `render.go:900-914` (response).
- Existing styles (`render.go:17-34`): `styleHeader` (63), `styleMuted` (240), `styleAccent`
  (39), `styleError` (196), `stylePauseBtn` (245), `styleResumeBtn` (220), `styleTableHead`
  (240), `stylePanelTitle` (63), `styleSelBG` (236), `styleSelMarker` (212), `styleSection`
  (240 bold).
- `truncPlain` (`render.go:88-108`) is **plain-text-only** (iterates runes, counts visible
  cells). It must never receive an ANSI-styled string. Coloring must therefore be applied
  **after** truncation (truncate plain → style segments).
- `sanitize` (`render.go:77-84`) strips control chars + DEL. It must run **before** any
  styling — request content is attacker-controlled (ANSI-injection invariant).
- `maxRenderedHeaders = 30` cap applies in both surfaces (headers + query); `(N more …)`
  markers are already muted.

**Design intent (approved shape — Developer may refine):**
- **New styles** (package-level, `render.go` styles block):
  - `styleKey` — cyan `39` (header/query keys, non-bold)
  - `styleLabel` — cyan `39` bold (field labels: `request_id:`, `matched:`, `timestamp:`,
    `request:`, `endpoint:`, `latency:`, `variant:`, and the `headers:`/`query:` block
    headers)
  - `styleRequestBody` — yellow `220` (non-JSON request body content)
  - `styleResponseBody` — green `76` (non-JSON response body content)
  - JSON token styles (inspector only): keys cyan `39`, string values default foreground
    (unstyled), numbers yellow `220`, booleans/null magenta `213`, punctuation muted `240`
  - Section headers: REQUEST → cyan `39` bold; RESPONSE → keep existing bold `240` +
    `tint(status)` (unchanged, AC11)
- **Segment-wise rendering:** per line, truncate the **plain** text first (current
  `truncPlain` call), then split key/value at the first `": "` and style each segment
  separately. Never pass an ANSI string to `truncPlain`. Gutter `"  │ "` stays muted.
- **Labels:** rename the body labels so the two payloads are unambiguous:
  `detailLines` and inspector-REQUEST: `body:` → `request body:`; inspector-RESPONSE:
  `body:` → `response body:`. **Label column widened from `%-12s` to `%-14s`** in both
  `detailLines` and `inspectLines` (stakeholder decision 3).
- **Inspector REQUEST section header (NEW):** a cyan-bold `REQUEST` line
  (`styleLabel`), mirroring the RESPONSE header minus the status tint (stakeholder
  decision 1). It is the only structural line-count change (+1 line per inspect view).
- **Inspector request body routed through `formatBody`** (stakeholder decision 2): the
  request body becomes multi-line pretty-printed JSON / plain text, mirroring the
  response body block, so it receives the same highlighting as the response.
- **JSON syntax highlighting:** only in the **inspector** (`inspectLines` + `formatBody`
  output), where bodies are multi-line. Implement as a pure, tea-free, unit-testable function
  (e.g. `highlightJSON(line string) string`) that tokenizes a single pretty-printed JSON line
  (handling escaped strings, numbers, booleans, null, punctuation) and returns it styled.
  Input is already sanitized/capped by `formatBody`. Invalid JSON or non-JSON lines pass
  through unchanged. A body is decided JSON-vs-tint **once per body** via a pure helper
  (e.g. `isJSONBody(raw)`) mirroring `formatBody`'s JSON-path condition exactly (size cap +
  `json.Valid`), so the highlight decision never disagrees with the pretty-print path.
- **`detailLines` body stays single-line:** it is a flattened blob (newlines → spaces); JSON
  highlighting does not apply there — it receives the `request body:` label + yellow tint
  only.
- **Truncated JSON lines** (e.g. a JSON line cut by `capLines`' `…` marker): the tokenizer
  rejects the truncated line as invalid → it renders **unstyled (default foreground)**, not
  yellow/green. Accepted by the stakeholder (decision 4) — **must be documented** in the
  architecture ADR and the TUI dashboard reference.
- Non-functional: no new dependencies (stdlib `encoding/json` already used by `formatBody`);
  bubbletea/lipgloss unchanged; all caps/limits unchanged.

**Acceptance Criteria:**
```
AC1 — Given a log entry with request headers
      When  detailLines renders (enter expansion)
      Then  each header line shows its key in cyan (39) and its value in the default
            foreground, and the line is no longer uniformly muted

AC2 — Given a log entry with query params
      When  detailLines renders
      Then  each query line shows its key in cyan and its value in the default foreground

AC3 — Given any log entry
      When  detailLines or inspectLines renders a fixed field (request_id, matched,
            timestamp, request, endpoint, latency, variant)
      Then  the label (left of the colon) is cyan bold and the value is default
            foreground, and the label column is %-14s wide (all fixed-field values align)

AC4 — Given a request with a non-empty body
      When  detailLines renders
      Then  the label reads "request body:" and the body content is tinted yellow (220)
            (single line, sanitized as today)

AC5 — Given an entry with both a request body and a response body (non-JSON)
      When  the i inspector renders
      Then  the REQUEST section shows "request body:" tinted yellow (220) and the RESPONSE
            section shows "response body:" tinted green (76), so the two payloads are
            visually distinct

AC6 — Given a body that parses as valid JSON
      When  the i inspector renders it (formatBody path)
      Then  it is syntax-highlighted: keys cyan, string values default foreground, numbers
            yellow (220), booleans/null magenta (213), punctuation muted (240)

AC7 — Given a JSON body in the REQUEST section of the inspector
      When  it renders
      Then  it is routed through formatBody (multi-line, pretty-printed) and receives the
            same JSON highlighting as AC6 while remaining inside the REQUEST section
            (distinct from the RESPONSE section)

AC8 — Given a non-JSON body (plain text) in either surface
      When  it renders
      Then  it is NOT JSON-highlighted — it falls back to the yellow (request) / green
            (response) tint per AC4/AC5

AC9 — Given body, header, or query content containing control characters or ANSI escapes
      When  any detail/inspect line renders
      Then  sanitize still strips them BEFORE styling (no ANSI injection through the new
            color path) — same invariant as today

AC10 — Given a viewport narrower than the content
      When  a colored detail/inspect line renders
      Then  truncation still works (plain-text truncation first, then styled segments) and
            the "…" marker renders without corrupting ANSI codes

AC11 — Given the RESPONSE section header in the inspector
      When  it renders
      Then  it keeps its existing bold-240 + status-tint styling (unchanged behavior);
            additionally a NEW cyan-bold "REQUEST" section header renders as the first
            line of the inspector (mirrors RESPONSE minus the tint)

AC12 — Given the empty cases (no headers, no query, empty body)
      When  either surface renders
      Then  the "(none)"/"(empty)" placeholders render unchanged (muted)

AC13 — Given the cap thresholds (maxRenderedHeaders, maxInspectBodyLines,
      maxInspectBodyBytes, maxInspectJSONBytes)
      When  limits are exceeded
      Then  the existing "(N more …)"/"… truncated" markers render unchanged and still count
            correctly — coloring never changes the caps

AC14 — Line-to-entry mapping invariant holds: no returned line contains \n after styling
      (ANSI codes add no newlines), so feedLineToEntry/entryToLine stay correct

AC15 — Given a JSON body line truncated by the cap (e.g. cut mid-string with "…")
      When  highlightJSON processes it
      Then  the line renders unstyled (default foreground), NOT tinted yellow/green; this
            behavior is documented in the architecture ADR and the TUI dashboard reference

AC16 — Tests: existing detail/inspect tests that assert the wholesale-muted bytes are
      updated to the new scheme (incl. the %-14s label column and any body/request-body
      shape changes); new unit tests cover highlightJSON (valid JSON with escapes, numbers,
      booleans, null; invalid JSON; plain text passthrough), key/value splitting,
      request-vs-response body tints, sanitize-before-style ordering, the REQUEST section
      header, and the isJSONBody decision matching formatBody
```

**Out of scope (explicitly frozen):** Mode-2 (WS-016 line output, startup banner, `FormatRequestLine`). TUI-only change. No new flags, no admin API (removed ADM-002), no backlog items (WS-011 forwarding, WS-012 WebSocket).
