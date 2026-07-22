# Phase 02 — MVP

> Minimum viable product. Must-have features only.

---

## WS-001 — Define webhook endpoints via YAML stubs

**Status:** `done` | **Priority:** `[Must]`

**User Story:**
> As a developer, I want to define webhook endpoints in YAML stub files so that I can version-control my webhook definitions and launch the simulator with a single command.

**Acceptance Criteria:**
```
Given valid YAML stub files in the stubs directory
When  I start the server
Then  all defined endpoints are loaded and exposed

Given a stub file with an invalid schema (missing id)
When  the server starts
Then  the invalid file is skipped with a descriptive log message

Given a stub file is created, modified, or deleted while the server is running
When  the file change is detected
Then  the endpoint is added, updated, or removed live without restart
```

**YAML Schema (per-file):**
```yaml
# <name>.yaml
id: order-created
method: POST
path: /webhooks/orders
response:
  status_code: 200
  headers:
    Content-Type: application/json
  body: '{"received": true}'
  delay: 0s
```

---

## WS-002 — Log incoming requests to file

**Status:** `done` | **Priority:** `[Must]`

**User Story:**
> As a developer testing an integration, I want every incoming webhook request logged to a file so that I can inspect what payloads and headers were actually received.

**Acceptance Criteria:**
```
Given the server is running and an endpoint is configured
When  a request hits that endpoint
Then  a JSON log entry is appended to logs/<webhook-name>.log
And   the log contains: timestamp, method, path, headers, body, response_status, latency_ms

Given the logs/ directory does not exist
When  a request arrives
Then  the directory is created automatically
```

---

## WS-003 — Custom responses with delay

**Status:** `done` | **Priority:** `[Must]`

**User Story:**
> As a developer testing timeout handling, I want to configure a simulated delay on any webhook endpoint so that I can verify my client handles slow responses correctly.

**Acceptance Criteria:**
```
Given an endpoint configured with delay: 2s
When  a request arrives
Then  the response is sent after exactly 2 seconds

Given an endpoint configured with delay: 0ms (or absent)
When  a request arrives
Then  the response is sent immediately with no artificial delay
```

---

## WS-004 — CLI serve command with graceful shutdown

**Status:** `done` | **Priority:** `[Must]`

**User Story:**
> As a developer, I want to start and stop the webhook simulator from the terminal so that it fits into my local development workflow.

**Acceptance Criteria:**
```
Given the server is running
When  I send SIGINT (Ctrl+C) or SIGTERM
Then  the server shuts down gracefully, flushes pending logs, and exits

Given the server is running
When  I specify --addr :9090
Then  the server listens on port 9090 instead of the default

Given the PORT environment variable is set
When  the server starts with no --addr flag
Then  the server listens on the port specified by PORT
```

CLI invocation: `go run . [--addr :8080] [--stubs ../stubs] [--log-dir logs]`

**Environment Variables:**
| Variable | Overrides | Default |
|----------|-----------|---------|
| `PORT` | `--addr` port | `:8080` |
| `STUBS_DIR` | `--stubs` | `../stubs` |
| `LOG_DIR` | `--log-dir` | `logs` |

---

## WS-005 — Dynamic request matching

**Status:** `done` | **Priority:** `[Must]`

**User Story:**
> As a developer, I want webhook endpoints to match on path, HTTP method, headers, payload, and query parameters so that I can simulate different webhook types with fine-grained routing.

**Acceptance Criteria:**
```
Given two endpoints: GET /webhooks/status and POST /webhooks/status
When  I send GET /webhooks/status
Then  the GET endpoint handles it, not the POST one

Given an endpoint with header constraints (X-Event: user.created)
When  a request arrives with a matching header value
Then  the endpoint matches
When  a request arrives with a non-matching header value
Then  it does not match (404)

Given an endpoint with query parameter constraints
When  a request arrives with matching query params
Then  the endpoint matches
When  a request arrives with missing or mismatched query params
Then  it does not match (404)

Given an endpoint with payload constraints
When  a request arrives with an exact payload match
Then  the endpoint matches
When  a request arrives with a different payload
Then  it does not match (404)

Given a request that does not match any endpoint
When  the request arrives
Then  the server responds with 404 and logs the unmatched request
```

---

## WS-015 — Echo mode: return request payload as response

**Status:** `done` | **Priority:** `[Must]`

**User Story:**
> As a developer debugging an integration, I want an endpoint with `echo: true` to return the request body verbatim as the response so that I can inspect exactly what my client is sending without needing a separate echo server.

**Acceptance Criteria:**
```
Given an endpoint configured with response.echo: true and response.body set to something else
When  a POST/PUT/PATCH request with a body arrives
Then  the endpoint returns the request body as the response body
And   the configured response.body is ignored

Given an endpoint configured with response.echo: true
When  a GET request with no body arrives
Then  the endpoint returns an empty response body

Given an endpoint configured without response.echo (or echo: false)
When  a request arrives
Then  the endpoint returns the configured response.body as normal
```

**YAML Schema:**
```yaml
id: debug-echo
path: /webhooks/echo
method: POST
response:
  status_code: 200
  echo: true
```