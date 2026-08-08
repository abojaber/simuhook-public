# Phase 18 — Smarter Matching (MAT)

> Match stubs the way real HTTP integrations actually call services: path globs,
> path regex, and partial-JSON request-body matching — while keeping today's exact
> `path`/`payload` behavior 100% backward-compatible. Fix the nondeterministic
> map-order winner when stubs overlap.

**Status:** `ready` | **Owner:** BA | **Date:** 2026-08-08 | **Goal:** match real services more closely

**Stakeholder scope sign-off (2026-08-08):**
1. Path glob with `**` multi-segment + `*` single-segment + `?` (hand-rolled matcher — stdlib has no `**`)
2. Path regex (`RE2`, stdlib `regexp`)
3. Partial JSON request body via a JSON **subset** spec (`payload_contains:`)
4. **Specificity ordering** for overlapping stubs: exact path > glob > regex; fewer-wildcard glob wins; ties → load order
5. Headers/query stay **exact** (out of scope this pass)

---

## MAT-001 — Path glob matching `[Should]`

**Status:** `ready`

**Story:**
> As a developer, I want stubs to match URL paths with `**` (multi-segment), `*` (single segment), and `?` (single character) glob patterns so that I can stub REST resources without enumerating every concrete path.

**Stub schema (additive):**
```yaml
id: users-by-id
method: GET
path_glob: /users/**
```

**ACs:**
```
Given a stub with path_glob "/users/*" (single-segment star)
When  a request arrives at /users/123
Then  the stub matches
When  a request arrives at /users/123/orders (extra segment)
Then  the stub does NOT match
When  a request arrives at /users
Then  the stub does NOT match (the * requires exactly one segment)

Given a stub with path_glob "/users/**" (multi-segment globstar)
When  a request arrives at /users/123
Then  the stub matches
When  a request arrives at /users/123/orders
Then  the stub matches
When  a request arrives at /users
Then  the stub matches (** may match zero segments)

Given a stub with path_glob "/files/report-?.pdf"
When  a request arrives at /files/report-1.pdf
Then  the stub matches
When  a request arrives at /files/report-12.pdf
Then  the stub does NOT match (the ? matches exactly one character)

Given a stub with path_glob containing no wildcards
When  a request arrives at that exact path
Then  the stub matches exactly (same as `path`)

Given a stub with path_glob "/"
When  a request arrives at "/"
Then  the stub matches (root and empty-path edge cases defined)
```

