# Phase 04 — Backlog

> Future ideas. Could-have and won't-have features.

---

## WS-010 — Interactive TUI dashboard

**Status:** `draft` | **Priority:** `[Could]`

**User Story:**
> As a developer, I want an interactive terminal dashboard showing live request counts, response times, and recent logs so that I can monitor the simulator in real time during demos.

> **Note:** A simpler CLI dashboard (WS-016) was already implemented — boxed startup banner + per-request log lines. This story covers a **full interactive TUI** with real-time screen updates, charts, hit-rate visualization, and keyboard navigation.

Leveraging `bubbletea` or similar Go TUI framework. The existing `StatsTracker` and ring buffer (50 entries) can be reused as the data source.

---

## WS-011 — Webhook forwarding / proxy mode

**Status:** `draft` | **Priority:** `[Could]`

**User Story:**
> As a developer, I want the simulator to forward incoming requests to a real upstream service and log both the request and the upstream response so that I can use it as an inspection proxy.

---

## WS-012 — WebSocket / SSE endpoint simulation

**Status:** `draft` | **Priority:** `[Could]`

**User Story:**
> As a developer, I want to simulate WebSocket or SSE endpoints so that I can test real-time integrations.

---

## WS-013 — Built-in webhook templates (Stripe, GitHub, Slack)

**Status:** `draft` | **Priority:** `[Won't]`

**User Story:**
> As a developer, I want pre-built webhook templates for popular services.

*Rejected — stakeholder explicitly requested custom/dynamic webhooks only. No service-specific presets.*

---

## WS-014 — REST API to manage config at runtime

**Status:** `draft` | **Priority:** `[Won't]`

**User Story:**
> As a developer, I want a REST API to CRUD webhook endpoints at runtime.

*Rejected — stakeholder explicitly said CLI-only. Config is file-based.*