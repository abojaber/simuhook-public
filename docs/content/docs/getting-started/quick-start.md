---
title: "Quick Start"
type: docs
weight: 2
---

# Quick Start

Five steps from zero to a working mocked endpoint.

## 1. Start the Server

```bash
simuhook
```

The server starts on `:8080`, watches the `stubs/` directory, and logs requests to `logs/`. On a terminal it opens the interactive TUI dashboard; on a non-TTY (like a pipe) it prints a startup banner and one line per request.

<!-- TODO(screenshots): drop startup-banner.png into docs/static/images/ and uncomment the figure line when ready:
  {{< figure src="/images/startup-banner.png" title="The startup banner — shown on non-TTY output (`--tui off` or piped)" >}} -->

## 2. Verify It's Running

```bash
curl http://localhost:8080/health
# → {"status":"ok"}
```

The repository ships with example stubs, including a `GET /health` endpoint.

## 3. Create Your First Stub

Create a YAML file in the stubs directory. Each file defines one endpoint.

```bash
mkdir stubs
```

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

Save the file. The watcher hot-reloads it instantly — no restart.

## 4. Send a Request

```bash
curl -X POST http://localhost:8080/webhooks/user \
  -H "X-Event: user.created" \
  -d '{"event":"user.created"}'
# → {"received":true,"event":"user.created"}   (with a 200ms delay)
```

Notice the request only matched because you sent the right header **and** the exact body. Change the body and the match fails:

```bash
curl -X POST http://localhost:8080/webhooks/user \
  -H "X-Event: user.created" \
  -d '{"event":"user.updated"}'
# → 404 {"error":"no matching endpoint"}
```

## 5. Check the Logs

Every request is logged as a JSON line to `logs/<id>.log`. Matched requests go to `logs/user-create.log`; unmatched requests go to `logs/_unmatched.log`.

```json
{
  "timestamp": "2026-08-18T10:30:00.123Z",
  "request_id": "uuid-v4",
  "webhook": "user-create",
  "method": "POST",
  "path": "/webhooks/user",
  "headers": {"X-Event": "user.created", "User-Agent": "curl/8.7.1"},
  "body": "{\"event\":\"user.created\"}",
  "query": {},
  "response_status": 201,
  "response_headers": {"X-Mock": "yes"},
  "response_body": "{\"received\":true,\"event\":\"user.created\"}",
  "latency_ms": 200.02,
  "matched": true
}
```

That's it. You now have a mocked webhook endpoint with header + payload matching, a simulated delay, and full request logging.

## Next

- Read the [Stubs reference]({{< relref "/docs/stubs" >}}) for all matching options.
- See the [curl cookbook]({{< relref "/docs/examples/curl-cookbook" >}}) for ready-to-paste commands.