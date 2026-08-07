# Simuhook

A mock HTTP server. Define endpoints as YAML files — the server watches the directory, matches incoming requests, and returns configured responses. Changes to stub files are applied live without restart.

## Quick Start

```bash
cd src && go run .
```

Starts on `:8080` (override with `--addr` or `PORT`). Watches `../stubs/` (override with `--stubs` or `STUBS_DIR`). Logs every request to `logs/` (override with `--log-dir` or `LOG_DIR`).

## CLI

```
simuhook [flags]

  --addr :8080          listen address (env PORT)
  --stubs ../stubs      stubs directory (env STUBS_DIR)
  --log-dir logs        log directory (env LOG_DIR)
  --tui auto|on|off     dashboard mode: auto = TUI on a TTY, line output otherwise
  --recent N            recent-log ring buffer size (default 50; also the TUI feed depth)
  --max-log-body N      cap stored request/response body size in bytes for logs and
                        dashboard (default 0 = unlimited; env LOG_MAX_BODY). Never affects
                        the actual HTTP response bytes.
  --help                print usage and exit
  --version             print version and exit
```

The TUI dashboard shows live request counts, latency stats (incl. an approximate p95), and a filterable recent-request feed. Press `?` in the TUI for the key reference. `enter` on a feed row expands the full request/response detail; `i` opens the standalone inspector.

## Build

```bash
go build -o simuhook ./src                    # dev build (--version prints "dev")
go build -ldflags "-X main.version=v0.1.0" -o simuhook ./src   # release build
```

## Stub Files (YAML)

Place `.yaml` / `.yml` files in the `stubs/` directory. Each file defines one endpoint. Files are loaded at startup and hot-reloaded on change. See [`stubs/README.md`](stubs/README.md) for a full reference and usage examples.

```yaml
# stubs/user-create.yaml
id: user-create
method: POST
path: /webhooks/user
headers:
  X-Event: user.created
payload: '{"event":"user.created"}'
response:
  status_code: 201
  body: '{"received":true,"event":"user.created"}'
  headers:
    X-Mock: "yes"
  delay: 200ms
```

| Field | Required | Description |
|---|---|---|
| `id` | yes | Unique endpoint identifier |
| `method` | no | HTTP method. Empty matches any method |
| `path` | yes | Exact request path to match |
| `headers` | no | Request headers that must match |
| `payload` | no | Exact request body to match |
| `response.status_code` | no | HTTP status code to return (default: 200) |
| `response.body` | no | Response body string |
| `response.headers` | no | Response headers |
| `response.delay` | no | Simulated latency (e.g. `200ms`, `1s`) |

### Request Matching

An endpoint matches when **all** specified conditions are met:

- `method` — must equal request method (empty = match any)
- `path` — exact match
- `headers` — all key/value pairs present in request
- `query` — all key/value pairs present in query string
- `payload` — exact body match

## Request Logging

Every request to a webhook endpoint is logged as a JSON line to `logs/<webhook-id>.log`. Unmatched requests go to `logs/_unmatched.log`.

```json
{
  "timestamp": "2025-01-15T10:30:00.123Z",
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

The log directory defaults to `logs/` — override with `--log-dir` or `LOG_DIR` env var. Unmatched requests are logged with `matched: false` and a `404` response status.

## Endpoint Management

Endpoints are managed entirely through YAML stub files in the stubs directory — create, edit, or delete a file and the watcher hot-reloads it live (no restart, no API). Endpoint matching supports method, exact path, query, headers, payload, and response variants with weights (round-robin on equal weight).

```bash
curl http://localhost:8080/ping                     # → 200 (from ping.yaml stub)
curl -X POST http://localhost:8080/ping            # → 404 (method mismatch)
```

## Architecture

```
src/
├── main.go               # Entry point: flags, --version, HTTP server, graceful shutdown
├── dashboard/            # bubbletea TUI dashboard
│   ├── dashboard.go      # Run() — program entry (alternate screen)
│   ├── model.go          # tea.Model, StatsSource, tick/prompt/key handling
│   ├── render.go         # feed/summary/detail rendering (segment-wise styling)
│   ├── render_detail.go  # pure helpers: band(), highlightJSON, renderStyledLine
│   ├── filter.go         # Filter, ParseFilter, Matches (pure)
│   └── *_test.go
└── mock/
    ├── types.go          # Endpoint, Response structs
    ├── registry.go       # Store, match, CRUD endpoints (thread-safe)
    ├── handler.go        # Single request-handling source of truth
    ├── stub.go           # YAML stub file parser
    ├── logger.go         # LogEntry, JSON Lines logging
    ├── stats.go          # StatsTracker: counts, ring, latency stats
    ├── templating.go     # Body/header template execution
    └── watcher.go        # File watcher (hot-reload)

stubs/                    # YAML stub definitions (hot-reloaded)
├── health.yaml
├── order-placed.yaml
├── ping.yaml
└── ...
```

## Testing

```bash
go test -v -race ./src/...
```
