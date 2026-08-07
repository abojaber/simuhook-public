# TUI Dashboard — Developer Reference (current implemented contract)

**Purpose:** the authoritative, code-verified description of the **shipped** TUI dashboard
for developers. Read this before touching `src/dashboard/`. This is NOT a design/spec doc —
it describes what the code does today. The v-next UX spec (draft, may differ from reality)
is `.ai/ui-ux-dashboard.md`.

**Owner:** BA | **Status:** current as of 2026-08-06 (commit `4d3c0d1` + later) | **Code:** `src/dashboard/` (`dashboard.go`, `model.go`, `render.go`, `filter.go`)

---

## 1. Run modes (scope)

| Mode | When | What renders |
|------|------|--------------|
| **TUI dashboard** | `--tui on`, or `--tui auto` on a TTY | One full-screen bubbletea program (alternate screen). **This doc.** |
| **Line output** | `--tui off`, or `--tui auto` on non-TTY | Startup banner (once) + WS-016 one-line-per-request (`mock.FormatRequestLine`) + `shutting down...`. **Out of scope for this doc — intentionally unchanged (stakeholder decision 2026-08-06).** |

The TUI is **one screen** — no tabs/pages/navigation. Everything below is a state or overlay
of that single screen.

---

## 2. Layout dispatch

`Model.View()` (`render.go`) dispatches on `m.wide()`:

```go
wideMinWidth  = 100
wideMinHeight = 14
func (m Model) wide() bool { return m.width >= wideMinWidth && m.height >= wideMinHeight }
```

- **Wide (two-column):** `viewWide()` — chrome rows + two side-by-side panels.
- **Narrow (vertical):** `viewNarrow()` — byte-identical legacy layout (pre-Phase-6).

All size math lives in one pure `geometry()` (`model.go`) — single source of truth. Never
add independent size math. Key fields: `leftTotal = w*2/5`, `rightTotal = w-leftTotal`,
`panelTotal = h-4-1`, `feedH = epsH = vpH`, `streamW`/`epW`/`pathW`.

---

## 3. Wide layout — chrome (top → bottom)

| Row | Renderer | Contents |
|-----|----------|----------|
| 1. Title row | `renderTitleRow` | `Simuhook Dashboard` (left) + `[ Pause Stream ]` / `[ Resume Stream ]` button (right, driven by `liveFollow`) |
| 2. SERVER INFO | `renderServerInfo` | chips `[ Address … ]`, `[ Stubs dir (N) ]`, `[ Uptime … ]`, `[ Req N ]`; plus `[ Endpoints (N) ▸ e ]` when the left panel is hidden |
| 3. Info line | `renderInfoDetail` | muted `Log dir: …` — ⚠️ admin URL was here; **removed by ADM-002 bugfix** (see `.ai/requirements/13-admin-url-residue.md`) |
| 4. METRICS & LATENCY | `renderMetrics` | `[ Avg …ms ] [ Min …ms ] [ Max …ms ] [ p95 …ms ]`, or `[ (no requests yet) ]` when `Latency.Count == 0` |
| 5. Panels | `renderPanels` | left endpoints + right stream, or stream only when left hidden |
| 6. Footer | `renderFooter` | `showing X of Y recent · ▶ live / ⏸ live · q quit · r refresh · enter detail · i inspect` + ` · filter: <f>` when active |

The filter chip lives on the **endpoints panel title** when the left panel is visible, and
**relocates to the stream title** when the left panel is hidden (`e`) or auto-collapsed
(`renderStreamTitle` vs `renderEndpointsTitle`).

---

## 4. Content panels

### 4.1 Left — REGISTERED ENDPOINTS (N)
- Title: `REGISTERED ENDPOINTS (N)` (+ filter chip).
- Column headers: `METHOD ENDPOINT STUB REQ P95` (`renderEndpointsHeaderRow`).
- Rows (`endpointRow`): method (`ANY` if empty, 6w), path (sanitized, `epW`), stub = endpoint **ID** (8w), `Req` (compact count, right-aligned 4w), `P95` (compact ms, right-aligned 6w; `—` when no latency data).
- Sorted by ID (stable). **Own viewport** `endpointsView` — scrolls with `shift+↑/↓` or `K`/`J` (wide layout only).
- Empty state: `(no endpoints)`.
- Auto-collapse: when `height < 20` on init/resize → `showEndpoints = false`, `autoCollapsed = true` (re-expands on resize unless user explicitly toggled `e`).

