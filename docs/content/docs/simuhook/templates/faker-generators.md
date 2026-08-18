---
title: "Faker Generators"
type: docs
weight: 3
---

# Faker Data Generators

Simuhook ships a curated set of **27 realistic random data generators**, powered by [gofakeit](https://github.com/brianvoe/gofakeit). Each call renders a realistic random value — a name, an email, a price — so your mock responses look like data from a real third-party service. All generators are thread-safe and return a fresh value per render.

## Person

| Function | Example output |
|----------|---------------|
| `{{Name}}` | `Markus Moen` |
| `{{FirstName}}` | `Fred` |
| `{{LastName}}` | `Moen` |
| `{{Email}}` | `alaynawuckert@kozey.biz` |
| `{{Phone}}` | `(570)245-7485` |

## Address

| Function | Example output |
|----------|---------------|
| `{{City}}` | `Lake Maxine` |
| `{{Country}}` | `Canada` |
| `{{State}}` | `Nebraska` |
| `{{Zip}}` | `38762` |
| `{{Street}}` | `98789 Hane Locks` |

## Company

| Function | Example output |
|----------|---------------|
| `{{Company}}` | `Moen, Pagac and Wuckert` |
| `{{JobTitle}}` | `Director` |

## Internet

| Function | Example output |
|----------|---------------|
| `{{IPv4Address}}` | `192.168.1.42` |
| `{{IPv6Address}}` | `2001:db8::1` |
| `{{DomainName}}` | `kozey.biz` |
| `{{MACAddress}}` | `ab:cd:ef:01:23:45` |
| `{{UserAgent}}` | `Mozilla/5.0 …` |

## Payment

| Function | Example output |
|----------|---------------|
| `{{CreditCardNumber}}` | `4287271570245748` |
| `{{Price}}` | `42.99` |

## Product & ID

| Function | Example output |
|----------|---------------|
| `{{ProductName}}` | `Incredible Metal Chair` |
| `{{ULID}}` | `pfsfktb87rcmj6bqha2fz9` |
| `{{UUID}}` | `0d1e6f2a-…` |

## Text & Misc

| Function | Example output |
|----------|---------------|
| `{{Word}}` | `river` |
| `{{Sentence}}` | `Record river mind.` |
| `{{Paragraph}}` | `Quia voluptatem…` |
| `{{HexColor}}` | `#a3e4f2` |
| `{{Bool}}` | `true` |

## Example — Webhook-Shaped Response

```yaml
id: customer-created
method: POST
path: /webhooks/customer
response:
  status_code: 200
  template: true
  body: '{"event":"customer.created","customer":{"id":"{{UUID}}","name":"{{Name}}","email":"{{Email}}","address":{"city":"{{City}}","country":"{{Country}}"}},"payment":{"card":"{{CreditCardNumber}}","amount":"{{Price}}","currency":"USD"}}'
```

Each request returns a full, realistic customer profile:

```json
{
  "event": "customer.created",
  "customer": {
    "id": "0d1e6f2a-9c4b-4f3a-8d9e-1a2b3c4d5e6f",
    "name": "Markus Moen",
    "email": "alaynawuckert@kozey.biz",
    "address": {"city": "Lake Maxine", "country": "Canada"}
  },
  "payment": {"card": "4287271570245748", "amount": "42.99", "currency": "USD"}
}
```

## Notes on Specific Generators

| Generator | Note |
|-----------|------|
| `{{ULID}}` | A **ULID-style identifier** — it delegates to gofakeit's `ID()` (20-char URL-safe lowercase). Not a sortable 26-char ULID (gofakeit has no ULID function). |
| `{{Sentence}}` / `{{Paragraph}}` | **Accept but ignore** word/paragraph-count args — deprecated upstream, args have no effect. |
| `{{Price}}` | Fixed range `(0.01, 999.99)`, rendered with two decimals. For other ranges use `{{RandomInt}}` + formatting. |
| `{{UUID}}` (function) | Random per render — **independent** from `{{.UUID}}` (field), so you can use both in one template. |

## Combining with JSONPath

Generators compose with [JSONPath]({{< relref "/docs/simuhook/templates/jsonpath" >}}) — echo request fields back surrounded by generated data. See [stubs/template-showcase.yaml](https://github.com/abojaber/simuhook/tree/main/stubs/template-showcase.yaml) for a worked example.