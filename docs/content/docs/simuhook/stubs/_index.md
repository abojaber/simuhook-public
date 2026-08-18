---
title: "Stubs"
type: docs
weight: 20
bookCollapseSection: true
---

# Stubs

Stubs are YAML files that define your mock endpoints. Each `.yaml`/`.yml` file in the stubs directory defines **one** endpoint. Files are loaded at startup and hot-reloaded on change — save the file and the server picks it up, no restart.

| Page | What it covers |
|------|----------------|
| [YAML Reference]({{< relref "/docs/simuhook/stubs/yaml-reference" >}}) | Every field in the stub format, with examples. |
| [Matching]({{< relref "/docs/simuhook/stubs/matching" >}}) | How Simuhook picks which stub handles a request. |
| [Path Patterns]({{< relref "/docs/simuhook/stubs/path-patterns" >}}) | Exact paths, globs, and regexes. |
| [Payload Matching]({{< relref "/docs/simuhook/stubs/payload-matching" >}}) | Exact and partial-JSON body matching. |