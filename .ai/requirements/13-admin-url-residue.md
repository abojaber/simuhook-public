# ADM-003 — Stale admin API URL still rendered in TUI + banner (admin URL residue)

Requirement file owner: BA. Status: `open` (severity Low, 2026-08-06; fix folded into ADM-002). Phase: cleanup/bugfix.

## 0. Origin

Discovered during the BA layout-contract audit (2026-08-06). The admin REST API
(`/_admin/endpoints`) was **removed from the product** by stakeholder decision ADM-002
(2026-08-05) — endpoint management is file-based only. The docs were updated, but the
**code deletion is still pending**, and three UI surfaces still advertise the dead endpoint.

## 1. Root cause

ADM-002's code-deletion scope ("banner line, dashboard admin URL lines + 2 dashboard tests")
has not been applied to the tree. The stale `/_admin/endpoints` URL strings remain hard-coded
in three render paths, plus two dashboard tests assert them.

## 2. Affected locations (actual → expected)

| # | Location | Actual (today) | Expected |
|---|----------|----------------|----------|
| B1 | `src/dashboard/render.go:347-348` — `renderInfoDetail` (wide layout info line) | `Log dir: …  Admin: http://localhost:8080/_admin/endpoints` | `Log dir: …` only (admin removed) |
| B2 | `src/dashboard/render.go:439,447` — `renderHeader` config line (narrow layout, `width >= 80` branch) | `…  Log dir: …  Admin: http://localhost:8080/_admin/endpoints` | admin URL removed from the config line |
| B3 | `src/main.go:129` — non-TUI startup banner `configLines` | `Admin API:  http://localhost:8080/_admin/endpoints` | line removed |

Tests that assert the stale strings (must be updated with the fix):
- `src/dashboard/dashboard_test.go:719` — narrow header view contains `http://localhost:8080/_admin/endpoints`
- `src/dashboard/dashboard_test.go:1505` — wide view contains `Admin: http://localhost:8080/_admin/endpoints`

Note: `src/dashboard/render.go:442-445` already drops the admin URL below 80 cols
("UXR-005 — narrow terminals: drop Admin URL") — only the `>= 80` branch and wide layout
are affected. `src/mock/admin.go` (`AdminEndpointsPath` const) dies with ADM-002 itself.

## 3. Impact

**Severity: Low.** Cosmetic/misleading only — no functional break, no panic, nothing 404s on
the wire (the endpoint simply doesn't exist). But every screen (wide TUI, narrow TUI, and the
non-TUI banner) advertises a removed feature, which is confusing and a support trap.

## 4. Acceptance criteria (fix — part of ADM-002)

```
AC1 — Given the server starts with the TUI disabled
     When  the startup banner prints
     Then  it contains NO "_admin" or "Admin API" text

AC2 — Given the wide TUI layout (width >= 100)
     When  the info line renders
     Then  it shows "Log dir: …" only — no admin URL

AC3 — Given the narrow TUI layout (width >= 80)
     When  the header config line renders
     Then  it shows no admin URL

AC4 — Given the full test suite
     When  it runs after the fix
     Then  all dashboard tests pass with the admin-URL assertions removed/replaced,
           gofmt -l src/ is empty, go vet clean
```

## 5. Related

- Removal decision: `.ai/requirements/11-admin-removal.md`, `.ai/requirements/04-backlog.md`
  (WS-014/ADM-001), `.ai/architecture.md`.
- Developer layout reference: `.ai/tui-dashboard-reference.md` §3 (info line documented as
  `Log dir: …` — the reference reflects the post-fix state).

## 6. Status

- 2026-08-06: found during layout audit; reported by BA; severity Low confirmed by
  stakeholder; recorded here. Fix tracked as part of **ADM-002** (admin code deletion).
  Story status → `open`, moves to `in-progress` when ADM-002 is implemented.
