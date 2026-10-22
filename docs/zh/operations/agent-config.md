# Agent 配置逐字段完整参考（sidecar / devops / capture）

本文只覆盖既有三类入口；第四类 synthetics 见四类 Agent 指南（本节仅中文）了解其职责与未实现边界——它当前没有可运行配置或发布包。下文 devops/capture 的 `env` 可以为空是代码现状；部署时必须显式填 gameId/env，强制校验在挂账清单。

三类 agent 各有一份 YAML 配置文件，加载链路极简：**读文件 → yaml.Unmarshal →
validate → run**。本文逐字段列出字段名、类型、默认值、含义与代码读取点
（`file:line`，与当前主线对齐；每个字段在代码里都有真实读取点——没有臆造字段）。
选型见 agent-comparison.md（本节仅中文）；安装见
agent-install.md。

**三条公共约定（三类 agent 一致）：**

1. **无 `${ENV}` 展开层**：配置加载是纯 `yaml.Unmarshal`，环境变量占位符
   **不会**展开。含密码的配置直接写值并保持 `root:0600`，或由部署侧模板渲染后下发。**密钥绝不进仓库**——
   示例中密码一律留空，证书只给文件路径引用。
2. **键名 lowerCamelCase**（`serverAddr`、`heartbeatFile`）；sidecar 额外接受
   旧式 PascalCase/snake_case 兼容键（见表注），但新配置必须用规范键——
   兼容键不是文档标准。
3. **出站 TLS 三段式**：`enabled=false` 时证书字段全部忽略（明文连接）；
   `enabled=true` 时字段原样进入 TLS 客户端配置。生产全部建议开启并配 `caFile`。

代码路径约定：下文 `devops/config/config.go` 指
`agents/devops/config/config.go`，`root.go` 指 `cmd/sidecar-agent/root.go`。

---

## sidecar（croupier-agent）

加载链路：`--config` 文件 → `yaml.Unmarshal`（`root.go:482-485`，旧键兼容
`root.go:173-241`）→ 默认值（`applyAgentConfigDefaults`，`root.go:450-466`）→
严格校验（`root.go:486-490`：`agent.gameId` 与 `agent.env` 非空，否则启动失败）。

### `agent.*`（root.go:301-313）

| 字段                    | 类型               | 默认                                    | 说明                                                         | 读取点                                                                            |
| ----------------------- | ------------------ | --------------------------------------- | ------------------------------------------------------------ | --------------------------------------------------------------------------------- |
| `agent.id`              | string             | 空 → 自动 `<hostname>-<unixts>`         | 注册用唯一 agent id                                          | `root.go:558` → `resolveAgentID`（root.go:662-673）                               |
| `agent.gameId`          | string             | 空（**必填**）                          | 游戏域标签；会话绑定唯一 (gameId, env)                       | 校验 `root.go:486-490`；随注册上报 `root.go:574`                                  |
| `agent.env`             | string             | 空（**必填**）                          | 环境标签（dev/prod/…）                                       | 同上，`root.go:575`                                                               |
| `agent.localAddr`       | string             | 空 → 回落 `host`/`port`，再 `0.0.0.0:0` | 本地 TCP 网关：SDK Provider 注册函数的入口                   | `root.go:545-556`（剥离 `tcp://` 前缀）                                           |
| `agent.labels`          | map<string,string> | 空                                      | 运维自定义标签，与自动系统标签（os/arch/hostname/ip 等）合并 | `root.go:563-565` → 注册载荷                                                      |
| `agent.invokeTimeoutMs` | int                | 0 → 15000                               | agent→Provider 同步调用预算（ms）；单请求 `timeoutMs` 优先   | `root.go:572`；钳制 `internal/agent/local_handler.go:110-121`（≤0→15s，>60s→60s） |
| `agent.transport`       | string             | 空                                      | **DEAD**：解析后无人读取（root.go:305、342-344）             | —                                                                                 |
| `agent.httpAddr`        | string             | 空                                      | **DEAD（行为面）**：只回显进一行启动日志（root.go:494）      | —                                                                                 |

### `server.*`（root.go:243-252）——上游控制面

