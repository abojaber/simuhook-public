# Architecture

## Overview

A Go CLI that simulates incoming webhook requests. Users define webhook configurations (endpoint URL, HTTP method, headers, payload template, delivery interval/batch), and the simulator dispatches them to target URLs on schedule.

## Commands

- `simuhook serve` — Start the HTTP server
- `simuhook config` — Manage webhook configurations
- `simuhook logs` — View delivery logs

## Components

```
src/
├── main.go              # Entry point
└── mock/
    ├── types.go         # Endpoint, Response structs
    ├── registry.go      # Store, match, CRUD endpoints
    ├── server.go        # HTTP handler
    ├── stub.go          # YAML stub file parser
    ├── logger.go        # Request logging (JSON Lines)
    └── watcher.go       # File watcher (hot-reload)

stubs/                   # YAML stub definitions
├── health.yaml
├── order-placed.yaml
├── ping.yaml
└── user-create.yaml
```

## API

| Method | Path | Description |
|--------|------|-------------|
| GET | `/_admin/endpoints` | List all endpoints |
| POST | `/_admin/endpoints` | Create endpoint (JSON body) |
| GET | `/_admin/endpoints/{id}` | Get one endpoint |
| PUT | `/_admin/endpoints/{id}` | Update endpoint |
| DELETE | `/_admin/endpoints/{id}` | Remove endpoint |

## Data Flow

1. YAML stubs loaded from `stubs/` directory at startup
2. File watcher hot-reloads changes live
3. HTTP server matches incoming requests against stubs
4. Matched requests get configured response (with optional delay)
5. Every request logged as JSON line to `logs/<webhook-id>.log`
6. Unmatched requests go to `logs/_unmatched.log`

## Key Decisions

### Internal packages
- `config`, `webhook`, `dispatcher`, `storage`, `api` — package structure follows standard Go layout
- `net/http` (stdlib) — no external HTTP framework

### Deliverables
- Runnable Go binary: `simuhook`
- REST API for CRUD on webhook configurations
- Background dispatcher that fires webhooks on schedule
- Delivery log with status, response code, latency, and retry count
