---
title: Agent 一键安装
icon: download
order: 4
category:
  - 运维手册
tag:
  - 部署
  - Agent
---

# Agent 一键安装

一条命令在游戏服上装好 agent：自动探测操作系统与 CPU 架构，从 GitHub Releases 下载匹配的 `croupier-bin-<os>-<arch>` 产物（匿名下载，无需 token），
只安装所选的 agent，并在安装后用 `-version` 自校验。脚本幂等——重复执行即覆盖升级。

**三类 agent 统一入口**（用 `--agent` 选择；选型理由见
三类 Agent 对比（仅中文小节））：

| `--agent`         | 二进制           | systemd 服务             | 配置文件                           |
| ----------------- | ---------------- | ------------------------ | ---------------------------------- |
| `sidecar`（默认） | `croupier-agent` | `croupier-agent`         | `/etc/croupier/agent.yaml`         |
| `devops`          | `devops-agent`   | `croupier-devops-agent`  | `/etc/croupier/devops-agent.yaml`  |
| `capture`         | `capture-agent`  | `croupier-capture-agent` | `/etc/croupier/capture-agent.yaml` |

```bash
# sidecar (business sidecar; historical default behavior unchanged)
curl -fsSL https://raw.githubusercontent.com/cuihairu/croupier/main/scripts/install.sh | bash -s --
# devops (bastion-style remote access over the internal network)
curl -fsSL https://raw.githubusercontent.com/cuihairu/croupier/main/scripts/install.sh | sudo bash -s -- --agent devops --server gm.example.com:19090 --game-id demo --with-service
# capture (CDC data-change capture)
curl -fsSL https://raw.githubusercontent.com/cuihairu/croupier/main/scripts/install.sh | sudo bash -s -- --agent capture --server gm.example.com:19090 --game-id demo --with-service
```

**离线/内网安装**（`--local`；在干净环境验证同一入口）：直接用本地目录里的二进制或官方归档，跳过 GitHub 下载：

```bash
bash scripts/install.sh --agent capture --local ./dist --bin-dir /tmp/bin --config-dir /tmp/cfg --silent
```

**初始配置生成**：无论是否传 `--with-service`，脚本都会为所选 agent 生成
初始配置（已存在则原样保留；带入 `--server`/`--agent-id`/`--game-id`/`--env` 的取值，
默认 id 为 `<role>-<短主机名>`）。三类 agent 的配置加载都没有 `${ENV}` 展开层；
含密码的配置保持 `root:0600`。逐字段解释见
Agent 配置逐字段参考。

需要逐项手工部署（目录布局、生产 systemd unit 模板）见二进制部署。

## 一键安装

**Linux**（x86_64 / ARM64 / ARMv7）：

```bash
curl -fsSL https://raw.githubusercontent.com/cuihairu/croupier/main/scripts/install.sh | bash -s --
```

**macOS**（Intel / Apple Silicon）：

```bash
curl -fsSL https://raw.githubusercontent.com/cuihairu/croupier/main/scripts/install.sh | bash -s --
```

**Windows**（PowerShell 5.1+，x64）：

```powershell
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/cuihairu/croupier/main/scripts/install.ps1)))
```

> Windows 的 `install.ps1` 目前只覆盖 sidecar（`croupier-agent`）；devops / capture
> 在 Windows 上暂走手工部署。

远程管道执行不便传参时，用环境变量：`CROUPIER_INSTALL_AGENT` /
`CROUPIER_INSTALL_VERSION` / `CROUPIER_INSTALL_LOCAL_DIR` /
`CROUPIER_INSTALL_BIN_DIR` / `CROUPIER_INSTALL_CONFIG_DIR` /
`CROUPIER_INSTALL_WITH_SERVICE=1` / `CROUPIER_INSTALL_SERVER` /
`CROUPIER_INSTALL_AGENT_ID` / `CROUPIER_INSTALL_GAME_ID` /
`CROUPIER_INSTALL_ENV` / `CROUPIER_INSTALL_SILENT=1`。

## 版本通道

Nightly 只由 `nightly.yml` 更新；单 Agent 归档、checksum、平台覆盖与能力成熟度见角色与构建产物（仅中文小节）。release 链路的变更必须合并且运行成功后，新的独立包才会出现在 Release 可下载；原安装脚本继续从单体包选择角色。ARMv7 只支持 sidecar/devops；capture 要求 amd64/arm64，capture 归档缺失不得视为受支持平台。

| 选项                       | 含义                                                              |
| -------------------------- | ----------------------------------------------------------------- |
| 默认（`--version latest`） | 最新**正式 release**（GitHub `releases/latest`，不含 prerelease） |
| `--version nightly`        | 每日构建（02:00 UTC 自动产出，全平台产物统一 `nightly-YYYYMMDD`） |
| `--version v0.1.5`         | 钉住特定版本，适合金丝雀/回滚                                     |

