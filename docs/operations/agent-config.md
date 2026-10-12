# Agent 配置逐字段全解（sidecar / devops / capture）

三类 agent 各自一个 YAML 配置文件，加载链极简：**读文件 → yaml.Unmarshal →
校验 → 运行**。本文逐字段列出字段名、类型、默认值、含义与代码读取处
（`file:line`，与当前主干对齐；每个字段都能在代码里找到读取点，无凭空字段）。
选型见 [agent-comparison.md](./agent-comparison)，安装见
[agent-install.md](./agent-install)。

**三条通用约定（三个 agent 一致）：**

1. **无 `${ENV}` 展开层**：配置加载是纯 `yaml.Unmarshal`，环境变量占位符
   **不会**被展开。含密码的配置文件直接写值并保持 `root:0600`，或部署侧
   模板渲染后再交付。**密钥绝不入仓库**——示例里密码一律留空、证书只给
   文件路径引用。
2. **键名 lowerCamelCase**（`serverAddr`、`heartbeatFile`）；sidecar 另接受
   旧式 PascalCase/snake_case 兼容键（见各表备注），但新配置一律用 canonical
   键，兼容键不是文档标准。
3. **出站 TLS 三段式**：`enabled=false` 时全部证书字段被忽略（裸连）；
   `enabled=true` 时按字段传给 TLS 客户端配置。生产一律建议启用 + `caFile`。

代码根路径约定：下文 `devops/config/config.go` 即
`agents/devops/config/config.go`，`root.go` 即 `cmd/sidecar-agent/root.go`。

---

## sidecar（croupier-agent）

加载链：`--config` 文件 → `yaml.Unmarshal`（`root.go:482-485`，含旧键兼容
`root.go:173-241`）→ 默认值（`applyAgentConfigDefaults`，`root.go:450-466`）→
强校验（`root.go:486-490`：`agent.gameId` 与 `agent.env` 非空，否则启动失败）。

### `agent.*`（root.go:301-313）

| 字段                    | 类型               | 默认                                           | 说明                                                         | 读取处                                                                            |
| ----------------------- | ------------------ | ---------------------------------------------- | ------------------------------------------------------------ | --------------------------------------------------------------------------------- |
| `agent.id`              | string             | 空 → 自动 `<hostname>-<unixts>`                | 注册用 agent 唯一 id                                         | `root.go:558` → `resolveAgentID`（root.go:662-673）                               |
| `agent.gameId`          | string             | 空（**必填**）                                 | 游戏域标签；会话绑定唯一 (gameId, env)                       | 校验 `root.go:486-490`；随注册上报 `root.go:574`                                  |
| `agent.env`             | string             | 空（**必填**）                                 | 环境标签（dev/prod/…）                                       | 同上 `root.go:575`                                                                |
| `agent.localAddr`       | string             | 空 → 由 `host`/`port` 兜底，再兜底 `0.0.0.0:0` | 本地 TCP 网关：SDK Provider 注册函数入口                     | `root.go:545-556`（剥 `tcp://` 前缀）                                             |
| `agent.labels`          | map<string,string> | 空                                             | 运维自定义标签，合并到自动系统标签（os/arch/hostname/ip 等） | `root.go:563-565` → 注册 payload                                                  |
| `agent.invokeTimeoutMs` | int                | 0 → 15000                                      | agent→Provider 同步调用预算（ms）；单请求 `timeoutMs` 优先   | `root.go:572`；钳制 `internal/agent/local_handler.go:110-121`（≤0→15s，>60s→60s） |
| `agent.transport`       | string             | 空                                             | **DEAD**：解析后无人读取（root.go:305, 342-344）             | —                                                                                 |
| `agent.httpAddr`        | string             | 空                                             | **DEAD（行为面）**：仅回显进启动日志一行（root.go:494）      | —                                                                                 |

### `server.*`（root.go:243-252）—— 上游控制面

