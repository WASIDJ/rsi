# AGENTS.md - rsi Developer & AI Agent Guide

Welcome to `rsi`! This document is designed to give AI coding assistants and developers immediate mental models, architectural clarity, critical invariants, and command recipes to work on this repository safely and effectively.

---

## 1. Project Overview & Mental Model

`rsi` is a lightweight, zero-dependency Go CLI tool designed to manage **Mihomo (Clash Meta)** on routers (specifically ASUS RT-AX86U / Merlin / Linux ARM64) and locally from macOS.

### Dual-Mode Architecture
The exact same binary runs in two modes depending on its environment:
1. **Router Native Mode**: If `/jffs` exists, `rsi` runs natively on the router, manipulating `/jffs/mihomo` files and interacting with the local Mihomo REST API (`http://192.168.50.1:9090`).
2. **macOS Remote Mode**: If `/jffs` does not exist, `rsi` acts as a remote CLI client, automatically proxying user commands to the router via SSH (`ssh -t $TARGET /jffs/mihomo/rsi ...`).

---

## 2. Directory & Component Map

```text
rsi/
├── cmd/
│   └── rsi/
│       └── main.go       # Entry point: Environment detection (dual-mode) & CLI subcommands
├── pkg/
│   ├── hy2/
│   │   ├── hy2.go        # Hysteria 2 URI (hysteria2://) parser into Mihomo proxy map
│   │   └── hy2_test.go   # Unit tests for URI parsing and query parameters
│   ├── sub/
│   │   └── sub.go        # HTTP downloader for airport subscriptions with gzip decompression
│   ├── api/
│   │   └── api.go        # Client for Mihomo REST API (PUT /configs, GET /version)
│   └── config/
│       └── config.go     # Config merger, node injection engine, and validation pipeline
├── Formula/
│   └── rsi.rb            # Official Homebrew formula (installed via brew install WASIDJ/rsi/rsi)
├── .github/
│   └── workflows/
│       └── release.yml   # Multi-arch release pipeline (darwin/arm64, darwin/amd64, linux/arm64, linux/amd64)
├── Makefile              # Developer task runner
└── go.mod / go.sum       # Minimal dependencies: only gopkg.in/yaml.v3
```

---

## 3. Critical Invariants (Must NEVER Break)

When editing or extending this codebase, adhere strictly to these rules:

### A. Cross-Device Link (EXDEV) Safety
- `/jffs` is physical NAND flash (`/dev/mtdblock9`), whereas `/tmp` is a RAM disk (`tmpfs`).
- In Linux, `os.Rename` across two different mount points fails with `EXDEV: invalid cross-device link`.
- **Rule**: Staging and candidate files must ALWAYS be written on the same filesystem before renaming (`/jffs/mihomo/config.yaml.new` -> `/jffs/mihomo/config.yaml`).

### B. Mihomo Safe-Path Constraint
- Mihomo is started with home directory `-d /tmp/mihomo`.
- The REST API `PUT /configs?force=true` rejects any configuration file path not located inside `/tmp/mihomo`.
- **Rule**: Never pass `/jffs/mihomo/config.yaml` directly to the API. Always use the symlink `/tmp/mihomo/config.yaml` (`ln -sf /jffs/mihomo/config.yaml /tmp/mihomo/config.yaml`).

### C. Custom Node Injection Hierarchy
- When custom nodes (e.g. Hysteria 2) are loaded from `/jffs/mihomo/custom_nodes.yaml`:
  1. Append nodes to `proxies`.
  2. Prepend a dedicated selector group `⚡ 自建节点` at index 0 of `proxy-groups`.
  3. In all outbound routing groups (`select`, `fallback`, `url-test`, `load-balance`), prepend the custom node names to the candidate list.
  4. Never remove or overwrite airport proxy groups or existing rules.

### D. Zero-Downtime Hot Reload
- Config updates must execute via Mihomo's REST API `PUT /configs?force=true` to preserve live TCP/UDP connections.
- Only fall back to `/jffs/mihomo/service.sh restart` if the API call fails or if the process is stopped.

---

## 4. Common Developer & Agent Commands

```sh
# 1. Run unit tests
make test
# or: go test -v ./...

# 2. Build local macOS binary
make build
# Output: bin/rsi

# 3. Cross-compile for router (Linux ARM64)
make router-arm64
# Output: bin/rsi-linux-arm64

# 4. Compile and deploy directly to router
make deploy
# or: bin/rsi router deploy

# 5. Clean build artifacts
make clean
```

---

## 5. CLI Command Reference & Data Flow

| Command | Action & Files Affected |
| :--- | :--- |
| `rsi sub set <URL>` | Downloads subscription -> saves to `/jffs/mihomo/subscription.yaml` & `sub.url` -> merges custom nodes -> pre-validates with `mihomo -t` -> replaces `config.yaml` -> API reload. |
| `rsi sub update` | Reads URL from `sub.url` and re-runs the subscription merge pipeline. |
| `rsi node add <hy2_url>` | Parses URI -> updates `/jffs/mihomo/custom_nodes.yaml` -> rebuilds `config.yaml` -> API reload. |
| `rsi node list` | Pretty-prints all entries from `/jffs/mihomo/custom_nodes.yaml`. |
| `rsi node rm <name>` | Removes node from `custom_nodes.yaml` -> rebuilds `config.yaml` -> API reload. |
| `rsi reload` | Re-merges cached subscription with custom nodes and calls API `PUT /configs`. |
| `rsi status` | Reads `/tmp/mihomo/core.pid`, `/proc/<pid>/status` (VmRSS, Threads), and queries `GET /version`. |
| `rsi log [-n lines] [-f]` | Tails `/tmp/mihomo/core.log` (RAM-bounded log file). |
