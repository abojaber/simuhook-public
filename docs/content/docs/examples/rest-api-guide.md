---
title: "REST API Guide"
type: docs
weight: 2
---

# RESTful API Simulation — Patterns

How to simulate RESTful API behavior with Simuhook stub files. Nine patterns, each with a YAML stub, the curl command, and a link to the shipped example.

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

## 1. Static Read Endpoint

Return a fixed JSON response for a GET request.

```yaml
id: health
method: GET
path: /api/health
response:
  status_code: 200
  body: '{"status":"ok"}'
```

```bash
curl http://localhost:8080/api/health
# → {"status":"ok"}
```

**See:** [health.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/health.yaml)

---

## 2. Create with Request Matching

Match on specific headers and/or payload to simulate different create behaviors.

```yaml
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

```bash
curl -X POST http://localhost:8080/api/users \
  -H "X-Event: user.created" \
  -d '{"event":"user.created"}'
# → {"received":true,"event":"user.created"}  (201)
```

**See:** [user-create.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/user-create.yaml), [order-placed.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/order-placed.yaml)

**Variation — header-only match:**

```yaml
headers:
  X-Event: order.placed
response:
  status_code: 202
  delay: 500ms
```

---

## 3. Query-Filtered Read

Require exact query parameters to match — simulates filtered list endpoints.

```yaml
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

```bash
curl "http://localhost:8080/api/search?type=webhook&version=1.0"
# → {"results":[]}
```

**See:** [ping.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/ping.yaml)

> **Note:** Query matching is all-or-nothing — every listed param must be present with the exact value. Missing or extra params cause a `404`.

---

## 4. Body-Constrained Create

Match only when the request body matches an exact payload.

```yaml
id: notification-create
method: POST
path: /api/notifications
payload: '{"type":"webhook","version":"1.0"}'
response:
  status_code: 200
  body: '{"status":"notification received"}'
```

```bash
curl -X POST http://localhost:8080/api/notifications \
  -d '{"type":"webhook","version":"1.0"}'
# → {"status":"notification received"}
```

**See:** [payload-match.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/payload-match.yaml)

> Useful for testing that your client sends the exact body structure you expect. Any deviation — extra fields, reordered keys — is a `404`.

---

## 5. Wildcard Method Handler

Leave `method` empty to match any HTTP verb on the same path.

```yaml
id: catch-all
path: /api/catch-all
response:
  status_code: 200
  body: '{"message":"matched any method"}'
```

```bash
curl http://localhost:8080/api/catch-all
curl -X DELETE http://localhost:8080/api/catch-all
# → {"message":"matched any method"}
```

**See:** [method-match.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/method-match.yaml)

> Ideal for health checks, pre-flight handlers, or debug endpoints where you don't care about the method.

---

## 6. Dynamic Payloads (Templates)

Use [Go templates]({{< relref "/docs/templates" >}}) to inject request-specific data into the response.

```yaml
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

```bash
curl -X POST http://localhost:8080/api/orders \
  -H "Content-Type: application/json" \
  -d '{"product":"widget","qty":2}'
# → {"id":"<uuid>","event":"order.created","timestamp":...,"method":"POST"}
```

**See:** [order-created.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/order-created.yaml)

Go further with [faker generators]({{< relref "/docs/templates/faker-generators" >}}) (`{{Name}}`, `{{Email}}`, `{{Price}}`…) and [JSONPath]({{< relref "/docs/templates/jsonpath" >}}) extraction in [template-showcase.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/template-showcase.yaml).

---

## 7. Error / Status Simulation

Return different responses based on weighted probability.

```yaml
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

```bash
curl -X POST http://localhost:8080/api/payments   # run several times
# → different status codes roughly 80/15/5
```

**See:** [flaky-webhook.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/flaky-webhook.yaml)

**Selection logic:**

| Condition | Behavior |
|-----------|----------|
| Only `response` (no `responses[]`) | Always returns the singular response |
| `responses[]` with `weight` | Weighted random selection |
| `responses[]` without `weight` | Round-robin rotation (cycles in order) |

> Each response variant can have its own `delay`, `echo`, `template`, `headers`, and `body`.

---

## 8. Debug Echo

Return the request body verbatim — inspect exactly what your client sends.

```yaml
id: debug-echo
method: POST
path: /api/echo
response:
  status_code: 200
  echo: true
```

```bash
curl -X POST http://localhost:8080/api/echo \
  -H "Content-Type: application/json" \
  -d '{"hello":"world"}'
# → {"hello":"world"}
```

**See:** [echo.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/echo.yaml)

> `echo: true` takes precedence over `template: true` and `body`. The configured `body` is ignored when echo is active.

---

## 9. Simulated Latency

Add artificial delays to test client timeout handling.

| Duration | YAML | Use Case |
|----------|------|----------|
| Instant | `delay: 0s` (or omit) | Normal operation |
| Slow | `delay: 500ms` | Moderate latency |
| Timeout | `delay: 2s` | Client timeout testing |
| Very slow | `delay: 5s` | Aggressive timeout testing |

```yaml
id: slow-endpoint
path: /api/slow
response:
  status_code: 200
  body: '{"ok":true}'
  delay: 2s
```

Delays can be set on the singular `response` or per-variant inside `responses[]`.

**See:** [user-create.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/user-create.yaml) (200ms), [order-placed.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/order-placed.yaml) (500ms), [flaky-webhook.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/flaky-webhook.yaml) (2s on the error variant)

---

## Path Matching Notes

Simuhook matches paths **exactly** (or via glob/regex — see [Path Patterns]({{< relref "/docs/stubs/path-patterns" >}})). A stub for `/api/users` will **not** match `/api/users/123` unless you use a wildcard:

| Approach | How |
|----------|-----|
| Per-resource stubs | Define separate stubs for `/api/users/1`, `/api/users/2`, etc. |
| Path glob | `path_glob: /api/users/**` matches any nesting depth under `/api/users`. |
| Path regex | `path_regex: ^/api/users/[0-9]+$` matches numeric IDs only. |

If no endpoint matches → **404** `{"error":"no matching endpoint"}`.