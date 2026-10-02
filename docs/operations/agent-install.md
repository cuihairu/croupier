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

在游戏服务器上一条命令安装 `croupier-agent`：自动识别操作系统与 CPU 架构，从
GitHub Releases（匿名可下，无需 token）下载对应 `croupier-bin-<os>-<arch>` 产物，
只安装其中的 agent，装完以 `--version` 自校验。幂等——重复执行即覆盖升级。

手动逐项部署（目录规划、生产 systemd 单元模板）见[二进制部署](./deploy-binary)。

## 一条命令安装

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

远程管道执行不便传参时可用环境变量：`CROUPIER_INSTALL_VERSION` /
`CROUPIER_INSTALL_BIN_DIR` / `CROUPIER_INSTALL_WITH_SERVICE=1`。

## 版本通道

| 参数                       | 含义                                                              |
| -------------------------- | ----------------------------------------------------------------- |
| 默认（`--version latest`） | 最新**稳定版**（GitHub `releases/latest`，不含 prerelease）       |
| `--version nightly`        | 每日构建（02:00 UTC 自动产出，全平台产物统一 `nightly-YYYYMMDD`） |
| `--version v0.1.5`         | 钉定具体版本，适合灰度/回滚                                       |

```bash
# 装每日构建
curl -fsSL .../scripts/install.sh | bash -s -- --version nightly
# 钉定版本
curl -fsSL .../scripts/install.sh | bash -s -- --version v0.1.5
```

## 安装位置

- Linux/macOS 默认 `/usr/local/bin`；不可写且无免密 sudo 时自动回退
  `~/.local/bin`（脚本会提示把该目录加入 PATH，不自动改 rc 文件）。`--bin-dir`
  可显式指定。
- Windows 默认 `%LOCALAPPDATA%\Programs\croupier\bin`，自动注册**用户级** PATH；
  `-BinDir` 可显式指定。

脚本只安装 `croupier-agent`；server（控制面）请另行部署，见
[部署形态总览](./deploy-overview)。

## 注册开机自启服务

```bash
# Linux（systemd，需 root）：自动落 /etc/croupier/agent.yaml（已存在则保留）
curl -fsSL .../scripts/install.sh | sudo bash -s -- --with-service

# macOS：root 装系统级 LaunchDaemon，普通用户装用户级 LaunchAgent
curl -fsSL .../scripts/install.sh | bash -s -- --with-service
```

- Linux 单元：`systemctl status croupier-agent`，配置在
  `/etc/croupier/agent.yaml`（模板取自同版本 tag 的 `configs/agent.yaml`）。
- macOS：`launchctl print system/com.github.cuihairu.croupier.agent`（或
  `gui/$UID/...`），日志在 `/tmp/com.github.cuihairu.croupier.agent*.log`。

Windows 需以管理员运行并先备好配置：

```powershell
& ([scriptblock]::Create((irm .../scripts/install.ps1))) -WithService -ConfigPath C:\croupier\agent.yaml
```

## 卸载

```bash
curl -fsSL .../scripts/install.sh | bash -s -- --uninstall
```

```powershell
& ([scriptblock]::Create((irm .../scripts/install.ps1))) -Uninstall
```

停止并移除服务、删除二进制；`agent.yaml` 配置与运行数据保留，需清理请手动删除。

## 平台覆盖面

| 产物                             | 架构识别（`uname -m` / `PROCESSOR_ARCHITECTURE`） |
| -------------------------------- | ------------------------------------------------- |
| `croupier-bin-linux-amd64`       | `x86_64` / `amd64`                                |
| `croupier-bin-linux-arm64`       | `aarch64` / `arm64`                               |
| `croupier-bin-linux-armv7`       | `armv7*` / `armv8l`（armv8 硬件跑 32 位用户态）   |
| `croupier-bin-darwin-amd64`      | Intel Mac                                         |
| `croupier-bin-darwin-arm64`      | Apple Silicon                                     |
| `croupier-bin-windows-amd64.zip` | `AMD64`                                           |

产物由 nightly 工作流产出（每日 02:00 UTC）；正式版随 `v*` tag 发布。同套
交叉编译参数本地复现：`CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ./cmd/agent`。

## 排错

- **命令找不到**：安装目录不在 PATH（脚本会打印提示行）；或重开终端让 Windows
  用户 PATH 生效。
- **404 下载失败**：该架构产物未发布（见上表覆盖面）或网络不通；产物全清单见
  [Releases](https://github.com/cuihairu/croupier/releases)。
- **已发布架构之外的平台**（如 windows-arm64、linux riscv64）：Go 交叉编译自建
  （命令见上节），产物本身是纯 Go 静态二进制。
- **服务起不来**：多半是 `/etc/croupier/agent.yaml` 里上游 Server 地址/证书未配，
  见 [Agent 配置全解](./config-agent)。
