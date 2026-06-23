# Webhooker

A mock HTTP server. Define endpoints as YAML files — the server watches the directory, matches incoming requests, and returns configured responses. Changes to stub files are applied live without restart.

## Quick Start

```bash
cd src && go run .
```

Starts on `:8080` (override with `--addr` or `PORT`). Watches `../stubs/` (override with `--stubs` or `STUBS_DIR`).

## Stub Files (YAML)

Place `.yaml` / `.yml` files in the `stubs/` directory. Each file defines one endpoint. Files are loaded at startup and hot-reloaded on change.

```yaml
# stubs/user-create.yaml
id: user-create
method: POST
path: /webhooks/user
headers:
  X-Event: user.created
payload: '{"event":"user.created"}'
response:
  status_code: 201
  body: '{"received":true,"event":"user.created"}'
  headers:
    X-Mock: "yes"
  delay: 200ms
```

| Field | Required | Description |
|---|---|---|
| `id` | yes | Unique endpoint identifier |
| `method` | no | HTTP method. Empty matches any method |
| `path` | yes | Exact request path to match |
| `headers` | no | Request headers that must match |
| `payload` | no | Exact request body to match |
| `response.status_code` | no | HTTP status code to return (default: 200) |
| `response.body` | no | Response body string |
| `response.headers` | no | Response headers |
| `response.delay` | no | Simulated latency (e.g. `200ms`, `1s`) |

### Request Matching

An endpoint matches when **all** specified conditions are met:

- `method` — must equal request method (empty = match any)
- `path` — exact match
- `headers` — all key/value pairs present in request
- `payload` — exact body match

## Admin API

| Method | Path | Description |
|---|---|---|
| `GET` | `/_admin/endpoints` | List all endpoints |
| `POST` | `/_admin/endpoints` | Create endpoint (JSON body) |
| `GET` | `/_admin/endpoints/{id}` | Get one endpoint |
| `PUT` | `/_admin/endpoints/{id}` | Update endpoint |
| `DELETE` | `/_admin/endpoints/{id}` | Remove endpoint |

```bash
# create via API (id must be unique, not conflicting with stub files)
curl -X POST http://localhost:8080/_admin/endpoints \
  -H "Content-Type: application/json" \
  -d '{"id":"my-endpoint","method":"GET","path":"/my-endpoint","response":{"status_code":200,"body":"bXkgZW5kcG9pbnQ="}}'

# call the mock endpoint
curl http://localhost:8080/ping                     # → 200 (from ping.yaml stub)
curl http://localhost:8080/my-endpoint              # → 200 my-endpoint (from API)
curl -X POST http://localhost:8080/ping            # → 404 (method mismatch)
```

## Architecture

```
src/
├── main.go
└── mock/
    ├── types.go      # Endpoint, Response structs
    ├── registry.go   # Store, match, CRUD endpoints
    ├── server.go     # HTTP handler
    ├── stub.go       # YAML stub file parser
    └── watcher.go    # File watcher (hot-reload)

stubs/
├── health.yaml
├── order-placed.yaml
├── ping.yaml
└── user-create.yaml
```

## Testing

```bash
go test -v -race ./src/...
```
