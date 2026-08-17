---
title: "Logging"
type: docs
weight: 4
---

# Logging

Every request to a webhook endpoint is logged as a **JSON line**:

| Log file | Contents |
|----------|----------|
| `logs/<webhook-id>.log` | Matched requests for the endpoint with that `id`. |
| `logs/_unmatched.log` | Requests that matched no endpoint (responded `404`). |

The log directory defaults to `logs/` — override with `--log-dir` or the `LOG_DIR` environment variable.

## Log Entry

```json
{
  "timestamp": "2026-08-18T10:30:00.123Z",
  "request_id": "uuid-v4",
  "webhook": "health",
  "method": "GET",
  "path": "/health",
  "headers": {"User-Agent": "curl/8.7.1"},
  "body": "",
  "query": {},
  "response_status": 200,
  "response_headers": {"Content-Type": "application/json"},
  "response_body": "",
  "latency_ms": 0.02,
  "matched": true
}
```

| Field | Description |
|-------|-------------|
| `timestamp` | Request time, RFC3339 with milliseconds (UTC). |
| `request_id` | Unique request ID (same value available in templates as `{{.RequestID}}`). |
| `webhook` | The endpoint `id` that matched (absent / empty for unmatched). |
| `method` | HTTP method. |
| `path` | Request path. |
| `headers` | Request headers (map). |
| `body` | Request body. |
| `query` | Query parameters (map). |
| `response_status` | Status code returned. |
| `response_headers` | Response headers (map). |
| `response_body` | Response body. |
| `latency_ms` | Total time to serve the request — **includes** any configured delay. |
| `matched` | `true` if a stub matched, `false` for a `404`. |

## Capping Stored Bodies

Large payloads blow up your logs. `--max-log-body N` (or `LOG_MAX_BODY`) caps the stored `body` / `response_body` at capture time, in bytes:

```bash
simuhook --max-log-body 1024
```

| Effect | Detail |
|--------|--------|
| **Limited** | `body`, `response_body` in log files and the dashboard are truncated (rune-safe). |
| **Never affected** | The actual HTTP response sent to clients, and `echo` / `template` rendering. |

Default is `0` = unlimited.

## Example — Matched vs Unmatched

```bash
curl http://localhost:8080/health        # → logs/health.log, matched: true
curl http://localhost:8080/nonexistent   # → logs/_unmatched.log, matched: false, 404
```

## Also See

- The [TUI dashboard]({{< relref "/docs/dashboard" >}}) shows the same data live, with counts, latency stats, and a filterable feed.