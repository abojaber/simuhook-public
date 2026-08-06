# ADM-002 — Remove the admin REST API from the product

Requirement file owner: BA. Status: `ready`. Phase: maintenance/removal. Date: 2026-08-05.

## 0. Decision

Stakeholder decision 2026-08-05: **remove all admin endpoints** (`/_admin/endpoints` and
`/_admin/endpoints/{id}`) and move the feature to the backlog at low priority (consolidated
as WS-014/ADM-001 in `04-backlog.md`). Endpoint management is file-based only — create/edit/
delete YAML in the stubs directory; the watcher hot-reloads live. **No replacement feature.
This is a pure deletion.**

The BA's Gap A analysis (admin API has no input validation → blank-id orphans, dead endpoints,
PUT `{}` wipe) was agreed by the stakeholder; the chosen solution is removal rather than
patching the handler.

## 1. Story

> As a developer maintaining Simuhook, I want the admin REST API removed from the codebase
> so that the product surface is file-based only and the unauthenticated CRUD attack surface
> is gone.

## 2. Acceptance Criteria

```
Given the codebase after removal
When  I search for NewAdminHandler, AdminEndpointsPath, or /_admin in src/
Then  there are no hits (except documentation references in comments that were
      deliberately retained)

Given the server starts with no admin mounts
When  I GET/POST/PUT/DELETE /_admin/endpoints (or /_admin/endpoints/{id})
Then  the request falls through to the mock surface (handled as an unmatched
      request → 404 + logged to _unmatched.log)

Given the server starts on a TTY (TUI mode)
When  the dashboard renders (both wide and narrow layouts)
Then  no "Admin:" URL text appears anywhere in the view

Given the server starts with TUI disabled
When  the startup banner prints
Then  it contains no "Admin API:" line

Given the full test suite
When  go test -race -count=1 ./... runs
Then  all tests pass (admin_test.go deleted; dashboard admin-URL assertions updated)
And   go vet ./... is clean
```

## 3. Scope — exact changes

| File | Change |
|------|--------|
| `src/mock/admin.go` | **DELETE** whole file |
| `src/mock/admin_test.go` | **DELETE** whole file (18 tests: 16 ACs + nil-panic + mux-precedence) |
| `src/main.go` | Remove lines 171-174 (the two admin mux mounts) and line 129 (the "Admin API:" banner line). Keep `net/http` imports only as still needed |
| `src/dashboard/render.go` | `renderInfoDetail` (~line 345-348): drop the Admin segment → "Log dir: X" only. Wide-layout config line (~439-448): drop "Admin: ..." from `configLine`; UXR-005 narrow-terminal logic that drops the admin URL becomes moot — the line now has no admin URL to drop (check: configLine may no longer need the width branch) |
| `src/dashboard/dashboard_test.go` | `TestModelHeaderConfig` (line ~719): remove the `"http://localhost:8080/_admin/endpoints"` assertion. `TestModelHeaderConfigNarrow`-style tests (~1501-1514): update width-80/79 assertions to expect NO Admin text at both widths. Line 735 comment ("admin API") → reword to "runtime registry add" |
| `src/dashboard/model.go` | Comment-only: line ~603 mentions "admin" in a config-lines comment — reword if it breaks accuracy |

**Do NOT delete / do NOT change:**
- `TestModelHeaderEndpointRefresh` (dashboard_test.go:730) — it uses `fakeEndpoints`, not the
  admin API; the runtime-refresh behavior it tests is still real (watcher path).
- `mock.Registry` CRUD methods (`Add/Update/Remove/Get/List`) — still used by the watcher.
- `07-admin-handler.md`, `04-backlog.md` WS-014/ADM-001, `contracts.md` note — retained as the
  reintroduction contract (docs are already updated; no further doc work in this story).

## 4. Non-goals

- No input-validation patch to the admin handler (removal supersedes it).
- No CLI subcommand replacement.
- No change to watcher/stub/handler/logging/stats behavior.
- `.ai/` docs already updated 2026-08-05 — do not re-touch.

## 5. Test plan

1. `go test -race -count=1 ./...` — green after deletions + dashboard test updates.
2. `go vet ./...` — clean.
3. `rg -n 'NewAdminHandler|AdminEndpointsPath|_admin' src/` — no hits.
4. Manual: `go run . --tui off` → banner has no Admin API line; curl `GET /_admin/endpoints`
   → 404 + an `_unmatched.log` entry. `go run .` on a TTY → dashboard shows no Admin URL at
   both wide (≥100 cols) and narrow widths.
