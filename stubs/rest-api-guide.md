# RESTful API Simulation — Cheat Sheet

> How to simulate RESTful API behavior using Webhooker stub files.
> Each pattern references a working example in this directory.

---

## CRUD ↔ HTTP Mapping

| Operation | HTTP Method | Typical Status | Example Stub |
|-----------|-------------|----------------|--------------|
| List / Read | `GET` | `200 OK` | `health.yaml`, `ping.yaml` |
| Create | `POST` | `201 Created` | `user-create.yaml` |
| Replace | `PUT` | `200 OK` | — |
| Partial update | `PATCH` | `200 OK` | — |
| Delete | `DELETE` | `204 No Content` | — |
| Any method | *(empty)* | `200 OK` | `method-match.yaml` |

---

## Patterns

### 1. Static read endpoint

Return a fixed JSON response for a GET request.

```yaml
# GET /api/health → {"status":"ok"}
id: health
method: GET
path: /api/health
response:
  status_code: 200
  body: '{"status":"ok"}'
```

**See:** `health.yaml`

---

### 2. Create with request matching

Match on specific headers and/or payload to simulate different create behaviors.

```yaml
# POST /api/users  (requires X-Event: user.created)
id: user-create
method: POST
path: /api/users
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

**See:** `user-create.yaml`, `order-placed.yaml`

**Variation — header-only match:**

```yaml
headers:
  X-Event: order.placed
response:
  status_code: 202
  delay: 500ms
```

**See:** `order-placed.yaml`

---

### 3. Query-filtered read

Require exact query parameters to match. Useful for simulating filtered list endpoints.

```yaml
# GET /api/search?type=webhook&version=1.0
id: filtered-search
method: GET
path: /api/search
query:
  type: webhook
  version: "1.0"
response:
  status_code: 200
  body: '{"results":[]}'
```

**See:** `ping.yaml` (uses `query:` with `ping_id` and `ping_child`)

> **Note:** Query matching is all-or-nothing — every listed param must be present with the exact value. Missing or extra params cause a 404.

---

### 4. Body-constrained create

Match only when the request body matches an exact payload.

```yaml
# POST /api/notifications  (body must match exactly)
id: notification-create
method: POST
path: /api/notifications
payload: '{"type":"webhook","version":"1.0"}'
response:
  status_code: 200
  body: '{"status":"notification received"}'
```

**See:** `payload-match.yaml`

> Useful for testing that your client sends the exact body structure you expect.

---

### 5. Wildcard method handler

Leave `method` empty to match any HTTP verb on the same path.

```yaml
# ANY /api/catch-all  (works with GET, POST, PUT, DELETE, etc.)
id: catch-all
path: /api/catch-all
response:
  status_code: 200
  body: '{"message":"matched any method"}'
```

**See:** `method-match.yaml`

> Ideal for health checks, pre-flight handlers, or debug endpoints where you don't care about the method.

---

### 6. Dynamic payloads (templates)

Use Go templates to inject request-specific data into the response.

```yaml
# POST /api/orders → dynamic UUID, timestamp, method, body
id: order-created
method: POST
path: /api/orders
response:
  status_code: 200
  template: true
  headers:
    X-Request-ID: "{{.RequestID}}"
    X-Timestamp: "{{.Now}}"
  body: '{"id":"{{.UUID}}","event":"order.created","timestamp":{{.Timestamp}},"method":"{{.Method}}"}'
```

**See:** `order-created.yaml`

**Available template variables:**

| Variable | Description |
|----------|-------------|
| `{{.UUID}}` | UUID v4 — unique per request |
| `{{.Now}}` | Current time in RFC3339 format |
| `{{.Timestamp}}` | Current Unix timestamp (seconds) |
| `{{.RequestID}}` | Unique request ID (also logged) |
| `{{.Method}}` | Incoming HTTP method |
| `{{.Path}}` | Incoming URL path |
| `{{.Body}}` | Raw request body |

**Available template functions:**

| Function | Description |
|----------|-------------|
| `{{Header "X-Key"}}` | Value of a request header |
| `{{Query "param"}}` | Value of a query parameter |
| `{{RandomInt 1 100}}` | Random integer in range |
| `{{RandomHex 8}}` | Random hex string of given length |

---

### 7. Error / status simulation

Return different responses based on weighted probability or round-robin rotation.

```yaml
# POST /api/payments → 80% success, 15% bad request, 5% server error (with 2s delay)
id: flaky-payment
method: POST
path: /api/payments
responses:
  - status_code: 200
    body: '{"status":"completed"}'
    weight: 80
  - status_code: 400
    body: '{"error":"insufficient_funds"}'
    weight: 15
  - status_code: 500
    body: '{"error":"internal_error"}'
    weight: 5
    delay: 2s