### 4.2 Right — LIVE REQUEST STREAM
- Title: `LIVE REQUEST STREAM` (+ filter chip when left hidden).
- Column headers: `TIME METHOD PATH STUB STATUS` (`renderStreamHeaders`).
- Rows (`streamRow`): `MARK(2) TIME(8) METHOD(6) PATH(pathW) STUB(10) STATUS(6, right-aligned, tinted)`.
  - TIME is the log timestamp in **local time** (`t.Local()`, DASH-002); `??:??:??` on parse failure.
  - STUB = endpoint ID (or `unmatched`).
  - STATUS tinted by class: 2xx green `76`, 3xx cyan `39`, 4xx yellow `220`, 5xx red `196`, other muted `240` (`tint`/`statusColor`).
- Main viewport `m.view`. Selected-row highlight is **segment-wise** (`styleSelBG` per segment — never wrap a string containing an inner `\x1b[0m` in one `Background()` style).
- Empty states: `(no requests yet)` when no logs; `(no entries match filter)` when a filter matches nothing.
- Sticky-bottom: `syncFeed()` re-anchors to bottom only if `AtBottom()` before `SetContent`; manual scroll offsets survive live updates.

---

## 5. States & overlays (one screen, mutually exclusive)

| State | Trigger | Behavior |
|-------|---------|----------|
| **Filter prompt** | `/` or `f` (non-prompt) | Footer replaced by `renderPromptBar`: `filter:` + token-highlighted text (`ep:`/`method:`/`status:` accent, bare muted) + `▌` cursor + `esc cancel · enter apply`; inline `✗ <err>` on invalid `status:` (stays in prompt). Prompt swallows all other keys; `q` cancels the prompt (does not quit). **Space key arrives as `KeySpace`, not `KeyRunes`** — both must append `" "`. |
| **Help overlay** | `?` (non-prompt) | `renderHelp` box appended below the footer; `?` or `esc` closes. |
| **Row expansion** | `enter` (non-prompt) | `detailLines` rendered inline under the selected row: `request_id`, `matched`, `timestamp` (RFC3339Nano), headers, query, body. Headers/query capped at `maxRenderedHeaders` (30) with `(N more …)` marker. Keyed by `RequestID`; survives ticks; auto-clears when the entry leaves the ring or stops matching the filter. `enter` again / `esc` / `c` collapse; collapsing resumes live-follow. |
| **Inspector** | `i` (non-prompt) | `inspectLines` — full REQUEST + RESPONSE sections (status tinted, headers, query, pretty-printed JSON body, `variant: N` when a response variant index is set). Works **standalone on the selected row** (DASH-004) and on an expanded row. `i` toggles/dismisses; position and live-follow restored on dismiss (`inspectFollow`). |
| **Live-follow pause** | `L` | Toggles `liveFollow`; paused → view freezes, `[ Resume Stream ]`, `⏸ live`; resume re-anchors bottom. Also pauses on scroll-away and resumes only on genuine bottom-ward movement (UXR-014 Bug B guard: `YOffset != before && AtBottom()`). |
| **Endpoints panel toggle** | `e` | Hides/shows the left panel (stream takes full width); user toggle overrides auto-collapse. |
| **Filter active** | `/`+apply | Feed filtered at render time only (ring never mutated); footer `showing X of Y recent (filtered)`; chip in panel title; `c`/`esc`/re-`/`+empty clears. |

---

## 6. Selection model (read carefully)

Selection = "the row that is highlighted / `enter`-expandable / `i`-inspectable".

```go
func (m Model) feedFits() bool { return len(m.feedLineToEntry) <= m.view.Height }
func (m Model) selectedIdx() int {
    if m.feedFits() { return clampSel(m.sel, len(m.feedEntries)) }  // fits: explicit sel
    return m.selectedIdxFromOffset()                                 // overflow: feedLineToEntry[YOffset]
}
```

- **Overflow** (more rendered lines than viewport): selection derives from `view.YOffset` —
  scrolling moves the selection. `sel` is re-synced from `YOffset` after every render/nav.
- **Fits** (all lines visible — UXR-014 fix): the viewport can't scroll (`maxYOffset=0`), so
  an explicit `sel int` drives selection. `j`/`k`/`↑`/`↓` move `sel` (clamped); `g`/`G` set
  `sel=0`/`sel=last`; `renderFeed()` clamps `sel` after every rebuild (survives ticks/filters).
- Nav keys **fall through to the viewport only in overflow mode**. In fits mode they return
  immediately after moving `sel`.

