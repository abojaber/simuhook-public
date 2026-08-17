---
title: "Matching"
type: docs
weight: 2
---

# Request Matching

An endpoint matches a request when **all** specified conditions are met:

| Field | Behavior |
|-------|----------|
| `method` | Must equal the request method. Empty matches any method. |
| `path` / `path_glob` / `path_regex` | Exactly one path matcher — exact, glob, or regex. |
| `headers` | All listed headers must be present with the exact values. |
| `query` | All listed query params must be present with the exact values. |
| `payload` / `payload_contains` | The request body must match exactly, or contain the JSON subset. |

Request matching is **AND logic**: every condition you specify must hold. Conditions you don't specify are ignored.

## Defaults and Special Values

| Value | Meaning |
|-------|---------|
| `method` omitted | Matches **any** HTTP method. |
| `headers` omitted | No header constraints — any headers match. |
| `query` omitted | No query constraints — any query matches. |
| `payload` omitted | No body constraint — any body (including empty) matches. |

## When Several Stubs Match

A request can match more than one stub. Simuhook picks the **most specific** match, deterministically:

1. **Path class** — exact `path` beats `path_glob`, which beats `path_regex`.
2. **Glob weight** — among globs, fewer wildcards win: `/users/*/orders` beats `/users/**`.
3. **Load order** — identical specificity falls back to the order stubs were loaded (stable across reloads).

A stub that fails any condition is never a candidate — constraints are evaluated **before** specificity comparison.

## Worked Example

Consider these two stubs:

```yaml
# stub A — matches any GET ending in /users (exact)
id: users-exact
method: GET
path: /users
response:
  status_code: 200
  body: '{"matched":"exact"}'
```

```yaml
# stub B — matches any GET under /users (glob)
id: users-glob
method: GET
path_glob: /users/**
response:
  status_code: 200
  body: '{"matched":"glob"}'
```

| Request | Result | Why |
|---------|--------|-----|
| `GET /users` | **A** (`{"matched":"exact"}`) | Both match on path, but exact `path` outranks `path_glob`. |
| `GET /users/123` | **B** (`{"matched":"glob"}`) | Stub A's exact path doesn't match; only the glob does. |
| `POST /users/123` | **404** | Stub B requires `method: GET`, so it's not a candidate. |

## No Match

If no endpoint matches, the server responds:

```json
{
  "error": "no matching endpoint"
}
```

with status `404` and `Content-Type: application/json`. Unmatched requests are still logged — to `logs/_unmatched.log` — with `matched: false`.