```

**See:** `flaky-webhook.yaml`

**Selection logic:**

| Condition | Behavior |
|-----------|----------|
| Only `response` (no `responses[]`) | Always returns the singular response |
| `responses[]` with `weight` | Weighted random selection |
| `responses[]` without `weight` | Round-robin rotation (cycles in order) |

> Each response variant can have its own `delay`, `echo`, `template`, `headers`, and `body`.

---

### 8. Debug echo

Return the request body verbatim — useful for inspecting exactly what your client sends.

```yaml
# POST /api/echo → returns your payload as-is
id: debug-echo
method: POST
path: /api/echo
response:
  status_code: 200
  echo: true
```

**See:** `echo.yaml`

> `echo: true` takes precedence over `template: true` and `body`. The configured `body` is ignored when echo is active.

---

### 9. Simulated latency

Add artificial delays to test client timeout handling.

| Duration | YAML | Use Case |
|----------|------|----------|
| Instant | `delay: 0s` (or omit) | Normal operation |
| Slow | `delay: 500ms` | Moderate latency |
| Timeout | `delay: 2s` | Client timeout testing |
| Very slow | `delay: 5s` | Aggressive timeout testing |

Delays can be set on the singular `response` or per-variant inside `responses[]`.

**See:** `user-create.yaml` (200ms), `order-placed.yaml` (500ms), `flaky-webhook.yaml` (2s on error variant)

---

## Path Limitation

Webhooker uses **exact path matching**. A stub for `/api/users` will **not** match `/api/users/123` or `/api/users/123/orders`.

**Workarounds:**

| Approach | How |
|----------|-----|
| Define per-resource stubs | Create separate stubs for `/api/users/1`, `/api/users/2`, etc. |
| Wildcard catch-all | Use a single broad path with method matching (but lose path info) |
| Accept the limitation | Use Webhooker for testing fixed known paths — ideal for webhook simulations, health checks, and predefined API contracts |

> If you need path-parameter matching (e.g., `/api/users/:id`), define one stub per expected ID or use templates to differentiate responses via body/headers.

---

## Request Matching — Quick Reference

An endpoint matches a request when **all** specified conditions are met:

| Field | Behavior | Matching |
|-------|----------|----------|
| `method` | Must equal request method | Empty = matches any method |
| `path` | Exact URL path | `/api/users` does NOT match `/api/users/1` |
| `headers` | All listed headers must be present with exact values | Header name matching is case-sensitive |
| `query` | All listed query params must be present with exact values | Omitted params = no match |
| `payload` | Request body must match exactly (byte-level) | Useful for enforcing request schemas |

If no endpoint matches → **404** response.

---

## Curl Reference

```bash
# 1. Static read
curl http://localhost:8080/api/health

# 2. Create with matching
curl -X POST http://localhost:8080/api/users \
  -H "X-Event: user.created" \
  -d '{"event":"user.created"}'

# 3. Query-filtered read
curl "http://localhost:8080/api/search?type=webhook&version=1.0"

# 4. Body-constrained create
curl -X POST http://localhost:8080/api/notifications \
  -d '{"type":"webhook","version":"1.0"}'

# 5. Wildcard method
curl http://localhost:8080/api/catch-all
curl -X DELETE http://localhost:8080/api/catch-all

# 6. Dynamic template
curl -X POST http://localhost:8080/api/orders \
  -H "Content-Type: application/json" \
  -d '{"product":"widget","qty":2}'

# 7. Flaky endpoint (run multiple times to see different responses)
curl -X POST http://localhost:8080/api/payments

# 8. Debug echo
curl -X POST http://localhost:8080/api/echo \
  -H "Content-Type: application/json" \
  -d '{"hello":"world"}'

# 9. 404 — undefined path
curl http://localhost:8080/api/undefined
```
