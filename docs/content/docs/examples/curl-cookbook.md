---
title: "Curl Cookbook"
type: docs
weight: 1
---

# Curl Cookbook

Every example below works against the real stub files shipped in the repository ([stubs/](https://github.com/abojaber/simuhook/tree/main/stubs)) with the server running on its default port:

```bash
simuhook
```

## Static Response

```bash
curl http://localhost:8080/health
# → {"status":"ok"}
```

Source: [health.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/health.yaml) — minimal static response.

## Wildcard Method

```bash
curl http://localhost:8080/catch-all
curl -X POST http://localhost:8080/catch-all
curl -X DELETE http://localhost:8080/catch-all
# → {"message":"matched any method"}
```

Source: [method-match.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/method-match.yaml) — empty `method` matches any verb.

## Query Parameter Matching

```bash
curl "http://localhost:8080/ping?ping_id=11022022&ping_child=43443"
# → {"message":"pong"}
```

Source: [ping.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/ping.yaml) — **both** params required with exact values. Missing either → `404`.

## Exact Payload Matching

```bash
curl -X POST http://localhost:8080/webhooks/notification \
  -d '{"type":"webhook","version":"1.0"}'
# → {"status":"notification received"}

# any deviation → 404
curl -X POST http://localhost:8080/webhooks/notification \
  -d '{"type":"webhook","version":"2.0"}'
# → 404 {"error":"no matching endpoint"}
```

Source: [payload-match.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/payload-match.yaml) — exact body match.

## Header + Payload Matching with Delay

```bash
curl -X POST http://localhost:8080/webhooks/user \
  -H "X-Event: user.created" \
  -d '{"event":"user.created"}'
# → {"received":true,"event":"user.created"}  (201, 200ms delay)
```

Source: [user-create.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/user-create.yaml).

## Header-Only Matching

```bash
curl -X POST http://localhost:8080/webhooks/order \
  -H "X-Event: order.placed"
# → {"acknowledged":true,"event":"order.placed"}  (202, 500ms delay)
```

Source: [order-placed.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/order-placed.yaml).

## Template Mode — Dynamic Body and Headers

```bash
curl -X POST http://localhost:8080/webhooks/orders \
  -H "Content-Type: application/json" \
  -d '{"product":"widget","qty":2}'
# → {"id":"<uuid>","event":"order.created","timestamp":...,"method":"POST","body":"{\"product\":\"widget\",\"qty\":2}"}
```

Source: [order-created.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/order-created.yaml) — fresh UUID, timestamp, request echo on every call.

## Faker + JSONPath Showcase

```bash
curl -s -X POST http://localhost:8080/template-showcase \
  -H "Content-Type: application/json" \
  -d '{"customer":{"id":"cust-42","name":"Ada Lovelace"}}'
# → {"request_id":"<uuid>","event":"customer.created","customer":{"id":"cust-42","echo_name":"Ada Lovelace","generated":{"name":"<faker>",...},"payment":{"card":"<faker>","amount":"<faker>","currency":"USD"},"meta":{...}}}
```

Source: [template-showcase.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/template-showcase.yaml) — JSONPath request extraction + faker data generators.

## Echo Mode

```bash
curl -X POST http://localhost:8080/webhooks/echo \
  -H "Content-Type: application/json" \
  -d '{"hello":"world"}'
# → {"hello":"world"}  (verbatim)
```

Source: [echo.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/echo.yaml).

## Path Glob

```bash
curl http://localhost:8080/users/123
curl http://localhost:8080/users/123/orders
curl http://localhost:8080/users
# → {"user":"found","matched_by":"path_glob"}
```

Source: [users-by-id.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/users-by-id.yaml) — `path_glob: /users/**`.

## Path Regex + Partial Payload

```bash
curl -X POST http://localhost:8080/orders/12345 \
  -H "Content-Type: application/json" \
  -d '{"event":"order.placed","amount":42,"note":"ignored extra field"}'
# → {"filtered":true,"matched_by":"path_regex + payload_contains"}
```

Source: [orders-filter.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/orders-filter.yaml) — `^/orders/[0-9]{5}$` regex + `payload_contains`.

## Weighted Response Variants

```bash
for i in 1 2 3 4 5 6 7 8 9 10; do
  curl -s -X POST http://localhost:8080/webhooks/payment -w " %{http_code}\n"
done
# ~ 200 200 400 200 200 200 200 500 200 200
```

Source: [flaky-webhook.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/flaky-webhook.yaml) — 80% success / 15% bad request / 5% slow server error.

## 404 — No Match

```bash
curl http://localhost:8080/nonexistent
# → 404 {"error":"no matching endpoint"}
```

Unmatched requests are logged to `logs/_unmatched.log`.