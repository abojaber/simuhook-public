---
title: "Delays"
type: docs
weight: 3
---

# Delays

`delay` adds artificial latency before the response is sent — useful for testing client timeouts, retries, and back-off behavior.

```yaml
id: slow-endpoint
path: /slow
response:
  status_code: 200
  body: '{"ok":true}'
  delay: 2s
```

```bash
time curl -s http://localhost:8080/slow
# → {"ok":true}
# real  0m2.003s
```

## Duration Syntax

`delay` accepts Go duration syntax:

| YAML | Duration |
|------|----------|
| `delay: 0s` (or omit) | Instant — normal operation |
| `delay: 200ms` | 200 milliseconds |
| `delay: 500ms` | Moderate latency |
| `delay: 2s` | Client timeout testing |
| `delay: 5s` | Aggressive timeout testing |
| `delay: 2m30s` | Two and a half minutes |

## Where Delays Apply

- The singular `response` — `response.delay`.
- Any variant inside `responses[]` — per-variant latency (each variant can have a different delay).

```yaml
id: flaky-webhook
method: POST
path: /webhooks/payment
responses:
  - status_code: 200
    body: '{"status":"completed"}'
    weight: 80
  - status_code: 500
    body: '{"error":"internal_error"}'
    weight: 5
    delay: 2s      # only this variant is slow
```

## Notes

- Delays are applied **after** the request is matched and template data is built, and the measured latency in the [logs]({{< relref "/docs/simuhook/features/logging" >}}) and dashboard **includes** the delay.
- The dashboard's latency stats (avg/min/max/p95) count the artificial delay, so a `delay: 2s` stub will show up in p95 — expected, since that's the latency your client experiences too.

## Also See

- [stubs/user-create.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/user-create.yaml) — 200ms delay.
- [stubs/order-placed.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/order-placed.yaml) — 500ms delay.
- [stubs/flaky-webhook.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/flaky-webhook.yaml) — 2s delay on one variant.