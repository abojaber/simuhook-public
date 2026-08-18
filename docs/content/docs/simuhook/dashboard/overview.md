---
title: "Overview"
type: docs
weight: 1
---

# Dashboard Overview

The dashboard is a single full-screen terminal view, refreshed once per second, showing everything happening in your Simuhook instance.

## Running the Dashboard

| Mode | When it starts |
|------|----------------|
| `--tui auto` (default) | On a terminal (TTY); falls back to one-line-per-request output otherwise. |
| `--tui on` | Always shows the dashboard, even piped. |
| `--tui off` | Never shows the dashboard; prints the startup banner + one line per request. |

```bash
simuhook            # dashboard on a normal terminal
simuhook --tui off  # line output
```

## What It Shows

### Wide Layout (terminal ≥ 100 columns wide)

At a normal terminal size the dashboard shows two side-by-side panels:

{{< screenshot-wip dashboard-wide >}}

**Left — REGISTERED ENDPOINTS**

A table of every loaded stub: method, endpoint path, stub ID, request count, and approximate p95 latency. Scroll it with `shift+↑` / `shift+↓` (or `K` / `J`). Hide it with `e` to give the stream full width.

**Right — LIVE REQUEST STREAM**

One row per request: time, method, path, stub ID, and a color-tinted status code (2xx green, 3xx cyan, 4xx yellow, 5xx red). The feed follows the newest request at the bottom unless you pause it.

### Narrow Layout

On smaller terminals the dashboard falls back to a vertical layout with the same information.

### Chrome

Across the top: the title row with a pause/resume button, server info chips (address, stubs dir, uptime, request count), the log directory, and metrics chips — **Average / Min / Max / p95 latency**. A footer shows feed position and quick keys.

## Inspecting a Request

Two ways to see full request + response detail:

| Key | What opens |
|-----|-----------|
| `enter` | Expands the selected row **inline** under it: request id, matched, timestamp, headers, query, request body, then a `RESPONSE` section with status, response headers, and response body. |
| `i` | Opens a standalone **inspector** view of the selected row: a `REQUEST` section header, then full REQUEST + RESPONSE sections with pretty-printed JSON bodies. |

{{< screenshot-wip dashboard-detail >}}

Color coding (in both views): request keys cyan, response keys magenta, request bodies yellow, response bodies green, JSON syntax highlighted. Request sections carry a dark-gray background band; response sections a dark-navy band, separated by a muted `─` rule.

## Also See

- [Key Bindings]({{< relref "/docs/simuhook/dashboard/key-bindings" >}}) — every key.
- [Filtering]({{< relref "/docs/simuhook/dashboard/filtering" >}}) — `ep:ping` / `method:POST` / `status:500` / free text.
- [Logging]({{< relref "/docs/simuhook/features/logging" >}}) — the same data written to JSON lines.