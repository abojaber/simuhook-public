# Contracts

> API contracts, data schemas, payload formats.

---

## Configuration Schema (YAML)

```yaml
# config.yaml
port: 8080                     # default listen port
log_dir: logs                  # directory for log output

webhooks:
  - name: string               # unique name, used for log file naming
    path: string               # URL path, must start with /
    method: GET | POST | PUT | PATCH | DELETE | HEAD
    match:                     # optional: request matching conditions
      headers:                 # optional: map of header name to regex/value
        X-Signature: ".+"
      query:                   # optional: map of query param to regex/value
        token: "abc123"
    response:
      status: 200              # HTTP status code, default 200
      headers:                 # optional: response headers
        Content-Type: application/json
        X-Request-ID: "{{.UUID}}"
      body: "{}"               # response body, supports Go template
      delay: 0s                # artificial delay, default 0
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
  "matched": true
}
```

---

## CLI Signature

```
simuhook serve     Start the webhook simulator
  --config  path    Path to YAML config file (default: config.yaml)
  --port    N       Listen port (default: 8080)

simuhook logs      View delivery logs
  --webhook name    Filter by webhook name
  --tail            Follow log output

simuhook validate  Validate config file syntax
  --config  path    Path to YAML config file
```