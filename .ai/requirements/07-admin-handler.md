# WS-017b — Admin API extraction (`mock.NewAdminHandler`)

**Status:** `removed` | **Owner:** BA | **Priority:** `[Could]` | **Date:** 2026-08-03 | **Contract:** existing `main.go:159-224` admin routes (behavior to preserve byte-for-byte)

> **REMOVED by stakeholder decision 2026-08-05.** The admin API was fully implemented
> (`505e12c`) and QA-verified (16/16 ACs, no bugs), but the stakeholder decided to remove all
> admin endpoints from the product ("all APIs can be handled manually in the stubs folder").
> The feature moved to the backlog at low priority (consolidated as WS-014/ADM-001). This file
> is retained in full as the **reintroduction contract** and the engineering record of what
> shipped. See `04-backlog.md` for the removal decision and the reintroduction requirements
> (input validation, watcher ownership, auth posture).
>
> **Removal tracking:** code deletion is a Developer task — see ADM-002 in `04-backlog.md`.

> **Implemented `505e12c` — QA verified 2026-08-03 (16/16 ACs PASS, no bugs, wire format byte-for-byte).** Deferred from WS-017 (stakeholder approved deferral 2026-08-02) to keep WS-017 focused on the request-handling path. Now promoted to the next feature.
>
> **Design approved by stakeholder 2026-08-03** — Q1-Q4 all approved as recommended (preserve POST 201 CT quirk; no server-side created_at; `Encode(&ep)` race fix ride-along; preserve no-`Allow`-on-405).

## Design decisions (2026-08-03, architect-verified facts)

| # | Decision | Outcome |
|---|----------|---------|
| Q1 | POST 201 Content-Type quirk (sniffed `text/plain; charset=utf-8`, header never set) | **Preserve** — keep WS-017b a pure refactor; fix via separate story if wanted |
| Q2 | `created_at` server-side stamping | **No** — preserve client-supplied/zero behavior; follow-up story (needs clock seam) |
| Q3 | `Encode(ep)` → `Encode(&ep)` race fix | **Approved** — byte-identical output (architect F9/F10), removes a `-race`-provable race |
| Q4 | no `Allow` header on 405 | **Preserve** — backlog if wanted |

### Architect-verified wire facts (correct the contract below)
- F1: `POST` 201 success sets **no** Content-Type (wire = sniffed `text/plain; charset=utf-8`).
- F2/F6: every error path & 405 go through `http.Error` → `text/plain; charset=utf-8` + `X-Content-Type-Options: nosniff` + `msg\n`.
- F3: `json.Encoder.Encode` appends `"\n"` on all success paths.
- F4: empty `List()` encodes `[]\n` (never `null` — List returns a non-nil slice).
- F5: `httptest.ResponseRecorder` reports `""` for POST 201 / DELETE 204 Content-Type (no sniffing) — tests must assert `""`.
- F7: decode errors: empty→`EOF`, `{`→`unexpected EOF`, `{"id":1}`→`json: cannot unmarshal …`.
- F8: `Payload`/`Response.Body` (`[]byte`) are **base64** in JSON; `Response.Delay` = integer ns.
- F10: latent data race (value-encode reads `rrIndex`) — fixed via `Encode(&ep)`.
- F11: `/_admin/endpoints//x` → ServeMux 307 (handler never runs); `/_admin/endpointsX` → mock surface; nested ids allowed (`a/b`).
- F12: `encoding/json` used only in admin block (remove from main.go); keep `strings` (used in banner).
- Order-of-checks invariants (observable, do not tidy): empty-id 400 precedes method switch (`PATCH /` → 400 not 405); POST/PUT decode precedes Add/Update (invalid JSON → 400 even for dup/unknown id).
- `registry.Update` sets `ep.ID = id` → PUT response reports path id, no second key.

## Story

> As a developer, I want the admin REST API handler extracted into a testable `mock.NewAdminHandler(registry) http.Handler` so that the control-plane routes are covered by unit tests instead of only manual curl smoke tests.

## Context

