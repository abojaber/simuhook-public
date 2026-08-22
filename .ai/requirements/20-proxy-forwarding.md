# Phase 20 — Transparent HTTP Forwarding (Proxy)

> Simuhook can act as a transparent man-in-the-middle for debugging
> integrations with external services: when no stub matches a request and a
> `--proxy` rule's path prefix matches, the request is forwarded as-is to the
> destination URL, both legs of the traffic are logged with full fidelity, and
> the upstream response is returned unchanged to the caller.

**Status:** `in-progress` | **Owner:** BA | **Date:** 2026-08-19 | **Goal:** debug real integrations without a real service

**Stakeholder decisions (signed off 2026-08-19):**
1. **D1 = A — path preserved as-is.** The proxy treats the destination as a base URL (scheme + host) and carries the caller's full original path + query over verbatim. No prefix stripping, no rewrite.
2. **D2 = A — full-fidelity logging.** Proxy traffic must be logged "exactly as-is" so a debugging partner can reproduce the failure: every incoming header value (multi-value included), the verbatim body (even invalid JSON), the exact outbound request actually issued upstream (after hop-by-hop stripping and `X-Forwarded-For` injection), and the full upstream response.
3. **D3 = B — additive fields, no breaking change.** New fidelity data lives in new `LogEntry` fields (`raw_headers`, `upstream_request`, `upstream_response`) with `map[string][]string` fidelity. The legacy `headers` / `response_headers` (first-value) fields and all TUI surfaces are untouched. `LogEntry.Headers` is NOT re-typed — that would ripple through matching, the TUI renderer, and every historical log consumer.
4. **D4 = A — failed round trips still log the attempt.** When the destination is unreachable, the log records `upstream_request` (the exact outbound request that would have been sent) so the debugging partner can see the attempted URL/body; `upstream_response` is omitted.

---

## PROXY-001 — Transparent forwarding fallback `[Should]`

**Status:** `ready`

**Story:**
> As a developer testing my integration against a mocked client, I want requests that match no stub to be forwarded to a real destination URL, so that I can see how my service behaves against a live third-party API — while still capturing all traffic for debugging.

**ACs:**
```
Given a handler configured with --proxy /api=https://api.example.com
When  a request arrives for /api/v1/orders (no stub matches)
Then  1. the request is forwarded to https://api.example.com/api/v1/orders
      2. the upstream response (status, headers, body) is returned to the caller unchanged
      3. the log entry has webhook "_proxy" and matched true
      4. the log entry records proxy_destination = https://api.example.com

Given two rules --proxy /api/stripe=https://api.stripe.com and
      --proxy /api=https://api.example.com
When  a request arrives for /api/stripe/v1/charges
Then  the longest prefix wins: the request goes to api.stripe.com

Given a request whose path matches no proxy rule and no stub
When  it is served
Then  the standard 404 {"error":"no matching endpoint"} is returned (proxy no-op)

Given a stub whose path matches a request
When  a proxy rule also matches the same prefix
Then  the stub wins (registry match precedes proxy fallback)
```

## PROXY-002 — Full-fidelity logging `[Should]`

**Status:** `ready`

**Story:**
> As a developer debugging a failed integration, I want the proxy to log the traffic "exactly as-is" — every header value (even multi-value or unusual ones) and the raw body (even invalid JSON) — plus the exact request/response exchanged between the proxy and the target endpoint, so that I can reproduce and reason about the failure from the log alone.

**ACs:**
```
Given a client sends a request with a repeated header (X-Debug: a + X-Debug: b)
      and an invalid JSON body (e.g. {"broken": json)
When  it is proxied
Then  1. raw_headers.X-Debug logs both values ["a","b"]
      2. headers.X-Debug (legacy field) logs first value "a" (untouched shape)
      3. body logs the invalid JSON verbatim
      4. upstream_request.body logs the same verbatim body
      5. upstream_request.url is the full outbound URL (destination + original path + query)
      6. upstream_request.headers preserves every value; hop-by-hop headers
         (Connection, Keep-Alive, TE, Trailer, Transfer-Encoding, Upgrade, …) are absent
      7. upstream_request.headers.X-Forwarded-For contains the caller's IP
      8. upstream_response.status / .headers / .body match the upstream reply exactly
         (multi-value response headers preserved as arrays)

Given a proxy round trip fails (destination unreachable)
When  the log entry is written
Then  1. upstream_request is present with the exact outbound request that was attempted
      2. upstream_response is absent (nil, omitted from JSON)
      3. response_status is 502 and the response body starts with "proxy error: "

Given a request is served by a local stub (not proxied)
When  the log entry is written
Then  raw_headers, upstream_request, and upstream_response are all absent
```

