# Header-count cap for UXR-008 detail expansion

**Title:** Cap header count in the existing UXR-008 `enter` detail expansion.

**Status:** `backlog` (not started). **Priority:** `[Could]`. **Origin:** UXR-013 / WS-018 design — Risks & edges, "Header-count amplification".

## Description

The UXR-013 inspector caps request headers at `maxInspectHeaders` (30) per section to stop attacker-controlled header maps (a client can send 500 headers) from inflating the inline feed block and `feedLineToEntry` without bound. The pre-existing UXR-008 `enter` expansion (`detailLines`) has the **same** exposure — request headers are rendered unbounded — but was intentionally left unchanged in WS-018 to honor AC5 (UXR-008 must stay byte-identical while not inspecting). This item brings the cap to `detailLines` too.

## Architecture / approach

- Apply the same 30-header cap + a `… N more` marker to request header/query rendering in `detailLines`.
- Reuse the same constant (`maxInspectHeaders`) or a shared cap.
- Must be done as a standalone change with its own UXR-008 regression coverage — cannot ride along in WS-018.

## Files to create / edit

- `src/dashboard/render.go` — `detailLines` header/query cap.
- Tests in `dashboard_test.go`.

## Steps

- [ ] Extract shared `maxInspectHeaders` constant.
- [ ] Apply cap in `detailLines` (request headers + query).
- [ ] Tests (500 headers → 30 + marker; byte-identical output below the cap).
- [ ] Note in architecture ADR / done.md as a follow-up.

## Notes

Explicitly deferred out of WS-018 to keep `enter` expansion untouched during inspector delivery.