# QA Report — PROXY-001 / 002 / 003 / 004

**Repo:** `/home/abojaber/project_public/simuhook` @ `aeff426` (proxy commits `e1850dd`, `82360b3`, `be47987`)
**Verifier:** QA (subagent, `opencode/deepseek-v4.1-flash-free`) · **Date:** 2026-10-19
**Method:** requirements read → full automated suite → live end-to-end smoke matrix → scripted pty TUI probe. **No source files modified.**

---

## 0. Environment note

`go.mod` pins `go 1.26.4`; no Go toolchain was preinstalled on the QA machine (apt candidate 1.22, incompatible). QA downloaded `go1.26.9` to `/tmp/opencode/go` and used it for all commands. All results are from that toolchain.

## 1. Automated suite (exact)

| Check | Command | Result |
|---|---|---|
| Build | `go build ./...` | **OK** |
| Binary | `go build -o /tmp/simuhook-qa .` | **OK** |
| Vet | `go vet ./...` | **clean** |
| Format | `gofmt -l .` | **empty** |
| Race suite | `go test -race -count=1 ./...` | `ok simuhook/dashboard 18.1s`, `ok simuhook/mock 2.6s` |

**Totals:** 514 top-level test functions (mock 232, dashboard 282) → 562 `=== RUN` entries incl. 48 subtests; **562 PASS / 0 FAIL / 0 SKIP**. (Brief expected ~448 — stale expectation, actual is higher.)

Named-test presence confirmed: all 5 `TestHandler_ProxyLogFidelity_*`, all 7 `TestProxyView_*`/`TestProxyEntries_*` (+ `TestProxyView_EmptyRawLines`), 24 `RawTee*`/`RawLog*`/`ParseRequestLinePath`/`HexEscape` tests in `src/mock/rawtee_test.go`. Focused proxy run: 77 PASS / 0 FAIL.

## 2. Live end-to-end smoke

Upstream: python3 `127.0.0.1:9191` + raw-socket `:9194` (poisoned hop-by-hop/duplicate response headers). Simuhook: `--proxy /api=http://127.0.0.1:9191 --proxy /api/nested=http://127.0.0.1:9193` (dead), `--stubs ../stubs` + `/tmp/qa-stubs`, `--tui off`. All processes stopped afterwards.

Evidence highlights:
- `POST /api/orders?foo=bar&baz=1` (`X-Debug: a`+`b`, invalid-JSON body, `TE`/`Keep-Alive`) → 201; upstream got path+query verbatim, `X-Debug: ["a","b"]`, `X-Forwarded-For: 127.0.0.1`, hop-by-hop absent.
- `/api/nested/x` → 502 from dead port (longest prefix picked `:9193` over `/api`) — prefix rule proven.
- `/nope` → 404 standard; without `--proxy` → 404 and **no `_raw.log`**.
- Hop-by-hop probe (7 headers + prior `XFF: 10.0.0.9`): log `raw_headers` = all inbound values; `upstream_request.headers` = only `Content-Length`, `X-Forwarded-For: ["10.0.0.9, 127.0.0.1"]`, `X-Keep-Me` → 7/7 stripped, prior XFF preserved.
- Malformed raw request (`Accept; json`, no colon): client got 400; `_raw.log` recorded `>>>` request (incl. bad line) and `<<< 400`; `_proxy.log` line count unchanged → no JSON entry.
- 100 KB body: `_raw.log` captured 65,559 B then 0 response bytes; upstream received full 100,010 B; client 201 — capture cap never affects served bytes.
- `>8 KB` request line: no capture. HTTPS destination: TLS round trip OK, `https://example.com/ext/` (no prefix stripping).
- TUI (scripted pty): `PROXY REQUESTS` header, structured line `07:45:13  POST  /api/orders → 201  _proxy  (1.3ms)`, `Raw Bytes` with `>>>`/`<<<`, footer `1 requests, 15 raw lines`; `1` returns to main view (`LIVE REQUEST STREAM` / `REGISTERED ENDPOINTS`, `/health`).

## 3. Per-story AC verdicts

### PROXY-001 — Transparent forwarding fallback → **NOT VERIFIED** (1 AC fails)

| AC | Verdict |
|---|---|
| 1. Forwarded to destination, path+query preserved | PASS |
| 2. Upstream response returned to caller **unchanged** | **FAIL** — repeated response headers collapsed to first value (Bug 1) |
| 3. `webhook "_proxy"` + `matched true` | PASS |
| 4. `proxy_destination` recorded | PASS |
| Longest prefix wins | PASS |
| No rule + no stub → standard 404 | PASS |
| Stub wins over matching proxy rule | PASS |

### PROXY-002 — Full-fidelity logging → **VERIFIED** (8/8 + failure-path ACs all PASS)

