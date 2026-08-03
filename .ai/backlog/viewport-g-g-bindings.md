# `g`/`G` viewport bindings

**Title:** Bind `g` / `G` (top / bottom) on the dashboard viewports.

**Status:** `backlog` (not started). **Priority:** `[Could]`. **Origin:** pre-existing; referenced in UXR-013 / WS-018 risks ("g/G viewport bindings — pre-existing, unrelated").

## Description

The two-column dashboard ADR and help docs reference `g`/`G` as stream-viewport bindings ("gg scroll to top, G scroll to bottom"), but these are **not bound** in bubbles v1.0.0 — the ADR/doc previously were corrected and the binding backlogged. `g`/`G` are a convenience for jumping to the top/bottom of a long feed without key-repeating.

## Architecture / approach

- bubbles v1.0.0 `bubbles/viewport` doesn't ship `g`/`G` natively. Implement as explicit `KeyMsg` handling in `model.Update` for the stream viewport, mapping `g` → `YOffset = 0` and `G` → `YOffset = max` (`AtBottom` via `GotoBottom`).
- Must not collide with existing bindings: `g` is free today; verify `G` is distinct (shift+g) and not used by the endpoints panel.
- Keep the binding only when not prompting and when inspect/expansion state permits (row pinning could conflict — decide: `G` clears expansion? Document choice).
- If the behavior diverges from the documented intent, update the help text and ADR rather than force a colliding key.

## Files to create / edit

- `src/dashboard/model.go` — `g`/`G` key handling.
- `src/dashboard/render.go` — help text.
- `.ai/architecture.md` — resolve the doc/binding mismatch.
- Tests + a doc/help-width check (help lines must stay 63 cells).

## Steps

- [ ] Confirm key-map collision scan (g/G vs all model + viewport keys).
- [ ] Implement g/G on stream viewport.
- [ ] Decide expansion interplay with G.
- [ ] Update help + ADR.
- [ ] Tests (g → top, G → bottom, expansion + g/G, help width preserved).

## Notes

Unrelated to WS-018; existed before the inspector work. It's listed here for completeness in the Risks & edges traceability.