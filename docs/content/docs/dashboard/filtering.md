---
title: "Filtering"
type: docs
weight: 3
---

# Filtering

Press `/` or `f` to open the filter prompt. Type a filter and press `enter` to apply; the feed shows only matching entries. Press `esc` or `c` to clear.

## Filter Syntax

| Prefix | Matches |
|--------|---------|
| `ep:<id>` | Endpoint with that stub ID |
| `method:<METHOD>` | HTTP method (e.g. `method:POST`) |
| `status:<code>` | Response status code (e.g. `status:500`) |
| Bare text | Any text in the log line |

Filters combine with spaces:

```
ep:ping status:200 method:GET
```

Matches entries for the `ping` endpoint that returned `200` via a `GET`.

## Examples

| Filter | What you get |
|--------|--------------|
| `ep:user-create` | Only requests matched by the `user-create` stub |
| `method:POST` | Only POST requests (matched or unmatched) |
| `status:500` | Only 5xx responses |
| `error` | Anything containing the text "error" (status, body, path — anywhere) |
| `method:POST status:200` | POSTs that succeeded |

## Behavior

- Filtering is **view-only** — the underlying feed and logs are never mutated.
- The footer shows how many entries matched: `showing X of Y recent (filtered)`.
- An active filter shows as a chip in the panel title and in the footer prompt bar.
- Invalid `status:` values show an inline error and keep you in the prompt until fixed.

{{< screenshot-wip dashboard-filter >}}

## Also See

- [Key Bindings]({{< relref "/docs/dashboard/key-bindings" >}}) — full key reference.