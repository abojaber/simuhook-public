---
title: "Changelog"
type: docs
weight: 70
---

# Changelog

## v0.9.0

Major feature release: transparent proxy forwarding, smarter matching, richer templates, and improved dashboard UX.

### Highlights

- **Transparent proxy** — `--proxy /path=https://dest` forwards unmatched requests to a real service, returning the upstream response unchanged. Full-fidelity logging captures every header value (multi-value preserved), the exact outbound request, and the full upstream reply. Raw byte capture at the connection level (`_raw.log`) records traffic even when Go's HTTP parser rejects malformed requests.
- **Proxy TUI view** — press `2` in the dashboard to see a dedicated proxy view with structured proxy log entries and raw bytes. Press `1` to return to the main dashboard.
- **Smarter matching** — path globs (`*`, `**`, `?`), regex paths, and partial-JSON payload matching (`payload_contains`). Most-specific match wins deterministically.
- **Richer templates** — 27 faker data generators (`{{Name}}`, `{{Email}}`, `{{UUID}}`, `{{Price}}`, …) and JSONPath request-body extraction (`{{JSONPath "$.user.name"}}`).
- **Unified cursor navigation** — `j`/`k`/`↑`/`↓` move a highlight cursor entry-by-entry with auto-scroll. `g`/`G` jump to top/bottom. One selection model for both fits and overflow modes.
- **Dashboard UX** — colorized request detail, response in the `enter` expansion, request/response visual separation with background bands.

### CLI

```
simuhook [flags]
  --addr :8080                listen address (env PORT)
  --stubs ../stubs            stubs directory (env STUBS_DIR)
  --log-dir logs              log directory (env LOG_DIR)
  --proxy /path=https://dest  transparent proxy rule (repeatable)
  --tui auto|on|off           dashboard mode: auto = TUI on a TTY, line output otherwise
  --recent N                  recent-log ring buffer size (default 50)
  --max-log-body N            cap stored body size for logs/dashboard (0 = unlimited; env LOG_MAX_BODY)
  --help, --version           usage / version
```

### Features In Depth

- **[Matching]({{< relref "/docs/simuhook/stubs/matching" >}})** — method, exact path, glob path, regex path, query, headers, exact payload, and partial-JSON payload matching. Most-specific match wins deterministically.
- **[Response templates]({{< relref "/docs/simuhook/templates" >}})** — variables, `Header`/`Query`/`RandomInt`/`RandomHex` functions, 27 faker data generators, and JSONPath request-body extraction.
- **[Proxy]({{< relref "/docs/simuhook/features/proxy" >}})** — transparent forwarding to real services with full-fidelity logging, raw byte capture, and a dedicated TUI view.
- **[Dashboard]({{< relref "/docs/simuhook/dashboard" >}})** — live dashboard with counts, latency stats (avg/min/max/p95), filterable feed, and per-request expansion and inspector views.
- **[Logging]({{< relref "/docs/simuhook/features/logging" >}})** — JSON Lines, per-endpoint files, unmatched request capture.

### Binary Assets

- `simuhook-darwin-arm64` (Apple Silicon)
- `simuhook-darwin-amd64` (Intel Mac)
- `simuhook-linux-arm64`
- `simuhook-linux-amd64`

See [Installation]({{< relref "/docs/simuhook/getting-started/installation" >}}) for download and setup.

---

## v0.1.0

Initial release. Mock HTTP server: define endpoints as YAML files, watch the directory, hot-reload live, and log every request as a JSON line. Ships with a full-screen TUI dashboard.

### Highlights

- **TUI dashboard** — live counts, latency stats (incl. approximate p95), filterable recent-request feed (`ep:`/`method:`/`status:` prefixes), per-endpoint stats, request/response inspector, pause/resume, g/G scroll jumps.
- **Mock engine** — YAML stubs with method/path/query/header/payload matching, `echo`/`template` modes, configurable delay, weighted response variants (round-robin on equal weight), hot-reload via fsnotify.
- **Logging** — every request (matched and unmatched) logged as a JSON line; `--max-log-body` caps stored body size.
- **`--version` flag** — print the version and exit. Release builds carry version via ldflags; dev builds report `dev`.

### Binary Assets

- `simuhook-darwin-arm64` (Apple Silicon)
- `simuhook-darwin-amd64` (Intel Mac)
- `simuhook-linux-arm64`
- `simuhook-linux-amd64`