| 字段                                       | 类型   | 默认                                 | 说明                                                                             | 读取点                                                                      |
| ------------------------------------------ | ------ | ------------------------------------ | -------------------------------------------------------------------------------- | --------------------------------------------------------------------------- |
| `server.addr`                              | string | 空 = **不连上游**（local-only 模式） | Croupier Server 控制面 `host:port`（约定 19090）                                 | `root.go:568`；为空时跳过并记日志，`internal/app/agent/upstream.go:258-261` |
| `server.transport`                         | string | 空 → `tcp`                           | 传输类型；**只有 `tcp` 合法**，其他值 panic                                      | `root.go:570` → `upstream.go:150-160`                                       |
| `server.insecure`                          | bool   | false（显式 true = 明文连接）        | false 时用下方证书字段构建 TLS                                                   | `root.go:586-608`                                                           |
| `server.caFile`                            | string | 空                                   | 校验 server 证书的 CA；设置了但文件缺失时**自动生成 dev CA** 到配置目录 `certs/` | `root.go:586-596` → `internal/devcert/devcert.go:33-58`                     |
| `server.tlsCertFile` / `server.tlsKeyFile` | string | 空                                   | mTLS 客户端证书/私钥                                                             | `root.go:600-601`                                                           |
| `server.serverName`                        | string | 空                                   | TLS SNI/主机名校验覆盖                                                           | `root.go:603`                                                               |
| `server.insecureSkipVerify`                | bool   | false                                | 跳过 server 证书链校验                                                           | `root.go:604`                                                               |

### `upstream.*`（root.go:358-363）——心跳与超时

| 字段                         | 类型      | 默认                   | 说明                                                                                   | 读取点                                                                 |
| ---------------------------- | --------- | ---------------------- | -------------------------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| `upstream.heartbeatInterval` | int（秒） | 0 → 内核 3s            | 控制面心跳周期                                                                         | `root.go:582` → `upstream.go:201-204` → `core/register/register.go:39` |
| `upstream.timeout`           | int（ms） | 0 → 请求 10s / 心跳 3s | 上游拨号与请求超时                                                                     | `root.go:580-581`（仅 >0 生效）                                        |
| `upstream.retryInterval`     | int       | 0                      | **DEAD**：解析后无人读取；重连退避固定 5s→60s×1.5（`core/register/register.go:44-45`） | —                                                                      |
| `upstream.maxRetries`        | int       | 0                      | **DEAD**：同上；内核失败阈值默认 2（register.go:42）                                   | —                                                                      |

### `log.*`（= core/logx.LogConfig，core/logx/logging.go:18-27）

| 字段                                                 | 类型             | 默认值                | 说明                                                                                                                                                                        | 读取点                                                        |
| ---------------------------------------------------- | ---------------- | --------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------- |
| `log.level`                                          | string           | 空 → `info`           | `debug\|info\|warn\|error`                                                                                                                                                  | `root.go:500-506`                                             |
| `log.format`                                         | string           | 空 → `console`        | `console\|json`                                                                                                                                                             | `root.go:507`                                                 |
| `log.output`                                         | string           | 空 = 仅 stdout        | **在本二进制里是轮转日志文件路径**（传给 `SetupLoggerWithFile` 的 file 参数），不是 `stdout/stderr` 枚举；控制台流由环境变量 `LOG_OUTPUT`/`CROUPIER_LOG_OUTPUT=stderr` 控制 | `root.go:509`；lumberjack 写文件 `core/logx/logging.go:43-51` |
| `log.maxSize` / `maxBackups` / `maxAge` / `compress` | int/int/int/bool | lumberjack 默认 100MB | 轮转参数                                                                                                                                                                    | `root.go:510-513`                                             |
| `log.file`                                           | string           | 空                    | **DEAD（本二进制）**：root.go:506-514 从不读取 `Logging.File`                                                                                                               | —                                                             |

### `tls.*`（root.go:125-132）——本地网关 TLS（SDK→agent）

| 字段                           | 类型   | 默认                          | 说明                                                                | 读取点                                                         |
| ------------------------------ | ------ | ----------------------------- | ------------------------------------------------------------------- | -------------------------------------------------------------- |
| `tls.enabled`                  | bool   | false（整块全零时强制 false） | 加密 `agent.localAddr` 监听；TLS 位于分帧协议之下，SDK 线上协议不变 | `root.go:625`                                                  |
| `tls.certFile` / `tls.keyFile` | string | 空                            | 网关服务端证书；enabled 且文件缺失 = **启动报错**                   | `root.go:627-628` → `internal/transport/tcp/server.go:266-269` |
| `tls.caFile`                   | string | 空                            | 设置即开启 **mTLS**（强制校验客户端证书）                           | `root.go:629` → server.go:237-248                              |
| `tls.insecureSkipVerify`       | bool   | false                         | 服务端降级：取消 mTLS 强制（ClientAuth=NoClientCert）               | `root.go:630` → server.go:250-252                              |
| `tls.serverName`               | string | 空                            | **DEAD**：未传入 server TLS 构建（root.go:626-631）                 | —                                                              |

### `outboundTLS.*`（同形 root.go:125-132）——外拨 Provider TLS

