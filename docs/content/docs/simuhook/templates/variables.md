---
title: "Variables"
type: docs
weight: 1
---

# Template Variables

When `template: true` is set on a response, the response `body` and header values are rendered as a Go template. These variables are available:

| Variable | Description |
|----------|-------------|
| `{{.UUID}}` | UUID v4 — unique per request |
| `{{.Now}}` | Current time in RFC3339 format |
| `{{.Timestamp}}` | Current Unix timestamp (seconds) |
| `{{.RequestID}}` | Unique request ID (also logged) |
| `{{.Method}}` | Incoming HTTP method |
| `{{.Path}}` | Incoming URL path |
| `{{.Body}}` | Raw request body |

## Example

```yaml
id: order-created
method: POST
path: /webhooks/orders
response:
  status_code: 200
  template: true
  headers:
    X-Request-ID: "{{.RequestID}}"
    X-Timestamp: "{{.Now}}"
  body: '{"id":"{{.UUID}}","event":"order.created","timestamp":{{.Timestamp}},"method":"{{.Method}}","body":"{{.Body}}"}'
```

Send a request:

```bash
curl -X POST http://localhost:8080/webhooks/orders \
  -H "Content-Type: application/json" \
  -d '{"product":"widget","qty":2}'
```

Response:

```json
{
  "id": "0d1e6f2a-9c4b-4f3a-8d9e-1a2b3c4d5e6f",
  "event": "order.created",
  "timestamp": 1765068000,
  "method": "POST",
  "body": "{\"product\":\"widget\",\"qty\":2}"
}
```

With the response headers:

```
X-Request-ID: 0d1e6f2a-9c4b-4f3a-8d9e-1a2b3c4d5e6f
X-Timestamp: 2026-08-18T10:30:00Z
```

## Notes

- `{{.Now}}` is captured when the request is received — all uses of it in one response render the same value.
- `{{.Body}}` renders the raw request body string. To pull out individual fields, see [JSONPath]({{< relref "/docs/simuhook/templates/jsonpath" >}}).
- `{{.UUID}}` (a field) is independent from the `{{UUID}}` [function]({{< relref "/docs/simuhook/templates/faker-generators" >}}) — both return fresh values per render, and either can be used anywhere in the template.