Multi-value `raw_headers`, legacy `headers` first-value untouched, invalid-JSON body verbatim, `upstream_request` (full URL, hop-by-hop stripped, XFF injected/preserved), `upstream_response` exact with multi-value arrays, failure path (upstream_request present / upstream_response omitted / 502 + `proxy error: `), stub-served entries free of proxy fields.

### PROXY-003 — Raw listener-level tee → **VERIFIED** (all ACs PASS live)

Well-formed capture, malformed-request 400-capture with no JSON entry, non-proxy path untouched, no-`--proxy` identity (no wrap, no `_raw.log`), >8 KB no-capture, 64 KB cap stops teeing while bytes still flow.

### PROXY-004 — Proxy TUI view → **VERIFIED** (unit + scripted pty)

Structured entries filtered to `_proxy`, raw bytes section, stub requests absent from proxy view but present on main dashboard, live-follow auto-scroll, placeholder text, keys `2`/`1`/`esc`/`q`/`L`/`?`.

## 4. Defects

**Bug 1 — Repeated upstream response headers collapsed to first value in the caller-visible response. Severity: High.**
Repro: upstream returns two `Set-Cookie` (+ any duplicate header) → `curl -D -` through the proxy shows only the first; direct-to-upstream shows both.
Root cause: `src/mock/handler.go:127` builds the replay from `firstValues(resp.Header)` and `:139-141` replays via `w.Header().Set(k, v)`. The `firstValues` shape is correct for the *legacy log field* (D3) but must not drive the response written to the caller — use `resp.Header` with `Add`/direct multi-value assignment.
Impact: realistic upstreams (multiple `Set-Cookie`, `Vary`, `WWW-Authenticate`) look broken through the proxy — the exact debugging scenario the story targets. JSON log does record both values, so it's diagnosable after the fact. Existing tests miss it because they assert *log* fidelity, not the caller-visible header set.
→ Fails PROXY-001 AC2. Blocks sign-off.

**Bug 2 — Hop-by-hop *response* headers forwarded to the caller. Severity: Medium.**
Upstream's `Connection`/`Keep-Alive`/`Trailer`/`Upgrade` are passed straight through; the request leg strips correctly (RFC 7230 §6.1 response side unimplemented). Same code block — cheap fix. Spec/hygiene gap, not an explicit AC.

**Bug 3 — Raw-tee 64 KB budget doesn't debit request-line bytes. Severity: Low.**
Request line written via `writeSink` without decrementing `budget` → up to ~23 B over the documented shared cap (observed 65,559 B; unit test tolerates +4 KB). Capture-cap semantics otherwise correct; never affects served bytes.

**Bug 4 — Proxy view body can be blank up to 1 s after pressing `2`. Severity: Low (cosmetic).**
`buildProxyContent()` runs only from the 1 s `tickMsg`; the `2` key only sets `viewMode`. Code-derived, not empirically confirmed (pty frame deltas couldn't isolate it).

## 5. Coverage gaps (non-blocking)

- PROXY-004 AC2 fields (method/status/latency/`_proxy` label) not asserted in CI; auto-scroll asserted only as a flag, not `GotoBottom`/offset.
- No end-to-end TUI test (live check needed an external pty harness).
- No regression test for caller-visible multi-value response headers (Bug 1) or response-side hop-by-hop (Bug 2).
- No chunked/streaming upstream test; no upstream timeout/DNS/TLS-failure test (only connection-refused).
- `TestRawTeeListener_NoProxy_NoWrap` wraps with nil rules rather than exercising the `main.go` guard (guard verified only by manual run).
- No proxy concurrency/load test; `-race` coverage only at tee/ring unit level.

## 6. Deviations from `20-proxy-forwarding.md`

1. **PROXY-001 AC2 not met** for repeated response headers (Bug 1) — the only AC-level deviation.
2. Request-side hop-by-hop stripping implemented, response-side not (Bug 2) — spec gap, not a contract break.
3. Doc drift: requirements + `architecture.md` say PROXY-003 ships "12 tests"; `rawtee_test.go` actually has 24 (more coverage than documented).
4. Everything else matches the spec exactly (field names/types, `omitempty`, markers, 8 KB budget, 64 KB cap, lazy `_raw.log`, auto-on wiring, longest-prefix, D1 verbatim forwarding, Q1 over-capture accepted).

## 7. Sign-off

| Story | Verdict |
|---|---|
| PROXY-001 | **NOT VERIFIED** — Bug 1 (High) fails AC2 |
| PROXY-002 | **VERIFIED** |
| PROXY-003 | **VERIFIED** |
| PROXY-004 | **VERIFIED** (unit-level in CI; interactive TUI not exercised, per instruction) |

**Recommendation:** fix Bug 1 (`w.Header().Add` / direct multi-value write from `resp.Header`) + add a caller-visible multi-value regression test before signing off PROXY-001; Bug 2 is a cheap same-block follow-up. Re-test: `curl -D -` against a duplicate-`Set-Cookie` upstream + `go test -race ./...`.