| 字段                                                                        | 说明                                                                 | 读取点                                                        |
| --------------------------------------------------------------------------- | -------------------------------------------------------------------- | ------------------------------------------------------------- |
| `enabled` + `certFile`/`keyFile`/`caFile`/`serverName`/`insecureSkipVerify` | 加密 agent→Provider 外拨连接（无既有 Provider 会话时的回退拨号路径） | `root.go:610-620` → `internal/agent/local_handler.go:176-179` |

### `ops.*`（root.go:93-97）——只有 3 个键可达

| 字段                  | 类型        | 默认                   | 说明                                                                                                                                         | 读取点                                                       |
| --------------------- | ----------- | ---------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------ |
| `ops.enabled`         | bool        | false（必须显式打开）  | 启用 ops 模块（系统信息/指标/进程/定时任务只读 RPC 面）                                                                                      | `root.go:639`                                                |
| `ops.metricsInterval` | Go duration | 空 → 30s               | 指标采集/上报周期；**解析失败静默忽略**（root.go:643-647）。注意 `OpsConfig.Validate()` 的 [3s,24h] 钳制**不被本二进制调用**；越界值原样通过 | `root.go:643-647` → `app.go:432-442` → `upstream.go:583-586` |
| `ops.metricsEnabled`  | bool        | false（覆盖默认 true） | 周期指标上报总开关                                                                                                                           | `root.go:642`；循环闸门 `upstream.go:277-283`                |

**诚实披露（DEAD 面）**：agentcore `OpsConfig` 的丰富字段——
`allowRestart`/`allowExec`/`managedProcesses`/`execAllowedCommands`/`execTimeout`/
`supervisorLog.*`/`snapshotDir`（`internal/app/agent/ops_config.go:11-55`）——
**不被本二进制解析**（sidecar 本地 OpsConfig 只有上表 3 键；未知键静默丢弃）。
本二进制里 restart/stop-start/exec-command RPC 一律拒绝（`ops_server.go:137,185,218,290`）。
`configs/agent.yaml` 中注释掉的 `managedProcesses:`/`supervisorLog:` 块对本
二进制无效果。

### 环境变量（运行时行为，不是 YAML 覆盖层）

| 变量                                                                                          | 默认                                                         | 用途                                | 读取点                                            |
| --------------------------------------------------------------------------------------------- | ------------------------------------------------------------ | ----------------------------------- | ------------------------------------------------- |
| `CROUPIER_CONFIG_DIR`                                                                         | `<exe dir>/etc` 存在则用之，否则 `/etc/croupier`（服务模式） | providers.yaml 与服务安装的配置目录 | `internal/app/agent/app.go:112`；`service.go:519` |
| `CROUPIER_AGENTLOCAL_PROBE_INTERVAL`                                                          | `5s`（≤0 关闭保活探测）                                      | Provider 保活探测周期               | `provider_keepalive.go:25-31`                     |
| `CROUPIER_AGENTLOCAL_PRUNE_INTERVAL` / `_MAX_AGE` / `_TASKRESULT_MAX_AGE`                     | `30s` / `2m` / `10m`                                         | 本地存储修剪节奏                    | `app.go:261-263`                                  |
| `CROUPIER_AGENTLOCAL_PROVIDER_HEARTBEAT`                                                      | `30s`                                                        | 静态 Provider 会话心跳              | `internal/app/agent/provider.go:314`              |
| `CROUPIER_EXTENSION_SYNC_API` / `_INTERVAL` / `_PROVIDERS_ONLY` / `_PROVIDER_OVERRIDE_STATIC` | 关 / `30s`                                                   | 扩展同步拉取面                      | `app.go:297-310`；`provider.go:49-50,304-310`     |
| `LOG_OUTPUT` / `CROUPIER_LOG_OUTPUT`                                                          | stdout；`stderr` 重定向控制台流                              | 控制台日志去向                      | `core/logx/logging.go:35-40`                      |

### 命令行与子命令（root.go:64-66、37）

- `--config/-f`（默认 `etc/agent.yaml`，**必填**，缺失报错）、`--mode/-m`、
  `--debug/-d`（两者只回显进启动日志，**不改变任何行为**，root.go:530）。
- 子命令：`agent`（等价根命令）、`version`、`service install|uninstall|start|stop|restart|status|run`
  （kardianos 服务管理，service.go:37-94）。

### sidecar 最小可运行配置

```yaml
agent:
  gameId: default # required
  env: dev # required
```

省略 `server.addr` = 不连上游、local-only 运行（upstream.go:258-261）；网关绑定
临时端口 `0.0.0.0:0`（要固定就加 `port: 19091`）。

### sidecar 推荐生产配置

