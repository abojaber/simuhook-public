# TUI Dashboard — UI/UX Specification

**Status:** `draft` | **Owner:** BA (AboJaber) | **Related:** WS-010 (implemented), this doc is the v-next UX contract

---

## 1. Goals

The dashboard is the operator's live view of the mock server. It must answer, at a glance:

1. **Is the server healthy and what is it configured to do?** (address, stubs, endpoints)
2. **What is happening right now?** (traffic, latency, errors)
3. **What happened recently?** (scrollable, filterable request feed)
4. **Can I act without breaking my flow?** (filter, refresh, quit — never trapped)

---

## 2. Visual Layout

Terminal layout is a fixed grid of **zones**. Sizes adapt to terminal height; the feed takes all remaining space.

```
┌──────────────────────────────────────────────────────────────────────────┐
│ ● Simuhook Dashboard                                   ┌─ header: status │
│ Address: :8080  Stubs: ../stubs (9)  Logs: logs      │   dot + title     │
│ Admin: http://localhost:8080/_admin/endpoints                          │
├──────────────────────────────────────────────────────────────────────────┤
│ Uptime 0:12:34 │ Reqs 128 │ p95 41ms │ avg 12ms │ 4xx 3 │ 5xx 0      │  stats strip
├──────────────────────────────────────────────────────────────────────────┤
│ Endpoints (9)  [filtered: ep:order  ⌫ x]                                │
│  ▸ POST  /webhooks/order → order-placed     ▮ requests: 42  p95: 18ms │  endpoints
│  ▸ GET   /ping → ping                       ▮ requests: 11  p95: 3ms  │  panel
│  ▾ (1 more…)                                                           │
├──────────────────────────────────────────────────────────────────────────┤
│ 13:12:01  POST /webhooks/order → 200  order-placed   (41ms)  ▮        │
│ 13:12:00  GET  /health     → 200  health              (1ms)  ▮        │
│ 13:11:59  GET  /nope       → 404  unmatched            (0ms)  ▮        │  request
│ 13:11:58  POST /webhooks/order → 201  order-created   (88ms)  ▮        │  feed
│ …                                                                       │
├──────────────────────────────────────────────────────────────────────────┤
│ filter: ep:order▌   esc cancel · enter apply                            │  prompt /
├──────────────────────────────────────────────────────────────────────────┤
│ ▼ live · 1.0s · showing 24 of 50 recent · / filter · r refresh · ? help │  footer
└──────────────────────────────────────────────────────────────────────────┘
```

### 2.1 Zones (top → bottom)

| # | Zone | Lines | Contents |
|---|------|-------|----------|
| 1 | **Status bar** | 1 | Live dot (`●` green = running), title, current time. Severity-tinted on error (lastErr). |
| 2 | **Config line** | 1–2 | Address, stubs dir + file count, log dir, admin API URL. Collapses to 1 line on narrow terminals. |
| 3 | **Stats strip** | 1 | Aggregated counters (all-requests, not ring): uptime, total requests, latency (p95/avg), error counts. |
| 4 | **Endpoints panel** | variable | Registered endpoints with per-endpoint request counts + p95. Collapsible (`e` toggles). |
| 5 | **Request feed** | variable (fills) | Chronological log lines (most recent at bottom). Sticky-bottom by default. |
| 6 | **Prompt bar** | 0–1 | Only visible during filter entry. |
| 7 | **Footer** | 1 | Live/scroll state, refresh interval, key hints, filter status. |

---

## 3. Information Design

### 3.1 Data hierarchy
- **Primary:** traffic — total requests, per-endpoint counts, errors, latency. Big numbers, top-left bias.
- **Secondary:** configuration — what endpoints exist, what the server listens on. Set-once info; should not dominate screen.
- **Tertiary:** raw detail — individual request lines (method, path, status, latency).

### 3.2 Color / style conventions
- **Accent (bold):** title, live dot, active filter.
- **Muted:** config lines, feed timestamps, footer hints, endpoint paths.
- **Semantic (sparingly):** status-code tints in the feed — `2xx` green, `3xx` cyan, `4xx` yellow, `5xx` red. Never color the whole screen; one glyph/segment only.
- **Alignment:** tabular. Method column fixed width, path left-aligned, numbers right-aligned. Stable columns prevent flicker.

### 3.3 Density rules
- Feed lines are single-line summaries (`time method path → status label (latency)`). Multi-line expansion is a future enhancement (`enter` on a row).
- Endpoint rows collapse: if more endpoints than vertical space, show `▾ (N more…)` with a hint to press `e` to expand or scroll.

---

## 4. Interaction Model

### 4.1 Key bindings (current + v-next)

