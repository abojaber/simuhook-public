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
When  I run `webhooker serve --config config.yaml`
Then  the server starts and exposes all defined endpoints

Given a config.yaml with an invalid schema
When  I run `webhooker serve --config config.yaml`
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

**Status:** `ready` | **Priority:** `[Must]`

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

CLI signature: `webhooker serve [--config path] [--port N]`

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