---

## 7. Key bindings (complete, current)

| Key | Action |
|-----|--------|
| `q` / `ctrl+c` | Quit (in prompt: cancels prompt first) |
| `r` | Manual refresh (re-snapshots + resync) |
| `/` or `f` | Enter filter prompt |
| `enter` | Apply filter (prompt) / expand or collapse row (non-prompt) |
| `esc` | Cancel prompt / clear filter / collapse / close help |
| `c` | Clear filter / collapse row |
| `e` | Toggle endpoints panel |
| `L` | Pause/resume live-follow |
| `?` | Toggle help |
| `i` | Toggle request/response inspector (standalone or on expanded row) |
| `j` / `↓`, `k` / `↑` | Move selection (`sel` in fits) / scroll feed (overflow) |
| `pgup` / `pgdn` | Page feed (overflow only — no-op in fits) |
| `g` / `G` | Jump top / bottom of feed (`g` pauses live-follow, `G` resumes; sets `sel` in fits) |
| `shift+↑` / `shift+↓` or `K` / `J` | Scroll endpoints panel (wide layout, left visible) |

`g`/`G` are **not** in bubbles v1.0.0's `DefaultKeyMap` — handled as explicit `KeyMsg`s.
`bubbles/viewport.DefaultKeyMap` also binds `d,u,f,b,h,l,space` — reserved for the viewport
when not consumed above.

---

## 8. Live updates & data sources

- **Refresh: fixed 1 s `tea.Tick` poll** (`interval: time.Second`, `refreshCmd`). `tickMsg`
  re-reads `Snapshot()` + re-syncs feed/endpoints. **No pump goroutine, `Program.Send` never
  used** (DASH-003). `StatsTracker.Notify()` exists but is NOT consumed by the dashboard
  (retained as a seam).
- **Sources:** `StatsSource` (`Snapshot() mock.StatsSnapshot`) and `EndpointSource`
  (`List() []*mock.Endpoint`). Dashboard is pure-read — never mutates the registry or logs.
- **Latency p95:** approximate, from a fixed 92-bucket logarithmic histogram (interpolated,
  clamped to `[Min,Max]`); exact Count/Sum/Min/Max. `Avg()` guards against `Count==0`.
- **`--recent N`** (default 50) = ring buffer / feed depth. **`--max-log-body N**` (default 0 =
  unlimited) caps `body`/`response_body` in logs/ring/inspector — never the wire response.

---

## 9. Rendering invariants & security

1. **Sanitize everything log-derived.** Path, webhook, header/query keys+values, body —
   request content is attacker-controlled; ANSI/control injection into the alternate screen
   is a real vulnerability. `sanitize()` maps `<0x20` and DEL to space. Never render raw.
2. **`fitBlock` / `truncPlain`:** every panel block is exactly its budgeted size; never use
   `MaxHeight` (it clips the border). Truncate to width per line; ANSI-aware.
3. **Segment-wise highlight:** status tint emits an inner reset — build selection highlight
   segment-by-segment with per-segment `Background`; never wrap reset-containing strings in a
   single `Background()` style.
4. **`gofmt` clean** (DASH-007) — keep it that way.
5. **Wide/narrow parity:** the narrow fallback must stay byte-identical to the legacy layout
   unless a change explicitly targets both layouts with tests.

---

## 10. Code map

| Concern | Location |
|---------|----------|
| Program entry / alt screen | `dashboard.go` `Run` |
| Model, state fields, keys, selection, feed sync | `model.go` (`Model` struct, `handleKeyMsg`, `selectedIdx`, `renderFeed`, `syncFeed`, `geometry`, `feedFits`, `clampSel`) |
| Rendering (all layouts, rows, overlays, inspector) | `render.go` (`View`→`viewWide`/`viewNarrow`, `streamRow`, `summaryLine`, `detailLines`, `inspectLines`, `buildFeed`, `renderFooter`/`PromptBar`/`Help`, `fitBlock`, `truncPlain`, `tint`, `sanitize`) |
| Filter parsing/matching (pure, tea-free) | `filter.go` (`Filter`, `ParseFilter`, `Matches`) |

---

## 11. Intentionally NOT in scope

- **Mode 2 (line output / banner)** — frozen (stakeholder 2026-08-06).
- v-next items from `.ai/ui-ux-dashboard.md` (draft): multi-line row expansion as a separate
  view, mouse support, charts/sparklines, TUI endpoint CRUD, color themes.
- Multiple screens / navigation — deliberately single-screen.
