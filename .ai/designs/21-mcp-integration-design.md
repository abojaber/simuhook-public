# MCP Integration Design for Simuhook

> **Status:** design-complete | **Owner:** BA | **Date:** 2026-10-09 | **Story:** 21

## Goal

Enable AI tools (opencode, claw, etc.) to connect to Simuhook via MCP (Model Context Protocol) for:
1. **Tracing request/response details** — query the log ring buffer, inspect headers/bodies/status
2. **Tracing proxy targets** — view proxied requests with upstream request/response fidelity
3. **Real-time monitoring** — get notified when new requests arrive

## Scope (v1)

| Included | Excluded (later) |
|----------|-------------------|
| Read-only tracing tools | Stub CRUD (create/update/delete) |
| Resource subscriptions (real-time) | Attach mode to running instance |
| Standalone `simuhook mcp` subcommand | HTTP/remote transport |
| 6 tools + 4 resources | Request injection via MCP |

## Architecture

`simuhook mcp` starts the full Simuhook server but replaces the TUI/line output with an MCP stdio server. The AI tool spawns it as a child process.

```
┌─ opencode ──────────────────────────┐
│  spawns: simuhook mcp               │
│  communicates: stdin/stdout (MCP)   │
└──────────────┬──────────────────────┘
               │ stdio (JSON-RPC)
┌──────────────▼──────────────────────┐
│  simuhook mcp                       │
│  ┌─────────────┐  ┌──────────────┐  │
│  │ MCP Server  │  │ HTTP Server  │  │
│  │ (stdio)     │  │ (:8080)      │  │
│  └──────┬──────┘  └──────┬───────┘  │
│         │                │          │
│  ┌──────▼────────────────▼───────┐  │
│  │  mock.Registry + StatsTracker │  │
│  │  + Logger + RawTee            │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
```

**Key decisions:**
- **Standalone mode**: MCP server IS the Simuhook instance. AI sends HTTP requests to :8080, queries MCP for results. No attach mode in v1.
- **Direct in-memory access**: MCP tools read from StatsTracker ring, Registry, RawTee ring — no file parsing, no IPC.
- **No TUI conflict**: `simuhook mcp` disables TUI/line output. MCP owns stdout. Logs go to files only.
- **Real-time via subscriptions**: MCP resource subscriptions push `notifications/resources/updated` when `StatsTracker.Notify()` fires.

## SDK Choice

**Recommended: Official SDK `github.com/modelcontextprotocol/go-sdk`**
- Tracks latest MCP spec (2026-07-28)
- Typed tool handlers via generics
- Resource subscriptions built-in
- `NewInMemoryTransports()` for unit testing
- Small dep footprint (jsonschema-go, uritemplate, segmentio/encoding)
- Approved exception precedent: bubbletea, gofakeit

**Alternative: stdlib-only** — MCP stdio is newline-delimited JSON-RPC. ~300 lines, zero new deps. But manual spec compliance.

**Decision:** Official Go SDK (stakeholder-approved).

## Tools (v1)

| Tool | Input | Output | Description |
|------|-------|--------|-------------|
| `get_recent_requests` | `method?`, `status?`, `webhook?`, `path_contains?`, `limit?` | `[]LogEntry` | Query ring buffer with filters |
| `get_request_detail` | `request_id` | `LogEntry` (full) | Full request/response with headers, body, response |
| `get_proxy_traffic` | `limit?` | `[]LogEntry` (proxy-only) | Proxied requests with upstream fidelity |
| `get_server_status` | — | `{uptime, total, unmatched, latency, endpoints, proxy_rules}` | Server stats + config |
| `list_stubs` | — | `[]Endpoint` | Registered stubs with match rules |
| `get_raw_log` | `limit?` | `[]string` | Raw tee lines (`>>>`/`<<<`) |

## Resources (v1)

| URI | Content | Subscription |
|-----|---------|-------------|
| `simuhook://requests` | Recent log entries as JSON | ✅ new request → push |
| `simuhook://stubs` | Registered stubs as JSON | ✅ stub reload → push |
| `simuhook://proxy` | Proxy traffic as JSON | ✅ new proxy request → push |
| `simuhook://status` | Server status as JSON | ✅ periodic update |

## CLI

```
simuhook mcp [flags]

  --addr :8080          listen address (env PORT)
  --stubs ../stubs      stubs directory (env STUBS_DIR)
  --log-dir logs        log directory (env LOG_DIR)
  --proxy /path=https://dest  transparent proxy rule (repeatable)
  --recent N            recent-log ring buffer size (default 50)
  --max-log-body N      cap stored body size (default 0)
```

No `--tui` flag (MCP owns stdout).

## Files

| File | Action | Purpose |
|------|--------|---------|
| `src/mcp/mcp.go` | NEW | Server setup, tool/resource registration |
| `src/mcp/tools.go` | NEW | Tool handlers |
| `src/mcp/resources.go` | NEW | Resource handlers + subscription logic |
| `src/mcp/mcp_test.go` | NEW | Tests via `NewInMemoryTransports()` |
| `src/main.go` | EDIT | `mcp` subcommand dispatch |
| `src/go.mod` | EDIT | Add go-sdk dependency |

## Steps

1. Create `feature/mcp` branch
2. Add go-sdk dependency
3. MCP server skeleton (`src/mcp/mcp.go`)
4. Tool handlers (`src/mcp/tools.go`)
5. Resource + subscription (`src/mcp/resources.go`)
6. Subcommand in `src/main.go`
7. Tests (`src/mcp/mcp_test.go`)
8. opencode.json MCP config
9. Docs (`.ai/` + Hugo)

## opencode.json

```json
{
  "mcp": {
    "simuhook": {
      "type": "local",
      "command": ["./src/simuhook", "mcp", "--stubs", "./stubs", "--log-dir", "logs"],
      "enabled": true
    }
  }
}
```

## Open Decisions

1. **SDK**: Official Go SDK (approved) vs stdlib-only implementation
2. **Branch name**: `feature/mcp`
3. **Real-time priority**: Ship with resource subscriptions (push) in v1, or is polling sufficient

## Security Considerations

- MCP server runs locally (stdio) — no network exposure for MCP itself
- HTTP server still binds to :8080 — same security posture as normal mode
- No authentication on MCP tools (same as the existing admin API was)
- `sanitize()` on all log-derived text (existing security from UXR-008)

## Dependencies

| Dependency | Version | Purpose |
|------------|---------|---------|
| `github.com/modelcontextprotocol/go-sdk` | latest | MCP protocol library |
| Existing: `github.com/charmbracelet/bubbletea` | v1.3.10 | TUI (not used in mcp mode) |
| Existing: `github.com/fsnotify/fsnotify` | v1.10.1 | Stub hot-reload |