- Admin routes currently live inline in `main.go:159-224` (`/_admin/endpoints` + `/_admin/endpoints/`), with **zero unit test coverage** — the same gap WS-017 closed for the request-handling path.
- `mock/registry.go` already exposes all needed operations (`List`, `Get`, `Add`, `Update`, `Remove`).
- Mirrors the WS-017 pattern: extract into `mock`, keep `main.go` as wiring, no behavior change.
- Retires most of TGR-04's admin gap (currently `[Won't]`).

## Scope

- New file `src/mock/admin.go` exposing `func NewAdminHandler(registry *Registry) http.Handler` (nil registry → panic, same convention as WS-017's `HandlerConfig`).
- Mounted on BOTH `/_admin/endpoints` and `/_admin/endpoints/` in `main.go` (mux.Handle + mux.Handle for the trailing-slash path).
- `main.go` admin route closures (lines 159-224) deleted, replaced with the two mounts. Banner/logging unchanged. Admin requests stay unlogged (current behavior).
- Tests in `src/mock/admin_test.go` via `httptest`.

## Contract (preserve byte-for-byte from current `main.go`)

| Route | Method | Success | Errors |
|-------|--------|---------|--------|
| `/_admin/endpoints` | GET | 200, `application/json`, `registry.List()` encoded | — |
| `/_admin/endpoints` | POST | 201, JSON of created endpoint | invalid JSON → 400; duplicate ID → 409 |
| `/_admin/endpoints` | other | — | 405 |
| `/_admin/endpoints/{id}` | GET | 200, JSON of endpoint | missing id → 400 `"missing endpoint id"`; unknown id → 404 |
| `/_admin/endpoints/{id}` | PUT | 200, JSON of updated endpoint | missing id → 400; invalid JSON → 400; unknown id → 404 |
| `/_admin/endpoints/{id}` | DELETE | 204, no body | missing id → 400; unknown id → 404 |
| `/_admin/endpoints/{id}` | other | — | 405 |

## ACs

```
Given a registry with endpoints registered
When  GET /_admin/endpoints
Then  200 with application/json listing all endpoints
Given a valid endpoint JSON body
When  POST /_admin/endpoints
Then  201 with the created endpoint JSON; the registry contains it
Given an invalid JSON body (or empty body)
When  POST /_admin/endpoints
Then  400 with the decode error message
Given an endpoint that already exists
When  POST /_admin/endpoints
Then  409 with the "already exists" error message
Given an existing endpoint id
When  GET /_admin/endpoints/{id}
Then  200 with the endpoint JSON
Given an unknown endpoint id
When  GET /_admin/endpoints/{id}
Then  404
Given a PUT with a valid body for an existing id
When  PUT /_admin/endpoints/{id}
Then  200 with the updated endpoint JSON; the registry entry is replaced and keeps the path id
Given a PUT with invalid JSON
When  PUT /_admin/endpoints/{id}
Then  400
Given an unknown id
When  PUT /_admin/endpoints/{id}
Then  404
Given an existing id
When  DELETE /_admin/endpoints/{id}
Then  204 with no body; the registry no longer contains it
Given an unknown id
When  DELETE /_admin/endpoints/{id}
Then  404
Given a path with no id (/_admin/endpoints/)
When  GET /_admin/endpoints/
Then  400 "missing endpoint id"
Given a wrong method (e.g. PATCH)
When  sending it to /_admin/endpoints
Then  405
And   admin requests are NOT written to the request log
```

## Non-goals

- No change to the admin request/response wire format (byte-for-byte preservation).
- No dashboard/CLI feature to drive endpoints at runtime.
- `logger.Log` dropped-error reporting and observer panic policy remain out of scope (from WS-017).

## Test plan (~13 tests, `httptest` + `t.Helper` request helper)

GET list (empty + populated), POST create (201 + registry contains), POST duplicate → 409, POST invalid JSON → 400, GET one → 200, GET unknown → 404, PUT update → 200 (and id forced to path id), PUT invalid JSON → 400, PUT unknown → 404, DELETE → 204 (registry empty after), DELETE unknown → 404, missing id → 400, wrong method → 405. Plus a nil-registry panic test.

## Verification gates

- `go test -race -count=1 ./...` green (240 existing + ~17 new — the "137 existing" figure in the story text is stale).
- `go vet ./...` clean.
- Manual curl smoke: start `--tui off`, GET/POST/GET-one/PUT/DELETE against `/_admin/endpoints`, confirm identical behavior to before (use base64 for body fields).
- `rg -n 'HandleFunc\("/_admin' src/` ⇒ no hits after extraction.

## Files touched (expected)

| File | Change |
|------|--------|
| `src/mock/admin.go` | NEW — `NewAdminHandler` |
| `src/mock/admin_test.go` | NEW — tests |
| `src/main.go` | replace inline closures with two mounts |
| `.ai/architecture.md` | WS-017b ADR; remove from Open Items |
| `.ai/requirements/04-backlog.md` | WS-017b status flip + this file |
