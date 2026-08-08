# Stubs Directory

Each `.yaml` file in this directory defines one webhook endpoint. Files are loaded automatically at startup and hot-reloaded on change — just save the file and the server picks it up.

---

## Available Stubs

| File | Endpoint | Method | Key Features |
|------|----------|--------|--------------|
| `health.yaml` | `GET /health` | GET | Minimal static response |
| `method-match.yaml` | `ANY /catch-all` | * | Wildcard method (empty = any) |
| `ping.yaml` | `GET /ping` | GET | Query parameter matching |
| `payload-match.yaml` | `POST /webhooks/notification` | POST | Exact request body matching |
| `user-create.yaml` | `POST /webhooks/user` | POST | Header + payload matching, delay |
| `order-placed.yaml` | `POST /webhooks/order` | POST | Header matching, custom status, delay |
| `order-created.yaml` | `POST /webhooks/orders` | POST | Dynamic templates (`{{.UUID}}`, `{{.Now}}`, etc.) |
| `echo.yaml` | `POST /webhooks/echo` | POST | Echo mode — returns your request body |
| `users-by-id.yaml` | `GET /users/**` | GET | Path glob matching (`**` / `*` / `?`) |
| `orders-filter.yaml` | `POST /orders/<5-digit-id>` | POST | Path regex + partial JSON body matching |

---

## How to Test

```bash
# Start the server
cd src && go run .

# Static response
curl http://localhost:8080/health

# Wildcard method — works with GET, POST, PUT, DELETE, etc.
curl http://localhost:8080/catch-all
curl -X POST http://localhost:8080/catch-all

# Query parameter matching — exact params required
curl "http://localhost:8080/ping?ping_id=11022022&ping_child=43443"

# Payload matching — exact body required
curl -X POST http://localhost:8080/webhooks/notification \
  -d '{"type":"webhook","version":"1.0"}'

# Header + payload matching + 200ms delay
curl -X POST http://localhost:8080/webhooks/user \
  -H "X-Event: user.created" \
  -d '{"event":"user.created"}'

# Header matching + 500ms delay
curl -X POST http://localhost:8080/webhooks/order \
  -H "X-Event: order.placed"

# Template mode — dynamic UUID, timestamp, method, body
curl -X POST http://localhost:8080/webhooks/orders \
  -H "Content-Type: application/json" \
  -d '{"product":"widget","qty":2}'

# Echo mode — returns exactly what you send
curl -X POST http://localhost:8080/webhooks/echo \
  -H "Content-Type: application/json" \
  -d '{"hello":"world"}'

# Path glob — ** spans zero or more segments
curl http://localhost:8080/users/123
curl http://localhost:8080/users/123/orders

# Path regex + partial JSON body matching — the body must CONTAIN
# {"event":"order.placed","amount":42} (extra fields are ignored)
curl -X POST http://localhost:8080/orders/12345 \
  -H "Content-Type: application/json" \
  -d '{"event":"order.placed","amount":42,"note":"ignored"}'

# 404 — no matching endpoint
curl http://localhost:8080/nonexistent
```

---

## YAML Reference

```yaml
# Minimal stub
id: my-endpoint
path: /some/path
response:
  status_code: 200

# Full stub with all options
id: my-endpoint
method: POST                              # HTTP method (empty = match any)
path: /webhooks/event                     # URL path (exact match)
path_glob: /users/**                       # URL path glob (exact `path` wins over glob when both could match)
path_regex: '^/orders/[0-9]{5}$'           # URL path regex (RE2 — anchor with ^…$ yourself)
headers:                                  # Request headers to match (optional)
  X-Event: event.created
query:                                    # Query params to match (optional)
  token: abc123
payload: '{"type":"test"}'                # Exact request body to match (optional)
payload_contains: '{"event":"user.created"}' # Partial JSON request-body match (optional)
response:
  status_code: 201                        # HTTP status (default: 200)
  body: '{"ok":true}'                     # Response body (optional)
  headers:                                # Response headers (optional)
    Content-Type: application/json
    X-Custom: myvalue
  delay: 300ms                            # Simulated latency (optional)
  echo: true                              # Return request body verbatim (optional)
  template: true                          # Enable Go template in body/headers (optional)
```

### Path matching (smarter matching — MAT-001/002)

At most **one** of `path` / `path_glob` / `path_regex` may be set per stub — a stub
setting more than one fails to load with a parse error naming the stub ID and the
conflicting fields.

| Field | Semantics |
|-------|-----------|
| `path` | Exact match on the raw URL path (unchanged behavior). |
| `path_glob` | Segment-wise glob: `**` = zero or more segments, `*` = exactly one segment, `?` = exactly one character. Trailing/leading slashes are normalized. An in-segment `*` (e.g. `/files/*.pdf`) is a load-time parse error. A glob with no wildcards behaves exactly like `path`. |
| `path_regex` | Go stdlib `regexp` (RE2). The pattern is **not** implicitly anchored — write `^…$` yourself when you want a full-path match. Invalid regexes are load-time parse errors. |

When several stubs could match a request, the most specific one wins
(MAT-004): exact path > glob > regex; among globs, fewer wildcards win
(`/users/*/orders` beats `/users/**`); identical specificity falls back to
load order, deterministically.

### Partial JSON body matching (`payload_contains` — MAT-003)

`payload_contains` matches when the request body is valid JSON and *contains*
the spec — extra request fields are ignored. Semantics:

- **Objects:** every spec key must be present and subset-match recursively
  (extra request keys ignored).
- **Arrays:** each spec element must match a **distinct** request element,
  order-independent.
- **Scalars:** deep equality with numeric cross-type equality (`42` matches
  `42.0`). `null` matches only a literal `null` body.
- A non-JSON request body never matches a `payload_contains` stub.

The spec must be quoted in YAML. At most one of `payload` / `payload_contains`
may be set — parse error otherwise. Invalid spec JSON is a load-time parse error.

### Feature Flags

| Flag | Location | Effect |
|------|----------|--------|
| `echo: true` | `response` | Returns the request body verbatim. Ignores `response.body`. |
| `template: true` | `response` | Processes `body` and header values as Go templates. |

### Template Variables (when `template: true`)

| Variable | Description |
|----------|-------------|
| `{{.UUID}}` | UUID v4 — unique per request |
| `{{.Now}}` | Current time in RFC3339 format |
| `{{.Timestamp}}` | Current Unix timestamp (seconds) |
| `{{.RequestID}}` | Unique request ID (also logged) |
| `{{.Method}}` | Incoming HTTP method |
| `{{.Path}}` | Incoming URL path |
| `{{.Body}}` | Raw request body |

### Template Functions (when `template: true`)

| Function | Description |
|----------|-------------|
| `{{Header "X-Key"}}` | Value of a request header |
| `{{Query "param"}}` | Value of a query parameter |
| `{{RandomInt 1 100}}` | Random integer in range |
| `{{RandomHex 8}}` | Random hex string of given length |

---

## Request Matching Logic

An endpoint matches a request when **all** specified conditions are met:

- `method` — must equal request method (empty matches any method)
- `path` | `path_glob` | `path_regex` — exactly one path matcher (exact path, glob, or regex)
- `headers` — all listed headers must be present with the exact values
- `query` — all listed query params must be present with the exact values
- `payload` | `payload_contains` — request body must match exactly (payload) or contain the JSON subset (payload_contains)

When several endpoints match, the **most specific** one wins (exact path > glob > regex, fewer glob wildcards first, load order last). If no endpoint matches, the server responds with `404`.
