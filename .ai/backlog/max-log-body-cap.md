# `--max-log-body` capture cap

**Title:** Add a `--max-log-body` capture cap knob for response/request bodies.

**Status:** `backlog` (not started). **Priority:** `[Could]`. **Origin:** UXR-013 / WS-018 design — Risks & edges, "Ring memory (bodies are in the ring)".

## Description

`StatsTracker` holds `--recent` (default 50) `LogEntry` values, each of which now carries both `Body` and `ResponseBody` after UXR-013. Echo-mode bodies are client-controlled and share one backing array (good), and non-echo bodies are operator-authored YAML (bounded). The residual exposure is a `template`-mode stub that interpolates a huge request body into memory. Today the cap is display-only; this is a **capture** cap to bound memory and disk if profiling ever shows pressure.

## Architecture / approach

- CLI flag `--max-log-body` (env `LOG_MAX_BODY`), default `0` = unlimited (preserve current behavior unless opted in).
- Enforced at capture time in `mock/handler.go` when building `LogEntry.Body` / `responseBody` before the observer runs.
- Truncate on a rune boundary; store just a truncated string. A cap on `response_body` and `body` bound the ring and the per-`echo` webhook log file (which currently grows ~2x per line after UXR-013).
- Pure additive knob — does not change the default wire/log format.

## Files to create / edit

- `src/main.go` — new `--max-log-body` flag + env binding.
- `src/mock/handler.go` — capture-time truncation (guarded by the config value), behind `HandlerConfig`.
- `src/mock/logger.go` — no change (truncation happens upstream).
- `src/dashboard` — optional display note of truncation; display cap already exists.
- Tests in `handler_test.go` / `main_test.go`.

## Steps

- [ ] Add config field + flag + env.
- [ ] Thread through `HandlerConfig`.
- [ ] Capture-time rune-safe truncate.
- [ ] Tests (cap applied, rune boundary, cap zero = unlimited).
- [ ] Update `contracts.md` (log schema note if stored truncated).

## Notes

Not on the WS-018 critical path. Enable only if profiling shows ring/log pressure.