| 字段                                       | 类型   | 默认                                | 说明                                                                           | 读取处                                                                 |
| ------------------------------------------ | ------ | ----------------------------------- | ------------------------------------------------------------------------------ | ---------------------------------------------------------------------- |
| `server.addr`                              | string | 空 = **不连上游**（本地-only 模式） | Croupier Server 控制面 `host:port`（约定 19090）                               | `root.go:568`；空时跳过并日志 `internal/app/agent/upstream.go:258-261` |
| `server.transport`                         | string | 空 → `tcp`                          | 传输类型；**只有 `tcp` 合法**，其他值 panic                                    | `root.go:570` → `upstream.go:150-160`                                  |
| `server.insecure`                          | bool   | false（显式 true = 裸连）           | false 时用下列证书字段构建 TLS                                                 | `root.go:586-608`                                                      |
| `server.caFile`                            | string | 空                                  | 校验服务端证书的 CA；设置了但文件缺失时**自动生成 dev CA** 到配置目录 `certs/` | `root.go:586-596` → `internal/devcert/devcert.go:33-58`                |
| `server.tlsCertFile` / `server.tlsKeyFile` | string | 空                                  | mTLS 客户端证书/私钥                                                           | `root.go:600-601`                                                      |
| `server.serverName`                        | string | 空                                  | TLS SNI/主机名校验覆盖                                                         | `root.go:603`                                                          |
| `server.insecureSkipVerify`                | bool   | false                               | 跳过服务端证书链校验                                                           | `root.go:604`                                                          |

### `upstream.*`（root.go:358-363）—— 心跳与超时

| 字段                         | 类型      | 默认                   | 说明                                                                                   | 读取处                                                                 |
| ---------------------------- | --------- | ---------------------- | -------------------------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| `upstream.heartbeatInterval` | int（秒） | 0 → 核心 3s            | 控制面心跳周期                                                                         | `root.go:582` → `upstream.go:201-204` → `core/register/register.go:39` |
| `upstream.timeout`           | int（ms） | 0 → 请求 10s / 心跳 3s | 上游拨号与请求超时                                                                     | `root.go:580-581`（仅 >0 生效）                                        |
| `upstream.retryInterval`     | int       | 0                      | **DEAD**：解析后无人读取；重连退避固定 5s→60s×1.5（`core/register/register.go:44-45`） | —                                                                      |
| `upstream.maxRetries`        | int       | 0                      | **DEAD**：同上；失败阈值核心默认 2 次（register.go:42）                                | —                                                                      |

### `log.*`（= core/logx.LogConfig，core/logx/logging.go:18-27）

| 字段                                                 | 类型             | 默认                  | 说明                                                                                                                                                                        | 读取处                                                        |
| ---------------------------------------------------- | ---------------- | --------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------- |
| `log.level`                                          | string           | 空 → `info`           | `debug\|info\|warn\|error`                                                                                                                                                  | `root.go:500-506`                                             |
| `log.format`                                         | string           | 空 → `console`        | `console\|json`                                                                                                                                                             | `root.go:507`                                                 |
| `log.output`                                         | string           | 空 = 只写 stdout      | **本二进制里它就是轮转日志文件路径**（传给 `SetupLoggerWithFile` 的文件参数），不是 `stdout/stderr` 枚举；控制台流由环境变量 `LOG_OUTPUT`/`CROUPIER_LOG_OUTPUT=stderr` 控制 | `root.go:509`；lumberjack 文件写 `core/logx/logging.go:43-51` |
| `log.maxSize` / `maxBackups` / `maxAge` / `compress` | int/int/int/bool | lumberjack 默认 100MB | 轮转参数                                                                                                                                                                    | `root.go:510-513`                                             |
| `log.file`                                           | string           | 空                    | **DEAD（本二进制）**：root.go:506-514 从不读取 `Logging.File`                                                                                                               | —                                                             |

### `tls.*`（root.go:125-132）—— 本地网关 TLS（SDK→agent）

| 字段                           | 类型   | 默认                          | 说明                                                          | 读取处                                                         |
| ------------------------------ | ------ | ----------------------------- | ------------------------------------------------------------- | -------------------------------------------------------------- |
| `tls.enabled`                  | bool   | false（整块为零时强制 false） | 加密 `agent.localAddr` 监听；TLS 在帧协议之下，SDK 线协议不变 | `root.go:625`                                                  |
| `tls.certFile` / `tls.keyFile` | string | 空                            | 网关服务端证书；enabled 且缺文件 = **启动报错**               | `root.go:627-628` → `internal/transport/tcp/server.go:266-269` |
| `tls.caFile`                   | string | 空                            | 设置即开 **mTLS**（强制校验客户端证书）                       | `root.go:629` → server.go:237-248                              |
| `tls.insecureSkipVerify`       | bool   | false                         | 服务端侧降级：取消 mTLS 强制（ClientAuth=NoClientCert）       | `root.go:630` → server.go:250-252                              |
| `tls.serverName`               | string | 空                            | **DEAD**：未传入服务端 TLS 构建（root.go:626-631）            | —                                                              |

