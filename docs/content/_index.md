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
| **Hot-reload YAML stubs** | Create, edit, or delete stub files and the server picks them up instantly — no restart, no API. |
| **Smart matching** | Match on method, path (exact / glob / regex), headers, query params, and request body — exact, partial-JSON, or template-driven. |
| **Dynamic responses** | Go templates with faker data generators and JSONPath request-body extraction return realistic, request-aware payloads. |
| **Live TUI dashboard** | Real-time counts, latency stats (including an approximate p95), and a filterable feed of recent requests. |

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

## Next Steps

- **[Getting Started]({{< relref "/docs/getting-started" >}})** — install, configure, and run your first stub.
- **[Stubs Reference]({{< relref "/docs/stubs" >}})** — the complete YAML reference and matching logic.
- **[Templates]({{< relref "/docs/templates" >}})** — dynamic responses with faker data and JSONPath.
- **[Examples]({{< relref "/docs/examples" >}})** — ready-to-paste curl commands and REST API patterns.

> **Note:** Simuhook will be open-sourced in the future. This documentation covers how to **use** the tool.