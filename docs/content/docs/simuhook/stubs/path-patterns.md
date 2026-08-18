---
title: "Path Patterns"
type: docs
weight: 3
---

# Path Patterns

Simuhook supports three ways to match the request path. A stub may set exactly **one** of them.

| Field | Semantics |
|-------|-----------|
| `path` | Exact match on the raw URL path: `/api/users` matches `/api/users` **only** — never `/api/users/1` or `/api/users/1/orders`. |
| `path_glob` | Segment-wise glob with `**`, `*`, and `?` wildcards. |
| `path_regex` | Go stdlib `regexp` (RE2). **Not** implicitly anchored — write `^…$` yourself for a full-path match. |

Setting more than one is a load-time parse error naming the stub ID and the conflicting fields.

## Exact Path — `path`

```yaml
id: health
path: /health
response:
  status_code: 200
```

| Request | Matches |
|---------|---------|
| `GET /health` | ✅ |
| `GET /health/extra` | ❌ |
| `GET /other` | ❌ |

## Glob — `path_glob`

Globs are matched **segment-wise**:

| Wildcard | Matches |
|----------|---------|
| `**` | Zero or more path segments |
| `*` | Exactly one path segment |
| `?` | Exactly one character within a segment |

```yaml
id: users-by-id
method: GET
path_glob: /users/**
response:
  status_code: 200
```

| Request | Matches |
|---------|---------|
| `GET /users/123` | ✅ (`**` spans one segment) |
| `GET /users/ada/orders` | ✅ (`**` spans two segments) |
| `GET /users` | ✅ (`**` may match zero segments) |

More examples:

```yaml
# Exactly one segment after /users
path_glob: /users/*

# Exactly four digits: /orders/1234
path_glob: /orders/????
```

**Glob rules:**

- Trailing/leading slashes are normalized (`/users/**/` ≡ `/users/**`).
- An in-segment `*` (e.g. `/files/*.pdf`) is a **load-time parse error** — only whole-segment `*` and `**`, and in-segment `?`, are supported.
- A glob with no wildcards behaves exactly like `path`.

## Regex — `path_regex`

The pattern is Go RE2 and must be **anchored by you** — `^…$` for a full-path match:

```yaml
id: orders-filter
method: POST
path_regex: ^/orders/[0-9]{5}$
response:
  status_code: 200
```

| Request | Matches |
|---------|---------|
| `POST /orders/12345` | ✅ |
| `POST /orders/1234` | ❌ (four digits — regex requires five) |
| `POST /orders/123456` | ❌ |
| `GET /orders/12345` | ❌ (method constraint) |

An invalid regex is a load-time parse error naming the stub ID and the pattern.

## Which One Wins?

When several stubs could match the same request, specificity decides (see [Matching]({{< relref "/docs/simuhook/stubs/matching" >}})):

```
exact path  >  glob  >  regex
```

Among globs, fewer wildcards win (`/users/*/orders` beats `/users/**`). Identical specificity falls back to load order.