```yaml
host: 0.0.0.0
port: 19091

server:
  addr: "croupier-server.internal:19090"
  transport: tcp
  insecure: false
  tlsCertFile: /etc/croupier/certs/agent.crt # path reference only; values never enter the repo
  tlsKeyFile: /etc/croupier/certs/agent.key
  caFile: /etc/croupier/certs/ca.crt
  serverName: croupier-server.internal
  insecureSkipVerify: false

agent:
  id: game-a-node-01
  gameId: game-a
  env: prod
  localAddr: "0.0.0.0:19091"
  labels: { tier: game-logic }
  invokeTimeoutMs: 15000

upstream:
  heartbeatInterval: 10
  timeout: 10000

log:
  level: info
  format: json
  output: /var/log/croupier/agent.log # note: in this binary, output is the log file path
  maxSize: 100
  maxBackups: 5
  maxAge: 14
  compress: true

ops:
  enabled: true
  metricsInterval: 30s
  metricsEnabled: true
```

---

## devops-agent

加载链路：`agents/devops/config/config.go:75-88`（ReadFile → Unmarshal →
Validate），**无兼容键、无 env 展开层**；校验失败以退出码 2 退出
（`cmd/devops-agent/main.go:29-34`）。

### `agent.*`（devops/config/config.go:26-37）

| 字段                  | 类型   | 默认                 | 说明                                                                         | 读取点                                                      |
| --------------------- | ------ | -------------------- | ---------------------------------------------------------------------------- | ----------------------------------------------------------- |
| `agent.id`            | string | 空（**必填**）       | 注册用唯一 id                                                                | 校验 `config.go:92-94`；注册载荷 `runner.go:109`            |
| `agent.serverAddr`    | string | 空（**必填**）       | 上游控制面 `host:port`（`tcp://` 前缀可选）                                  | 校验 `config.go:95-97`；`runner.go:286`（controlconn.Dial） |
| `agent.gameId`        | string | 空（**必填**）       | 游戏域标签                                                                   | 校验 `config.go:98-100`；`runner.go:110`                    |
| `agent.env`           | string | 空                   | 环境标签                                                                     | `runner.go:111`                                             |
| `agent.heartbeatFile` | string | 空（= 关闭心跳文件） | 可监管心跳 mtime 戳文件（supervisor 采样 last_heartbeat_unix）；间隔默认 10s | `main.go:37`；`core/supervisable/options.go:29-31`          |

### `outboundTLS.*`（devops/config/config.go:16-23）

| 字段                   | 类型   | 默认  | 说明                                  | 读取点                                            |
| ---------------------- | ------ | ----- | ------------------------------------- | ------------------------------------------------- |
| `enabled`              | bool   | false | false = 明文连接上游                  | `runner.go:124-126`（tlsFromCfg；nil → Insecure） |
| `certFile` / `keyFile` | string | 空    | mTLS 客户端证书/私钥                  | `runner.go:128-129`                               |
| `caFile`               | string | 空    | 校验 server 证书的 CA（生产强烈建议） | `runner.go:130`                                   |
| `serverName`           | string | 空    | TLS SNI 覆盖                          | `runner.go:131`                                   |
| `insecureSkipVerify`   | bool   | false | 跳过 server 证书校验                  | `runner.go:132`                                   |

（`controlconn.Dial` 侧常量：连接 5s / 接收 30s / 发送 10s，`agents/controlconn/controlconn.go:42-44`，不可配置。）

### `devops.*`（devops/config/config.go:52-65）

| 字段                | 类型                         | 默认                   | 说明                                                                                                      | 读取点                                                                                         |
| ------------------- | ---------------------------- | ---------------------- | --------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| `devops.enabled`    | bool                         | **false**              | false = 只上线注册不接会话；true = 启用会话面（必须先配 allowlist）                                       | `runner.go:68`                                                                                 |
| `devops.allowlist`  | `[]{host, ports, protocols}` | 空                     | **主机 allowlist（硬边界）**：enabled=true 时必填且非空（config.go:102-119）；host 非空、port ∈ (0,65535] | 校验 `config.go:102-119`；匹配 `agents/devops/session/allowlist.go:23-49`；装配 `runner.go:90` |
| `devops.sessionDir` | string                       | `data/devops/sessions` | 会话审计记录目录（session recorder；装配失败 = 退出）                                                     | 默认 `config.go:131-136`；`runner.go:83`                                                       |
| `devops.execlogDir` | string                       | `data/devops/execlog`  | 执行审计日志目录（execlog lumberjack，默认 100MB × 10 备份 × 30 天）                                      | 默认 `config.go:138-143`；`runner.go:76`                                                       |
| `devops.guacdAddr`  | string                       | 空                     | **DEAD（保留）**：声明注释写明「skeleton 批不接线」；代码从不读取（config.go:62-64）                      | —                                                                                              |