```bash
# Install the daily build
curl -fsSL .../scripts/install.sh | bash -s -- --version nightly
# Pin a version
curl -fsSL .../scripts/install.sh | bash -s -- --version v0.1.5
```

## 安装位置

- Linux/macOS 默认 `/usr/local/bin`；不可写且无免密 sudo 时自动回落
  `~/.local/bin`（脚本会提示把该目录加入 PATH，不擅自改 rc 文件）。`--bin-dir`
  可显式指定。
- Windows 默认 `%LOCALAPPDATA%\Programs\croupier\bin`，自动注册**用户级** PATH；
  `-BinDir` 可显式指定。

脚本只安装所选 agent；server（控制面）需另行部署，见
部署拓扑概览。

## 注册开机自启服务

```bash
# Linux (systemd, requires root): auto-generates /etc/croupier/<agent>.yaml (kept if it already exists)
curl -fsSL .../scripts/install.sh | sudo bash -s -- --with-service
curl -fsSL .../scripts/install.sh | sudo bash -s -- --agent devops --server gm.example.com:19090 --game-id demo --with-service
curl -fsSL .../scripts/install.sh | sudo bash -s -- --agent capture --server gm.example.com:19090 --game-id demo --with-service

# macOS: root installs a system-level LaunchDaemon, a regular user installs a user-level LaunchAgent
curl -fsSL .../scripts/install.sh | bash -s -- --with-service
```

- Linux unit：`systemctl status croupier-agent`（devops/capture 为
  `croupier-devops-agent` / `croupier-capture-agent`），日志
  `journalctl -u <服务名> -f`。
- macOS：`launchctl print system/com.github.cuihairu.croupier.agent`（或
  `gui/$UID/...`；devops/capture 为 `...croupier.devops-agent` /
  `...croupier.capture-agent`），日志在 `/tmp/com.github.cuihairu.croupier.*.log`。

Windows 需管理员运行并提前备好配置：

```powershell
& ([scriptblock]::Create((irm .../scripts/install.ps1))) -WithService -ConfigPath C:\croupier\agent.yaml
```

## 卸载

```bash
curl -fsSL .../scripts/install.sh | bash -s -- --uninstall                      # Uninstall sidecar (default)
curl -fsSL .../scripts/install.sh | sudo bash -s -- --agent devops --uninstall
curl -fsSL .../scripts/install.sh | sudo bash -s -- --agent capture --uninstall
```

```powershell
& ([scriptblock]::Create((irm .../scripts/install.ps1))) -Uninstall
```

会停止并移除对应服务、删除二进制；配置文件与运行数据保留——需要清理请手工删除。

## 平台覆盖

| 产物                             | 架构探测（`uname -m` / `PROCESSOR_ARCHITECTURE`） |
| -------------------------------- | ------------------------------------------------- |
| `croupier-bin-linux-amd64`       | `x86_64` / `amd64`                                |
| `croupier-bin-linux-arm64`       | `aarch64` / `arm64`                               |
| `croupier-bin-linux-armv7`       | `armv7*` / `armv8l`（armv8 硬件跑 32 位用户态）   |
| `croupier-bin-darwin-amd64`      | Intel Mac                                         |
| `croupier-bin-darwin-arm64`      | Apple Silicon                                     |
| `croupier-bin-windows-amd64.zip` | `AMD64`                                           |

产物由 nightly workflow 产出（每天 02:00 UTC）；正式 release 以 `v*` tag 发布。同样的
交叉编译参数本地可复现：`CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ./cmd/sidecar-agent`。

> **命名说明（2026-10-09 用户令；2026-10-11 方案 A 落地）**：源码目录是 `cmd/sidecar-agent`，产物二进制名仍为 `croupier-agent`（对部署契约零影响）——目录才是定名 **`sidecar-agent`**，见 Agent 清单（仅中文小节）§9。

## 故障排查

- **命令找不到**：安装目录不在 PATH（脚本会打印提示行）；或重开终端让 Windows
  用户 PATH 生效。
- **404 下载失败**：该架构的产物未发布（见上方覆盖表）或网络不可达；产物全列表见
  [Releases](https://github.com/cuihairu/croupier/releases)。
- **超出已发布架构的平台**（如 windows-arm64、linux riscv64）：用 Go 自行交叉编译
  （命令见上一节）；产物本身是纯 Go 静态二进制。
- **服务起不来**：多半是上游 Server 地址/证书未配置；逐字段解释见
  Agent 配置逐字段参考；sidecar 已有全量参考见
  Agent 配置完整参考。
