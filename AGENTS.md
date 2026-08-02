# Simuhook

A mock HTTP server. Define endpoints as YAML files — the server watches the directory, matches incoming requests, and returns configured responses. Changes to stub files are applied live without restart.

## Project

Simuhook — a Go CLI tool that simulates incoming webhook requests. Users configure webhook endpoints, payloads, headers, and delivery schedules, then the simulator fires them at designated targets so teams can test their integrations without relying on real third-party services.

## Tech Stack

- **Language:** Go 1.21+
- **Approach:** Stdlib-first (net/http), no external frameworks unless discussed
- **Configuration:** JSON file and environment variables
- **Storage:** YAML files for stubs, JSON lines for logs

## Working Directory

`/Users/abojaber/developer/simuhook`

## Conventions

- Write idiomatic Go following standard project layout conventions
- Handle errors gracefully — every HTTP dispatch must handle timeouts, retries, and status reporting
- Use interfaces for testability (e.g. an `http.RoundTripper` for mocking HTTP)
- Graceful shutdown handling (`os.Signal`)
- Idempotency support via `X-Idempotency-Key` header
- All decisions should be traceable to a user story or requirement

## Commands

- Build: `go build -o simuhook ./src`
- Test: `go test -v -race ./src/...`
- Run: `cd src && go run .`
- Lint: no lint command configured
