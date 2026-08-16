# Phase 19 — Richer Templates (TPL)

> Give stub authors realistic data generators and request-body extraction so
> webhook responses look like real API output — without hand-coding every field
> in YAML. One story, one dependency (`gofakeit/v7`), plus a hand-rolled
> JSONPath selector (stdlib `encoding/json`).

**Status:** `verified` | **Owner:** BA | **Date:** 2026-08-08 | **Goal:** match real services more closely

**Stakeholder decisions (signed off 2026-08-08):**
1. **D1 = A** — JSONPath via a hand-rolled selector (`{{JSONPath "$.user.id"}}`), stdlib `encoding/json`, no new deps for this part.
2. **D2 = C** — faker via `github.com/brianvoe/gofakeit/v7` (5.4k stars, 310+ funcs, zero deps, MIT). Only **approved exception** to stdlib-first (UI precedent: bubbletea).
3. **D3 = A** — one story: faker funcs + JSONPath + request-field access + docs.
4. **Naming** — direct gofakeit names in the FuncMap (`{{Name}}`, `{{Email}}`, `{{City}}`, …).

---

## TPL-001 — Realistic data generator functions `[Should]`

**Status:** `ready`

**Story:**
> As a developer, I want response templates to generate realistic random data — names, emails, phones, addresses, payments, internet fields — so that my mocked webhook responses look like a real service without hand-coding every field.

**Curated FuncMap additions** (each wraps the corresponding `gofakeit` function; all thread-safe, zero-arg unless noted):

| FuncMap name | gofakeit func | Returns | Category |
|---|---|---|---|
| `Name` | `gofakeit.Name()` | `"Markus Moen"` | Person |
| `FirstName` | `gofakeit.FirstName()` | `"Fred"` | Person |
| `LastName` | `gofakeit.LastName()` | `"Moen"` | Person |
| `Email` | `gofakeit.Email()` | `"alaynawuckert@kozey.biz"` | Person |
| `Phone` | `gofakeit.Phone()` | `"(570)245-7485"` | Person |
| `City` | `gofakeit.City()` | `"Lake Maxine"` | Address |
| `Country` | `gofakeit.Country()` | `"Canada"` | Address |
| `State` | `gofakeit.State()` | `"Nebraska"` | Address |
| `Zip` | `gofakeit.Zip()` | `"38762"` | Address |
| `Street` | `gofakeit.Street()` | `"98789 Hane Locks"` | Address |
| `Company` | `gofakeit.Company()` | `"Moen, Pagac and Wuckert"` | Company |
| `JobTitle` | `gofakeit.JobTitle()` | `"Director"` | Company |
| `IPv4Address` | `gofakeit.IPv4Address()` | `"192.168.1.42"` | Internet |
| `IPv6Address` | `gofakeit.IPv6Address()` | `"2001:db8::1"` | Internet |
| `DomainName` | `gofakeit.DomainName()` | `"kozey.biz"` | Internet |
| `MACAddress` | `gofakeit.MacAddress()` | `"ab:cd:ef:01:23:45"` | Internet |
| `UserAgent` | `gofakeit.UserAgent()` | `"Mozilla/5.0 …"` | Internet |
| `CreditCardNumber` | `gofakeit.CreditCardNumber(nil)` | `"4287271570245748"` | Payment |
| `Price` | `gofakeit.Price(0.01, 999.99)` | `"42.99"` | Payment |
| `ProductName` | `gofakeit.ProductName()` | `"Incredible Metal Chair"` | Product |
| `ULID` | `gofakeit.ID()` | `"pfsfktb87rcmj6bqha2fz9"` | ID |
| `HexColor` | `gofakeit.HexColor()` | `"#a3e4f2"` | Misc |
| `Word` | `gofakeit.Word()` | `"river"` | Text |
| `Sentence` | `gofakeit.Sentence(3)` | `"Record river mind."` | Text |
| `Paragraph` | `gofakeit.Paragraph(1, 3, 12, " ")` | `"Quia voluptatem…"` | Text |
| `Bool` | `gofakeit.Bool()` | `true` | Misc |
| `UUID` | `gofakeit.UUID()` | `"0d1e6f2a-…"` | ID |

