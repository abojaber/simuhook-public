# Phase 02 — MVP

> Minimum viable product. Must-have features only.

---

## WS-001 — Define webhook endpoints via YAML config

**Status:** `ready` | **Priority:** `[Must]`

**User Story:**
> As a developer, I want to define webhook endpoints in a YAML configuration file so that I can version-control my webhook definitions and launch the simulator with a single command.

**Acceptance Criteria:**
```
Given a valid config.yaml file defining one or more webhook endpoints
When  I run `simuhook serve --config config.yaml`
Then  the server starts and exposes all defined endpoints

Given a config.yaml with an invalid schema
When  I run `simuhook serve --config config.yaml`
Then  the CLI exits with a descriptive validation error and a non-zero exit code
```

**YAML Schema:**
```yaml
webhooks:
  - name: order-created
    path: /webhooks/orders
    method: POST
    response:
      status: 200
      headers:
        Content-Type: application/json
      body: '{"received": true}'
      delay: 0ms
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

**Status:** `ready` | **Priority:** `[Must]`

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

**Status:** `ready` | **Priority:** `[Must]`

**User Story:**
> As a developer, I want to start and stop the webhook simulator from the terminal so that it fits into my local development workflow.

**Acceptance Criteria:**
```
Given the server is running
When  I send SIGINT (Ctrl+C)
Then  the server shuts down gracefully, flushes pending logs, and exits

Given the server is running
When  I specify --port 9090
Then  the server listens on port 9090 instead of the default
```

CLI signature: `simuhook serve [--config path] [--port N]`

---

## WS-005 — Dynamic request matching

**Status:** `ready` | **Priority:** `[Must]`

**User Story:**
> As a developer, I want webhook endpoints to match on path AND HTTP method so that I can simulate different webhook types on the same base path.

**Acceptance Criteria:**
```
Given two endpoints: GET /webhooks/status and POST /webhooks/status
When  I send GET /webhooks/status
Then  the GET endpoint handles it, not the POST one

Given a request to an unconfigured path
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
webhooks:
  - name: debug-echo
    path: /webhooks/echo
    method: POST
    response:
      status_code: 200
      echo: true
```