### `outboundTLS.*`（root.go:125-132 同构）—— 拨出 Provider TLS

| 字段                                                                        | 说明                                                                   | 读取处                                                        |
| --------------------------------------------------------------------------- | ---------------------------------------------------------------------- | ------------------------------------------------------------- |
| `enabled` + `certFile`/`keyFile`/`caFile`/`serverName`/`insecureSkipVerify` | 加密 agent→Provider 的拨出连接（无既存 Provider 会话时的兜底拨号路径） | `root.go:610-620` → `internal/agent/local_handler.go:176-179` |

### `ops.*`（root.go:93-97）—— 仅 3 个键可达

| 字段                  | 类型        | 默认                   | 说明                                                                                                                                          | 读取处                                                       |
| --------------------- | ----------- | ---------------------- | --------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------ |
| `ops.enabled`         | bool        | false（必须显式开）    | 开启 ops 模块（系统信息/指标/进程/定时任务只读 RPC 面）                                                                                       | `root.go:639`                                                |
| `ops.metricsInterval` | Go duration | 空 → 30s               | 指标采集上报周期；**解析失败静默忽略**（root.go:643-647）。注意 `OpsConfig.Validate()` 的 [3s,24h] 钳制**不被本二进制调用**，超范围值原样透传 | `root.go:643-647` → `app.go:432-442` → `upstream.go:583-586` |
| `ops.metricsEnabled`  | bool        | false（覆盖默认 true） | 周期指标上报总开关                                                                                                                            | `root.go:642`；循环门 `upstream.go:277-283`                  |

**诚实声明（DEAD 面）**：agentcore `OpsConfig` 的富字段
`allowRestart`/`allowExec`/`managedProcesses`/`execAllowedCommands`/`execTimeout`/
`supervisorLog.*`/`snapshotDir`（`internal/app/agent/ops_config.go:11-55`）
**不被本二进制解析**（sidecar 本地 OpsConfig 只有上表 3 键，未知键静默丢弃）。
本二进制里重启/停启/执行命令 RPC 恒拒绝（`ops_server.go:137,185,218,290`）。
`configs/agent.yaml` 里注释掉的 `managedProcesses:`/`supervisorLog:` 块对本
二进制无效。

### 环境变量（运行时行为，非 YAML 覆盖层）

| 变量                                                                                          | 默认                                                        | 作用                                     | 读取处                                            |
| --------------------------------------------------------------------------------------------- | ----------------------------------------------------------- | ---------------------------------------- | ------------------------------------------------- |
| `CROUPIER_CONFIG_DIR`                                                                         | `<exe目录>/etc` 存在则用之，否则 `/etc/croupier`（service） | providers.yaml 与 service 安装的配置目录 | `internal/app/agent/app.go:112`；`service.go:519` |
| `CROUPIER_AGENTLOCAL_PROBE_INTERVAL`                                                          | `5s`（≤0 关探活）                                           | Provider 保活探测周期                    | `provider_keepalive.go:25-31`                     |
| `CROUPIER_AGENTLOCAL_PRUNE_INTERVAL` / `_MAX_AGE` / `_TASKRESULT_MAX_AGE`                     | `30s` / `2m` / `10m`                                        | 本地存储清理节律                         | `app.go:261-263`                                  |
| `CROUPIER_AGENTLOCAL_PROVIDER_HEARTBEAT`                                                      | `30s`                                                       | 静态 Provider 会话心跳                   | `internal/app/agent/provider.go:314`              |
| `CROUPIER_EXTENSION_SYNC_API` / `_INTERVAL` / `_PROVIDERS_ONLY` / `_PROVIDER_OVERRIDE_STATIC` | 关 / `30s`                                                  | 扩展同步拉取面                           | `app.go:297-310`；`provider.go:49-50,304-310`     |
| `LOG_OUTPUT` / `CROUPIER_LOG_OUTPUT`                                                          | stdout；`stderr` 重定向控制台流                             | 控制台日志去向                           | `core/logx/logging.go:35-40`                      |

