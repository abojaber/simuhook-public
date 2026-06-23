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
headers:                                  # Request headers to match (optional)
  X-Event: event.created
query:                                    # Query params to match (optional)
  token: abc123
payload: '{"type":"test"}'                # Exact request body to match (optional)
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
- `path` — exact match on the URL path
- `headers` — all listed headers must be present with the exact values
- `query` — all listed query params must be present with the exact values
- `payload` — request body must match exactly

If no endpoint matches, the server responds with `404`.
