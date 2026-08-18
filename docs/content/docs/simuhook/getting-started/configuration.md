---
title: "Configuration"
type: docs
weight: 3
---

# Configuration

Simuhook is configured through CLI flags and environment variables. Flags take precedence over environment variables.

## CLI Flags

```
simuhook [flags]

  --addr :8080          listen address (env PORT)
  --stubs ../stubs      stubs directory (env STUBS_DIR)
  --log-dir logs        log directory (env LOG_DIR)
  --tui auto|on|off     dashboard mode: auto = TUI on a TTY, line output otherwise
  --recent N            recent-log ring buffer size (default 50; also the TUI feed depth)
  --max-log-body N      cap stored request/response body size in bytes for logs and
                        dashboard (default 0 = unlimited; env LOG_MAX_BODY). Never affects
                        the actual HTTP response bytes.
  --help                print usage and exit
  --version             print version and exit
```

## Environment Variables

| Variable | Default | Overrides |
|----------|---------|-----------|
| `PORT` | `:8080` | `--addr` |
| `STUBS_DIR` | `stubs/` | `--stubs` |
| `LOG_DIR` | `logs/` | `--log-dir` |
| `LOG_MAX_BODY` | `0` (unlimited) | `--max-log-body` |

## Common Scenarios

**Run on a different port:**

```bash
simuhook --addr :9000
# or
PORT=:9000 simuhook
```

**Point at a different stub directory:**

```bash
simuhook --stubs /path/to/my/stubs
# or
STUBS_DIR=/path/to/my/stubs simuhook
```

**Run without the TUI, capturing line output:**

```bash
simuhook --tui off
```

**Cap stored log bodies to 1 KB** (useful when payloads carry large binary blobs):

```bash
simuhook --max-log-body 1024
```

Bodies larger than the cap are truncated in the logs and dashboard — the actual HTTP response served to clients is never affected.

## The TUI Dashboard

With `--tui auto` (the default) Simuhook starts the full-screen dashboard whenever it detects a terminal (TTY) and falls back to one-line-per-request output otherwise. Use `--tui on` to force the dashboard even on a non-TTY, or `--tui off` to always use line output.

Inside the dashboard, press `?` for the key reference. See the [Dashboard]({{< relref "/docs/simuhook/dashboard" >}}) section for full details.