**ACs:**
```
Given a stub with template: true
When  a response body or header uses any of the listed faker funcs ({{Name}},
      {{Email}}, {{City}}, {{Price}}, …)
Then  the output is a plausible value of the documented type (non-empty string,
      valid email/phone/IP/date where applicable)
And   different requests produce (with high probability) different random values
And   faker funcs compose with existing funcs and fields ({{.UUID}}, {{RandomInt}},
      {{Header}}, {{Query}}) and with Go template control flow ({{if}}, {{range}})

Given a stub uses {{RandomInt}} or {{RandomHex}}
When  the template renders
Then  behavior is byte-for-byte unchanged (existing funcs untouched)

Given a template uses a faker func name that clashes with a field (UUID)
When  it renders
Then  {{UUID}} (function) and {{.UUID}} (field) are both available and independent
      — referencing either renders a UUID string without collision
```

**Notes:**
- `RandomString` (gofakeit's slice-picker) and `Number`/`Int` (would shadow `RandomInt`) are deliberately **NOT** added.
- `ULID` maps to `gofakeit.ID()` (20-char URL-safe lowercase string, NOT a ULID). `gofakeit.ULID()` does not exist in v7.15.0 — verified against source. FuncMap key stays `ULID` (template-facing), doc says "ULID-style identifier". **(D9)**
- `Sentence`/`Paragraph` are deprecated upstream — args are accepted (variadic) but silently ignored. Wired as-is, documented. Pinned at v7.15.0 so they won't disappear until a deliberate upgrade. **(D10)**
- `Price` is wired as a **zero-arg closure** returning `fmt.Sprintf("%.2f", gofakeit.Price(0.01, 999.99))` → e.g. `"42.99"`. Fixed range baked in. **(D11)**
- All funcs are zero-arg except `Sentence(n int)`, `Paragraph(a, b, c, sep)`, `Price(a, b float64)`. No FuncMap func raises an error for the curated subset.

---

## TPL-002 — Hand-rolled JSONPath selector `[Should]`

**Status:** `ready`

**Story:**
> As a developer, I want to extract a specific field from the JSON request body in a response template so that I can echo back structured request data without parsing the raw `.Body` string.

**Syntax:**
| Path | Meaning |
|------|---------|
| `$` | The whole request body (as JSON string) |
| `$.user` | Object key `user` |
| `$.user.name` | Nested object key |
| `$.items[0]` | Array index 0 |
| `$.items[0].name` | Nested into an array element |
| `$.items[1].tags[0]` | Deeper nesting |

**ACs:**
```
Given a template stub receiving a JSON request body like
      {"user":{"name":"Ada","roles":["admin","dev"]},"amount":42,"ok":true,"tags":null}
When  the template uses {{JSONPath "$.user.name"}}
Then  it renders "Ada"
When  it uses {{JSONPath "$.roles[1]"}}
Then  it renders "dev"
When  it uses {{JSONPath "$.amount"}}
Then  it renders "42" (numbers render unquoted)
When  it uses {{JSONPath "$.ok"}}
Then  it renders "true" (booleans render unquoted)
When  it uses {{JSONPath "$.tags"}}
Then  it renders "null"
When  it uses {{JSONPath "$.roles"}}
Then  it renders a valid JSON array string "[admin","dev"]" (objects/arrays marshal to JSON)
When  it uses {{JSONPath "$"}}
Then  it renders the entire request body as a JSON string

Given a request body that is NOT valid JSON (e.g. "plain text" or empty)
When  a template uses {{JSONPath "$.x"}}
Then  it renders "" (empty string) — no panic, no fallback of the whole template

Given a path that does not exist ({{JSONPath "$.user.missing"}} or {{JSONPath "$.roles[5]"}})
When  the template renders
Then  it renders "" (empty string)

Given a request body is valid JSON and the path is well-formed
When  the template uses {{JSONPath}}
Then  the body is parsed exactly once per request (cached in TemplateData.parsedBody)
And   the template renders the extracted value inline
And   {{.Body}} still renders the full raw body string (unchanged field)
```

**Notes:**
- Return semantics: string → as-is; numbers/booleans → unquoted (`fmt.Sprintf`); objects/arrays → `json.Marshal` (valid JSON); `null` → `"null"`; missing/invalid → `""`.
- Parsing happens once per request in `BuildTemplateData` (unexported `parsedBody any`), silently nil on parse failure. `JSONPath` closure reads it — no per-call re-parse.
- Out of scope: JSONPath filters (`$.items[?(@.x==1)]`), wildcards (`$.items[*]`), recursive descent (`$..x`). A future backlog item if requested.

---

## Files

| File | Change |
|---|---|
| `src/go.mod` / `src/go.sum` | +`github.com/brianvoe/gofakeit/v7` |
| `src/mock/templating.go` | +`parsedBody any` on `TemplateData`; `BuildTemplateData` parses body; funcMap gains `JSONPath` + curated faker funcs |
| `src/mock/jsonpath.go` (**new**) | dot/bracket path traversal over `parsedBody` |
| `src/mock/templating_test.go` | +tests per AC |
| `src/mock/jsonpath_test.go` (**new**) | +tests per AC |
| `src/mock/handler_test.go` | +handler round-trip test |
| `stubs/README.md` | Template Functions table expanded; JSONPath section |
| `.ai/requirements/contracts.md` | Template functions table updated |
| `stubs/template-showcase.yaml` (**new**) | faker + JSONPath demo stub |
| `.ai/architecture.md` | ADR entry |

## Test Plan

- **jsonpath_test.go** (~15): nested keys, array index, deep nesting, missing path, invalid index, non-JSON body, empty path, root `$`, return types (string/number/bool/null/object/array), parse-once caching behavior if observable.
- **templating_test.go** (+~10): every faker func present in the FuncMap returns a plausible non-empty value; `{{JSONPath}}` through `ExecuteBodyTemplate`; faker + `{{if}}` composition; `.Body` still works; `BuildTemplateData` nil/empty body → `parsedBody` nil; `{{UUID}}` vs `{{.UUID}}` independence; existing tests unchanged (backward-compatible).
- **handler_test.go** (+~2): full handler round-trip with a template stub using faker + JSONPath; body with non-JSON request → `JSONPath` returns `""`, response still 200.

## §4 — Approved Design

**Full design:** `.ai/designs/19-richer-templates-design.md` (632 lines, 10 sections). Architecture reviewed and approved 2026-08-16.

**Key decisions (D9–D16, stakeholder-approved):**

| # | Decision | Resolution |
|---|----------|------------|
| D9 | `ULID` missing from gofakeit → substitute `gofakeit.ID()` | **ID** — 20-char URL-safe lowercase, template key stays `ULID` |
| D10 | `Sentence`/`Paragraph` deprecated, args ignored | **Accept as-is** + document |
| D11 | `Price` requires two float64 args | **Zero-arg wrapper** with fixed range `(0.01, 999.99)`, returns `string` |
| D12 | `$` on non-JSON body | **`""`** — no JSON, no extraction |
| D13 | `$` and objects/arrays render via canonical re-marshal | **`json.Marshal`** — not byte-exact (sorted keys, compacted) |
| D14 | Number rendering | **`strconv.FormatFloat(v, 'g', -1, 64)`** — 42→"42", 1e21→"1e+21" |
| D15 | Whitespace in path keys | **Zero trimming** — strict grammar, spaces rejected |
| D16 | gofakeit version | **Pin v7.15.0** — upgrades are deliberate stories |

**Implementation shape (from design):**
- Package-level `fakerFuncMap` (27 static entries, built once, read-only) merged with per-request `buildTemplateFuncMap` (4 existing funcs + `JSONPath` closure).
- New `src/mock/jsonpath.go`: `tokenizeJSONPath` (strict grammar) + `evalJSONPath` (pure traversal) + `makeJSONPathFunc` (closure over `TemplateData.parsedBody`).
- `TemplateData` gains unexported `parsedBody any`, set in `BuildTemplateData` via existing `parseJSONBody` from `subset.go`. `handler.go` untouched.
- Phases A→D: dep → JSONPath → funcMap → docs.

**Status log:**

- 2026-08-08: Backlog item created (`.ai/backlog/richer-templates.md`) with D1–D3 decisions. Stakeholder confirmed **D1=A, D2=C, D3=A** and **direct naming**. Story written. Status → `ready`; handoff to Architect.
- 2026-08-16: Architect design delivered and verified (`.ai/designs/19-richer-templates-design.md`, 632 lines). D9–D16 approved. Status remains `ready`; handoff to Developer.
- 2026-08-16: Implemented directly by BA (Developer subagent failed twice to deliver; stakeholder directed "fix the gofakeit dep" → full Phase A–D executed in-session). gofakeit v7.15.0 promoted to direct dep; `jsonpath.go` + `jsonpath_test.go` new; `templating.go` funcMap/TemplateData updated; handler + templating tests added; docs + showcase stub. 448 tests green with `-race`, vet/gofmt clean, live smoke verified. Status → `in-progress` (pending QA).
- 2026-08-16: **QA verified — ALL ACs PASS** (report `.ai/qa/19-richer-templates-report.md`). Full suite 448 tests green under `-race`; vet/gofmt clean; live HTTP round-trip confirmed JSONPath extraction, faker determinism, and D12 non-JSON behavior. No defects/gaps/deviations. Status → `verified`.