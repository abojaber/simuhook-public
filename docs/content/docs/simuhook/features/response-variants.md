---
title: "Response Variants"
type: docs
weight: 2
---

# Response Variants

A stub can define **several** possible responses with a `responses[]` list instead of a single `response`. This lets you simulate flaky or varied third-party behavior: mostly success, sometimes an error, occasionally a timeout.

```yaml
id: flaky-webhook
method: POST
path: /webhooks/payment
responses:
  - status_code: 200
    body: '{"status":"completed","transaction_id":"txn_12345"}'
    weight: 80
  - status_code: 400
    body: '{"error":"insufficient_funds"}'
    weight: 15
  - status_code: 500
    body: '{"error":"internal_error"}'
    weight: 5
    delay: 2s
```

Send the same request several times and you'll see roughly: 80% succeed, 15% get a `400`, 5% get a slow `500`.

```bash
for i in 1 2 3 4 5 6 7 8 9 10; do
  curl -s -o /dev/null -w "%{http_code}\n" -X POST http://localhost:8080/webhooks/payment
done
# ~ 200 200 200 200 200 200 200 400 200 500
```

## Selection Logic

| Condition | Behavior |
|-----------|----------|
| Only `response` (no `responses[]`) | Always returns the singular response. |
| `responses[]` with `weight` | Weighted random selection. |
| `responses[]` without `weight` | Round-robin rotation, cycled in order. |

## Per-Variant Options

Each variant supports everything the singular `response` supports, independently:

- `status_code` — HTTP status (default 200)
- `body` — response body
- `headers` — response headers
- `delay` — simulated latency
- `echo` — echo mode
- `template` — template mode

## Use Cases

- **Retry testing** — verify your client retries on `5xx` and backs off.
- **Error handling** — confirm error paths show the right messages.
- **Timeout behavior** — the slow `500` variant above exercises both timeout and error handling at once.

## Also See

- [stubs/flaky-webhook.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/flaky-webhook.yaml) — the shipped example stub.
- [Delays]({{< relref "/docs/simuhook/features/delays" >}}) — per-variant latency.