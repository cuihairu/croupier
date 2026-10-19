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

在游戏服务器上一条命令安装 agent：自动识别操作系统与 CPU 架构，从
GitHub Releases（匿名可下，无需 token）下载对应 `croupier-bin-<os>-<arch>` 产物，
只安装其中的所选 agent，装完以 `-version` 自校验。幂等——重复执行即覆盖升级。

**三类 agent 统一入口**（`--agent` 选型；选型依据见
[三类 Agent 选型（本节仅有中文版）](/zh/operations/agent-comparison)）：

| `--agent`         | 二进制           | systemd 服务             | 配置文件                           |
| ----------------- | ---------------- | ------------------------ | ---------------------------------- |
| `sidecar`（默认） | `croupier-agent` | `croupier-agent`         | `/etc/croupier/agent.yaml`         |
| `devops`          | `devops-agent`   | `croupier-devops-agent`  | `/etc/croupier/devops-agent.yaml`  |
| `capture`         | `capture-agent`  | `croupier-capture-agent` | `/etc/croupier/capture-agent.yaml` |

```bash
# sidecar（业务旁挂，历史默认行为不变）
curl -fsSL https://raw.githubusercontent.com/cuihairu/croupier/main/scripts/install.sh | bash -s --
# devops（内网堡垒机式远程接入）
curl -fsSL https://raw.githubusercontent.com/cuihairu/croupier/main/scripts/install.sh | sudo bash -s -- --agent devops --server gm.example.com:19090 --game-id demo --with-service
# capture（CDC 数据变更捕获）
curl -fsSL https://raw.githubusercontent.com/cuihairu/croupier/main/scripts/install.sh | sudo bash -s -- --agent capture --server gm.example.com:19090 --game-id demo --with-service
```

**离线/内网安装**（`--local`，干净环境验证同一入口）：直接取目录下的二进制
或官方归档，跳过 GitHub 下载：

```bash
bash scripts/install.sh --agent capture --local ./dist --bin-dir /tmp/bin --config-dir /tmp/cfg --silent
```

**生成初始配置**：无论是否 `--with-service`，脚本都会生成所选 agent 的初始
配置（已存在则保留不改；含 `--server`/`--agent-id`/`--game-id`/`--env` 的值，
缺省 id 为 `<role>-主机短名`）。三个 agent 的配置加载均无 `${ENV}` 展开层，
含密码的配置保持 `root:0600`；逐字段说明见
[Agent 配置逐字段全解](./agent-config)。

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

> Windows 的 `install.ps1` 目前仅覆盖 sidecar（`croupier-agent`）；devops /
> capture 的 Windows 安装暂走手动部署。

远程管道执行不便传参时可用环境变量：`CROUPIER_INSTALL_AGENT` /
`CROUPIER_INSTALL_VERSION` / `CROUPIER_INSTALL_LOCAL_DIR` /
`CROUPIER_INSTALL_BIN_DIR` / `CROUPIER_INSTALL_CONFIG_DIR` /
`CROUPIER_INSTALL_WITH_SERVICE=1` / `CROUPIER_INSTALL_SERVER` /
`CROUPIER_INSTALL_AGENT_ID` / `CROUPIER_INSTALL_GAME_ID` /
`CROUPIER_INSTALL_ENV` / `CROUPIER_INSTALL_SILENT=1`。

## 版本通道

Nightly 由 `nightly.yml` 唯一更新；独立 Agent 归档、checksum、平台范围和能力成熟度见 [职能与构建产物（本节仅有中文版）](/zh/operations/agent-comparison)。发布链修改合并并运行成功后，才可从 Release 下载新增独立包；原安装器继续从总包选择角色。ARMv7 仅 sidecar/devops，capture 需使用 amd64/arm64，不能把归档缺少 capture 当作已支持平台。

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

脚本只安装所选 agent；server（控制面）请另行部署，见
[部署形态总览](./deploy-overview)。

## 注册开机自启服务

```bash
# Linux（systemd，需 root）：自动生成 /etc/croupier/<agent>.yaml（已存在则保留）
curl -fsSL .../scripts/install.sh | sudo bash -s -- --with-service
curl -fsSL .../scripts/install.sh | sudo bash -s -- --agent devops --server gm.example.com:19090 --game-id demo --with-service
curl -fsSL .../scripts/install.sh | sudo bash -s -- --agent capture --server gm.example.com:19090 --game-id demo --with-service

# macOS：root 装系统级 LaunchDaemon，普通用户装用户级 LaunchAgent
curl -fsSL .../scripts/install.sh | bash -s -- --with-service
```

- Linux 单元：`systemctl status croupier-agent`（devops/capture 为
  `croupier-devops-agent` / `croupier-capture-agent`），日志
  `journalctl -u <服务名> -f`。
- macOS：`launchctl print system/com.github.cuihairu.croupier.agent`（或
  `gui/$UID/...`；devops/capture 为 `...croupier.devops-agent` /
  `...croupier.capture-agent`），日志在 `/tmp/com.github.cuihairu.croupier.*.log`。

Windows 需以管理员运行并先备好配置：

```powershell
& ([scriptblock]::Create((irm .../scripts/install.ps1))) -WithService -ConfigPath C:\croupier\agent.yaml
```

## 卸载

```bash
curl -fsSL .../scripts/install.sh | bash -s -- --uninstall                      # 卸 sidecar（默认）
curl -fsSL .../scripts/install.sh | sudo bash -s -- --agent devops --uninstall
curl -fsSL .../scripts/install.sh | sudo bash -s -- --agent capture --uninstall
```

```powershell
& ([scriptblock]::Create((irm .../scripts/install.ps1))) -Uninstall
```

停止并移除对应服务、删除二进制；配置文件与运行数据保留，需清理请手动删除。

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
交叉编译参数本地复现：`CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ./cmd/sidecar-agent`。

> **定名注（2026-10-09 用户令；2026-10-11 方案 A 落地）**：源码目录 `cmd/sidecar-agent`，产物二进制名保持 `croupier-agent`（部署契约零影响）——目录即定名 **`sidecar-agent`**，见 [Agent 清单（本节仅有中文版）](/zh/design/agents-inventory) §9。

## 排错

- **命令找不到**：安装目录不在 PATH（脚本会打印提示行）；或重开终端让 Windows
  用户 PATH 生效。
- **404 下载失败**：该架构产物未发布（见上表覆盖面）或网络不通；产物全清单见
  [Releases](https://github.com/cuihairu/croupier/releases)。
- **已发布架构之外的平台**（如 windows-arm64、linux riscv64）：Go 交叉编译自建
  （命令见上节），产物本身是纯 Go 静态二进制。
- **服务起不来**：多半是配置里上游 Server 地址/证书未配，逐字段说明见
  [Agent 配置逐字段全解](./agent-config)；sidecar 的既有全解见
  [Agent 配置全解](./config-agent)。
