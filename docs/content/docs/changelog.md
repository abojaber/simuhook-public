---
title: "Changelog"
type: docs
weight: 70
---

# Changelog

## v0.1.0

Mock HTTP server: define endpoints as YAML files, watch the directory, hot-reload live, and log every request as a JSON line. Ships with a full-screen TUI dashboard.

### Highlights

- **Colorized request detail** — `enter` expansion and `i` inspector show cyan labels/keys, yellow request bodies, green response bodies, and JSON syntax highlighting.
- **Response in the `enter` expansion** — expanding a feed row shows the full response alongside the request: status-tinted `RESPONSE` header, response headers, and a multi-line response body. Unmatched requests show `RESPONSE 404`.
- **Request/response visual separation** — a muted `─` separator plus full-width background bands (REQUEST dark gray `236`, RESPONSE dark navy `17`) make the two sides instantly distinguishable; response keys are magenta, request keys cyan.
- **`--version` flag** — print the version and exit. Release builds carry `v0.1.0` via ldflags; dev builds report `dev`.
- **TUI dashboard** — live counts, latency stats (incl. approximate p95), filterable recent-request feed (`ep:`/`method:`/`status:` prefixes), per-endpoint stats, request/response inspector, pause/resume, g/G scroll jumps.
- **Mock engine** — YAML stubs with method/path/query/header/payload matching, `echo`/`template` modes, configurable delay, weighted response variants (round-robin on equal weight), hot-reload via fsnotify.
- **Logging** — every request (matched and unmatched) logged as a JSON line; `--max-log-body` caps stored body size.

### CLI

```
simuhook [flags]
  --addr :8080          listen address (env PORT)
  --stubs ../stubs      stubs directory (env STUBS_DIR)
  --log-dir logs        log directory (env LOG_DIR)
  --tui auto|on|off     dashboard mode: auto = TUI on a TTY, line output otherwise
  --recent N            recent-log ring buffer size (default 50)
  --max-log-body N      cap stored body size for logs/dashboard (0 = unlimited; env LOG_MAX_BODY)
  --help, --version     usage / version
```

### Features In Depth

- **[Matching]({{< relref "/docs/stubs/matching" >}})** — method, exact path, glob path, regex path, query, headers, exact payload, and partial-JSON payload matching. Most-specific match wins deterministically.
- **[Response templates]({{< relref "/docs/templates" >}})** — variables, `Header`/`Query`/`RandomInt`/`RandomHex` functions, 27 faker data generators, and JSONPath request-body extraction.
- **[Dashboard]({{< relref "/docs/dashboard" >}})** — live dashboard with counts, latency stats (avg/min/max/p95), filterable feed, and per-request expansion and inspector views.
- **[Logging]({{< relref "/docs/features/logging" >}})** — JSON Lines, per-endpoint files, unmatched request capture.

### Binary Assets

- `simuhook-darwin-arm64` (Apple Silicon)
- `simuhook-darwin-amd64` (Intel Mac)
- `simuhook-linux-arm64`
- `simuhook-linux-amd64`

See [Installation]({{< relref "/docs/getting-started/installation" >}}) for download and setup.