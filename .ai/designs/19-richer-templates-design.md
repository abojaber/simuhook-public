# Design — Phase 19: Richer Templates (TPL-001 / TPL-002)

> Implementation contract for the Developer. Source of truth: `.ai/requirements/19-richer-templates.md`
> (status `ready`, stakeholder decisions D1–D3 signed off 2026-08-08). All gofakeit signatures below
> were verified **directly against the `v7.15.0` source tree** (tag `brianvoe/gofakeit@v7.15.0`,
> fetched 2026-08-16), not from memory or the README.
>
> **One dependency:** `github.com/brianvoe/gofakeit/v7` at **exactly `v7.15.0`** (D16).
> **One new source file:** `src/mock/jsonpath.go` (+ `jsonpath_test.go`). Everything else edits
> existing files. No admin handler, no CLI flag changes, no types.go/handler.go structural changes.

---

## §0 — Verified findings table (gofakeit v7.15.0, all 161+ top-level funcs verified)

Verification method: full `v7.15.0` tarball unpacked; `grep -rn "^func <Name>"` over `*.go`. Every
row below states the **exact signature as it exists in the tag**, status (`exists` / `deprecated` /
`missing`), required args, and the proposed FuncMap wiring.

| FuncMap name | gofakeit v7.15.0 signature | Status | Args | FuncMap wiring |
|---|---|---|---|---|
| `Name` | `func Name() string` (person.go:44) | exists | none | `"Name": gofakeit.Name` |
| `FirstName` | `func FirstName() string` (person.go:54) | exists | none | `"FirstName": gofakeit.FirstName` |
| `LastName` | `func LastName() string` (person.go:70) | exists | none | `"LastName": gofakeit.LastName` |
| `Email` | `func Email() string` (person.go:236) | exists | none | `"Email": gofakeit.Email` |
| `Phone` | `func Phone() string` (person.go:218) | exists | none | `"Phone": gofakeit.Phone` |
| `City` | `func City() string` (address.go:121) | exists | none | `"City": gofakeit.City` |
| `Country` | `func Country() string` (address.go:155) | exists | none | `"Country": gofakeit.Country` |
| `State` | `func State() string` (address.go:129) | exists | none | `"State": gofakeit.State` |
| `Zip` | `func Zip() string` (address.go:145) | exists | none | `"Zip": gofakeit.Zip` |
| `Street` | `func Street() string` (address.go:57) | exists | none | `"Street": gofakeit.Street` |
| `Company` | `func Company() string` (company.go:4) | exists | none | `"Company": gofakeit.Company` |
| `JobTitle` | `func JobTitle() string` (company.go:66) | exists | none | `"JobTitle": gofakeit.JobTitle` |
| `IPv4Address` | `func IPv4Address() string` (internet.go:86) | exists | none | `"IPv4Address": gofakeit.IPv4Address` |
| `IPv6Address` | `func IPv6Address() string` (internet.go:98) | exists | none | `"IPv6Address": gofakeit.IPv6Address` |
| `DomainName` | `func DomainName() string` (internet.go:12) | exists | none | `"DomainName": gofakeit.DomainName` |
| `MACAddress` | `func MacAddress() string` (internet.go:110) | exists | none | `"MACAddress": gofakeit.MacAddress` |
| `UserAgent` | `func UserAgent() string` (internet.go:160) | exists | none | `"UserAgent": gofakeit.UserAgent` |
| `CreditCardNumber` | `func CreditCardNumber(cco *CreditCardOptions) string` (payment.go:102) | exists | 1 — **must pass `nil`** | `"CreditCardNumber": func() string { return gofakeit.CreditCardNumber(nil) }` |
| `Price` | `func Price(min, max float64) float64` (payment.go:49) | exists | **2 — required** | fixed-range wrapper, see D11 |
| `ProductName` | `func ProductName() string` (product.go:68) | exists | none | `"ProductName": gofakeit.ProductName` |
| `ULID` | **no `ULID` function anywhere in v7.15.0** (0 grep hits across all `.go` files) | **MISSING** | (**substitute `func ID() string`**, id.go:20) | `"ULID": gofakeit.ID` — see D9 |
| `HexColor` | `func HexColor() string` (color.go:34) | exists | none | `"HexColor": gofakeit.HexColor` |
| `Word` | `func Word() string` (word_general.go:10 — **not in text.go**, moved to `word_general.go`) | exists | none | `"Word": gofakeit.Word` |
| `Sentence` | `func Sentence(wordCount ...int) string` (text.go:151) | exists, **DEPRECATED** | variadic, **silently ignored** | `"Sentence": gofakeit.Sentence` — see D10 |
| `Paragraph` | `func Paragraph(paragraphCount ...any) string` (text.go:169) | exists, **DEPRECATED** | variadic `any`, **silently ignored** | `"Paragraph": gofakeit.Paragraph` — see D10 |
| `Bool` | `func Bool() bool` (misc.go:10) | exists | none | `"Bool": gofakeit.Bool` (renders `"true"`/`"false"`) |
| `UUID` | `func UUID() string` (id.go:54, version 4) | exists | none | `"UUID": gofakeit.UUID` |

**Overrides / discrepancies vs. the story's table (all flagged):**

1. **`ULID` is MISSING** — the story's `gofakeit.ULID()` would not compile. Substitution (D9): `gofakeit.ID()`
   returns a **20-character** URL-safe lowercase string drawn from the alphabet
   `"23456789abcdefgghjkmnpqrstuvwxyz"` (id.go:4–16: `idLength = 20`, 5 bits/char via `idAlphabetMask`).
   Example from a live run: `"pfsfktb87rcmj6bqha2fz9"`. **This is NOT a ULID** (ULID = 26-char Crockford
   base32). The FuncMap **key** stays `ULID` (template-facing), but it delegates to `ID()`. Documented in
   `stubs/README.md` as "ULID-style identifier" — see §9 D9.
