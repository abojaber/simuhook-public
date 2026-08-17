---
title: "Templates"
type: docs
weight: 30
bookCollapseSection: true
---

# Templates

With `template: true` on a response, Simuhook renders the response `body` and header values as [Go templates](https://pkg.go.dev/text/template), injecting request-specific data into every response.

| Page | What it covers |
|------|----------------|
| [Variables]({{< relref "/docs/templates/variables" >}}) | Request data available inside a template: `.UUID`, `.Now`, `.Body`, and more. |
| [Functions]({{< relref "/docs/templates/functions" >}}) | `Header`, `Query`, `RandomInt`, `RandomHex`. |
| [Faker Generators]({{< relref "/docs/templates/faker-generators" >}}) | 27 realistic random data generators: names, addresses, payment details, and more. |
| [JSONPath]({{< relref "/docs/templates/jsonpath" >}}) | Extract fields from the JSON request body with `{{JSONPath "$.user.name"}}`. |

## Quick Example

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

Every request gets a fresh UUID, timestamp, and the request's own method and body echoed back.