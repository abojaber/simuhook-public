---
title: "Installation"
type: docs
weight: 1
---

# Installation

## Download a Binary

Pre-built binaries are available for the following platforms:

| Platform | Architecture |
|----------|--------------|
| macOS | Apple Silicon (arm64) |
| macOS | Intel (amd64) |
| Linux | arm64 |
| Linux | amd64 |

Download the binary for your platform, extract it, make it executable, and move it somewhere on your `PATH`:

```bash
chmod +x simuhook-darwin-arm64
mv simuhook-darwin-arm64 /usr/local/bin/simuhook
```

## Build from Source

Requires [Go 1.21+](https://go.dev/dl/).

```bash
git clone https://github.com/abojaber/simuhook
cd simuhook
go build -o simuhook ./src
```

This produces the `simuhook` binary in the current directory.

## Verify

```bash
simuhook --version
# → dev (development build)
```

Release builds print the release version (e.g. `v0.1.0`).

## Future: `go install`

A `go install` package will be available once Simuhook is published and released.