2. **`Sentence`/`Paragraph` are DEPRECATED** — upstream doc comments at text.go:150/154 and 168/174 state
   the parameters are **ignored** and will be removed in the next major release. The wiring passes the raw
   gofakeit funcs through; variadic signatures make any call form template-parse-safe. Story's `Sentence(3)`
   and `Paragraph(1, 3, 12, " ")` still **compile** (variadic accepts them) but the arguments have **zero
   effect** on output length. Accepted as-is (D10) — no wrapper, no doc lie: README says "word count/paragraph
   count args are accepted but ignored (upstream deprecated)".
3. **`Price` requires two float64 args** — there is **no zero-arg `Price()`** variant in v7.15.0. The story
   shows `gofakeit.Price(0.01, 999.99)` — matches the real signature. Behavior verified (payment.go:62):
   `math.Floor(float64Range(f, min, max)*100) / 100` → a 2-decimal `float64` in `[0.01, 999.99]`.
   **Decision D11:** wrap as a zero-arg closure returning a **string** so template authors call `{{Price}}`
   with the story's fixed range baked in (see §9 D11 for the alternative).
4. **`CreditCardNumber(nil)` is what makes it random** — payment.go:107: `nil` → `&CreditCardOptions{}` →
   type randomly picked across all card types, Luhn-valid number returned. The explicit `nil` argument is
   mandatory; there is no zero-arg variant.
5. **`Word` lives in `word_general.go`** (not `text.go`) in v7.15.0 — irrelevant to wiring, noted to avoid a
   review nit. Zero-arg.
6. **`MACAddress` casing** — gofakeit's identifier is `MacAddress` (lowercase `a`), not `MACAddress`. The
   template-facing FuncMap key is `MACAddress` (matches the story); the Go reference is `gofakeit.MacAddress`.
7. **Duplicate `g` in `ID()`'s alphabet** (`"…abcdeg ghjk…"`) is an upstream literal — 32 entries with `g`
   twice. Harmless (uniform-ish distribution, URL-safe). Do **not** "fix" it with a custom alphabet; keep the
   stock call so upgrades behave identically.
8. **`Bool` returns Go `bool`** — text/template renders it via `%v` → `"true"`/`"false"`. No wrapper needed.

Funcs deliberately **NOT** added (story §Notes, unchanged): `RandomString`, `Number`, `Int`, and everything
else outside the curated 27. The existing `RandomInt`/`RandomHex` stay as-is (AC: byte-for-byte unchanged).

---

## §1 — Dependency decision: `github.com/brianvoe/gofakeit/v7` @ `v7.15.0`

**Decision (D16):** add `github.com/brianvoe/gofakeit/v7 v7.15.0` to `src/go.mod`.

Rationale — verified from the tag's artifacts, not its marketing page:

| Criterion | Finding |
|---|---|
| License | MIT (declared in module info and license file) |
| Transitive deps | **Zero** — the tag's `go.mod` contains only `module` + `go 1.22`; no `require` block. Net new closure for Simuhook = 0 packages besides gofakeit itself |
| Go version fit | gofakeit requires `go 1.22`; Simuhook's `src/go.mod` declares `go 1.26.4` — forward-compatible |
| Top-level func API | All 27 curated funcs are **package-level functions** (`gofakeit.Name()`, …), matching the D4 naming decision verbatim — no `NewFaker` plumbing in templates |
| Thread safety | `var GlobalFaker *Faker = New(0)` (faker.go:13). `New(0)` → `source.NewCrypto()` seed → `NewFaker(rand.NewPCG(...), true)` where `Locked: true` installs a `sync.Mutex` (faker.go:16–22, 38–51). Every `GlobalFaker` call takes the mutex → safe under concurrent request goroutines and `-race` |
| Determinism option | `Seed()` exists but is **out of scope** (no `--seed` flag; story does not ask for it) |
| Curation | We expose only the 27-function curated subset, not a wholesale func dump — keeps the template surface documented and reviewable |

**Pinning (D16):** exact version `v7.15.0` in `go.mod`, `go.sum` committed. Rationale: `Sentence` /
`Paragraph` are deprecated upstream and will break in the next major; pinning shields Simuhook from
surprise signature churn until a deliberate upgrade story. Upgrade path = new story, update this ADR.

Alternative rejected: `fakerGo`? No. brianvoe/gofakeit was the stakeholder's **D2 = C** (5.4k stars,
zero deps, MIT), and it is the only approved stdlib-first exception built on the bubbletea UI precedent.

---

## §2 — FuncMap construction

### Shape

- A package-level immutable table for the **static** faker entries:

  ```go
  // fakerFuncMap is the curated gofakeit surface (27 entries, §0). Built once,
  // read-only afterwards. Safe to read from any goroutine; text/template copies
  // it at parse time and never mutates the shared map.
  var fakerFuncMap = template.FuncMap{ /* static entries only */ }
  ```

  Static entries = everything except `JSONPath` (which needs the per-request `data` closure).
  `CreditCardNumber`/`Price` are small closures over gofakeit calls but are still **stateless** — they
  belong in the static table.

