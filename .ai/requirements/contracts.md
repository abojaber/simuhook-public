# Contracts

> API contracts, data schemas, payload formats.

---

## Configuration Schema (YAML per-file stubs)

Each YAML file in the stubs directory defines one endpoint.

```yaml
# <endpoint-id>.yaml
id: string               # unique endpoint ID, used for log file naming
method: GET | POST | PUT | PATCH | DELETE | HEAD  # empty string = match all
path: string             # URL path, must start with /
headers:                 # optional: request header matching conditions
  X-Signature: "abc123"  # exact value match
query:                   # optional: query parameter matching conditions
  token: "abc123"        # exact value match
payload: "{}"            # optional: request body exact match

response:                # singular response (used when responses[] is absent)
  status_code: 200       # HTTP status code, default 200
  headers:               # optional: response headers
    Content-Type: application/json
    X-Request-ID: "{{.RequestID}}"
  body: "{}"             # response body, supports Go template
  delay: 0s              # artificial delay, default 0
  echo: false            # if true, return request body verbatim overrides body
  template: false        # if true, process body and headers as Go templates

responses:               # optional: multiple response variants
  - status_code: 200
    body: '{"status":"ok"}'
    weight: 80           # weight for random selection (optional, omit for round-robin)
    delay: 0s
  - status_code: 500
    body: '{"error":"fail"}'
    weight: 20
    delay: 2s
```

---

## Log Entry Schema (JSON Lines)

```json
{
  "timestamp": "2025-01-15T10:30:00.123Z",
  "request_id": "uuid-v4",
  "webhook": "order-created",
  "method": "POST",
  "path": "/webhooks/orders",
  "headers": {
    "Content-Type": "application/json",
    "X-Signature": "abc123"
  },
  "body": "{\"order_id\": 42}",
  "query": {},
  "response_status": 200,
  "latency_ms": 45.2,
  "matched": true,
  "response_headers": {
    "Content-Type": "application/json"
  },
  "response_body": "{\"status\":\"ok\"}"
}
```
> **WS-018 / UXR-013:** `response_headers` is always an object (never null); only headers Simuhock explicitly set via `w.Header().Set()` are captured (not Content-Length, Date, or sniffed Content-Type). `response_body` is the exact bytes sent, uncapped; for echo endpoints the request body appears here too (carried as the response). Both fields are additive — existing log consumers can ignore unknown keys.

---

## CLI Signature

```
go run . [flags]     Start the mock HTTP server

Flags:
  --addr     string   Listen address (default ":8080")
  --stubs    string   Stubs directory path (default "../stubs")
  --log-dir  string   Log output directory (default "logs")

Environment variables (override defaults):
  PORT       string   Listen port (e.g. "9090")
  STUBS_DIR  string   Stubs directory path
  LOG_DIR    string   Log output directory

Admin REST API (runtime endpoint management):
  GET    /_admin/endpoints          List all endpoints
  POST   /_admin/endpoints          Add an endpoint (JSON body)
  GET    /_admin/endpoints/{id}     Get a specific endpoint
  PUT    /_admin/endpoints/{id}     Update a specific endpoint
  DELETE /_admin/endpoints/{id}     Remove a specific endpoint

---

## Dashboard Output

The CLI prints two types of information on startup and during runtime.

### Startup Banner

Printed once at startup, showing server configuration and loaded endpoints:

```
╔══════════════════════════════════════════════════════════╗
║                   Simuhook Mock Server                   ║
╠══════════════════════════════════════════════════════════╣
║ Address:    :8080                                        ║
║ Stubs:      ../stubs (9 files)                           ║
║ Log dir:    logs                                         ║
║ Admin API:  http://localhost:8080/_admin/endpoints       ║
╠══════════════════════════════════════════════════════════╣
║  Endpoints:                                              ║
║    GET   /health → health                                ║
║    POST  /webhooks/echo → echo                           ║
║    ...                                                   ║
╚══════════════════════════════════════════════════════════╝
```

### Per-Request Lines

Printed after each response, one line per request:

```
HH:MM:SS  METHOD /path → STATUS  (endpoint-id)
```

Examples:
```
14:23:01  GET   /health → 200  (health)
14:23:15  POST  /nonexistent → 404  (unmatched)
14:23:20  POST  /webhooks/echo → 200  (echo)
```

---

## StatsTracker (In-Memory)

Tracked for every request via `mock.StatsTracker`:

| Method | Description |
|--------|-------------|
| `NewStatsTracker(maxLogs int)` | Create tracker with ring buffer capacity |
| `RecordRequest(entry LogEntry)` | Push a log entry, update endpoint hit counter |
| `Snapshot() StatsSnapshot` | Thread-safe copy of all stats |
| `Uptime() time.Duration` | Time since tracker creation |

**`StatsSnapshot` fields:**

| Field | Type | Description |
|-------|------|-------------|
| `Uptime` | `time.Duration` | Server uptime |
| `HitCounts` | `map[string]uint64` | Per-endpoint request count |
| `RecentLogs` | `[]LogEntry` | Last N log entries (in order) |
| `TotalLogs` | `uint64` | Total log entries recorded |
```