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
| 6 | Interface | **CLI only** with a simple terminal UI (not a web app, not a REST API). |

---

## Inferred Constraints

- Config is file-based (YAML), not a database. No migrations, no SQL.
- CLI-first means subcommands: `simuhook serve`, `simuhook config`, `simuhook logs`.
- Fully dynamic webhooks mean a template engine for payloads (e.g., Go templates).
- Logs as files means structured logging (JSON lines) for grep-ability.

---

## Scope Summary

A Go CLI tool that:
1. Reads webhook endpoint configs from YAML files.
2. Starts an HTTP server exposing those endpoints.
3. Logs every incoming request to `logs/` with full detail (headers, body, timestamp, latency).
4. Allows users to define custom responses (status code, headers, body, delay).