- A per-call heap for the **request-bound** entries:

  ```go
  func buildTemplateFuncMap(data *TemplateData, headers, query map[string]string) template.FuncMap {
      m := template.FuncMap{
          "Header":    func(key string) string { return headers[key] },
          "Query":     func(key string) string { return query[key] },
          "RandomInt": randomInt,
          "RandomHex": randomHex,
          "JSONPath":  makeJSONPathFunc(data),
      }
      for name, fn := range fakerFuncMap {
          m[name] = fn
      }
      return m
  }
  ```

### Concurrency argument (the "shared map" question)

1. `text/template`'s `Funcs()` **copies** the provided map into the template during `Parse`; after parse the
   template's internal func table is read-only for execution. Confirmed by the stdlib contract:
   "[Funcs must be] called before Parse" and the tree owns its copy.
2. Concurrent `ExecuteBodyTemplate` calls each run their own `Parse` on their own template instance. They
   **read** `fakerFuncMap` concurrently. Concurrent map reads are safe in Go; the table is never written
   after `init`.
3. gofakeit's own RNG state lives once in `GlobalFaker`, mutex-guarded (faker.go:16–22) — the FuncMap
   entries merely **call** gofakeit; the mutex serializes the random reads, not our map.
4. The merge loop in `buildTemplateFuncMap` writes to the *fresh private* map `m`, never to the shared
   table. No data race possible.

### Backward compatibility (byte-identical existing behavior)

- The 4 existing funcs (`Header`, `Query`, `RandomInt`, `RandomHex`) keep their exact names, signatures,
  and semantics — `executeBodyTemplate`'s lookup order doesn't change; new entries are additive and cannot
  collide with them (`JSONPath` + 27 fakers name-checked against the existing 4 and TemplateData fields: no
  collisions).
- `ExecuteBodyTemplate` and `ExecuteHeaderTemplate` keep their public signatures and their original-fallback
  behavior on parse/exec error (existing tests must remain untouched — §7).
- The only internal change: inline `template.FuncMap{...}` literal → `buildTemplateFuncMap` call.
- `Header`/`Query` remain closures over the per-request maps (exactly the current behavior).

---

## §3 — JSONPath algorithm (complete contract)

Two pure functions in the new file `src/mock/jsonpath.go`, package `mock`, parallel in style to `subset.go`
(top comment block, no globals, unexported helpers, exported only where a test needs them).

### Grammar — `tokenizeJSONPath(path string) ([]string, error)`

Tokens are `.`-separated keys and `[N]` integer brackets. The token stream is `[]string`; index vs. key is
**disambiguated at eval time by the container type** (see below) — a regular `[]string` is sufficient.

```
jpath     := "$" token*
token     := "." key | "[" digits "]"
key       := (letter | digit | "_" | "-")+     // [A-Za-z0-9_-]; no spaces, no dots inside a key
digits    := digit+                             // [0-9]+, no sign, no leading space
```

Rules (strict, per story ACs + edge cases):

| Input | Result |
|---|---|
| `$` | `[]` (root), no error |
| `$.user.name` | `["user","name"]` |
| `$.roles[1]` | `["roles","1"]` |
| `$.items[0].name` | `["items","0","name"]` |
| `$[0]` | `["0"]` (index into a root array) |
| `"user.name"` (**no `$` prefix**) | **error** — all template-facing paths must start with `$` (D12/D13 context; strictness avoids silent foot-guns) |
| `.user` (leading dot) | **error** — "no leading dots" |
| `$user` | **error** — after `$` only `.` or `[` allowed |
| `$.` (trailing/empty key) | **error** |
| `$.user[]` | **error** — empty brackets |
| `$.user[-1]` | **error** — negative indices unsupported (out of scope, same as filters/wildcards) |
| `$.user[abc]` | **error** — non-digit inside brackets |
| `$.user name` | **error** — spaces are never tolerated (D15: zero whitespace trimming) |
| `$..x` / `$.x[*]` / `$.x[?(@.a)]` | **error** — filters/wildcards/recursive descent are out of scope and rejected at tokenize (better than silently mis-evaluating) |
| `$.0` | `["0"]` — a numeric **key** (dot form). Only `[N]` is an index intent; on a map it is still a key lookup |

Implementation sketch (keep it dumb — a hand `for` loop, not a regex):

```go
func tokenizeJSONPath(path string) ([]string, error) {
	if path == "" {
		return nil, fmt.Errorf("jsonpath: empty path")
	}
	if path[0] != '$' {
		return nil, fmt.Errorf("jsonpath: path must start with '$': %q", path)
	}
	var toks []string
	rest := path[1:]
	for len(rest) > 0 {
		switch rest[0] {
		case '.':
			rest = rest[1:]
			if rest == "" || rest[0] == '.' || rest[0] == '[' {
				return nil, fmt.Errorf("jsonpath: empty key in %q", path)
			}
			i := 0
			for i < len(rest) && isKeyChar(rest[i]) {
				i++
			}
			if i == 0 {
				return nil, fmt.Errorf("jsonpath: bad key in %q", path)
			}
			toks = append(toks, rest[:i])
			rest = rest[i:]
		case '[':
			rest = rest[1:]
			i := 0
			for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
				i++
			}
			if i == 0 || i >= len(rest) || rest[i] != ']' {
				return nil, fmt.Errorf("jsonpath: bad index in %q", path)
			}
			toks = append(toks, rest[:i])
			rest = rest[i+1:]
		default:
			return nil, fmt.Errorf("jsonpath: unexpected %q in %q", rest[0], path)
		}
	}
	return toks, nil
}
```

### Eval — `evalJSONPath(root any, tokens []string) (string, error)`