### 命令行与子命令（root.go:64-66, 37）

- `--config/-f`（默认 `etc/agent.yaml`，**必填**，缺省报错）、`--mode/-m`、
  `--debug/-d`（两者仅启动日志回显，**不改变任何行为**，root.go:530）。
- 子命令：`agent`（等价根命令）、`version`、`service install|uninstall|start|stop|restart|status|run`
  （kardianos 服务管理，service.go:37-94）。

### sidecar 最小可跑配置

```yaml
agent:
  gameId: default # 必填
  env: dev # 必填
```

`server.addr` 缺省 = 不连上游、本地-only 运行（upstream.go:258-261），网关绑
`0.0.0.0:0` 临时端口（想要固定端口加 `port: 19091`）。

### sidecar 生产建议配置

```yaml
host: 0.0.0.0
port: 19091

server:
  addr: "croupier-server.internal:19090"
  transport: tcp
  insecure: false
  tlsCertFile: /etc/croupier/certs/agent.crt # 证书路径引用，值不入仓库
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
  output: /var/log/croupier/agent.log # 注意：本二进制里 output 即日志文件路径
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

加载链：`agents/devops/config/config.go:75-88`（ReadFile → Unmarshal →
Validate），**无兼容键、无 env 展开层**；校验失败 exit 2
（`cmd/devops-agent/main.go:29-34`）。

### `agent.*`（devops/config/config.go:26-37）

| 字段                  | 类型   | 默认                 | 说明                                                                             | 读取处                                                      |
| --------------------- | ------ | -------------------- | -------------------------------------------------------------------------------- | ----------------------------------------------------------- |
| `agent.id`            | string | 空（**必填**）       | 注册用唯一 id                                                                    | 校验 `config.go:92-94`；注册 payload `runner.go:109`        |
| `agent.serverAddr`    | string | 空（**必填**）       | 上游控制面 `host:port`（`tcp://` 前缀可省）                                      | 校验 `config.go:95-97`；`runner.go:286`（controlconn.Dial） |
| `agent.gameId`        | string | 空（**必填**）       | 游戏域标签                                                                       | 校验 `config.go:98-100`；`runner.go:110`                    |
| `agent.env`           | string | 空                   | 环境标签                                                                         | `runner.go:111`                                             |
| `agent.heartbeatFile` | string | 空（= 关闭心跳文件） | supervisable 心跳 mtime 打点文件（监管方采样 last_heartbeat_unix）；间隔默认 10s | `main.go:37`；`core/supervisable/options.go:29-31`          |

### `outboundTLS.*`（devops/config/config.go:16-23）

| 字段                   | 类型   | 默认  | 说明                              | 读取处                                            |
| ---------------------- | ------ | ----- | --------------------------------- | ------------------------------------------------- |
| `enabled`              | bool   | false | false = 裸连上游                  | `runner.go:124-126`（tlsFromCfg；nil → Insecure） |
| `certFile` / `keyFile` | string | 空    | mTLS 客户端证书/私钥              | `runner.go:128-129`                               |
| `caFile`               | string | 空    | 校验服务端证书 CA（生产建议必配） | `runner.go:130`                                   |
| `serverName`           | string | 空    | TLS SNI 覆盖                      | `runner.go:131`                                   |
| `insecureSkipVerify`   | bool   | false | 跳过服务端证书校验                | `runner.go:132`                                   |

（`controlconn.Dial` 侧常量：连接 5s / 收 30s / 发 10s，`agents/controlconn/controlconn.go:42-44`，不可配。）

### `devops.*`（devops/config/config.go:52-65）

