---
title: One-Command Agent Installation
icon: download
order: 4
category:
  - Operations Manual
tag:
  - Deployment
  - Agent
---

# One-Command Agent Installation

Install the agent on a game server with a single command: it auto-detects the operating system and CPU architecture, downloads the matching `croupier-bin-<os>-<arch>` artifact from GitHub Releases (anonymous download, no token needed),
installs only the selected agent from it, and self-verifies with `-version` after installation. Idempotent — re-running it overwrites and upgrades.

**Unified entry point for the three agent types** (choose with `--agent`; see
Three Agent Types Comparison (Chinese-only section) for the selection rationale):

| `--agent`           | Binary           | systemd service          | Config file                        |
| ------------------- | ---------------- | ------------------------ | ---------------------------------- |
| `sidecar` (default) | `croupier-agent` | `croupier-agent`         | `/etc/croupier/agent.yaml`         |
| `devops`            | `devops-agent`   | `croupier-devops-agent`  | `/etc/croupier/devops-agent.yaml`  |
| `capture`           | `capture-agent`  | `croupier-capture-agent` | `/etc/croupier/capture-agent.yaml` |

```bash
# sidecar (business sidecar; historical default behavior unchanged)
curl -fsSL https://raw.githubusercontent.com/cuihairu/croupier/main/scripts/install.sh | bash -s --
# devops (bastion-style remote access over the internal network)
curl -fsSL https://raw.githubusercontent.com/cuihairu/croupier/main/scripts/install.sh | sudo bash -s -- --agent devops --server gm.example.com:19090 --game-id demo --with-service
# capture (CDC data-change capture)
curl -fsSL https://raw.githubusercontent.com/cuihairu/croupier/main/scripts/install.sh | sudo bash -s -- --agent capture --server gm.example.com:19090 --game-id demo --with-service
```

**Offline/intranet installation** (`--local`; verifies the same entry point in a clean environment): use binaries or official archives from a local directory directly, skipping the GitHub download:

```bash
bash scripts/install.sh --agent capture --local ./dist --bin-dir /tmp/bin --config-dir /tmp/cfg --silent
```

**Initial config generation**: whether or not `--with-service` is passed, the script generates the
initial configuration for the selected agent (kept as-is if it already exists; includes values from `--server`/`--agent-id`/`--game-id`/`--env`,
with the default id being `<role>-<short hostname>`). Config loading for all three agents has no `${ENV}` expansion layer;
configs containing passwords stay `root:0600`. For field-by-field explanations see
Agent Configuration Field-by-Field Reference.

For manual item-by-item deployment (directory layout, production systemd unit templates), see Binary Deployment.

## One-Command Install

**Linux** (x86_64 / ARM64 / ARMv7):

```bash
curl -fsSL https://raw.githubusercontent.com/cuihairu/croupier/main/scripts/install.sh | bash -s --
```

**macOS** (Intel / Apple Silicon):

```bash
curl -fsSL https://raw.githubusercontent.com/cuihairu/croupier/main/scripts/install.sh | bash -s --
```

**Windows** (PowerShell 5.1+, x64):

```powershell
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/cuihairu/croupier/main/scripts/install.ps1)))
```

> The Windows `install.ps1` currently covers only the sidecar (`croupier-agent`); Windows
> installation of devops / capture still goes through manual deployment for now.

When piping remotely makes passing arguments inconvenient, use environment variables: `CROUPIER_INSTALL_AGENT` /
`CROUPIER_INSTALL_VERSION` / `CROUPIER_INSTALL_LOCAL_DIR` /
`CROUPIER_INSTALL_BIN_DIR` / `CROUPIER_INSTALL_CONFIG_DIR` /
`CROUPIER_INSTALL_WITH_SERVICE=1` / `CROUPIER_INSTALL_SERVER` /
`CROUPIER_INSTALL_AGENT_ID` / `CROUPIER_INSTALL_GAME_ID` /
`CROUPIER_INSTALL_ENV` / `CROUPIER_INSTALL_SILENT=1`.

## Version Channels

Nightly is updated solely by `nightly.yml`; for standalone Agent archives, checksums, platform coverage, and capability maturity see Roles and Build Artifacts (Chinese-only section). A release-chain change may only make new standalone packages downloadable from a Release after it is merged and runs successfully; the original installer keeps selecting roles from the monolithic package. ARMv7 supports only sidecar/devops; capture requires amd64/arm64, and a missing capture archive must not be treated as a supported platform.

| Option                       | Meaning                                                                                       |
| ---------------------------- | --------------------------------------------------------------------------------------------- |
| Default (`--version latest`) | Latest **stable release** (GitHub `releases/latest`, prereleases excluded)                    |
| `--version nightly`          | Daily build (auto-produced at 02:00 UTC, all-platform artifacts uniformly `nightly-YYYYMMDD`) |
| `--version v0.1.5`           | Pin a specific version, suitable for canary/rollback                                          |

