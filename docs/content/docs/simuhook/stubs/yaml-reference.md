---
title: "YAML Reference"
type: docs
weight: 1
---

# YAML Reference

Each `.yaml` / `.yml` file in the stubs directory defines one endpoint. Every field is optional unless marked **required**.

## Minimal Stub

```yaml
id: my-endpoint
path: /some/path
response:
  status_code: 200
```

## Full Stub — Every Option

```yaml
id: my-endpoint
method: POST                              # HTTP method (empty = match any)
path: /webhooks/event                     # URL path (exact match)
path_glob: /users/**                       # URL path glob (see Path Patterns)
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

## Field Reference

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `id` | **yes** | string | Unique endpoint identifier. Used as the stub name in logs and the dashboard. |
| `method` | no | string | HTTP method the request must use (`GET`, `POST`, …). Empty matches **any** method. |
| `path` | no* | string | Exact request path to match. |
| `path_glob` | no* | string | Segment-wise glob path match (`**`, `*`, `?`). |
| `path_regex` | no* | string | Go RE2 regex path match (not implicitly anchored). |
| `headers` | no | map | Request headers that must be present with exact values. |
| `query` | no | map | Query params that must be present with exact values. |
| `payload` | no | string | Exact request body the request must match (byte-level). |
| `payload_contains` | no | string | Partial JSON spec the request body must *contain*. |
| `response.status_code` | no | int | HTTP status code to return (default: `200`). |
| `response.body` | no | string | Response body. Ignored when `echo: true`. |
| `response.headers` | no | map | Response headers to send. |
| `response.delay` | no | duration | Simulated latency before responding (e.g. `200ms`, `1s`). |
| `response.echo` | no | bool | Return the request body verbatim. |
| `response.template` | no | bool | Process `body`/headers as a Go template. |
| `responses[]` | no | list | Multiple response variants with weights (see below). |

\* Exactly **one** of `path` / `path_glob` / `path_regex` per stub. Setting more than one fails to load with a parse error naming the stub ID and the conflicting fields. A stub with none of them is also invalid — you must match on something.

## Response Variants

A stub can define several possible responses instead of a single `response`:

```yaml
id: flaky-webhook
method: POST
path: /webhooks/payment
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

| Condition | Behavior |
|-----------|----------|
| Only `response` (no `responses[]`) | Always returns the singular response. |
| `responses[]` with `weight` | Weighted random selection. |
| `responses[]` without `weight` | Round-robin rotation, cycled in order. |

Each variant supports the same options as `response` — `status_code`, `body`, `headers`, `delay`, `echo`, `template` — independently.

## YAML Notes

- Strings that look like numbers or booleans (`"1.0"`, `"true"`) must be **quoted** in YAML so they stay strings — important for `query` values and `payload`.
- The `payload` / `payload_contains` spec must be quoted if it could otherwise be parsed as a YAML value:

```yaml
payload: '{"type":"webhook","version":"1.0"}'
```

- `delay` accepts Go duration syntax: `200ms`, `1s`, `2m30s`, `0s`.

## See Also

- [Matching]({{< relref "/docs/simuhook/stubs/matching" >}}) — how requests are routed to stubs.
- [Path Patterns]({{< relref "/docs/simuhook/stubs/path-patterns" >}}) — `path_glob` and `path_regex` details.
- [Payload Matching]({{< relref "/docs/simuhook/stubs/payload-matching" >}}) — `payload_contains` semantics.
- [Templates]({{< relref "/docs/simuhook/templates" >}}) — dynamic bodies with `template: true`.