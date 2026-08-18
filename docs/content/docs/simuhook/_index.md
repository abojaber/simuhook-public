---
title: "Docs"
type: docs
weight: 1
bookCollapseSection: false
---

# Simuhook Documentation

Documentation for developers who use Simuhook to mock webhook endpoints during development.

## Sections

- **[Getting Started]({{< relref "/docs/simuhook/getting-started" >}})** — install, run, and configure Simuhook.
- **[Stubs]({{< relref "/docs/simuhook/stubs" >}})** — the YAML stub language: reference, matching, path patterns, payload matching.
- **[Templates]({{< relref "/docs/simuhook/templates" >}})** — dynamic responses: variables, functions, faker data generators, JSONPath.
- **[Features]({{< relref "/docs/simuhook/features" >}})** — echo mode, response variants, delays, logging.
- **[Dashboard]({{< relref "/docs/simuhook/dashboard" >}})** — the terminal UI: overview, key bindings, filtering.
- **[Examples]({{< relref "/docs/simuhook/examples" >}})** — curl cookbook and REST API simulation patterns.
- **[Changelog]({{< relref "/docs/simuhook/changelog" >}})** — release history.

## What Simuhook Is

A mock HTTP server. You define endpoints as YAML files in a `stubs/` directory. The server watches the directory, matches incoming requests against the configured stubs, and returns your configured responses — status code, headers, body, optional delay. Every request is logged as a JSON line, and an optional full-screen TUI dashboard shows live statistics.

Changes to stub files are applied live. Stop thinking about restarts and start thinking about your integration.