| 字段                | 类型                         | 默认                   | 说明                                                                                                | 读取处                                                                                         |
| ------------------- | ---------------------------- | ---------------------- | --------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| `devops.enabled`    | bool                         | **false**              | false = 只注册上线不接会话；true = 启用会话面（须先配 allowlist）                                   | `runner.go:68`                                                                                 |
| `devops.allowlist`  | `[]{host, ports, protocols}` | 空                     | **主机白名单（硬边界）**：enabled=true 时必填非空（config.go:102-119）；host 非空、port ∈ (0,65535] | 校验 `config.go:102-119`；匹配 `agents/devops/session/allowlist.go:23-49`；装配 `runner.go:90` |
| `devops.sessionDir` | string                       | `data/devops/sessions` | 会话审计记录目录（session recorder，装配失败即退出）                                                | 默认 `config.go:131-136`；`runner.go:83`                                                       |
| `devops.execlogDir` | string                       | `data/devops/execlog`  | 执行审计日志目录（execlog lumberjack，默认 100MB×10 备份×30 天）                                    | 默认 `config.go:138-143`；`runner.go:76`                                                       |
| `devops.guacdAddr`  | string                       | 空                     | **DEAD（预留）**：声明注释即「骨架批次不接」，代码从不读取（config.go:62-64）                       | —                                                                                              |

`AllowHost` 结构（config.go:42-48）：`host` string、`ports` []int、
`protocols` []string。

### 命令行（cmd/devops-agent/main.go）

- `-config`（默认 `configs/devops-agent.yaml`）、`-version`。
- 退出码契约：2 = 配置错误；0 = 正常停止；1 = 运行错误
  （main.go:29-34 + supervisable）。

### devops 最小可跑配置

```yaml
agent:
  id: devops-bastion-01
  serverAddr: 10.0.0.5:19090
  gameId: demo
```

（注册上线、心跳可见；`devops.enabled` 缺省 false，不接任何远程会话。）

### devops 生产建议配置

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
  enabled: false # 会话通道接线前恒 false；接线后按白名单开启
  allowlist:
    - host: 10.0.0.21
      ports: [22]
      protocols: [ssh]
  sessionDir: /var/lib/croupier/devops/sessions
  execlogDir: /var/lib/croupier/devops/execlog