`AllowHost` 结构（config.go:42-48）：`host` string、`ports` []int、
`protocols` []string。

### 命令行（cmd/devops-agent/main.go）

- `-config`（默认 `configs/devops-agent.yaml`）、`-version`。
- 退出码约定：2 = 配置错误；0 = 正常停止；1 = 运行时错误
  （main.go:29-34 + supervisable）。

### devops 最小可运行配置

```yaml
agent:
  id: devops-bastion-01
  serverAddr: 10.0.0.5:19090
  gameId: demo
```

（上线注册心跳可见；`devops.enabled` 默认 false，不接任何远程会话。）

### devops 推荐生产配置

```yaml
agent:
  id: devops-bastion-01
  serverAddr: 10.0.0.5:19090
  gameId: demo
  env: prod
  heartbeatFile: /var/lib/croupier/devops/agent-heartbeat

outboundTLS:
  enabled: true
  caFile: /etc/croupier/certs/ca.pem
  certFile: /etc/croupier/certs/devops.pem
  keyFile: /etc/croupier/certs/devops-key.pem
  serverName: croupier-control.internal

devops:
  enabled: false # always false until the session channel is wired; enable per allowlist after wiring
  allowlist:
    - host: 10.0.0.21
      ports: [22]
      protocols: [ssh]
  sessionDir: /var/lib/croupier/devops/sessions
  execlogDir: /var/lib/croupier/devops/execlog
```

**已知边界**：`devops.enabled=true` 时会话建立会得到
`session channel ... not implemented yet (skeleton)` 拒绝
（`agents/devops/session/session.go:71-73`）——协议通道另行批次接线；当前安装的意义 =
加入注册/心跳/审计监管面。

---

## capture-agent

加载链路：`agents/capture/config/config.go:90-103`（纯 Unmarshal，无 env 展开）→
`Validate()`（config.go:106-135）；校验失败以退出码 2 退出
（`cmd/capture-agent/main.go:29-34`）。

### `agent.*`（capture/config/config.go:28-39）

| 字段                  | 类型   | 默认           | 说明                                    | 读取点                                       |
| --------------------- | ------ | -------------- | --------------------------------------- | -------------------------------------------- |
| `agent.id`            | string | 空（**必填**） | 注册用唯一 id（gates 按它定位会话拉取） | 校验 `config.go:107-109`；`runner.go:139` 等 |
| `agent.serverAddr`    | string | 空（**必填**） | 上游控制面 `host:port`                  | 校验 `config.go:110-112`；`runner.go:286`    |
| `agent.gameId`        | string | 空（**必填**） | 游戏域标签（gates/告警按它圈范围）      | 校验 `config.go:113-115`；`runner.go:296`    |
| `agent.env`           | string | 空             | 环境标签                                | `runner.go:297`                              |
| `agent.heartbeatFile` | string | 空（= 关）     | 同 devops（supervisable 心跳文件）      | `main.go:37`                                 |

### `outboundTLS.*`（capture/config/config.go:18-25）

六个字段与 devops 同名同义；读取点 `runner.go:563-576`（tlsFromCfg）→
`agents/controlconn/controlconn.go:36-60`。无 Validate 约束。

### `capture.*`（capture/config/config.go:69-80）

| 字段                               | 类型   | 默认                          | 说明                                                                                                                                                                                                       | 读取点                                                        |
| ---------------------------------- | ------ | ----------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------- |
| `capture.enabled`                  | bool   | **false（有意默认关）**       | CDC 总开关；false = 只上线注册不消费                                                                                                                                                                       | 校验 `config.go:116`；`runner.go:304`                         |
| `capture.bookmarkDir`              | string | 空 → `data/capture/bookmarks` | 书签文件目录（每源一个文件 `mysql.json`，tmp+rename 原子写）                                                                                                                                               | `config.go:138-144`（BookmarkPath）；`runner.go:122`          |
| `capture.bookmarkFlushIntervalSec` | int    | 0 → 5s                        | 书签周期落盘间隔（≤0 一律回落 5s）；ctx 取消时无条件补一次落盘                                                                                                                                             | `config.go:147-152`（FlushInterval）；`runner.go:361,498-510` |
| `capture.sourceType`               | string | 空                            | 数据源类型（`mysql` \| `postgres` \| `mongodb`）；enabled=true 必填。mongodb 要求源为副本集（change streams 是硬前置；standalone 在 `Open` 被拒），书签文件 `mongodb.json`，allowlist 形态 `db.collection` | 校验 `config.go:175-182`                                      |
| `capture.dsnEnv`                   | string | 空                            | 存放连接串的环境变量名（明文 DSN 只从环境变量取——不进配置文件、不进日志）；enabled=true 必填                                                                                                               | 校验 `config.go:175-182`                                      |
| `capture.options`                  | map    | 空                            | 非敏感连接参数（serverId/flavor/tables/slotName/publication/database…）；`options.tables` 必填（逗号分隔 `db.table`/`db.collection` allowlist）                                                            | 校验 `config.go:175-182`                                      |
| `capture.mysql`                    | object | 空                            | **旧版兼容，新部署勿用**：仅 compat 解析代码容忍；不再写入任何示例/文档                                                                                                                                    | `runner.go:150,521`                                           |
| `capture.gates`                    | object | 空 = 只采集不下判             | 闸门层规则；**文件规则只是回落**——面板发布过 `capture.gates` 后完全覆盖                                                                                                                                    | 校验 `config.go:125-133`；热推送 `runner.go:212-244`          |