```
cur := root
for each token:
    switch tv := cur.(type) {
    case map[string]any:    // token is a KEY
        v, ok := tv[token]; if !ok → ""
        cur = v
    case []any:             // token is an INDEX (disambiguated by container)
        n, err := strconv.Atoi(token); if err → "" (shouldn't happen post-tokenize)
        if n < 0 || n >= len(tv) → ""
        cur = tv[n]
    case nil:               // traversed INTO null → "" (JSONPath null semantics)
        → ""
    default:                // scalar reached before path exhausted → type mismatch → ""
        → ""
    }
return renderJSONValue(cur)
```

`renderJSONValue(v any) string` — the return-semantics table (story §Notes):

| Go value | Rendered | Example | Means |
|---|---|---|---|
| `string` | raw, as-is | `"Ada"` → `Ada` | unquoted string |
| `float64` | `strconv.FormatFloat(v, 'g', -1, 64)` | `42` → `42`; `42.5` → `42.5`; `1e21` → `1e+21` | **`g` format**, shortest round-trip (D14) |
| `bool` | `"true"` / `"false"` | `true` → `true` | unquoted |
| `map[string]any` / `[]any` | `json.Marshal` → string | `[admin,dev]`-ish array → valid JSON | **canonical re-marshal**, not byte-exact (§ D13) |
| `nil` | `"null"` | `$.tags` → `null` | literal JSON null |
| missing path / OOB / type mismatch | `""` | `$.user.missing` | empty string, no panic |
| tokenize error | `""` | malformed path | empty string |

**Root & empty-body rules (D12):**

| Situation | Result |
|---|---|
| `$` on a parsed object/array | whole value `json.Marshal`-ed — canonical JSON string |
| `$` on a parsed scalar | that scalar rendered per the table above |
| **body not valid JSON** (`parsedBody == nil`) | `""` — even for `$`. Non-JSON body ⇒ no data ⇒ empty. **Not** `"null"` (D12) |
| empty body (`len(body) == 0`) | `""` |
| `null` literal body (`null` parses OK) | `null` is a *valid JSON value* ⇒ `$` renders `"null"` |

### Closure — `makeJSONPathFunc(data *TemplateData) func(string) string`

```go
// makeJSONPathFunc returns the template func bound to this request's parsed body.
// Never panics: tokenize/eval errors and nil bodies all yield "".
func makeJSONPathFunc(data *TemplateData) func(string) string {
	return func(path string) string {
		if data.parsedBody == nil {
			return ""
		}
		toks, err := tokenizeJSONPath(path)
		if err != nil {
			return ""
		}
		out, err := evalJSONPath(data.parsedBody, toks)
		if err != nil {
			return ""
		}
		return out
	}
}
```

Note `evalJSONPath` returns `(string, error)` for symmetry/testability but in practice never errors after
tokenization succeeds (all failures surface as `""` via the type-switch). Keep the error slot anyway — the
story's AC surface (invalid path → `""`, no panic, whole template still renders) is satisfied by the closure
swallowing everything.

**Parse-once caching (AC):** the closure reads `data.parsedBody` — set exactly once in `BuildTemplateData`
(§4). Multiple `{{JSONPath …}}` calls in one template hit the same cached value, never re-parse. There is no
per-call `json.Unmarshal`.

### Style rules

- `jsonpath.go` mirrors `subset.go`: package-comment block up top, small pure funcs, no globals, no external
  imports beyond `encoding/json`, `fmt`, `strconv`. Exported surface: `TokenizeJSONPath` + `EvalJSONPath`
  **only if** tests are placed in `package mock_test`; the in-package tests (`package mock`) can use the
  unexported versions — **prefer unexported**, exporting grows the API for no caller (nothing outside the
  package uses JSONPath).

---

## §4 — TemplateData change

### Verified: nothing outside `templating.go` references `TemplateData` fields

