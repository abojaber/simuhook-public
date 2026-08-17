---
title: "Echo Mode"
type: docs
weight: 1
---

# Echo Mode

`echo: true` makes Simuhook return the request body **verbatim** as the response.

```yaml
id: debug-echo
method: POST
path: /webhooks/echo
response:
  status_code: 200
  echo: true
```

```bash
curl -X POST http://localhost:8080/webhooks/echo \
  -H "Content-Type: application/json" \
  -d '{"hello":"world"}'
# → {"hello":"world"}
```

## Why It's Useful

Echo mode is a debugging tool — it shows you exactly what your client is sending, byte for byte. Combined with the [TUI dashboard]({{< relref "/docs/dashboard" >}}) you can see the request headers, query, body, and the echoed response all in one place.

## Behavior

| Setting | Effect |
|---------|--------|
| `echo: true` | Returns the request body as-is. |
| `echo: true` with a `body` configured | `body` is **ignored** — echo wins. |
| `echo: true` with `template: true` | `template` is ignored — echo wins. |

> `echo: true` takes precedence over `template: true` and `body`. The configured `body` is ignored when echo is active.

## Also See

- [stubs/echo.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/echo.yaml) — the shipped example stub.