### `capture.mysql.*`（agents/capture/source/mysql.go:16-24）

| 字段                | 类型             | 默认    | 说明                                                                                                            | 读取点                                                                    |
| ------------------- | ---------------- | ------- | --------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------- |
| `host` / `port`     | string / uint16  | 空 / 0  | 源库地址                                                                                                        | `mysql.go:78-79`；GTID 探测 `mysql_probe.go:16`；就绪探测 `runner.go:521` |
| `user` / `password` | string           | 空 / 空 | 复制账号（**明文字段**：配置文件 root:0600；最小权限 = REPLICATION SLAVE + REPLICATION CLIENT + 订阅表 SELECT） | `mysql.go:80-81`                                                          |
| `serverId`          | uint32           | 0       | binlog 订阅 slave server_id；**不得**与源实例或其他订阅者冲突（冲突 = server 断流）                             | `mysql.go:77`                                                             |
| `tables`            | map<string,bool> | 空      | **enabled=true 时必填非空**（config.go:120-122）；订阅 allowlist，键 `"db.table"` 精确匹配，值 false = 排除     | `mysql_pump.go:77-80`（MatchesTable → source/event.go:58-64）             |

**源库前置**（源侧运维项，不属于本配置）：`binlog_format=ROW`、
`binlog_row_image=FULL`、`gtid_mode=ON`（探测验证，`mysql_probe.go:29-31`）。

### `capture.gates.*`（capture/config/config.go:45-66）

规则模板是代码，规则体是纯数据（无 DSL）。全部在启动时完整校验；**`event.ops`
是唯一不在 Validate 内校验的小节**——非法值会让 `runner.New` 的 buildGateSet
降级为 pass-all 并告警 `capture gates disabled, invalid file rules`
（`runner.go:152-157`），而不是拒绝启动。

#### `event.ops`（闭集 insert/update/delete）

| 值                         | 说明                                                                     | 读取点                                            |
| -------------------------- | ------------------------------------------------------------------------ | ------------------------------------------------- |
| `[]`（省略）               | 放行全部事件（零成本透传）                                               | `agents/capture/filter/event.go:36-38`            |
| `insert`/`update`/`delete` | allowlist；闭集外的值产生构建错误 `event filter op %q not in closed set` | `filter/event.go:25-28`；求值 `runner.go:406-415` |

#### `reconcile.rules[*]`（对账闸门；agents/capture/gate/reconcile.go:49-59）

| 字段                        | 类型     | 默认          | 约束                                                                                               | 读取点                                              |
| --------------------------- | -------- | ------------- | -------------------------------------------------------------------------------------------------- | --------------------------------------------------- |
| `name`                      | string   | —             | 必填                                                                                               | `reconcile.go:63-65`                                |
| `itemTable` / `ledgerTable` | string   | —             | 必须是两段 `"db.table"`（表名不含点）；两者必须不同；**同一张表不得跨 reconcile 规则复用**         | `reconcile.go:66-74`；唯一性 `reconcile.go:110-124` |
| `itemKey` / `ledgerKey`     | string   | —             | 必填；键形态：整数字符串 = 位置行镜像下标（C1 行镜像是位置数组 JSON），字段名 = 对象键（向前兼容） | `reconcile.go:75-77`；键提取 `reconcile.go:288-310` |
| `windowSec`                 | int      | 0 → **300**   | ≥0；配对窗口（设计默认 ±5 分钟）                                                                   | `reconcile.go:78-80`；`Window()` :93-98             |
| `severity`                  | string   | 空 → `warn`   | 闭集 `warn\|critical`                                                                              | `reconcile.go:81-85`；默认 :380-383                 |
| `exemptKeys`                | []string | `[]`          | 精确匹配的键豁免（批量管道冲正等）                                                                 | `reconcile.go:198-204,284-286`                      |
| `maxPending`                | int      | 0 → **10000** | ≥0；每规则窗口内条目上限（环型 FIFO：被逐出的借记 = 告警、贷记 = 静默）                            | `reconcile.go:86-88`；`cap()` :101-106              |