```

**已知边界**：`devops.enabled=true` 时会话建立会得到
`session channel ... not implemented yet (skeleton)` 拒绝
（`agents/devops/session/session.go:71-73`）——协议通道另批接线；当前安装
意义 = 纳入注册/心跳/审计监管面。

---

## capture-agent

加载链：`agents/capture/config/config.go:90-103`（纯 Unmarshal，无 env 展开）→
`Validate()`（config.go:106-135）；校验失败 exit 2
（`cmd/capture-agent/main.go:29-34`）。

### `agent.*`（capture/config/config.go:28-39）

| 字段                  | 类型   | 默认           | 说明                                    | 读取处                                       |
| --------------------- | ------ | -------------- | --------------------------------------- | -------------------------------------------- |
| `agent.id`            | string | 空（**必填**） | 注册用唯一 id（闸配置按其定位会话拉取） | 校验 `config.go:107-109`；`runner.go:139` 等 |
| `agent.serverAddr`    | string | 空（**必填**） | 上游控制面 `host:port`                  | 校验 `config.go:110-112`；`runner.go:286`    |
| `agent.gameId`        | string | 空（**必填**） | 游戏域标签（闸/告警按此 scope）         | 校验 `config.go:113-115`；`runner.go:296`    |
| `agent.env`           | string | 空             | 环境标签                                | `runner.go:297`                              |
| `agent.heartbeatFile` | string | 空（= 关闭）   | 同 devops（supervisable 心跳文件）      | `main.go:37`                                 |

### `outboundTLS.*`（capture/config/config.go:18-25）

六字段与 devops 同名同义；读取处 `runner.go:563-576`（tlsFromCfg）→
`agents/controlconn/controlconn.go:36-60`。无 Validate 约束。

### `capture.*`（capture/config/config.go:69-80）

| 字段                               | 类型   | 默认                          | 说明                                                                  | 读取处                                                        |
| ---------------------------------- | ------ | ----------------------------- | --------------------------------------------------------------------- | ------------------------------------------------------------- |
| `capture.enabled`                  | bool   | **false（刻意缺省关）**       | CDC 总开关；false = 只注册上线不消费                                  | 校验 `config.go:116`；`runner.go:304`                         |
| `capture.bookmarkDir`              | string | 空 → `data/capture/bookmarks` | 位点文件目录（每源一文件 `mysql.json`，tmp+rename 原子写）            | `config.go:138-144`（BookmarkPath）；`runner.go:122`          |
| `capture.bookmarkFlushIntervalSec` | int    | 0 → 5s                        | 位点周期落盘间隔（≤0 一律回落 5s）；另在 ctx 取消时无条件终刷         | `config.go:147-152`（FlushInterval）；`runner.go:361,498-510` |
| `capture.mysql`                    | 对象   | 空                            | **enabled=true 时必填**（config.go:117-119）；见下表                  | `runner.go:150,521`                                           |
| `capture.gates`                    | 对象   | 空 = 只采不判                 | 闸层规则；**文件规则只是兜底**——面板发布过 `capture.gates` 后整体覆盖 | 校验 `config.go:125-133`；热下发 `runner.go:212-244`          |

### `capture.mysql.*`（agents/capture/source/mysql.go:16-24）

| 字段                | 类型             | 默认    | 说明                                                                                                            | 读取处                                                                    |
| ------------------- | ---------------- | ------- | --------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------- |
| `host` / `port`     | string / uint16  | 空 / 0  | 源库地址                                                                                                        | `mysql.go:78-79`；GTID 探针 `mysql_probe.go:16`；就绪探活 `runner.go:521` |
| `user` / `password` | string           | 空 / 空 | 复制账号（**明文字段**：配置文件 root:0600；最小权限 = REPLICATION SLAVE + REPLICATION CLIENT + 订阅表 SELECT） | `mysql.go:80-81`                                                          |
| `serverId`          | uint32           | 0       | binlog 订阅 slave server_id；**不得**与源实例或其他订阅者冲突（冲突 = 服务端断流）                              | `mysql.go:77`                                                             |
| `tables`            | map<string,bool> | 空      | **enabled=true 时必填非空**（config.go:120-122）；订阅白名单，键 `"库.表"` 精确匹配，`false` 值 = 排除          | `mysql_pump.go:77-80`（MatchesTable → source/event.go:58-64）             |

**源库前置条件**（源侧运维项，非本配置）：`binlog_format=ROW`、
`binlog_row_image=FULL`、`gtid_mode=ON`（探针校验，`mysql_probe.go:29-31`）。

### `capture.gates.*`（capture/config/config.go:45-66）

规则模板是代码、规则本体是纯数据（无 DSL）。启动即全量校验；**`event.ops`
是唯一不在 Validate 里校验的段**——非法值会让 `runner.New` 的 buildGateSet
降级为全通过并告警 `capture gates disabled, invalid file rules`
（`runner.go:152-157`），而不是拒绝启动。

#### `event.ops`（闭集 insert/update/delete）

| 值                         | 说明                                                            | 读取处                                            |
| -------------------------- | --------------------------------------------------------------- | ------------------------------------------------- |
| `[]`（缺省）               | 放行全部事件（零成本直通）                                      | `agents/capture/filter/event.go:36-38`            |
| `insert`/`update`/`delete` | 白名单；闭集外值构建报错 `event filter op %q not in closed set` | `filter/event.go:25-28`；评估 `runner.go:406-415` |

#### `reconcile.rules[*]`（对账闸；agents/capture/gate/reconcile.go:49-59）

| 字段                        | 类型     | 默认          | 约束                                                                                           | 读取处                                              |
| --------------------------- | -------- | ------------- | ---------------------------------------------------------------------------------------------- | --------------------------------------------------- |
| `name`                      | string   | —             | 必填                                                                                           | `reconcile.go:63-65`                                |
| `itemTable` / `ledgerTable` | string   | —             | 必为两段 `"库.表"`（表名不含点）；两者不同；**同一表不得跨 reconcile 规则复用**                | `reconcile.go:66-74`；唯一性 `reconcile.go:110-124` |
| `itemKey` / `ledgerKey`     | string   | —             | 必填；键形态：整数字符串 = 行像位置下标（C1 行像为位置数组 JSON），字段名 = 对象键（前向兼容） | `reconcile.go:75-77`；取键 `reconcile.go:288-310`   |
| `windowSec`                 | int      | 0 → **300**   | ≥0；配对窗口（±5 分钟设计默认）                                                                | `reconcile.go:78-80`；`Window()` :93-98             |
| `severity`                  | string   | 空 → `warn`   | 闭集 `warn\|critical`                                                                          | `reconcile.go:81-85`；缺省 :380-383                 |
| `exemptKeys`                | []string | `[]`          | 精确匹配键豁免（批处理流水等）                                                                 | `reconcile.go:198-204,284-286`                      |
| `maxPending`                | int      | 0 → **10000** | ≥0；单规则在窗条目上限（环形 FIFO：挤出的借记=告警、贷记=静默）                                | `reconcile.go:86-88`；`cap()` :101-106              |

命中 ID：`reconcile.missing_ledger`（reconcile.go:27,389）。

#### `cep.offHours[*]`（时段闸；agents/capture/gate/cep.go:36-43）

| 字段          | 类型                  | 默认        | 约束                                                                                 | 读取处                                               |
| ------------- | --------------------- | ----------- | ------------------------------------------------------------------------------------ | ---------------------------------------------------- |
| `name`        | string                | —           | 必填                                                                                 | `cep.go:83-85`                                       |
| `table`       | string                | —           | 两段 `"库.表"`                                                                       | `cep.go:86-88`                                       |
| `ops`         | []string              | `[]` = 全部 | 闭集三值                                                                             | `cep.go:89-91`（checkOps :459-468）；运行时 :446-456 |
| `windows[]`   | `[]{startMin,endMin}` | —           | **必填非空**；startMin ∈ [0,1439]、endMin ∈ [1,1440]；左闭右开；`start>end` = 跨零点 | `cep.go:92-98`；跨零点 :481-494                      |
| `tzOffsetMin` | int                   | 0 = UTC     | ∈ [-720, 840]（东八区 480）                                                          | `cep.go:99-101`                                      |
| `severity`    | string                | 空 → `warn` | 闭集                                                                                 | `cep.go:102`（:471-478）                             |

命中 ID：`cep.off_hours`（cep.go:15）；按事件时间判定（binlog 真值），
无时间戳回落墙钟（cep.go:293-296）。

#### `cep.burst[*]`（数量突变闸；cep.go:50-60）

| 字段                     | 类型    | 默认             | 约束                                                                 | 读取处                             |
| ------------------------ | ------- | ---------------- | -------------------------------------------------------------------- | ---------------------------------- |
| `name` / `table` / `ops` | —       | —                | 同 offHours                                                          | `cep.go:108-116`                   |
| `deltaField`             | string  | —                | 必填；数值增量字段（键形态同 itemKey）；取不到/非数值 → 跳过并计数   | `cep.go:117-119`；:313-315         |
| `threshold`              | float64 | —                | >0；single = 单事件 \|delta\| 阈值；window = 滑窗累计 \|delta\| 阈值 | `cep.go:120-122`                   |
| `mode`                   | string  | —                | 闭集 `single\|window`；window 须 windowSec>0                         | `cep.go:123-130`                   |
| `windowSec`              | int     | —（window 必填） | 滑动窗口秒                                                           | `cep.go:125-127`                   |
| `maxEvents`              | int     | 0 → **10000**    | ≥0；累计桶环容量（FIFO 弃旧）                                        | `cep.go:131-133`；`capOf` :409-414 |
| `severity`               | string  | 空 → `warn`      | 闭集                                                                 | `cep.go:134`                       |

命中 ID：`cep.burst_delta`（cep.go:16）；window 命中后清桶防刷屏（cep.go:332）。

#### `cep.probing[*]`（连续试探闸；cep.go:67-78）

| 字段                     | 类型    | 默认          | 约束                                                   | 读取处                     |
| ------------------------ | ------- | ------------- | ------------------------------------------------------ | -------------------------- |
| `name` / `table` / `ops` | —       | —             | 同 offHours                                            | `cep.go:140-148`           |
| `keyField`               | string  | —             | 必填；行键（C1 降级：键=行键而非账号）                 | `cep.go:149-151`；:349-351 |
| `deltaField`             | string  | —             | 必填；数值增量字段                                     | `cep.go:149-151`           |
| `smallBelow`             | float64 | —             | >0；单次 \|delta\| 低于此 = 小变更；≥ 此值重置该键序列 | `cep.go:152-154`；:353-359 |
| `count`                  | int     | —             | ≥2；窗口内第 N 次小变更触发（每键每窗至多一次告警）    | `cep.go:155-157`；:372-374 |
| `windowSec`              | int     | —             | >0                                                     | `cep.go:158-160`           |
| `maxTracked`             | int     | 0 → **10000** | ≥0；追踪键上限（FIFO 确定性弃旧）                      | `cep.go:161-163`；:361     |
| `severity`               | string  | 空 → `warn`   | 闭集                                                   | `cep.go:163`               |

命中 ID：`cep.probing`（cep.go:17）。

#### `lineage.rules[*]`（血缘闸；agents/capture/gate/lineage.go:19-25）

| 字段                     | 类型     | 默认        | 约束                             | 读取处             |
| ------------------------ | -------- | ----------- | -------------------------------- | ------------------ |
| `name` / `table` / `ops` | —        | —           | 同上（table 两段式）             | `lineage.go:30-38` |
| `allowAccounts`          | []string | —           | **必填非空**（至少含应用写账号） | `lineage.go:39-41` |
| `severity`               | string   | 空 → `warn` | 闭集                             | `lineage.go:42`    |

命中 ID：`lineage.whitelist_violation`（lineage.go:10）。**降级态**：binlog
ROW 不带会话账号 → 每条命中判定跳过并计数（`Degraded()`，lineage.go:89-92），
注册标签 `captureLineage=degraded`（`runner.go:535-548`），面板渲染降级徽标。

### 热下发与位点（运维要点）

- **闸规则热下发**：面板发布 `capture.gates`（config_versions）→ agent 每
  **30s** 轮询（`core/configsync/configsync.go:15`，**不可经 YAML 配置**）→
  整体校验（与面板 dry-run 同一实现）→ 原子换指针；失败保留旧版、游标不进、
  下轮重试（`runner.go:212-244`；`core/configsync/configsync.go:19-56`）。
  文件规则仅在从未发布过时生效（runner.go:159-167）。
- **位点**：`<bookmarkDir>/mysql.json`（GTID set + updatedAt）。断点续传 =
  从位点 `StartSyncGTID`；无位点 = 探 `gtid_executed` 从当下起（不回放历史）。
  **位点文件损坏 = 拒绝启动**（exit 1，人工介入，不静默重扫；
  `runner.go:324-350`；`source/source.go:50-53`）。

### capture 最小可跑配置

```yaml
agent:
  id: capture-01
  serverAddr: 127.0.0.1:19090
  gameId: demo
