# Richer templates

**Title:** Richer response templates — data generators, JSONPath extraction, request-field access

**Status:** backlog (scoping in progress — decision options drafted 2026-08-08, awaiting stakeholder choice)

**Priority:** `[Should]` (drove by the "match real services more closely" goal)

## Description

Today `template: true` stubs get four funcs and seven fields (`templating.go`):
`{{Header "X"}}`, `{{Query "p"}}`, `{{RandomInt a b}}`, `{{RandomHex n}}` and
`.UUID`, `.Now`, `.Timestamp`, `.RequestID`, `.Method`, `.Path`, `.Body`.
Go's `text/template` already provides conditionals/loops (`{{if}}`, `{{range}}`,
`{{with}}`) — currently undocumented.

Teams hand-roll realistic payload variability in YAML (hardcoded emails, names,
phonenumbers) or copy request data out of `.Body` as a raw string — there is no
way to *extract* a specific field from a JSON request body (no JSONPath), and
no data generators beyond `RandomInt`/`RandomHex`.

**Goal:** let stub authors write response bodies that (1) reuse the request's
structured data (e.g. echo `{{JSONPath "$.user.id"}}`), and (2) generate
realistic random data (emails, names, phones, dates) with one-line funcs —
no schema changes, no breaking changes, no new external dependencies (stdlib-first).

## Decision options (drafted 2026-08-08, awaiting stakeholder choice)

### D1 — JSONPath / request-data extraction

- **Option A — Hand-rolled JSON selector func `{{JSONPath ...}}` (recommended).**
  Small pure function over `encoding/json` supporting `$.user.name`,
  `$.items[0].id`, nested paths. No new dependency — consistent with the
  stdlib-first convention. Reading the request body; falls back to empty string
  when the body isn't valid JSON. Optionally also expose request headers/query
  via the same selector.
- **Option B — Full JSONPath library** (`github.com/PaesslerAG/jsonpath` or
  similar). More power (wildcards, filters) but breaks stdlib-first; requires
  stakeholder override approval (precedent: bubbletea was approved for the TUI
  only).
- **Option C — Skip extraction this pass** (data funcs only, smallest scope).

### D2 — Data generator funcs

- **Option A — Realistic generators (recommended).** `RandomEmail`, `RandomName`,
  `RandomPhone`, `RandomDate`/`RandomDateTime`, `RandomIP`, `RandomULID`,
  `RandomPayload("<kind>")`. Serves the "real services" goal: integrations see
  plausible data on every call.
- **Option B — Minimal set.** Keep only `RandomString(n)` (alphanumeric) +
  make `RandomInt` inclusive. Smallest useful addition.
- **Option C — Faker dependency** (`github.com/go-faker/faker/v4`). Rich dataset
  but again an external dep; needs override approval.

### D3 — Scope shape

- **Option A — One story: data funcs + JSONPath + request-field access
  (recommended).** Single additive change to `templating.go` + tests. No schema,
  no flags, no TUI. Independently shippable.
- **Option B — Two stories:** JSONPath first, data funcs second (smaller
  reviewable units).
- **Option C — JSONPath excluded** (data funcs only).

**Recommendation:** D1-A, D2-A, D3-A.

## Architecture / approach (candidate direction — subject to Architect review on pickup)

- All changes confined to `src/mock/templating.go` (+ `templating_test.go`),
  the `funcMap` inside `ExecuteBodyTemplate`, and `TemplateData` if request
  context needs extending.
- `JSONPath` implementation: hand-rolled selector over `map[string]any` from
  `json.Unmarshal` of `data.Body`; path tokens `.key` and `[N]`; returns
  `fmt.Sprint` of the value; empty when missing/invalid/non-JSON body. Pure and
  tea-free (same discipline as `highlightJSON`).
- Data funcs use the existing `crypto/rand` helpers (`randomInt`/`randomHex`
  precedent). `RandomULID` needs a hand-rolled ULID or random-hex
  (ULID-vs-random decision at implementation).
- Template error fallback behavior (parse/execute failure → original string)
  is unchanged and already tested (`TestExecuteBodyTemplate_InvalidSyntax`).
- Docs: `stubs/README.md` template section + `contracts.md`; sample stub
  showing JSONPath echo + generated fields.

## Files to create/edit

- `src/mock/templating.go` — funcMap additions (JSONPath selector, data funcs)
- `src/mock/templating_test.go` — new tests per AC
- `stubs/README.md`, `.ai/requirements/contracts.md` — template func reference
- New sample stub (e.g. `stubs/order-created.yaml` extended or a new demo file)
- `.ai/architecture.md` — short ADR entry on pickup
- Requirement story file created on pickup (per BA process)

## Steps

- [ ] Stakeholder picks D1/D2/D3 options.
- [ ] BA writes user story (`[Should]`, additive, ACs per option).
- [ ] Architect designs selector + func set.
- [ ] Developer implements + tests; full suite green (`-race`, vet, gofmt).
- [ ] QA verifies ACs.
- [ ] Documentation + sample stub.

## Status checklist

- [x] Backlog item created (2026-08-08)
- [ ] Decision options confirmed
- [ ] Story written
- [ ] Implemented
- [ ] Tested / race clean
- [ ] QA verified
- [ ] Moved out of backlog / removed