命中 ID：`reconcile.missing_ledger`（reconcile.go:27、389）。

#### `cep.offHours[*]`（时间窗闸门；agents/capture/gate/cep.go:36-43）

| 字段          | 类型                  | 默认        | 约束                                                                                 | 读取点                                               |
| ------------- | --------------------- | ----------- | ------------------------------------------------------------------------------------ | ---------------------------------------------------- |
| `name`        | string                | —           | 必填                                                                                 | `cep.go:83-85`                                       |
| `table`       | string                | —           | 两段 `"db.table"`                                                                    | `cep.go:86-88`                                       |
| `ops`         | []string              | `[]` = 全部 | 三值闭集                                                                             | `cep.go:89-91`（checkOps :459-468）；运行时 :446-456 |
| `windows[]`   | `[]{startMin,endMin}` | —           | **必填非空**；startMin ∈ [0,1439]、endMin ∈ [1,1440]；左闭右开；`start>end` = 跨零点 | `cep.go:92-98`；跨零点 :481-494                      |
| `tzOffsetMin` | int                   | 0 = UTC     | ∈ [-720, 840]（UTC+8 = 480）                                                         | `cep.go:99-101`                                      |
| `severity`    | string                | 空 → `warn` | 闭集                                                                                 | `cep.go:102`（:471-478）                             |

命中 ID：`cep.off_hours`（cep.go:15）；按事件时间（binlog 事实）判定，无时间戳时回落墙钟（cep.go:293-296）。

#### `cep.burst[*]`（数量突增闸门；cep.go:50-60）

| 字段                     | 类型    | 默认             | 约束                                                                     | 读取点                             |
| ------------------------ | ------- | ---------------- | ------------------------------------------------------------------------ | ---------------------------------- |
| `name` / `table` / `ops` | —       | —                | 同 offHours                                                              | `cep.go:108-116`                   |
| `deltaField`             | string  | —                | 必填；数值增量字段（键形态同 itemKey）；缺失/非数值 → 跳过并计数         | `cep.go:117-119`；:313-315         |
| `threshold`              | float64 | —                | >0；single = 单事件 \|delta\| 阈值；window = 滑动窗口累计 \|delta\| 阈值 | `cep.go:120-122`                   |
| `mode`                   | string  | —                | 闭集 `single\|window`；window 要求 windowSec>0                           | `cep.go:123-130`                   |
| `windowSec`              | int     | —（window 必填） | 滑动窗口秒数                                                             | `cep.go:125-127`                   |
| `maxEvents`              | int     | 0 → **10000**    | ≥0；累计桶环容量（FIFO 逐出最旧）                                        | `cep.go:131-133`；`capOf` :409-414 |
| `severity`               | string  | 空 → `warn`      | 闭集                                                                     | `cep.go:134`                       |

命中 ID：`cep.burst_delta`（cep.go:16）；窗口命中后清空桶防止告警风暴（cep.go:332）。

#### `cep.probing[*]`（连续试探闸门；cep.go:67-78）

| 字段                     | 类型    | 默认          | 约束                                                     | 读取点                     |
| ------------------------ | ------- | ------------- | -------------------------------------------------------- | -------------------------- |
| `name` / `table` / `ops` | —       | —             | 同 offHours                                              | `cep.go:140-148`           |
| `keyField`               | string  | —             | 必填；行键（C1 降级：键 = 行键而非账号）                 | `cep.go:149-151`；:349-351 |
| `deltaField`             | string  | —             | 必填；数值增量字段                                       | `cep.go:149-151`           |
| `smallBelow`             | float64 | —             | >0；单次 \|delta\| 低于此 = 小额变动；≥ 此值重置该键序列 | `cep.go:152-154`；:353-359 |
| `count`                  | int     | —             | ≥2；窗口内第 N 次小额变动触发（每键每窗口最多一次告警）  | `cep.go:155-157`；:372-374 |
| `windowSec`              | int     | —             | >0                                                       | `cep.go:158-160`           |
| `maxTracked`             | int     | 0 → **10000** | ≥0；跟踪键上限（FIFO 确定性逐出）                        | `cep.go:161-163`；:361     |
| `severity`               | string  | 空 → `warn`   | 闭集                                                     | `cep.go:163`               |

命中 ID：`cep.probing`（cep.go:17）。

#### `lineage.rules[*]`（血缘闸门；agents/capture/gate/lineage.go:19-25）