```

（`capture.enabled` 缺省 false：注册上线、心跳可见，不碰数据库；
`agents/capture/config/config_test.go:73-140` 同形态证明。）

### capture 生产建议配置

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
    password: "" # 明文字段：本文件 root:0600；部署侧模板渲染后写入
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
          allowAccounts: ["game-app"] # 生产建议走面板热下发而非文件
          severity: critical
```

生产建议：规则本体走面板「风控 → 规则」发布（config_versions 热下发 +
版本回滚），文件 `gates` 只作兜底骨架；`mysql.password` 用 root:0600 配置
文件或部署侧模板渲染，绝不入仓库与日志。

---

## 附录：三 agent 快速对照（配置面）

|          | sidecar                                                                                                          | devops                           | capture              |
| -------- | ---------------------------------------------------------------------------------------------------------------- | -------------------------------- | -------------------- |
| 配置文件 | `agent.yaml`                                                                                                     | `devops-agent.yaml`              | `capture-agent.yaml` |
| 必填     | `agent.gameId`+`agent.env`                                                                                       | `agent.id`+`serverAddr`+`gameId` | 同 devops            |
| 兼容旧键 | 有（PascalCase/snake_case）                                                                                      | 无                               | 无                   |
| 缺省形态 | 连不上上游也起（本地-only）                                                                                      | 只注册不接会话                   | 只注册不消费         |
| 死字段   | `name`/`agent.transport`/`httpAddr`/`upstream.retryInterval`/`maxRetries`/`log.file`/`tls.serverName`/ops 富字段 | `devops.guacdAddr`               | 无                   |
