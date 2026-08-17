---
title: "Functions"
type: docs
weight: 2
---

# Template Functions

Beyond the request [variables]({{< relref "/docs/templates/variables" >}}), templates can call functions — either built-in request helpers or random generators.

## Request Helpers

| Function | Description |
|----------|-------------|
| `{{Header "X-Key"}}` | Value of a request header (empty string if absent). |
| `{{Query "param"}}` | Value of a query parameter (empty string if absent). |

## Random Helpers

| Function | Description |
|----------|-------------|
| `{{RandomInt 1 100}}` | Random integer in the inclusive range. |
| `{{RandomHex 8}}` | Random hex string of the given length. |

The full set of realistic random data generators — `{{Name}}`, `{{Email}}`, `{{Company}}`, etc. — is on the [Faker Generators]({{< relref "/docs/templates/faker-generators" >}}) page.

## Example

```yaml
id: greeting
method: POST
path: /webhooks/greeting
response:
  status_code: 200
  template: true
  body: '{"greeting":"Hello {{Header \"X-Caller\"}}","source":"{{Query \"channel\"}}","luck":"{{RandomInt 1 100}}","token":"{{RandomHex 8}}"}'
```

Send a request:

```bash
curl -X POST "http://localhost:8080/webhooks/greeting?channel=web" \
  -H "X-Caller: Ada" \
  -d '{}'
```

Response:

```json
{
  "greeting": "Hello Ada",
  "source": "web",
  "luck": "42",
  "token": "3f9a1c0b"
}
```

## Notes

- `{{Header}}` / `{{Query}}` return the **first** value of the header / query parameter.
- Header names are case-insensitive (HTTP semantics); query parameter names are exact.
- Functions compose with variables: `{{Header "X-User"}}` and `{{.Body}}` can appear in the same template.