**Log entry example (proxy request):**
```json
{
  "webhook": "_proxy",
  "method": "POST",
  "path": "/api/orders",
  "headers": {"X-Debug": "a", "Content-Type": "application/json"},
  "body": "{\"broken\": json",
  "response_status": 201,
  "proxy_destination": "http://127.0.0.1:9191",
  "raw_headers": {"X-Debug": ["a", "b"], "Content-Type": ["application/json"], "User-Agent": ["curl/8.7.1"]},
  "upstream_request": {
    "method": "POST",
    "url": "http://127.0.0.1:9191/api/orders",
    "headers": {"X-Debug": ["a", "b"], "Content-Type": ["application/json"], "X-Forwarded-For": ["127.0.0.1"]},
    "body": "{\"broken\": json"
  },
  "upstream_response": {
    "status": 201,
    "headers": {"Content-Type": ["application/json"]},
    "body": "{\"created\":true}"
  }
}
```

**Tests:** `TestHandler_ProxyLogFidelity_MultiValueHeaders`, `TestHandler_ProxyLogFidelity_InvalidJSONBody`, `TestHandler_ProxyLogFidelity_UpstreamRequestCapture`, `TestHandler_ProxyLogFidelity_ErrorStillCapturesUpstreamRequest`, `TestHandler_ProxyLogFidelity_RawHeadersAbsentForStub`.

---

## PROXY-003 — Raw listener-level tee for malformed requests `[Should]`

**Status:** `ready` | **Decisions locked 2026-08-19, revised 2026-08-21 (D5 expanded, Q1–Q3 resolved)**

**Story:**
> As a developer debugging a broken integration, I want the raw bytes of proxy-bound requests AND responses captured verbatim — including requests Go's HTTP parser rejects (e.g. a header line with no colon) — so that I can see exactly what a buggy client sent and what the server replied, even when the request never reaches the handler.

**Problem being solved:** Go's `net/http` server parses the request before our handler runs. A malformed header line (`Accept; json\r\n`, no colon) makes the stdlib server answer `400 Bad Request` itself — the request never reaches `mock.Handler`, so nothing is logged today. PROXY-002's structured log cannot capture what it never sees. The only capture point is one layer down: wrap the `net.Listener` so every accepted connection's raw reads AND writes pass through the tee on the way to Go's parser.

**Locked decisions:**
- **D1 = A — path-based capture.** Only connections whose request-line path starts with a configured proxy prefix are captured. Stub-served requests and non-proxy paths on the same listener pass through untouched. This is the "proxy only, no change for simulate API" guarantee.
- **D2 = A — auto-on when `--proxy` is set.** No new flags. When `len(proxyRules) == 0`, the listener is never wrapped — zero behavior change, zero overhead, no new files for mock-only runs.
- **D3 = A — plain text dump.** Raw bytes go to `<log-dir>/_raw.log` as human-readable text with directional markers: `>>>` for request bytes (client→proxy), `<<<` for response bytes (proxy→client). Connection separator lines delimit entries. Non-printable bytes (outside 0x20–0x7E, `\t`, `\r`, `\n`) are hex-escaped as `\xNN`.
- **D4 = A — per-connection cap (~64 KB default).** Shared across request + response bytes. A giant body can't balloon the log; mirrors the `--max-log-body` philosophy (capture cap never affects actual bytes served).
- **D5 = B — request AND response capture.** Both `Read()` and `Write()` are intercepted on the client-facing `net.Conn`. Request bytes get `>>>` prefix lines, response bytes get `<<<` prefix lines. Both count against the shared 64 KB cap. Go's 400 response to a malformed request is also captured (valuable — user sees both the bad request AND the rejection).
- **D6 = A — decide once per connection.** The capture decision is made from the first request line; keep-alive reuse on the same connection inherits the decision. Documented limitation.

