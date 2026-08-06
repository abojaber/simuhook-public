# Observer panic policy

**Title:** Observer panic policy — decide and enforce what happens when a `Handler` observer panics

**Status:** backlog (from DASH-001 gap analysis, decision D6 — stakeholder agreed 2026-08-05)

**Priority:** low (`[Could]`)

## Description

`mock.Handler` calls its observers **synchronously** after the response is written
(`src/mock/handler.go:172-174`, WS-017 semantics — observers run on the request path):

```go
for _, o := range h.observers {
    o(entry)
}
```

Today the only observers are `stats.RecordRequest` (in-memory counters) and `logger.Log`
(JSON-lines file write, drops errors). Neither is expected to panic, but there is **no
guard**: if an observer panics, the panic propagates out of `ServeHTTP`. `net/http` recovers
handler panics per-connection (logs the stack, closes that connection), so the process
survives — but the request's connection is killed, the panic stack is dumped into the
server log, and any *later* observers in the loop never run (e.g. the log line for a request
would be missing if `stats` panicked first). As the observer list grows (future features),
this becomes a real robustness gap.

This is a **policy decision**, not just a code change: should observers be allowed to panic
the request path at all?

## Architecture / approach (candidate directions)

1. **Per-observer `recover` in the handler loop (recommended direction).** Wrap each `o(entry)`
   call in a closure with `defer recover()`; log the panic (observer name/index + stack) to
   the standard logger and continue with the remaining observers. Guarantees one observer's
   failure can never break request handling or starve later observers. Cost: a recover per
   observer per request (negligible; recover is cheap and observers are 2 today).
2. **Document "observers must never panic" and enforce nothing** (accepted risk). Cheapest,
   but leaves the failure mode in place for future observers.
3. **Run observers in a separate goroutine / via a channel** (like the old "pump" idea for
   Notify). Removes panics from the request path entirely, but changes WS-017's
   synchronous-after-write semantics and adds ordering/lifecycle complexity. **Not recommended**
   for the current 2-observer world.

Direction 1 keeps WS-017 semantics (synchronous, after write) and makes the contract safe.

## Files to create/edit

- `src/mock/handler.go` — wrap the observer loop (`:172-174`) with per-observer recover +
  panic logging.
- `src/mock/handler_test.go` — new test: a panicking observer does not stop later observers,
  does not break the HTTP response, and the panic is logged (capture via `log.SetOutput`).

## Steps

- [ ] Confirm policy with stakeholder (direction 1 preferred).
- [ ] Implement recover-per-observer + panic logging in `handler.go`.
- [ ] Tests: panicking first observer → second observer still runs, response intact;
      panic message/stack appears in the captured log output; `-race` clean.
- [ ] Update `architecture.md` WS-017 ADR with the observer panic policy.
- [ ] QA: verify ACs below.

## Status checklist

- [x] Backlog item created (2026-08-05, D6)
- [ ] Policy confirmed
- [ ] Implemented
- [ ] Tested / race clean
- [ ] QA verified
- [ ] Moved out of backlog / removed
