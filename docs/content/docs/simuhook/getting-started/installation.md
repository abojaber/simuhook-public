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
| Windows | amd64 |

Download the binary for your platform, extract it, make it executable, and move it somewhere on your `PATH`:

```bash
chmod +x simuhook-darwin-arm64
mv simuhook-darwin-arm64 /usr/local/bin/simuhook
```

## Build from Source

Requires [Go 1.21+](https://go.dev/dl/).

```bash
git clone https://github.com/abojaber/simuhook-public
cd simuhook-public
go build -o simuhook .
```

This produces the `simuhook` binary in the current directory.

## Verify

```bash
simuhook --version
# → dev (development build)
```

Release builds print the release version (e.g. `v0.9.0`).

## Install via `go install`

```bash
go install github.com/abojaber/simuhook-public@v0.9.0
```

This installs the `simuhook` binary into your `GOBIN` (default `~/go/bin`). Make sure `GOBIN` is on your `PATH`.