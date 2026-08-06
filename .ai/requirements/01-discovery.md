# Phase 01 — Discovery

> Stakeholder interviews, domain questions, initial scoping.

---

## Stakeholder Answers

| # | Question | Answer |
|---|----------|--------|
| 1 | Webhook types | **Custom only.** Webhooks are fully dynamic — users define path, query params, payload, headers, response expectations. |
| 2 | Target volume | Users create one or more endpoint configurations. |
| 3 | Throughput | No hard limit. System should handle whatever is thrown at it. |
| 4 | Key use cases | Testing integrations, demos, and local development workflows. |
| 5 | Persistence | Yes. Configurations stored as **YAML files**. Delivery logs stored in a `logs/` directory. |
| 6 | Interface | **CLI first** with an admin REST API for runtime management (`/_admin/endpoints`). No web app. |

> **Superseded 2026-08-05:** the admin REST API was removed from the product (stakeholder
> decision — file-based YAML management + hot-reload covers runtime changes). Interface is now
> **CLI only**. See `04-backlog.md` (WS-014/ADM-001).

---

## Inferred Constraints

- Config is file-based (YAML), not a database. No migrations, no SQL.
- CLI-first — run directly (`go run .` or the compiled binary) with flags `--addr`, `--stubs`, `--log-dir`. No subcommands.
- Fully dynamic webhooks mean a template engine for payloads (e.g., Go templates).
- Logs as files means structured logging (JSON lines) for grep-ability.

---

## Scope Summary

A Go CLI tool that:
1. Reads webhook endpoint configs from YAML stub files (hot-reloaded on change).
2. Starts an HTTP server exposing those endpoints.
3. Matches incoming requests by method, path, headers, query params, and payload.
4. Returns configured responses with optional delay, echo mode, Go template rendering, and response variability (weighted random / round-robin).
5. Logs every incoming request to `logs/` with full detail (headers, body, timestamp, latency, matched status).
6. Logs every incoming request to `logs/` with full detail (headers, body, timestamp, latency, matched status).
7. Prints a boxed startup banner with server config and all loaded endpoints.
8. Prints a one-liner per request showing timestamp, method, path, status, and matched endpoint.
9. Maintains an in-memory ring buffer of the last 50 requests with per-endpoint hit counters.

> **Note (2026-08-05):** item about the admin REST API (`/_admin/endpoints`) in the original
> scope was **removed from the product** by stakeholder decision — endpoint management is
> file-based (YAML + hot-reload) only.
