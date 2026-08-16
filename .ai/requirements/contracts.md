# Contracts

> API contracts, data schemas, payload formats.

---

## Configuration Schema (YAML per-file stubs)

Each YAML file in the stubs directory defines one endpoint.

```yaml
# <endpoint-id>.yaml
id: string               # unique endpoint ID, used for log file naming
method: GET | POST | PUT | PATCH | DELETE | HEAD  # empty string = match all
path: string             # URL path, must start with / (exact match)
path_glob: string        # optional: URL path glob (MAT-001): ** = 0+ segments,
                         #   * = one segment, ? = one character. In-segment *
                         #   is a parse error. Mutually exclusive with path
                         #   and path_regex.
path_regex: string       # optional: URL path regex (MAT-002), Go stdlib RE2.
                         #   NOT implicitly anchored — write ^...$ yourself.
                         #   Mutually exclusive with path and path_glob.
headers:                 # optional: request header matching conditions
  X-Signature: "abc123"  # exact value match
query:                   # optional: query parameter matching conditions
  token: "abc123"        # exact value match
payload: "{}"            # optional: request body exact match
payload_contains: '{"event":"user.created"}'
                         # optional: partial JSON request-body match (MAT-003).
                         #   Request body must be valid JSON containing the
                         #   spec as a subset (extra fields ignored; arrays
                         #   order-independent, distinct elements; numeric
                         #   equality across 42/42.0; null matches only null).
                         #   Mutually exclusive with payload.

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

> **Smarter matching (MAT-001..004):** at most one of `path` / `path_glob` /
> `path_regex` and at most one of `payload` / `payload_contains` may be set —
> load-time parse errors name the stub ID and conflicting fields. When
> multiple endpoints match, the most specific wins: exact path > glob > regex;
> fewer glob wildcards first; then load order (deterministic, independent of
> map iteration order). `Endpoint` serializes `path_glob`, `path_regex`, and
> `payload_contains` (omitempty); compiled regex/glob/seq fields are
> unexported and never serialized.

---

## Template Functions (when `response.template: true`)

All funcs are niladic unless noted. Sources: existing funcs (unchanged) +
`gofakeit/v7` curated subset (TPL-001) + `JSONPath` (TPL-002).

### Existing
| FuncMap name | Signature | Behavior |
|---|---|---|
| `Header` | `func(string) string` | Request header value (missing → "") |
| `Query` | `func(string) string` | Query parameter value (missing → "") |
| `RandomInt` | `func(int, int) int` | Random int in `[min, max)`; degenerate range → min |
| `RandomHex` | `func(int) string` | Random lowercase hex string of given length |

### Faker generators (gofakeit v7.15.0, pinned D16)
| FuncMap name | gofakeit wiring | Return |
|---|---|---|
| `Name` / `FirstName` / `LastName` | `gofakeit.Name()` etc. | string |
| `Email` | `gofakeit.Email()` | string |
| `Phone` | `gofakeit.Phone()` | string |
| `City` / `Country` / `State` / `Zip` / `Street` | `gofakeit.City()` etc. | string |
| `Company` / `JobTitle` | `gofakeit.Company()` etc. | string |
| `IPv4Address` / `IPv6Address` / `DomainName` / `MACAddress` / `UserAgent` | `gofakeit.IPv4Address()` etc. | string |
| `CreditCardNumber` | closure over `gofakeit.CreditCardNumber(nil)` | string (Luhn-valid) |
| `Price` | closure over `gofakeit.Price(0.01, 999.99)` formatted `"%.2f"` (D11) | string e.g. `"42.99"` |
| `ProductName` | `gofakeit.ProductName()` | string |
| `ULID` | `gofakeit.ID()` (**D9**: no ULID in gofakeit; 20-char URL-safe) | string |
| `HexColor` | `gofakeit.HexColor()` | string `#rrggbb` |
| `Word` | `gofakeit.Word()` | string |
| `Sentence` | `gofakeit.Sentence` — **deprecated upstream, args ignored (D10)** | string |
| `Paragraph` | `gofakeit.Paragraph` — **deprecated upstream, args ignored (D10)** | string |
| `Bool` | `gofakeit.Bool()` | `"true"`/`"false"` |
| `UUID` | `gofakeit.UUID()` | string (v4) |

### JSONPath (TPL-002)
| FuncMap name | Signature | Behavior |
|---|---|---|
| `JSONPath` | `func(string) string` | Extract from parsed JSON request body. `$` = root; `$.a.b` = keys; `$[0]` = array index. Missing/invalid/non-JSON body → `""` (no panic); `null` → `"null"`; objects/arrays → canonical `json.Marshal`; numbers → `FormatFloat('g', -1, 64)` (D14). Keys are `[A-Za-z0-9_-]`, no spaces (D15). Filters/wildcards/recursive descent rejected. Body parsed exactly once per request into `TemplateData.parsedBody`. |

### Fields (unchanged, separate namespace from funcs)
`{{.UUID}}` `{{.Now}}` `{{.Timestamp}}` `{{.RequestID}}` `{{.Method}}` `{{.Path}}` `{{.Body}}`. `{{UUID}}` (func) and `{{.UUID}}` (field) are independent.

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
  "response_body": "{\"status\":\"ok\"}",
  "response_variant": 0
}
```
> **WS-018 / UXR-013:** `response_headers` is always an object (never null); only headers Simuhock explicitly set via `w.Header().Set()` are captured (not Content-Length, Date, or sniffed Content-Type). `response_body` is the exact bytes sent, uncapped by default; for echo endpoints the request body appears here too (carried as the response). Both fields are additive — existing log consumers can ignore unknown keys.
> **`--max-log-body` / `LOG_MAX_BODY` (backlog: max-log-body-cap):** when set (> 0), `body` and `response_body` are truncated to at most N bytes at capture time, walking back to the nearest UTF-8 rune boundary (a split multi-byte rune is dropped rather than emitting invalid bytes). This is **capture-time-only** — it never affects the bytes written to the actual HTTP response, nor echo/template rendering, which always use the full, uncapped body. Default `0` = unlimited (current behavior, unchanged unless opted in).
> **`response_variant` (backlog: response-variant-index):** present only for endpoints configured with a weighted/round-robin `responses:` list — the 0-based index of the variant `PickResponse` selected for this request. **Omitted entirely** (not `null`) when the endpoint used its singular `response:` (no variants configured) — a pointer field distinguishes "not applicable" from "variant index 0". Additive; existing log consumers can ignore it.

---

## CLI Signature

```
go run . [flags]     Start the mock HTTP server

Flags:
  --addr     string   Listen address (default ":8080")
  --stubs    string   Stubs directory path (default "../stubs")
  --log-dir  string   Log output directory (default "logs")
  --max-log-body int  Cap stored request/response body size in bytes for logs
                       and dashboard; 0 = unlimited (default 0)

Environment variables (override defaults):
  PORT          string   Listen port (e.g. "9090")
  STUBS_DIR     string   Stubs directory path
  LOG_DIR       string   Log output directory
  LOG_MAX_BODY  string   Same as --max-log-body (non-negative integer)
```

> **Admin REST API REMOVED (stakeholder decision 2026-08-05).** The `/_admin/endpoints` CRUD
> routes no longer exist — endpoint management is file-based only (edit/create/delete YAML in
> `STUBS_DIR`; the watcher hot-reloads). The full historical wire contract is preserved in
> `07-admin-handler.md` as the reintroduction contract (backlog WS-014/ADM-001).

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