`grep -rn "TemplateData" src/ --include="*.go"` (excluding `_test.go`) → hits are all in `templating.go`,
plus `handler.go` uses the type only via `BuildTemplateData` (line 105), `ExecuteHeaderTemplate` (120), and
`ExecuteBodyTemplate` (136). **types.go is untouched** (zero hits). Therefore adding an unexported field is
safe: no JSON serialization of `TemplateData` exists anywhere (it is never marshaled — structs with
unexported fields don't serialize anyway), and no constructor outside `templating.go` builds a
`TemplateData` literal.

### The change

```go
type TemplateData struct {
	UUID      string
	Now       string
	Timestamp int64
	RequestID string
	Method    string
	Path      string
	Body      string

	// parsedBody caches the request body parsed as JSON (any), set in
	// BuildTemplateData via parseJSONBody. nil when the body is empty or
	// not valid JSON. Read by the JSONPath template func; never re-parsed.
	parsedBody any
}
```

```go
func BuildTemplateData(requestID, method, path string, body []byte) *TemplateData {
	td := &TemplateData{ /* existing literals unchanged */ Body: string(body) }
	if v, ok := parseJSONBody(body); ok {
		td.parsedBody = v
	}
	return td
}
```

**Reuse, do not duplicate:** `parseJSONBody` is the exact existing helper in `subset.go` (lines 26–35):
`func parseJSONBody(raw []byte) (any, bool)`, empty → `(nil,false)`, `json.Unmarshal` error → `(nil,false)`,
else `(v,true)`. Because it uses `json.Unmarshal`, numbers arrive as `float64` — **no `json.Decoder.UseNumber`**
decision needed (story's number rendering is float64-based; documented in §3 and D14).

### Parse-cost and truncation interaction (verified)

- `BuildTemplateData` is called only when `resp.Template` is true (handler.go:103–111) — so the JSON parse
  happens **once per template-stub request**, not for every request server-wide.
- The parse operates on the **full raw request body** passed into `BuildTemplateData`. The `--max-log-body`
  truncation happens later, at **log capture time in the logger** (contracts.md note) — it never feeds back
  into the template data. So a big body is fully parsed for templates regardless of `LOG_MAX_BODY`; that is
  intended (template correctness over log size), and the cost is documented in §6.

---

## §5 — Concurrency & safety

| Concern | Analysis |
|---|---|
| Shared `fakerFuncMap` | Written once at init; `text/template.Funcs()` copies per-parse; concurrent **reads** of the package map are safe; merge writes go into per-call private maps. No race (§2). |
| gofakeit global RNG | `GlobalFaker` = `New(0)` → `NewFaker(..., true)` → `Locked: true` + `sync.Mutex` (faker.go:16–22). Every top-level call serializes on the mutex. Safe under concurrent request goroutines; validated by the existing `TestHandler_Concurrent_Race` + the new tests under `-race`. |
| JSONPath over `parsedBody` | `parsedBody` is per-request (`BuildTemplateData` allocates a fresh `TemplateData` each call) and read-only after construction; not shared between requests. No locking needed. |
| Fallback-on-error | All new funcs return safely: tokenize/eval errors → `""`; no func in the FuncMap can panic (gofakeit's deprecated Sentence/Paragraph return `""` on internal failure; Price has no error path). The existing `ExecuteBodyTemplate` parse/exec fallback (return original string) is preserved byte-for-byte — a template that references an unknown func still falls back to the raw body, as today. |
| `-race` validation plan | Phase C/D gates: `go test -race ./src/...` full suite (currently 366 tests green) + `go vet ./src/...` + `gofmt -l`. New `TestHandler_*` request concurrency variants re-use the existing concurrent pattern. |

---

## §6 — Edge cases and risks table

| # | Edge / risk | Analysis | Decision |
|---|---|---|---|
| EC1 | `{{UUID}}` (func) vs `{{.UUID}}` (field) namespace collision | text/template keeps **functions and data fields in separate namespaces** (spec §Actions/pipelines). A bare identifier at the start of a command/pipe resolves against the func table; `.UUID` is a field access on the dot. A func named `UUID` does **not** shadow the field `.UUID`. Both can be referenced in the same template. Verified conceptually and will be pinned by an explicit test (`TestUUIDFuncFieldIndependence`). | No action; test it |
| EC2 | faker func **output** containing `{{` or `}}` | text/template does **not** re-parse function output — executed pipelines write literal text. A faked email like `"re{{acy@foo}}"` would render literally, not template-parse. Safe by construction (Go stdlib behavior). | No action |
| EC3 | Large request body parse cost | `json.Unmarshal` runs once per template-stub request on the full body (§4). A 1 MB body is ~3–10 ms worst case; the parse happens inside `BuildTemplateData` **before** the delay sleep (handler.go:101–115), so it does not extend the measured Delivery latency for non-delay stubs, but does add wall-clock for the caller. Recommend keeping it simple: accept the cost. If profiling ever flags it: lazily parse on first `JSONPath` call with a `sync.Once`-style flag on TemplateData — **out of scope for this story** (would violate the "parsed exactly once" AC's spirit). | Accept; note in README |
| EC4 | Deprecated `Sentence`/`Paragraph` vs linters | `go vet` does **not** flag calls to deprecated functions (only `staticcheck`'s SA1019 does; no lint configured — rules.md). The upstream doc comment promises removal in next major → the **pin (D16)** is the mitigation. | Accept + pin |
| EC5 | `Paragraph` `...any` accepts anything — stub author misuse | `{{Paragraph "x"}}` compiles at template-parse (variadic `any`), args silently ignored at exec. **No parse error, no exec error, no panic** — worst case a confused author, which the README wording ("arguments ignored") mitigates. Same for `Sentence` with `...int`. | Accept; document |
| EC6 | `Price` fixed range | Story locks `(0.01, 999.99)`; authors wanting other ranges use `{{RandomInt}}` + formatting (story §Notes). The FuncMap closure bakes the range in; no knobs. | Fixed; documented |
| EC7 | `json.Marshal` on `map[string]any` with non-string keys | Struct literals only in tests: none — JSON parse always yields `string` keys. `json.Marshal` cannot fail on `map[string]any`/`[]any` values from JSON. The closure's error return is belt-and-suspenders. | No action |
| EC8 | `evalJSONPath` on `nil` mid-path (`$.a.b` where `a` is `null`) | Type-switch `nil` → `""` immediately (JSONPath semantics: path through null = missing). Consistent with story ACs. | Decision locked |
| EC9 | `$.items[5]` out of bounds | `[]any` case, index check → `""`. | Decision locked |
| EC10 | Path token equal to numeric-looking map **key** (`$.0`) | Container disambiguation: on `map[string]any`, token `"0"` is a key lookup (works); on `[]any` it's the index. | Decision locked |
| EC11 | Template func name collision with new entries | Name-checked: no FuncMap entry collides with the 4 existing funcs, nor with TemplateData fields in a way that breaks (EC1 covers the one overlap). `JSONPath`, the 27 fakers — all unique. | Test-verified |
| EC12 | gofakeit upgrade churn (post-pin) | Next major removes `Sentence`/`Paragraph` args or signatures. The pin isolates; upgrade becomes a tracked story. | Pin (D16) |
| EC13 | **Undefined behavior preserved:** fallback-on-template-error | Existing contract: any parse/exec error → original template string returned (unrendered). New funcs never introduce errors; preserved. | Assert in tests |

---

## §7 — Test plan mapping

### §7.1 `src/mock/jsonpath_test.go` (new, ~30 table-driven tests)

Style: `package mock` (uses unexported `tokenizeJSONPath`/`evalJSONPath`); table-driven on a fixture body
`{"user":{"name":"Ada","roles":["admin","dev"]},"amount":42,"ok":true,"tags":null}` (story's AC fixture).

| AC (TPL-002) | Concrete test func | Plan |
|---|---|---|
| `$.user.name` → `"Ada"` | `TestEvalJSONPath_NestedKey` | table row |
| `$.roles[1]` → `"dev"` | `TestEvalJSONPath_ArrayIndex` | table row |
| `$.amount` → `"42"` (unquoted number) | `TestEvalJSONPath_NumberUnquoted` | asserts exact `"42"` (FormatFloat `g`) |
| `$.ok` → `"true"` | `TestEvalJSONPath_BoolUnquoted` | table row |
| `$.tags` → `"null"` | `TestEvalJSONPath_NullValue` | table row |
| `$.roles` → valid JSON array string | `TestEvalJSONPath_ObjectArrayMarshal` | `json.Valid` assert on output + exact string |
| `$` → whole body | `TestEvalJSONPath_Root` | root object marshaled; plus scalar-root (`42`) and string-root variants |
| non-JSON body → `""` | `TestEvalJSONPath_NonJSONBody` | `root = nil` (mimics `parsedBody == nil`) + `$` and `$.x` rows |
| missing path / OOB / type mismatch → `""` | `TestEvalJSONPath_MissingAndOOB` | `$.user.missing`, `$.roles[5]`, `$.name.deep`, `$.roles.name` (string-as-index) |
| tokenize grammar | `TestTokenizeJSONPath_Valid` / `TestTokenizeJSONPath_Invalid` | valid: `$`, `$.a.b`, `$.a[0]`, `$[0]`, `$.a[0].b`, `$.0`; invalid: `""`, `a`, `.a`, `$a`, `$.`, `$.a[]`, `$.a[-1]`, `$.a[x]`, `$.a b`, `$..a`, `$.a[*]`, `$.a[?(@.b)]` |
| 1e21 → `"1e+21"` (FormatFloat `g`) | `TestEvalJSONPath_LargeNumberExponent` | D14 pin |
| 42.5 → `"42.5"` | `TestEvalJSONPath_FloatPrecision` | D14 pin |
| deep nesting `$.items[1].tags[0]` | `TestEvalJSONPath_DeepNesting` | story syntax table row |
| parse-once (observable only via closure) | `TestJSONPathClosure_ReadsCachedParsedBody` | build one `TemplateData` via `BuildTemplateData`, call closure twice → same output; assert by counting nothing (structural) |
| closure swallows errors → `""` | `TestJSONPathClosure_InvalidPathEmpty` | `makeJSONPathFunc` + malformed path → `""`, no panic |
| closure nil body → `""` | `TestJSONPathClosure_NilParsedBody` | `&TemplateData{}` → `""` |

### §7.2 `src/mock/templating_test.go` (add ~12, **do not touch the existing 7**)

Existing tests that MUST stay untouched (backward-compat assertions):
`TestRandomInt_MaxLessThanMin`, `TestRandomInt_NormalRange`, `TestExecuteBodyTemplate_InvalidSyntax`,
`TestExecuteBodyTemplate_UnknownField`, `TestExecuteBodyTemplate_BodyWithTemplateLikeChars`,
`TestBuildTemplateData_NilBody`, `TestBuildTemplateData_NonEmptyBody`. `TestBuildTemplateData_*` need one
**additive** check each (new assert) only if the maintainer prefers; contracts are: nil/empty body →
`parsedBody == nil`, valid body → non-nil. Adding to an existing test is allowed if the original assertions
are unchanged.

| AC (TPL-001) | Concrete test func |
|---|---|
| every faker func present + plausible non-empty | `TestFakerFuncs_PresentAndPlausible` (loop over all 27 FuncMap keys; render `{{Name}}`-style through `ExecuteBodyTemplate`; assert non-empty; for Email/Phone/IP/HexColor assert shape via light regex/prefix; for Bool assert `true`/`false`) |
| faker funcs compose with existing funcs + fields | `TestTemplate_ComposeFakerWithFieldsAndFuncs` (`{{.RequestID}}-{{Email}}-{{Query "q"}}`) |
| faker + `{{if}}` control flow | `TestTemplate_FakerWithIf` |
| `{{UUID}}` vs `{{.UUID}}` independence | `TestUUIDFuncFieldIndependence` (assert both callable in one template; `{{UUID}}` ≠ `{{.UUID}}` values) |
| existing `{{RandomInt}}`/`{{RandomHex}}` unchanged | `TestExistingFuncs_ByteIdentical` renders `{{RandomInt 1 1}}.` → `"1"` style exact-match assertions (regression guard) |
| JSONPath through `ExecuteBodyTemplate` | `TestTemplate_JSONPathBound` (body valid JSON; `{{JSONPath "$.user.name"}}`) |
| `{{.Body}}` still full raw body | `TestTemplate_BodyFieldUnchanged` |
| BuildTemplateData parse caching | `TestBuildTemplateData_ParsedBodyNilEmpty` + `TestBuildTemplateData_ParsedBodyValid` (additive to existing files, or new funcs) |
| header template with faker | `TestHeaderTemplate_FakerFunc` (via `ExecuteHeaderTemplate`) |
| fallback preserved (bad func name) | `TestExecuteBodyTemplate_UnknownFunc_FallsBack` (new func name `{{Nope}}` → original string) |
| all 27 keys exactly as curated | `TestFuncMapKeys_ExactCuratedSet` (no `RandomString`/`Number`/`Int` leakage) |

### §7.3 `src/mock/handler_test.go` (add ~2, keep all existing)

Existing template-related handler tests that MUST stay green: `TestHandler_TemplateBody`,
`TestHandler_TemplateHeader_HasHeaderAndQueryFuncs`, `TestHandler_TemplateTimestampIsPreDelay`,
`TestHandler_EchoOverridesTemplate`, `TestHandler_ResponseHeadersAndBody_Template`, plus the whole race/
integration set — none change.

| Story | Concrete test func |
|---|---|
| full round-trip: faker + JSONPath stub responds 200, structured body, echo of request field | `TestHandler_TemplateFakerJSONPath_RoundTrip` |
| non-JSON request body + JSONPath → `""`, response still 200 | `TestHandler_NonJSONBody_JSONPathEmpty` |

### §7.4 Test command gates

- Phase B: `go test -v -race ./src/mock -run 'Test(JSONPath|Tokenize|Eval)'`
- Phase C: `go test -v -race ./src/...` (full suite), `go vet ./src/...`, `gofmt -l src`
- Phase D: same full gates (doc-only otherwise)

---

## §8 — File-by-file change list + implementation order (phases A→D, each independently green)

### Phase A — Dependency (green: build + vet)

| File | Change |
|---|---|
| `src/go.mod` | add `require github.com/brianvoe/gofakeit/v7 v7.15.0` |
| `src/go.sum` | generated |

Commands: `cd src && go get github.com/brianvoe/gofakeit/v7@v7.15.0 && go mod tidy`
Gate: `go build -o simuhook ./src && go vet ./src/...` (no source changes yet; unused import NOT added yet in
this phase — the wire is verified by `go list -m`).

### Phase B — JSONPath (green standalone)

| File | Change |
|---|---|
| `src/mock/jsonpath.go` (**new**) | `tokenizeJSONPath`, `evalJSONPath`, `renderJSONValue`, `makeJSONPathFunc`, `isKeyChar` — §3 contract |
| `src/mock/jsonpath_test.go` (**new**) | §7.1 table tests |

Gate: `go test -v -race ./src/mock -run JSONPath -count=1` (+ `go vet`).

### Phase C — templating.go FuncMap/TemplateData + tests (green with A+B)

| File | Change |
|---|---|
| `src/mock/templating.go` | import `gofakeit`; `fakerFuncMap` package var; `buildTemplateFuncMap`; `parsedBody any` field; `BuildTemplateData` calls `parseJSONBody`; `ExecuteBodyTemplate` switches to `buildTemplateFuncMap` |
| `src/mock/templating_test.go` | §7.2 additions (existing 7 untouched) |
| `src/mock/handler_test.go` | §7.3 additions (existing untouched) |

Gate: full `go test -v -race ./src/...` (expect ~380+ tests, zero failures), `go vet`, `gofmt -l`.

### Phase D — Docs + sample stub

| File | Change |
|---|---|
| `stubs/README.md` | Template Functions table expanded (27 faker funcs + `JSONPath`), note DEPRECATED Sentence/Paragraph args, ULID→ID note, Price fixed-range note, `$` root semantics, JSONPath syntax table (copy of story's) |
| `.ai/requirements/contracts.md` | **Add** a "Template Functions" contract section mirroring the expanded table (currently contracts.md has NO template table — this is a new section; verified today) |
| `.ai/architecture.md` | ADR entry: TPL-001/002 — gofakeit dep (D2/D16), JSONPath design, ULID substitution (D9), deprecated funcs accepted (D10), Price wrapper (D11), `$` semantics (D12/D13), number format (D14), whitespace strictness (D15). Follow the existing "Key Decisions" ADR format |
| `stubs/template-showcase.yaml` (**new**) | demo stub — one `template: true` endpoint with faker funcs (Name, Email, City, ULID, Price, UUID) + `{{JSONPath "$.customer.id"}}` echo; referenced from README |

Gate: `go test -v -race ./src/...` (unchanged, doc-only) + manual `go run .` smoke with the showcase stub.

---

## §9 — Stakeholder decisions needed (numbered D9–D16, matching BA review)

### D9 — `ULID` substitution
- **Problem:** gofakeit v7.15.0 has **no `ULID` func** (verified: 0 hits in source). Story's `gofakeit.ULID()`
  cannot compile.
- **Options:** **(a)** FuncMap `ULID` → `gofakeit.ID()` (20-char URL-safe lowercase, e.g.
  `"pfsfktb87rcmj6bqha2fz9"`); **(b)** drop `ULID` from curated set; **(c)** add a separate ULID dep (build-your-own),
  violating single-dep decision.
- **Recommendation: (a)** — zero new deps, name stays `ULID` in templates, doc says "ULID-style identifier".
- **Implication:** output format is 20 chars, not 26; anyone relying on ULID sortability/format must know.
  Accepted wording goes into README/contracts.

### D10 — Sentence/Paragraph deprecated, accepted as-is
- **Problem:** both are `Deprecated:` upstream; `Sentence(wordCount ...int)` / `Paragraph(paragraphCount ...any)`
  swallow args.
- **Options:** **(a)** wire as-is, document; **(b)** wrap with sensible arg-based behavior of our own; **(c)** drop.
- **Recommendation: (a)** — a wrapper that honors word count duplicates upstream work for no testable gain;
  dropping contradicts the story's curated table.
- **Implication:** `{{Sentence 3}}` output length is whatever random text is — cannot be tuned via args.
  Pin (D16) protects against upstream removal until a deliberate upgrade.

### D11 — `Price` wrapper shape
- **Problem:** gofakeit's `Price(min, max float64) float64` is **2-arg only**; story calls `Price()` in templates.
- **Options:** **(a)** zero-arg FuncMap closure `func() string` with fixed range `(0.01, 999.99)` formatted
  `"%.2f"` (→ `"42.99"`); **(b)** two-arg FuncMap `Price(min, max float64) string` (templates must pass a range);
  **(c)** return raw `float64` instead of string.
- **Recommendation: (a)** — story text says "Price uses the fixed range (0.01, 999.99)" and its example is a
  string `"42.99"`. Zero-arg keeps the template surface uniform with the other fakers.
- **Implication:** authors cannot vary the range; documented (story explicitly relegates other ranges to
  `{{RandomInt}}` + formatting). If the two-arg form is ever wanted it's a one-line change (option b).

### D12 — `$` on a **non-JSON** body: `""` vs `"null"`
- **Problem:** story says non-JSON body + any JSONPath → `""`; but `$` on the root of a *present-but-nil* parse
  could be argued to render `"null"`.
- **Options:** **(a)** non-JSON/empty body ⇒ `parsedBody == nil` ⇒ `""` for **every** path including `$`;
  **(b)** `$` renders `"null"` when parse failed, `""` only for mid-path misses.
- **Recommendation: (a)** — cleaner invariant ("no JSON, no extraction"); matches the story's literal AC
  ("non-JSON → empty string").
- **Implication:** a template echoing `{{JSONPath "$"}}` on a text/plain request body renders empty, never
  `"null"`. A body that parses to the JSON literal `null` DOES render `"null"` (valid JSON value).

### D13 — `$` and nested objects/arrays render via **canonical re-marshal**
- **Problem:** objects/arrays are rendered as JSON strings; `json.Marshal` re-encodes (key order
  **alphabetical** for maps, compacted whitespace) — never the raw slice of the request body.
- **Options:** **(a)** canonical re-marshal (`json.Marshal`); **(b)** byte-exact slice of the raw body.
- **Recommendation: (a)** — byte-exactness requires indexes into the raw bytes (invasive, brittle); canonical
  JSON is deterministic and always valid. Documented "not byte-exact".
- **Implication:** key order may differ from the request; numbers beyond float64 precision round. Acceptable
  for a mock tool; tests assert `json.Valid` + exact known output for fixtures.

### D14 — Numbers render with `strconv.FormatFloat(v, 'g', -1, 64)`
- **Problem:** story says "numbers unquoted (`fmt.Sprintf`)" — exact format matters: `42` → `"42"`,
  `42.5` → `"42.5"`, `1e21` → `"1e+21"`, `0.1+0.2` → `"0.30000000000000004"` (float64 reality).
- **Options:** **(a)** `'g', -1` shortest round-trip (recommended); **(b)** `'f', -1` (never exponent — gives a
  22-digit monster for `1e21`); **(c)** custom.
- **Recommendation: (a).** Matches story examples (`42`, `42.5`) and stays honest about float64.
- **Implication:** very large/small numbers render in scientific notation (`1e+21`); tests pin both plain and
  exponent cases.

### D15 — Zero whitespace trimming in path keys
- **Problem:** JSON keys may contain spaces; tokenizer is strict.
- **Options:** **(a)** reject any space (and any unlisted char) at tokenize → error → `""`; **(b)** trim
  whitespace leniently.
- **Recommendation: (a)** — strict grammar (story: "strict: no leading dots, no spaces"); leniency hides
  typos like `$.user name`.
- **Implication:** keys containing spaces/dots/unicode are unreachable via JSONPath (dot conflict;
  documented). Curly-brace or bracket-key quoting (`$['key with space']`) is **out of scope**.

### D16 — Pin `github.com/brianvoe/gofakeit/v7` at `v7.15.0`
- **Problem:** upstream moves; `Sentence`/`Paragraph` deprecated for removal.
- **Options:** **(a)** pin exact `v7.15.0` (recommended); **(b)** float (`v7`); **(c)** `latest`.
- **Recommendation: (a)** — verified against this exact tag; go.sum committed. Upgrades become deliberate,
  reviewed stories.
- **Implication:** no automatic patch pickup; upgrade story owns the diff, vet of any signature changes, and
  this ADR's update.

---

## Appendix A — Grounding artifacts verified during this design

| Artifact | Role |
|---|---|
| `.ai/requirements/19-richer-templates.md` — status `ready`, D1–D3, story ACs, files, test plan | Source of truth |
| `src/mock/templating.go` — 96 lines; current `TemplateData`, funcMap, `BuildTemplateData` | Change site (§2/§4) |
| `src/mock/subset.go` — `parseJSONBody` signature (lines 26–35) to reuse verbatim | §4 mandated reuse |
| `src/mock/handler.go` — lines 101–140: `BuildTemplateData` call site, delay order, header/body exec | Concurrency + timing notes (§4/§6) |
| `stubs/README.md` lines 151–171 — current Template Variables + Functions tables | Doc expansion site (§8 Phase D) |
| `.ai/requirements/contracts.md` — verified it has **no** template-functions table today | New section needed (§8 Phase D) |
| `github.com/brianvoe/gofakeit@v7.15.0` full source (fetched 2026-08-16) | All §0 signatures verified line-by-line |