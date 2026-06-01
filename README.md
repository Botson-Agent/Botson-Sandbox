# botson-sandbox

A standalone Go module providing a **gVisor-backed sandbox execution environment** for the [botson-agent](https://github.com/xSaVageAU/botson-agent) project (and any other Go program that needs secure, isolated command execution on Linux).

## What's inside

| File | Purpose |
|---|---|
| `sandbox/manager.go` | `Sandbox` struct — spawns, kills, and manages `runsc` (gVisor) containers |
| `sandbox/manager_linux.go` | Linux-specific `runsc` exec implementation |
| `sandbox/manager_windows.go` | Windows stub (returns `errUnsupported`) |
| `sandbox/rootfs.go` | `RootfsManager` — downloads Alpine Linux minirootfs, unpacks, copies, and manages custom templates |
| `sandbox/target.go` | `Target` interface + `HostTarget` implementation (executes directly on host OS) |
| `sandbox/config.go` | OCI-compliant `config.json` generation for gVisor bundles |
| `sandbox/session.go` | SQLite-backed session persistence (message history) |

## Requirements (Linux only for sandbox mode)

- `runsc` (gVisor) installed at `/usr/local/bin/runsc`
- Unprivileged user namespaces enabled (`kernel.unprivileged_userns_clone = 1`)
- Any Linux amd64 or arm64 host

## Install

```bash
go get github.com/xSaVageAU/botson-sandbox@latest
```

## Usage

```go
import "github.com/xSaVageAU/botson-sandbox/sandbox"

// Create a rootfs manager pointing at your cache dir
rm := sandbox.NewRootfsManager("/home/user/.botson-agent/cache")

// Spawn a new sandbox (Alpine Linux inside gVisor)
mgr := sandbox.NewManager(rm, sandboxID, bundleDir)
if err := mgr.Start(); err != nil {
    log.Fatal(err)
}
defer mgr.Stop()

// Execute a command inside the sandbox
stdout, stderr, code, err := mgr.Exec("echo hello from gVisor")

// Or run on the host directly
host := sandbox.NewHostTarget()
stdout, stderr, code, err = host.Exec("echo hello from host")
```

## Relationship to botson-agent

This module was extracted from `botson-agent/internal/sandbox` so that:
1. The agent core has **zero dependency** on sandbox internals at compile time
2. The sandbox can evolve independently (different gVisor versions, new backends, etc.)
3. Other projects can consume the sandbox without pulling in the full agent

To wire it back into botson-agent, add it as a Go dependency and implement `core.Executor` as a thin wrapper around `*sandbox.Sandbox`.

## License

MIT
