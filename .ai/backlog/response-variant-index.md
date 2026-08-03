# `response_variant` index in the log line

**Title:** Log which weighted response variant was chosen.

**Status:** `backlog` (not started). **Priority:** `[Could]`. **Origin:** UXR-013 / WS-018 design — "Not captured (accepted, backlog candidate)".

## Description

Multiple-response pillars with weights use round-robin / weighted selection (`PickResponse`). A request observer currently knows only the *result* (`ResponseBody`, `ResponseStatus`, `ResponseHeaders`) but not *which* variant of the stub matched. When a webhook has several weighted variants, operators cannot tell from the log which variant a given request hit. Add a `response_variant` index to the log entry.

## Architecture / approach

- Add `ResponseVariant *int` (pointer or `-1` = not applicable) to `mock.LogEntry` so single-variant responses can mark "no variant" distinctly.
- Populate in `mock/handler.go` where `PickResponse` returns the selected `Response` — record its index in the stub's variant list.
- JSON tag `response_variant`, additive / backward-compatible.
- Dashboard inspector (UXR-013) can optionally surface it in the RESPONSE section head.

## Files to create / edit

- `src/mock/logger.go` — `LogEntry.ResponseVariant`.
- `src/mock/handler.go` — capture the selected variant index.
- `src/mock/registry.go` — confirm `PickResponse` exposes/returns the index.
- `.ai/requirements/contracts.md` — log schema note.
- Tests in `handler_test.go` / `registry_test.go`.

## Steps

- [ ] Confirm `PickResponse` can return/derive the selected index.
- [ ] Add `LogEntry.ResponseVariant` (+ JSON tag).
- [ ] Populate in handler; render in inspector (optional).
- [ ] Update `contracts.md`.
- [ ] Tests (single variant → nil/unset; multi-variant → correct index; round-robin over two requests show 0 then 1).

## Notes

Small, self-contained additive. Deferred so UXR-013 can land on schedule.