| Key | Current | v-next |
|-----|---------|--------|
| `q` / `ctrl+c` | Quit (cancels prompt first) | unchanged |
| `r` | Refresh now | unchanged |
| `/` `f` | Filter prompt | unchanged |
| `enter` | Apply filter | expand selected feed row (when not in prompt) |
| `esc` | Cancel prompt / clear filter | unchanged |
| `c` | Clear filter | unchanged |
| `?` | Toggle help | unchanged |
| `↑/k ↓/j` `pgup/pgdn` `g/G` | Scroll feed | unchanged |
| `e` | — | Toggle endpoints panel collapse |
| `t` | — | Toggle stats strip on/off (short terminals) |
| `1`–`9` | — | Jump to per-endpoint filtered view (feed filtered by that endpoint) |
| `L` | — | Pause/resume live-follow (freeze feed while inspecting) |

### 4.2 Key principles
1. **No dead ends.** Every key has a visible escape (`esc` cancels anything). Prompt swallows navigation keys — never trapped mid-typing.
2. **Stable layout.** Zones never jump; only *content* changes on refresh. (Fix already applied for endpoint order.)
3. **Live-follow by default, manual scroll wins.** If the user scrolled up, keep the offset until they scroll back down (`g` or new `L`).
4. **Filter is visible state.** An active filter is always shown in the footer AND a compact chip in the endpoints panel header.

### 4.3 Prompt UX
- Prompt bar replaces the footer; the rest of the screen stays visible (context preserved).
- Live syntax feedback: `ep:order` highlights as tokens are typed.
- Enter with an invalid `status:` keeps you in prompt and shows inline error (`✗ status:abc`), not a silent dismiss.

---

## 5. States

| State | Trigger | Behavior |
|-------|---------|----------|
| **Empty** | no requests yet | Stats strip shows `—`; feed shows muted `(no requests yet)`; everything else normal. |
| **Live** | default | Feed sticks to bottom, updates each tick. |
| **Scrolled** | user scrolls up | Footer shows `⏸ scrolled`; offset preserved across updates. |
| **Filtered** | active filter | Feed + endpoint panel filtered; footer shows `showing X of Y recent (filtered)`. |
| **Prompt** | `/` pressed | Prompt bar visible; navigation swallowed; `esc`/`q` cancels. |
| **Help** | `?` | Overlay appended below footer; `?` or `esc` closes. |
| **Error** | watcher/logger failure | Status-dot turns red, one-line `lastErr` in footer; dashboard stays alive. |

---

## 6. Layout Adaptation

- **Minimum size:** 80×24 (target). Below 80 cols: config collapses to one line, stats strip drops p95→avg only.
- **Height < 20:** endpoints panel collapses automatically (still visible via `e`).
- **Width < 60:** feed lines truncate from the right (method/path kept, latency dropped first).
- The viewport always owns the remaining space; never render the feed taller than the screen.

---

## 7. Implementation Notes (for Developer)

- Sort endpoints by ID at refresh (done, `2b578b6`) — do NOT re-sort per render; cache the sorted slice in the model.
- Per-endpoint request counts + p95 already available via `Snapshot().EndpointLatency` (WS-010a) — the endpoints panel can light up with zero extra mock changes.
- Status-code tinting: pure string post-processing in `render.go` (a `tint(code) string` helper wrapping the status segment in lipgloss); no model changes.
- Collapse state (`e`, `t`) lives in the Model; `headerHeight()`/`syncFeed()` must account for it.
- Live-follow pause (`L`): a bool on the Model; `syncFeed()` skips `GotoBottom` while paused; footer shows `⏸ live` vs `▶ live`.
- Keep `dashboard` tea-free in `filter.go`; prompt-token highlighting is a pure `render` concern.

---

## 8. Out of Scope (v-next backlog)

- Multi-line row expansion / request detail view
- Mouse support (bubbletea supports it; deferred)
- Charts / sparklines in the stats strip
- Endpoint create/delete from the TUI (admin API already covers this)
- Color themes / configurable palette

---

## 9. Acceptance Criteria (v-next)

```
Given a running dashboard with 9 endpoints
When  the dashboard ticks
Then  the endpoint list order is stable (sorted by ID)
Given endpoints exceed available vertical space
When  the panel is open
Then  it collapses to "▾ (N more…)" and the feed still fills the screen
Given the user presses L
When  the feed is live
Then  live-follow pauses, footer shows "⏸ live", and new requests do not scroll the view
Given the user presses L again
When  paused
Then  live-follow resumes at the bottom
Given the user presses e
When  the endpoints panel is open
Then  it collapses; pressing e again expands it
Given a filter is active
When  the user presses c or esc
Then  the chip in the endpoints panel header and the footer filter text clear
Given the user presses enter while not in prompt mode
When  the feed has rows
Then  the selected row expands to show headers/body/query detail
```
