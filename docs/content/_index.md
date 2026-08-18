---
title: "Simuhook"
type: docs
weight: 1
---

# Simuhook — Mock HTTP Server for Webhook Testing

Define mock endpoints as YAML files. Simuhook watches the directory, matches incoming requests against your stubs, and returns the configured responses — all hot-reloaded live, no restarts.

Test your integrations against realistic webhook endpoints without depending on real third-party services.

## Why Simuhook?

| | |
|---|---|
| <i class="fa-solid fa-rotate" aria-hidden="true"></i> **Hot-reload YAML stubs** | Create, edit, or delete stub files and the server picks them up instantly — no restart, no API. |
| <i class="fa-solid fa-sliders" aria-hidden="true"></i> **Smart matching** | Match on method, path (exact / glob / regex), headers, query params, and request body — exact, partial-JSON, or template-driven. |
| <i class="fa-solid fa-wand-magic-sparkles" aria-hidden="true"></i> **Dynamic responses** | Go templates with faker data generators and JSONPath request-body extraction return realistic, request-aware payloads. |
| <i class="fa-solid fa-gauge-high" aria-hidden="true"></i> **Live TUI dashboard** | Real-time counts, latency stats (including an approximate p95), and a filterable feed of recent requests. |

> [!TIP]
> **No restarting, ever.** Edit a stub file while the server runs and the watcher hot-reloads it live. Point your tests at Simuhook and iterate as fast as your editor saves.

## Quick Start

```bash
simuhook
```

Then in another terminal:

```bash
curl http://localhost:8080/health
# → {"status":"ok"}
```

The server starts on `:8080`, serves the stubs in `stubs/`, and logs every request as a JSON line to `logs/`.

> [!NOTE]
> The repository ships with example stubs, so the quick start above works out of the box. See the [curl cookbook]({{< relref "/docs/examples/curl-cookbook" >}}) for more commands.

## Next Steps

- <i class="fa-solid fa-rocket" aria-hidden="true"></i> **[Getting Started]({{< relref "/docs/getting-started" >}})** — install, configure, and run your first stub.
- <i class="fa-solid fa-book" aria-hidden="true"></i> **[Stubs Reference]({{< relref "/docs/stubs" >}})** — the complete YAML reference and matching logic.
- <i class="fa-solid fa-wand-magic-sparkles" aria-hidden="true"></i> **[Templates]({{< relref "/docs/templates" >}})** — dynamic responses with faker data and JSONPath.
- <i class="fa-solid fa-list-check" aria-hidden="true"></i> **[Examples]({{< relref "/docs/examples" >}})** — ready-to-paste curl commands and REST API patterns.

> [!NOTE]
> Simuhook will be open-sourced in the future. This documentation covers how to **use** the tool.