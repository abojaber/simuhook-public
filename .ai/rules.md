# Rules

## General

- Follow existing code style and conventions in the project
- Prefer simple solutions over clever ones
- Write tests alongside code
- Handle errors gracefully with clear messages
- Commit messages should be concise and descriptive

## CLI Conventions

- Use `--flag` for long flags, `-f` for short
- Exit 0 on success, non-zero on error
- Print errors to stderr, output to stdout
- Support `--help` and `--version` on every command

## Go Conventions

- Go 1.21+.
- No external frameworks unless discussed (stdlib-first approach).
- Write unit tests alongside every package (use Go's `testing` package).
- Keep functions small and single-purpose.
- Use interfaces for testability (e.g. an `http.RoundTripper` for mocking HTTP).
- Configuration via JSON file and environment variables.
- Graceful shutdown handling (`os.Signal`).
- Idempotency support via `X-Idempotency-Key` header.

## Testing

- Create test plans from BA user stories and acceptance criteria.
- Cover: happy path, edge cases, error handling, performance, and security.
- Use `curl` or a simple Go test harness for API testing.
- Use `go test -race` to detect data races.
- Use `ab` (ApacheBench) or `wrk` for load testing if available.
- Verify each acceptance criterion explicitly and log pass/fail.
- Test HTTP dispatch: payload correctness, header propagation, retry logic, timeouts.
- Report bugs with severity, steps to reproduce, expected vs actual, and logs.

## Backlog Convention

Deferred or uncertain features go in `.ai/backlog/` as individual markdown files.
- Each file has: title, description, architecture/approach, files to create/edit, steps, status checklist.
- Move items out of `backlog/` into session todos when actively implementing.
- Remove the file if decided against (no "won't do" markers).
- File naming: `kebab-case-description.md`

## Project-Specific

Add project-specific conventions here.
