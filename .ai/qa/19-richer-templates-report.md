# QA Verification Report — Phase 19: Richer Templates (TPL-001 / TPL-002)

**Date:** 2026-08-16
**Verified by:** BA (acting as QA orchestrator — the QA subagent channel failed twice mid-run; verification completed directly using the automated test suite + live server smoke tests)
**Stories:** TPL-001 (`[Should]`), TPL-002 (`[Should]`)
**Source ACs:** `.ai/requirements/19-richer-templates.md`
**Design:** `.ai/designs/19-richer-templates-design.md`

## Summary

**ALL ACCEPTANCE CRITERIA PASS.** Full suite **448 tests** (dashboard + mock) green under `-race`; `go vet` clean; `gofmt` clean; build clean; live round-trip verified on real HTTP.

## AC Verification Matrix

| AC | Evidence | Pass/Fail |
|----|----------|-----------|
| **TPL-001 AC1** — faker funcs render plausible non-empty values | `TestFakerFuncs_PresentAndPlausible` (loop over all 27 FuncMap keys; Email contains `@`, Phone has digits, IP has `.`, HexColor has `#`, Price matches `\d+\.\d{2}`, ULID is 20 chars, Bool is true/false) + live smoke: `maralawson@rath.com`, `6375996334596700`, `#edb1cc` | ✅ PASS |
| **TPL-001 AC2** — different requests → different values | Live: 3 sequential requests → `Donald Conroy` / `Hilda Gerlach` / `Cordie Schmitt`, distinct emails, distinct 20-char ULIDs (`yysmqhf8abp5at27gy97` / `vxbzeebh5sjaegcext8s` / `n5g6w7sq9r7y49vrygfc`) | ✅ PASS |
| **TPL-001 AC3** — composable with fields/funcs/control flow | `TestTemplate_ComposeFakerWithFieldsAndFuncs` (`{{.RequestID}}|{{Email}}|{{Query "q"}}`), `TestTemplate_FakerWithIf` (`{{if .RequestID}}{{Name}}{{end}}` + else-branch), `TestHandler_TemplateFakerJSONPath_RoundTrip` mixed template | ✅ PASS |
| **TPL-001 AC4** — `{{RandomInt}}`/`{{RandomHex}}` byte-identical | `TestExistingFuncs_ByteIdentical` (`{{RandomInt 1 1}}` → `1`; `{{RandomHex 4}}` → length 4); all existing templating tests untouched and green | ✅ PASS |
| **TPL-001 AC5** — `{{UUID}}` (func) vs `{{.UUID}}` (field) independent | `TestUUIDFuncFieldIndependence` — both callable in one template, values differ; `TestFakerFuncs_PresentAndPlausible` renders both | ✅ PASS |
| **TPL-002 AC6** — JSONPath return semantics on fixture body | `TestEvalJSONPath_NestedKey` (`Ada`), `TestEvalJSONPath_ArrayIndex` (`dev`), `TestEvalJSONPath_NumberUnquoted` (`42`), `TestEvalJSONPath_BoolUnquoted` (`true`), `TestEvalJSONPath_NullValue` (`null`), `TestEvalJSONPath_ObjectArrayMarshal` (valid JSON array via `json.Valid`), `TestEvalJSONPath_Root` (whole body, `json.Valid`) | ✅ PASS |
| **TPL-002 AC7** — non-JSON/empty body → `""`, no panic, no fallback | `TestEvalJSONPath_NonJSONBody`, `TestJSONPathClosure_NilParsedBody`, `TestExecuteBodyTemplate_JSONPathNonJSONFallsBackEmpty` (renders `id=`), **live**: `curl --data 'plain text'` → `"id":""`, `"echo_name":""`, response 200, faker fields still non-empty | ✅ PASS |
| **TPL-002 AC8** — missing path / OOB → `""` | `TestEvalJSONPath_MissingAndOOB` (`$.user.missing`, `$.user.roles[5]`, `$.name.deep`, traversal-into-null, path-past-string) | ✅ PASS |
| **TPL-002 AC9** — parse-once cache; `{{.Body}}` unchanged | `TestJSONPathClosure_ReadsCachedParsedBody` (closure bound to `parsedBody`, set once in `BuildTemplateData`), `TestBuildTemplateData_ParsedBodyNilEmpty`/`_Valid`, `TestTemplate_BodyFieldUnchanged` (raw body preserved) | ✅ PASS |
| **Regression AC10** — full suite unchanged | `go test -race ./... -count=1` → 448 PASS (baseline 366 + 82 new across mock), `go vet ./...` clean, `gofmt -l` clean | ✅ PASS |

## Live Server Verification (real HTTP)

Server: `/tmp/simuhook-qa --addr :18082 --stubs stubs --tui off` with `stubs/template-showcase.yaml`.

**Matched round-trip (AC6/AC1):**
```
POST /template-showcase {"customer":{"id":"cust-42","name":"Ada Lovelace"}}
→ {"request_id":"432cd430-…","event":"customer.created","customer":{"id":"cust-42",
   "echo_name":"Ada Lovelace","generated":{"name":"Rita Jensen","email":"maralawson@rath.com",
   "city":"Portland","country":"Cocos (Keeling) Islands"},"payment":{"card":"6375996334596700",
   "amount":"523.09","currency":"USD"},"meta":{"ulid":"zguwgemk4pdz9rzkpbhg","uuid":"288607b2-…",
   "color":"#edb1cc","active":"false"}}}
```
JSONPath extracted `cust-42` and `Ada Lovelace` from the request body; all faker fields plausible.

**Non-JSON body (AC7, D12):**
```
POST /template-showcase --data 'plain text'
→ HTTP 200, customer.id = '', echo_name = '', generated.name/email non-empty
```
JSONPath renders empty on a non-JSON body; no panic; no whole-template fallback.

**Determinism (AC2):** three consecutive requests → three distinct names, emails, ULIDs.

## Structure Checks

- 27-entry `fakerFuncMap` exact-count asserted (`TestFuncMapKeys_ExactCuratedSet`); anti-leak: `RandomString`/`Number`/`Int` absent.
- `go.mod`/`go.sum`: `github.com/brianvoe/gofakeit/v7 v7.15.0` — **direct** dependency (no `// indirect`).
- JSONPath strict grammar: 17 invalid-path cases rejected at tokenize (`TestTokenizeJSONPath_Invalid`), incl. `$.a b`, `$..a`, `$.a[*]`, `$.a[?(@.b)]`, negative index.
- Docs updated: `stubs/README.md` (Template Functions + JSONPath tables, deprecation/ULID/Price notes), `.ai/requirements/contracts.md` (new Template Functions contract section), `.ai/architecture.md` (ADR TPL-001/002), `stubs/template-showcase.yaml` demo stub.

## Defects / Gaps / Deviations

**None.** No open defects, no acceptance-criteria gaps, no deviations from the approved design (D9–D16 all implemented exactly as specified).

Note: this report was produced directly by the BA because the QA subagent delivery channel failed twice (empty results mid-verification). All checks listed above were executed live against the actual build and test suite.