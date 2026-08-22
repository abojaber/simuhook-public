---
title: "Proxy"
type: docs
weight: 5
---

# Proxy

Simuhook can act as a transparent man-in-the-middle for debugging integrations with external services. When no stub matches a request and a `--proxy` rule's path prefix matches, the request is forwarded as-is to the destination URL, both legs of the traffic are logged with full fidelity, and the upstream response is returned unchanged to the caller.

## Enabling the Proxy

Pass one or more `--proxy` flags at startup:

```bash
simuhook --proxy /api/stripe=https://api.stripe.com \
         --proxy /api=https://api.example.com
```

Each rule maps a **path prefix** to a **destination base URL**. The caller's original path and query string are carried over verbatim — no prefix stripping, no rewriting.

| Behavior | Detail |
|----------|--------|
| **Longest prefix wins** | `/api/stripe/v1/charges` matches `/api/stripe` before `/api`. |
| **Stubs take priority** | If a stub matches the request, the proxy rule is never consulted. |
| **Catch-all** | `--proxy /=https://fallback.example.com` forwards everything that matches no stub. |
| **No proxy** | Without `--proxy`, behavior is identical to a standard mock-only run. |

## What Gets Forwarded

When a request matches a proxy rule:

1. The request is forwarded to the destination with hop-by-hop headers stripped (`Connection`, `Keep-Alive`, `TE`, etc.).
2. `X-Forwarded-For` is appended with the caller's client IP.
3. The upstream response (status, headers, body) is returned to the caller unchanged.
4. A structured log entry is written with `webhook: "_proxy"` and full-fidelity fields.

## Log Fields

Proxy requests log additional fields beyond the standard log entry:

| Field | Description |
|-------|-------------|
| `webhook` | Always `"_proxy"` for proxied requests. |
| `proxy_destination` | The destination base URL (e.g. `https://api.stripe.com`). |
| `raw_headers` | Every incoming header value, including multi-value headers (map of arrays). |
| `upstream_request` | The exact outbound request: method, full URL, headers (after hop-by-hop stripping + XFF), body. |
| `upstream_response` | The full upstream reply: status, headers (multi-value preserved), body. |

When the destination is unreachable, `upstream_request` is still logged (the exact attempt), `upstream_response` is absent, and the response status is `502` with a `"proxy error: ..."` body.

## Raw Byte Capture

Go's `net/http` parser rejects malformed requests (e.g. a header line with no colon) with a `400` before the handler runs — nothing reaches the structured logger. Simuhook wraps the network listener to capture raw bytes at the connection level.

When `--proxy` is active, raw request and response bytes are appended to `<log-dir>/_raw.log`:

```
--- conn 127.0.0.1:54321 → /api/orders (proxy: https://api.example.com) ---
>>> POST /api/orders HTTP/1.1
>>> Host: localhost:8080
>>> Content-Type: application/json
>>> 
>>> {"event":"order.created"}
<<< HTTP/1.1 201 Created
<<< Content-Type: application/json
<<< 
<<< {"id":"ord_123"}
```

| Marker | Direction |
|--------|-----------|
| `>>>` | Client → Proxy (request) |
| `<<<` | Proxy → Client (response) |

Non-printable bytes are hex-escaped as `\xNN`. Each connection is capped at 64 KB shared across request and response bytes.

## Proxy View in the Dashboard

Press `2` to switch to the **proxy view** in the TUI dashboard. It shows:

1. **Structured proxy log entries** — filtered from the recent log buffer where `webhook == "_proxy"`, rendered with the same format as the main feed (time, method, path, status, latency).
2. **Raw bytes** — the raw tee ring buffer lines with `>>>`/`<<<` markers.

Press `1` to return to the main dashboard.

| Key | Action |
|-----|--------|
| `1` | Switch to main dashboard |
| `2` | Switch to proxy view |
| `L` | Pause / resume live-follow (in proxy view) |

## Example

```bash
# Start with a proxy rule
simuhook --proxy /api=https://httpbin.org

# Send a request — no stub needed, forwarded to httpbin
curl http://localhost:8080/api/post -X POST -d '{"hello":"world"}'
# → 200 from httpbin (full response)

# Check the structured log
cat logs/_proxy.log | jq .
# → webhook: "_proxy", proxy_destination, upstream_request, upstream_response

# Check the raw bytes
cat logs/_raw.log
# → >>> POST /api/post HTTP/1.1 ...
# → <<< HTTP/1.1 200 OK ...
```

## Also See

- [Configuration]({{< relref "/docs/simuhook/getting-started/configuration" >}}) — `--proxy` flag reference.
- [Dashboard]({{< relref "/docs/simuhook/dashboard" >}}) — proxy view key bindings.
- [Logging]({{< relref "/docs/simuhook/features/logging" >}}) — JSON line log format.
