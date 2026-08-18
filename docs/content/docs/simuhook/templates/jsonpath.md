---
title: "JSONPath"
type: docs
weight: 4
---

# JSONPath

`{{JSONPath "$…"}}` extracts a field from the JSON request body. The body is parsed **once per request**; `{{.Body}}` still renders the full raw body string.

## Syntax

| Path | Meaning |
|------|---------|
| `$` | The whole request body (as JSON string) |
| `$.user` | Object key `user` |
| `$.user.name` | Nested object key |
| `$.items[0]` | Array index 0 |
| `$.items[0].name` | Nested into an array element |
| `$.items[1].tags[0]` | Deeper nesting |

## Return Semantics

| Value | Renders as |
|-------|-----------|
| String | Unquoted (`Ada`) |
| Number | Unquoted (`42`, `42.5`) |
| Boolean | `true` / `false` |
| `null` | `null` |
| Object / array | Compact JSON (`{"a":1}`) |
| Missing path, out-of-bounds index, **non-JSON body** | Empty string (no panic, no fallback) |

## Example

```yaml
id: customer-echo
method: POST
path: /webhooks/customer-echo
response:
  status_code: 200
  template: true
  body: '{"received":{"id":"{{JSONPath "$.customer.id"}}","name":"{{JSONPath "$.customer.name"}}"},"full_payload":{{JSONPath "$"}}}'
```

Send a request:

```bash
curl -X POST http://localhost:8080/webhooks/customer-echo \
  -H "Content-Type: application/json" \
  -d '{"customer":{"id":"cust-42","name":"Ada Lovelace"}}'
```

Response:

```json
{
  "received": {"id": "cust-42", "name": "Ada Lovelace"},
  "full_payload": {"customer":{"id":"cust-42","name":"Ada Lovelace"}}
}
```

> Note the unquoted `{{JSONPath "$"}}` at the end — it renders the compact JSON object directly (numbers and booleans render unquoted, so this is safe for valid JSON request bodies).

## Restrictions

JSONPath keys are **`[A-Za-z0-9_-]`** only — no spaces, dots, or unicode in keys.

Not supported:

- Filters: `$.x[?(@.a)]`
- Wildcards: `$.x[*]`
- Recursive descent: `$..x`
- Negative indices: `$.items[-1]`

## Worked Example

[stubs/template-showcase.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/template-showcase.yaml) is a complete demo combining JSONPath extraction with [faker generators]({{< relref "/docs/simuhook/templates/faker-generators" >}}):

```bash
curl -s -X POST http://localhost:8080/template-showcase \
  -H "Content-Type: application/json" \
  -d '{"customer":{"id":"cust-42","name":"Ada Lovelace"}}'
```

It echoes your `customer.id` / `customer.name` back while generating a realistic event body around them.