```bash
# Install the daily build
curl -fsSL .../scripts/install.sh | bash -s -- --version nightly
# Pin a version
curl -fsSL .../scripts/install.sh | bash -s -- --version v0.1.5
```

## Install Locations

- Linux/macOS defaults to `/usr/local/bin`; falls back automatically to
  `~/.local/bin` when it is not writable and passwordless sudo is unavailable (the script prints a hint to add that directory to PATH and does not modify rc files itself). `--bin-dir`
  can be specified explicitly.
- Windows defaults to `%LOCALAPPDATA%\Programs\croupier\bin`, automatically registering the **user-level** PATH;
  `-BinDir` can be specified explicitly.

The script installs only the selected agent; the server (control plane) must be deployed separately, see
Deployment Topologies Overview.

## Registering a Boot-Time Auto-Start Service

```bash
# Linux (systemd, requires root): auto-generates /etc/croupier/<agent>.yaml (kept if it already exists)
curl -fsSL .../scripts/install.sh | sudo bash -s -- --with-service
curl -fsSL .../scripts/install.sh | sudo bash -s -- --agent devops --server gm.example.com:19090 --game-id demo --with-service
curl -fsSL .../scripts/install.sh | sudo bash -s -- --agent capture --server gm.example.com:19090 --game-id demo --with-service

# macOS: root installs a system-level LaunchDaemon, a regular user installs a user-level LaunchAgent
curl -fsSL .../scripts/install.sh | bash -s -- --with-service
```

- Linux unit: `systemctl status croupier-agent` (for devops/capture it is
  `croupier-devops-agent` / `croupier-capture-agent`), logs via
  `journalctl -u <service name> -f`.
- macOS: `launchctl print system/com.github.cuihairu.croupier.agent` (or
  `gui/$UID/...`; for devops/capture it is `...croupier.devops-agent` /
  `...croupier.capture-agent`), logs at `/tmp/com.github.cuihairu.croupier.*.log`.

Windows requires running as administrator and a config prepared in advance:

```powershell
& ([scriptblock]::Create((irm .../scripts/install.ps1))) -WithService -ConfigPath C:\croupier\agent.yaml
```

## Uninstall

```bash
curl -fsSL .../scripts/install.sh | bash -s -- --uninstall                      # Uninstall sidecar (default)
curl -fsSL .../scripts/install.sh | sudo bash -s -- --agent devops --uninstall
curl -fsSL .../scripts/install.sh | sudo bash -s -- --agent capture --uninstall
```

```powershell
& ([scriptblock]::Create((irm .../scripts/install.ps1))) -Uninstall
```

Stops and removes the corresponding service and deletes the binary; config files and runtime data are kept — delete them manually if cleanup is needed.

## Platform Coverage

| Artifact                         | Architecture detection (`uname -m` / `PROCESSOR_ARCHITECTURE`) |
| -------------------------------- | -------------------------------------------------------------- |
| `croupier-bin-linux-amd64`       | `x86_64` / `amd64`                                             |
| `croupier-bin-linux-arm64`       | `aarch64` / `arm64`                                            |
| `croupier-bin-linux-armv7`       | `armv7*` / `armv8l` (armv8 hardware running a 32-bit userland) |
| `croupier-bin-darwin-amd64`      | Intel Mac                                                      |
| `croupier-bin-darwin-arm64`      | Apple Silicon                                                  |
| `croupier-bin-windows-amd64.zip` | `AMD64`                                                        |

Artifacts are produced by the nightly workflow (daily at 02:00 UTC); official releases are published with `v*` tags. The same
cross-compilation parameters reproduce locally: `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ./cmd/sidecar-agent`.

> **Naming note (user decree 2026-10-09; Plan A landed 2026-10-11)**: the source directory is `cmd/sidecar-agent`, and the produced binary name remains `croupier-agent` (zero impact on the deployment contract) — the directory is the definitive name **`sidecar-agent`**, see Agent Inventory (Chinese-only section) §9.

## Troubleshooting

- **Command not found**: the install directory is not in PATH (the script prints a hint line); or reopen the terminal so the Windows
  user PATH takes effect.
- **404 download failure**: the artifact for that architecture was not published (see the coverage table above) or the network is unreachable; for the full artifact list see
  [Releases](https://github.com/cuihairu/croupier/releases).
- **Platforms beyond the published architectures** (e.g. windows-arm64, linux riscv64): cross-compile with Go yourself
  (command in the previous section); the artifact itself is a pure-Go static binary.
- **Service won't start**: most likely the upstream Server address/certificate is not configured; for field-by-field explanations see
  Agent Configuration Field-by-Field Reference; the existing full reference for the sidecar is at
  Agent Configuration Complete Reference.
