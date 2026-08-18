---
title: "Payload Matching"
type: docs
weight: 4
---

# Payload Matching

Simuhook can constrain a stub to specific request bodies — two ways, depending on whether you need an exact byte match or a partial JSON match.

| Field | Meaning | Use when |
|-------|---------|----------|
| `payload` | The request body must match **exactly** (byte-level). | You want to enforce that your client sends precisely the body you expect. |
| `payload_contains` | The request body must be valid JSON that *contains* the given spec. | You want to match on a few fields and ignore everything else. |

A stub may set at most **one** of the two — setting both is a load-time parse error. Specs must be quoted in YAML; invalid spec JSON is a load-time parse error.

## Exact Match — `payload`

```yaml
id: notification-create
method: POST
path: /webhooks/notification
payload: '{"type":"webhook","version":"1.0"}'
response:
  status_code: 200
  body: '{"status":"notification received"}'
```

| Request body | Matches |
|--------------|---------|
| `{"type":"webhook","version":"1.0"}` | ✅ |
| `{"type":"webhook","version":"1.0","extra":true}` | ❌ (not byte-identical) |
| `{"version":"1.0","type":"webhook"}` | ❌ (key order differs) |
| `{"type":"webhook"}` | ❌ |

> Exact matching compares the raw body string. Key order matters — this intentionally mirrors what a strict server would reject.

## Partial JSON Match — `payload_contains`

```yaml
id: orders-filter
method: POST
path_regex: ^/orders/[0-9]{5}$
payload_contains: '{"event":"order.placed","amount":42}'
response:
  status_code: 200
```

| Request body | Matches |
|--------------|---------|
| `{"event":"order.placed","amount":42}` | ✅ |
| `{"event":"order.placed","amount":42,"note":"ignored"}` | ✅ (extra fields ignored) |
| `{"event":"order.placed"}` | ❌ (missing `amount`) |
| `this is not json` | ❌ (non-JSON bodies never match) |

### Subset Semantics

`payload_contains` matches when the request body is valid JSON and *contains* the spec:

- **Objects** — every spec key must be present and subset-match recursively; extra request keys are ignored.
- **Arrays** — each spec element must match a **distinct** request element, order-independent.
- **Scalars** — deep equality with cross-type numeric equality (`42` matches `42.0`).
- **`null`** — matches only a literal `null` value.
- A non-JSON request body never matches.

### Mixed Example

Spec:

```json
{"event":"order.placed","items":[{"sku":"A-1","qty":2},{"sku":"B-2","qty":1}]}
```

Matches:

```json
{"event":"order.placed","items":[{"sku":"B-2","qty":1},{"sku":"A-1","qty":2,"color":"red"}],"extra":"ignored"}
```

The array elements are matched order-independently against distinct request elements, and the extra `color` and top-level `extra` fields are ignored.

## Choosing Between Them

- Use `payload` when you need to verify your client sends an exact contract.
- Use `payload_contains` when you want one stub to serve any request that "looks like" an event — e.g. any body carrying `"event":"order.placed"` regardless of the other fields the real API appends.