**Notes:**
- New optional `path_glob` field; `path` (exact) unchanged. At most one of `path`/`path_glob`/`path_regex` may be set — parse error otherwise (see MAT-005).
- Matcher is a pure function, hand-rolled (stdlib `path.Match` has no `**`); segment-wise recursive: `**` = 0+ segments, `*` = one whole segment, `?` = one char, literal otherwise.
- URL decoding: matching operates on the raw request path as received (same as today's exact match — no change).

---

## MAT-002 — Path regex matching `[Should]`

**Status:** `ready`

**Story:**
> As a developer, I want stubs to match URL paths against a regular expression so that I can match parameterized routes like `/orders/{5-digit-id}` without glob gymnastics.

**Stub schema (additive):**
```yaml
id: orders-by-id
method: GET
path_regex: ^/orders/[0-9]{5}$
```

**ACs:**
```
Given a stub with path_regex "^/orders/[0-9]{5}$"
When  a request arrives at /orders/12345
Then  the stub matches
When  a request arrives at /orders/1234 (four digits)
Then  the stub does NOT match
When  a request arrives at /orders/123456 (six digits)
Then  the stub does NOT match

Given a stub with an invalid path_regex (e.g. "([unclosed")
When  the stub file is parsed
Then  parsing fails with an error naming the stub ID and the regex (skipped like invalid delay)

Given a stub with both path and path_glob set
When  the stub file is parsed
Then  parsing fails with an error (exclusivity — at most one path matcher)
```

**Notes:**
- Uses Go stdlib `regexp` (RE2). Pattern is anchored by the user (`^...$`); no implicit anchoring — documented.
- Compiled once at load (see MAT-005); invalid regex fails stub parse (watcher logs `stub: skip` / `parse error`, consistent with existing `delay` handling).

---

## MAT-003 — Partial JSON request-body matching (`payload_contains`) `[Should]`

**Status:** `ready`

**Story:**
> As a developer, I want stubs to match a request body when it *contains* the fields I specify (ignoring extra fields) so that I can match on the fields that matter without copying the full request verbatim.

**Stub schema (additive):**
```yaml
id: user-created-filter
method: POST
payload_contains: '{"event":"user.created","user":{"id":42}}'
```

**ACs:**
```
Given a stub with payload_contains {"event":"user.created"}
When  a request body is {"event":"user.created","user_id":42,"extra":true}
Then  the stub matches (extra request keys are ignored)
When  a request body is {"event":"order.placed"}
Then  the stub does NOT match (required key absent or different)

Given a stub with payload_contains {"user":{"id":42}}
When  a request body is {"user":{"id":42,"name":"Ada","roles":["admin"]}}
Then  the stub matches (nested subset — extra nested keys ignored)
When  a request body is {"user":{"id":43}}
Then  the stub does NOT match

Given a stub with payload_contains {"items":[1,2]}
When  a request body is {"items":[2,1,3]}
Then  the stub matches (array subset — order-independent; each spec element matches a distinct request element)
When  a request body is {"items":[1]}
Then  the stub does NOT match (spec element 2 absent)

Given a stub with payload_contains {"amount":42}
When  a request body is {"amount":42.0}
Then  the stub matches (numeric equality across int/float representation)

Given a stub with payload_contains '{"a":1}'
When  a request body is "not-json"
Then  the stub does NOT match (non-JSON request body never matches)

Given a stub with both payload and payload_contains set
When  the stub file is parsed
Then  parsing fails with an error (exclusivity)
Given a stub with payload_contains that is not valid JSON (e.g. "{oops")
When  the stub file is parsed
Then  parsing fails with an error naming the stub ID
```

**Notes:**
- Spec parsed once at load into `any` (`map[string]any`); must be quoted in YAML.
- Request body parsed as JSON per request (only when a `payload_contains` stub is a candidate — parse lazily in the match path).
- Subset semantics: spec objects → every spec key must be present and subset-match (extra request keys ignored); spec arrays → each spec element must subset-match a distinct request element (order-independent); scalars deep-equal with numeric cross-type equality (compare via `float64`).
- Out of scope: JSONPath expressions, regex on body text (future backlog items if requested).

---

## MAT-004 — Specificity ordering for overlapping stubs `[Should]`

**Status:** `ready`

**Story:**
> As a developer, I want the most specific stub to win when several stubs could match a request so that my exact routes are never shadowed by wildcard catch-alls.

**ACs:**
```
Given stubs: exact path "/users/me" and glob "/users/*" and regex "^/users/.*"
When  a request arrives at /users/me
Then  the exact path stub wins

Given stubs: glob "/users/*/orders" and glob "/users/**"
When  a request arrives at /users/1/orders
Then  the fewer-wildcard glob ("/users/*/orders") wins

Given stubs: glob "/users/*" and regex "^/users/[a-z]+$"
When  a request arrives at /users/ada
Then  the glob stub wins (exact > glob > regex class order)

Given two stubs of identical specificity (same class, same wildcard count) that could both match
When  a request arrives
Then  the stub registered FIRST (load order) wins deterministically — the same stub wins every time regardless of map iteration order

Given the existing behavior for non-overlapping stubs
When  a request arrives
Then  matching is unchanged (all existing exact-match tests still pass)
```

**Notes:**
- Replace the current first-match-wins map iteration with a single-pass "best match" using a `better(a, b)` comparator:
  path class (exact 0 < glob 1 < regex 2) → glob wildcard count (fewer wins; regex = infinite → any glob beats any regex) → load order (monotonic `seq`).
- Load order: new `seq uint64` counter on `Endpoint`, assigned in `Registry.Add`, preserved on `Registry.Update` (an update keeps its original slot).
- Method/headers/query/payload constraints are still AND-ed before specificity comparison — an endpoint that fails any constraint is not a candidate.
- Nondeterminism fixed: Go map iteration order no longer influences the winner.

---

## MAT-005 — Schema validation, compile-at-load, and TUI display `[Should]`

**Status:** `ready`

**Story:**
> As a developer, I want stub schema validation at load time and correct display of pattern-based stubs so that typos surface as clear parse errors and glob/regex stubs are not blank in the dashboard.

**ACs:**
```
Given a stub file that sets more than one of path / path_glob / path_regex
When  the file is parsed
Then  a parse error names the stub ID and the conflicting fields

Given a stub file that sets both payload and payload_contains
When  the file is parsed
Then  a parse error names the stub ID and the conflicting fields

Given a stub with a valid path_regex
When  the stub is loaded
Then  the regex is compiled once at load (invalid regex → parse error, consistent with invalid delay handling)

Given a stub with path_glob or path_regex (and no literal path)
When  the dashboard endpoints panel renders
Then  the effective pattern is shown in the Method|Endpoint|Stub column (not blank)
When  the startup banner renders (non-TUI)
Then  the effective pattern is shown there too

Given a programmatic endpoint (via Registry.Add/Update) with an invalid path_regex
When  Add/Update is called
Then  an error is returned (safety net beyond YAML parsing)
```

**Notes:**
- `Endpoint` gains `PathGlob string`, `PathRegex string` (compiled once into an unexported field), `PayloadContains any`, `seq uint64`.
- `stubFile` gains matching YAML fields; parse-time validation mirrors existing `delay` handling (error → watcher logs skip/parse error).
- Display helper `DisplayPath()` returns the effective pattern (path → path_glob → path_regex) — used by the dashboard endpoints panel (`render.go:328`) and startup banner (`main.go:125`).

---

## MAT-006 — Docs and sample stubs `[Should]`

**Status:** `ready`

**Story:**
> As a developer, I want the new matching capabilities documented and demonstrated so that I can discover and use them.

**ACs:**
```
Given the stub schema reference
When  I read stubs/README.md and .ai/requirements/contracts.md
Then  path_glob, path_regex, and payload_contains are documented with semantics, exclusivity rules, and examples

Given a new glob sample stub (e.g. users-by-id.yaml using path_glob)
When  the server starts with the sample stubs
Then  the stub loads and matches /users/<id> requests

Given a new regex + payload_contains sample stub (e.g. orders-filter.yaml)
When  the server starts with the sample stubs
Then  the stub loads and matches /orders/<5-digit-id> POSTs whose body contains the specified fields

Given the architecture reference
When  I read .ai/architecture.md
Then  a "Stubs and matching" ADR documents the matching dispatch, glob semantics, subset semantics, and specificity ordering
```

---

## Test Plan (mock package + one dashboard test)

Per `.ai/rules.md` testing conventions (cover happy path, edge cases, error handling; verify each AC explicitly):

- **Glob matcher** (pure unit tests): `**` across segments, `*` exactly one segment, `?` exactly one char, literals, empty pattern, root `/`, trailing slash, no-wildcard == exact, zero-segment `**`.
- **Regex**: match/no-match, invalid regex → parse error, Add/Update compile error.
- **Subset**: nested objects, array subset (order-independent, distinct elements), extra request keys ignored, scalar equality, `42` vs `42.0`, non-JSON request → no match, payload+payload_contains → error, non-JSON spec → error.
- **Specificity**: exact > glob > regex; fewer-wildcard glob wins; identical-specificity tie → load order deterministic (register via Add, assert winner stable across repeated Match calls); existing `TestRegistry_MatchExactPath` etc. unchanged.
- **Handler integration**: a `path_glob` + `payload_contains` request returns the configured response and logs `Matched=true`; unmatched still 404.
- **Dashboard**: `DisplayPath()` renders glob/regex in endpoints panel row.

**Estimated:** ~20 new test functions.

---

## Approved Design (§4, signed off by stakeholder 2026-08-08)

The full Architect design is recorded in the implementation session. Key decisions (all
**approved** by stakeholder 2026-08-08):

- **D1 — Glob wildcard weight:** `**`=2, `*`=1 (fewer wins) so `/users/*/orders` beats
  `/users/**` deterministically. Precomputed into `ep.globWeight` at load.
- **D2 — In-segment `*`** (`/files/*.pdf`): load-time **parse error** (fails loudly, mirrors
  invalid-`delay`/invalid-regex precedent).
- **D3 — Top-level `payload_contains` types:** object, array, scalar, and `null` all allowed.
  `null` matches only a literal `null` body (disambiguated by unexported `hasPayloadContains`).
- **D4 — Empty/unset path fields:** endpoint never matches (byte-identical to today's
  `path: ""`). No "match any path" change.
- **D5 — Trailing-slash normalization for glob only** (`splitPathSegs` trims leading/trailing
  slashes); exact `path` and regex stay raw.
- **D6 — Regex anchoring user-only** (`^…$` not implicit); documented prominently.
- **D7 — `?` chars don't count toward wildcard weight** (`/users/??` beats `/users/*`).
- **D8 — Hot-reload preserves load-order slot** (watcher duplicate→Update keeps `seq`).

**Design shape (implementation-ready):**
- `types.go`: +`PathGlob`, `PathRegex`, `PayloadContains` (exported, JSON-tagged); +
  unexported `pathRe *regexp.Regexp`, `pathGlobSegs []string`, `globWeight int`,
  `hasPayloadContains bool`, `seq uint64`. Compiled fields set at load under the registry
  write lock (published before serving → race-free).
- New `matcher.go`: `compileEndpoint` (path/payload exclusivity + glob pre-split + weight +
  regex compile), `parsePayloadContains`, `pathClass` (exact 0 < glob 1 < regex 2 < none 3),
  `ep.matchPath`, `splitPathSegs`/`matchGlobSegs`/`matchGlobSegment`,
  `globWildcardWeight`, `ep.DisplayPath`.
- New `subset.go`: `parseJSONBody` (once per Match, lazy), `subsetMatch` (objects → key
  subset; arrays → order-independent distinct-element with backtracking; scalars → float64
  cross-type equality), `subsetMatchArray`/`subsetArrayRec`.
- `registry.go`: +`nextSeq`, +`ErrDuplicate` sentinel; `Add`/`Update` compile + `seq`
  semantics; `Match` → single-pass best-match via `better(a,b)` (class → globWeight → seq);
  free `matchPath` deleted.
- `stub.go`: +3 YAML fields; `ParseStubFile` wires them + `parsePayloadContains` +
  `compileEndpoint` (errors wrapped `stub <id>: …`).
- `watcher.go`: `errors.Is(err, ErrDuplicate)` branch in `LoadAll` + `Watch` (compile
  failures logged as skip/parse-error, not "duplicate, updating").
- Display: `DisplayPath()` → `render.go:328` (`sanitize(ep.DisplayPath())`),
  `render.go:495` (also sanitized — hardens legacy row), `main.go:125` banner.
- Error messages name the stub ID and conflicting fields; in-segment `*` and invalid regex
  produce load-time errors (watcher logs skip).
- **Phases:** A glob → B regex → C subset → D specificity → E display + docs. Each phase
  independently shippable, full suite green at each boundary.
- **Test files:** new `matcher_test.go`, `subset_test.go`, `specificity_test.go`; additions
  to `stub_test.go`, `handler_test.go`, `dashboard_test.go` (~25 new tests). Existing
  exact-match tests untouched (byte-identical behavior).

---

## Status log

- 2026-08-08: Stakeholder confirmed scope (globs `**`+`*`+`?`, regex, JSON subset, specificity ordering, path+body only). Plan approved. Status → `ready`; handoff to Architect for design.
- 2026-08-08: Architect design delivered (10 sections, full detail). Stakeholder **approved all 8 design decisions (D1–D8)**. Status → `ready` (design approved); handoff to Developer.
- 2026-08-08: Implemented by Developer (Phases A–E). New files `matcher.go`, `subset.go` + 3 test files; ~48 new tests; suite 415 (dashboard 258, mock 157), `-race`/vet/gofmt clean; live smoke incl. watcher reload verified.
- 2026-08-08: QA verified **MAT-001..006 all PASS** — every AC exercised by a regression-proving test; 415/415 green; live smoke (glob/regex/subset/backward-compat) + watcher reload confirmed. One low-severity cosmetic deviation found: `Registry.Update` returned compile errors bare (no endpoint ID) — **fixed** (wrapped `endpoint <id>: …` to match `Add` and design §4) + regression assertion added. Status → `verified`.