**Resolved defaults (accepted 2026-08-21):**
- **Q1 — Over-capture acceptable.** Stub-served requests whose path starts with a proxy prefix will be captured (tee can't distinguish stub vs proxy from the request line alone). Accepted as a superset of the proxy surface.
- **Q2 — Unbounded `_raw.log` acceptable.** 64 KB/conn cap is the only limit. No rotation in this iteration.
- **Q3 — Synchronous sink writes acceptable.** A slow log disk can stall a proxy-path Read/Write, never fail it. Best-effort debugging tool.

**ACs:**
```
Given a server started with --proxy /api=https://api.example.com
When  a client sends a well-formed request for /api/orders (all headers valid)
Then  1. the request is proxied normally (unchanged from PROXY-001)
      2. _raw.log contains the connection separator with the matching rule
      3. _raw.log contains the raw request bytes with >>> markers (request line, headers, body)
      4. _raw.log contains the raw response bytes with <<< markers (status line, headers, body)

Given a client sends a request whose path starts with a proxy prefix but has
      a malformed header line (e.g. "Accept; json" with no colon)
When  Go's net/http server rejects it with 400 before the handler runs
Then  1. the caller receives the standard 400 (unchanged Go behavior)
      2. _raw.log still records the full raw request bytes verbatim with >>> markers
      3. _raw.log records Go's 400 response bytes with <<< markers
      4. no JSON log entry is written (webhook "_proxy" absent — handler never ran)

Given a request arrives for a path that matches NO proxy rule (e.g. /health)
      on the same listener
When  it is served by a stub or returned 404
Then  _raw.log contains nothing for that connection (no capture)

Given a server started WITHOUT --proxy
When  it serves any traffic
Then  1. the listener is never wrapped (identity pass-through)
      2. no _raw.log file is created
      3. behavior is byte-identical to today's mock-only run

Given a connection whose request line cannot be parsed within the 8 KB budget
When  it is read
Then  no capture occurs for that connection (D6 fallback: no-capture)

Given a proxy-bound request with a body larger than the 64 KB capture cap
When  it is read
Then  _raw.log stores the first 64 KB (request + response combined), then stops teeing
      (the bytes still flow to Go's parser uncapped — capture cap only)
```

**Design:** reviewed and approved 2026-08-21. Types: `RawTeeListener` (wraps `net.Listener`), `RawTeeConn` (wraps `net.Conn`, intercepts `Read()` + `Write()`), `rawTeeSink` (mutex-guarded append to `_raw.log`, lazily opened, fail-open). 12 tests. Full design in the Architect's delivery (session 2026-08-21).

**Out of scope (locked):** TLS termination (simuhook is plain HTTP), HTTP/2 prior-knowledge, per-request re-decision on keep-alive connections (D6), any change to the JSON `LogEntry` schema or TUI, upstream connection capture (proxy→target bytes — already logged by PROXY-002's structured fields).

---

## PROXY-004 — Proxy TUI view with structured log entries `[Should]`

**Status:** `ready`

**Story:**
> As a developer debugging a proxied integration, I want a dedicated proxy view in the TUI dashboard that shows all proxy requests as structured log entries (like the main feed but filtered to proxy traffic only), so that I can monitor proxy activity separately from stub traffic and see both structured entries and raw bytes in one place.

**Decision:** the proxy view (viewMode == 1, accessible via `2` key) shows structured proxy log entries filtered from `StatsTracker.RecentLogs` where `Webhook == "_proxy"`, rendered using the same `summaryLine()` pipeline as the main feed. Below the structured entries, a "Raw Bytes" section shows the raw tee ring buffer lines (from `RawTeeSource.RawLogLines()`). The view auto-scrolls when live-follow is active.

**Implementation:**
- `proxyEntries()` helper filters `m.last.RecentLogs` to entries where `Webhook == "_proxy"`
- `buildProxyContent()` builds the full proxy view content: structured entries + raw bytes section
- `renderRawLogView()` renders header ("PROXY REQUESTS"), viewport, and footer with request + raw line counts
- Tick handler calls `buildProxyContent()` to update viewport content on each tick
- Key `1` switches to main dashboard, `2` switches to proxy view

**ACs:**
```
Given a server started with --proxy /api=https://api.example.com
When  a client sends a request for /api/orders (no stub matches, proxied)
Then  1. the proxy view shows the request as a structured log entry (same format as main feed)
      2. the structured entry shows time, method, path, status, webhook "_proxy", and latency
      3. the raw bytes section shows the >>>/<<< markers from the raw tee ring buffer

Given a server started with --proxy
When  a client sends a request that matches a stub (not proxied)
Then  1. the proxy view does NOT show the request (filtered to _proxy only)
      2. the main dashboard shows the request normally

Given the proxy view is active and live-follow is on
When  new proxy requests arrive
Then  the view auto-scrolls to show the newest entries

Given the proxy view is active and no proxy requests have been made
When  the view renders
Then  it shows "(no proxy requests yet)" placeholder text
```

**Tests:** `TestProxyView_StructuredEntries`, `TestProxyView_NoProxyEntries`, `TestProxyView_FiltersOnlyProxyWebhook`, `TestProxyView_NilSourceWithProxyLogs`, `TestProxyView_RawBytesSection`, `TestProxyEntries_Empty`, `TestProxyEntries_Mixed`.

---

## Out of scope

- WebSocket / SSE proxying (WS-012, backlog).
- Load balancing / retries across multiple destinations.
- Prefix stripping or URL rewriting (D1 — path is preserved verbatim).
- Admin API (removed ADM-002 — endpoint management is file-based only).