| 字段                     | 类型     | 默认        | 约束                             | 读取点             |
| ------------------------ | -------- | ----------- | -------------------------------- | ------------------ |
| `name` / `table` / `ops` | —        | —           | 同上（两段表名）                 | `lineage.go:30-38` |
| `allowAccounts`          | []string | —           | **必填非空**（至少含应用写账号） | `lineage.go:39-41` |
| `severity`               | string   | 空 → `warn` | 闭集                             | `lineage.go:42`    |

命中 ID：`lineage.whitelist_violation`（lineage.go:10）。**降级态**：binlog
ROW 无会话账号 → 全部命中跳过并计数（`Degraded()`，lineage.go:89-92）；
注册标签 `captureLineage=degraded`（`runner.go:535-548`）；面板渲染降级徽标。

### 热推送与书签（运维要点）

- **闸门规则热推送**：面板发布 `capture.gates`（config_versions）→ agent 每
  **30s** 轮询（`core/configsync/configsync.go:15`，**不可经 YAML 配置**）→
  完整校验（与面板 dry-run 同一实现）→ 原子指针替换；失败保留旧版本、游标不前进、
  下轮重试（`runner.go:212-244`；`core/configsync/configsync.go:19-56`）。
  文件规则只在从未发布过时生效（runner.go:159-167）。
- **书签**：`<bookmarkDir>/mysql.json`（GTID set + updatedAt）。续传 = 从书签
  `StartSyncGTID`；无书签 = 探测 `gtid_executed` 从当下起（不回放历史）。
  **书签文件损坏 = 拒绝启动**（exit 1，需人工介入，不静默重扫；
  `runner.go:324-350`；`source/source.go:50-53`）。

### capture 最小可运行配置

```yaml
agent:
  id: capture-01
  serverAddr: 127.0.0.1:19090
  gameId: demo
```

（`capture.enabled` 默认 false：上线注册、心跳可见、绝不触碰数据库；
`agents/capture/config/config_test.go:73-140` 证明同形态。）

### capture 推荐生产配置

```yaml
agent:
  id: capture-01
  serverAddr: 10.0.0.5:19090
  gameId: demo
  env: prod
  heartbeatFile: /var/lib/croupier/capture/agent-heartbeat

outboundTLS:
  enabled: true
  caFile: /etc/croupier/certs/ca.pem
  certFile: /etc/croupier/certs/capture.pem
  keyFile: /etc/croupier/certs/capture-key.pem
  serverName: croupier-control.internal

capture:
  enabled: true
  bookmarkDir: /var/lib/croupier/capture/bookmarks
  bookmarkFlushIntervalSec: 5
  mysql:
    host: 10.0.1.20
    port: 3306
    user: repl
    password: "" # plaintext field: this file is root:0600; written via deployment-side template rendering
    serverId: 4242
    tables:
      game.items: true
      game.player_wallet: true
  gates:
    event:
      ops: [insert, update, delete]
    reconcile:
      rules:
        - name: item-ledger
          itemTable: game.items
          ledgerTable: game.ledger
          itemKey: "1"
          ledgerKey: "0"
          windowSec: 300
          severity: critical
    cep:
      burst:
        - name: wallet-drain
          table: game.player_wallet
          deltaField: "2"
          threshold: 100000
          mode: single
          severity: critical
    lineage:
      rules:
        - name: items-accounts
          table: game.items
          allowAccounts: ["game-app"] # in production, prefer panel hot push over file rules
          severity: critical
```

生产建议：规则体经面板「风控 → 规则」发布（config_versions 热推送 +
版本回滚）；文件 `gates` 只作回落骨架；`mysql.password` 放 root:0600 配置
文件或部署侧模板渲染——绝不进仓库、不进日志。

---

## 附录：三类 Agent 快速对比（配置面）

|          | sidecar                                                                                               | devops                           | capture              |
| -------- | ----------------------------------------------------------------------------------------------------- | -------------------------------- | -------------------- |
| 配置文件 | `agent.yaml`                                                                                          | `devops-agent.yaml`              | `capture-agent.yaml` |
| 必填     | `agent.gameId`+`agent.env`                                                                            | `agent.id`+`serverAddr`+`gameId` | 同 devops            |
| 旧键兼容 | 有（PascalCase/snake_case）                                                                           | 无                               | 无                   |
| 默认姿态 | 上游不可达也启动（local-only）                                                                        | 只注册不接会话                   | 只注册不消费         |
| 死字段   | `name`/`agent.transport`/`httpAddr`/`upstream.retryInterval`/`maxRetries`/`log.file`/`tls.serverName` | `devops.guacdAddr`               | 无                   |
