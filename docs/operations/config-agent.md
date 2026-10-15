---
title: Agent 配置全解
icon: settings
order: 11
category:
  - 运维手册
tag:
  - 配置
  - agent
---

# Agent 配置全解

对应 `configs/agent.yaml`（模板见仓库）。本二进制**无环境变量展开层**：配置只从 YAML 读取；运行时环境变量仅有 `CROUPIER_CONFIG_DIR` / `CROUPIER_AGENTLOCAL_*` 等（见 agent-config.md 环境变量表），CLI 参数仅 `--config` 指定配置文件路径。

## 完整键位

### server（上游连接）

| 键                          | 默认    | 说明                                                                       |
| --------------------------- | ------- | -------------------------------------------------------------------------- |
| `server.addr`               | 必填    | Server transport 入口。生产指向 L4 LB（HAProxy/NLB/Service），不指具体实例 |
| `server.insecure`           | `false` | 默认 TLS；置 `true` 才裸连（仅 dev 排障）                                  |
| `server.serverName`         | `""`    | TLS SNI/校验名（与证书 CN 不一致时使用）                                   |
| `server.insecureSkipVerify` | `false` | 跳过证书校验（仅排障，禁止生产开启）                                       |

### agent（身份与可达性）

| 键                           | 默认                                  | 说明                                                                                                |
| ---------------------------- | ------------------------------------- | --------------------------------------------------------------------------------------------------- |
| `agent.id`                   | `""`                                  | 留空自动生成；多 Agent 部署建议显式指定（拓扑页可读性）                                             |
| `agent.gameId` / `agent.env` | 必填                                  | 作用域绑定；留空启动失败（单 agent 单 game+env）                                                    |
| `agent.localAddr`            | 空→host:port 兜底，再兜底 `0.0.0.0:0` | 本地 TCP 监听（游戏服函数注册入口）                                                                 |
| `agent.httpAddr`             | 必填                                  | **Server→Agent 回调地址**，必须是 Server 视角可达的地址（容器网络用服务名，裸机用 IP）              |
| `agent.labels`               | `{}`                                  | 自定义标签（机房/机型等，节点页过滤用）                                                             |
| `agent.invokeTimeoutMs`      | `15000`                               | Agent→游戏服同步调用默认预算（毫秒）；请求 `metadata["timeoutMs"]` 声明更小值时取更小者，上限 60000 |

> `httpAddr` 是最常见的部署错误：写 `0.0.0.0` 或 `localhost` 会导致 Server 回调失败——job 下发/文件传输走不通。

### upstream（连接韧性）

| 键                           | 默认            | 说明                                                                           |
| ---------------------------- | --------------- | ------------------------------------------------------------------------------ |
| `upstream.heartbeatInterval` | `0`→核心默认 3s | 心跳秒数；Server 侧按 3 倍间隔判失联                                           |
| `upstream.retryInterval`     | `5`             | **DEAD**：解析后无人读取；重连退避固定 5s→60s×1.5（core/register/register.go） |
| `upstream.maxRetries`        | `3`             | **DEAD**：解析后无人读取（同上）                                               |
| `upstream.timeout`           | `10000`         | 请求超时（毫秒）                                                               |

### tls（本地 `:19091` 监听的入站 TLS）

`tls.enabled / certFile / keyFile / caFile / insecureSkipVerify`——加密 SDK Provider 拨入 Agent 本地网关的连接；TLS 在分帧之下，wire 协议不变。`enabled: true` 时 `certFile` + `keyFile` 必配（缺失启动即报错）；`caFile` 配置后启用双向 TLS（Provider 须出示 CA 签发的客户端证书）。单条连接握手失败仅丢弃该连接（记告警日志），不影响 Accept 循环与其余会话。SDK 侧需支持 TLS 拨号选项（六语言 SDK 逐个跟进，未跟进前保持 `enabled: false`）。

### outboundTLS（Agent → 本地 Provider 出站调用链 TLS）

Agent 出站调用游戏侧 Provider（本地网关 `callLocalProvider` 拨号）时的客户端 TLS：`enabled / certFile / keyFile / caFile / serverName / insecureSkipVerify`。`enabled: false`（默认）时该链路保持明文（兼容回退）。注意与 `server:` 段分工：**agent → server 上游链路 TLS 由 `server:` 段驱动**（`server.insecure: false` 时启用，证书字段同在 `server` 段），`outboundTLS` 只作用于 Agent 调本地 Provider 这条出站链路。生产推荐内部 CA 签发，见 [TLS 与证书](./tls-certificates)。

### ops（指标上报与进程监管）

| 键                    | 默认                                        | 说明                               |
| --------------------- | ------------------------------------------- | ---------------------------------- |
| `ops.enabled`         | `false`                                     | 系统指标采集上报（须显式开启）     |
| `ops.metricsInterval` | `30s`                                       | 采集周期（也是托管进程的采样周期） |
| `ops.metricsEnabled`  | `false`（YAML 缺省即 false，需显式置 true） | 指标开关（关掉采集但保留上报通道） |

> 注：早期部署存在 `ops.metrics_interval` / `ops.metrics_enabled` snake_case 旧键，仍被兼容解析；新配置一律用上表 lowerCamelCase 键。

### ops.managedProcesses（托管进程监管，supervisor）

> **sidecar 二进制不解析此块（死配置）**：本节为 agentcore 设计面，sidecar agent 的 OpsConfig 仅 3 键（enabled/metricsInterval/metricsEnabled），managedProcesses 从不解析。

每个条目以逻辑名为键，声明一个 agent 托管进程。S1（只读监控）已交付：每 `ops.metricsInterval` 采样一次 RSS/CPU，超阈值在面板打标记（不自动杀进程），快照随 metrics 上报捎带。

**管理状态**：运维手动 stop 的进程期望状态落盘 supervisor 日志目录 `desired_state.json`——`autoRestart: true` 只恢复**崩溃**的进程，agent 重启后开机自启不误启运维主动停掉的进程；手动 start/restart 恢复期望运行。文件缺失或损坏时 fail-open（等同无记录，沿用 autoRestart 既有语义）。

**调度状态**（§10.6）：`schedule[]` 声明定时 start/stop/restart。定时触发与恢复环是两条执行路径——定时停与手动停同信号（stopCh），恢复环不会把定时停误拉起；触发走显式动作同一漏斗（期望状态与 `schedule_fire` 事件随流，与 `manual_start`/`manual_stop` 事件分型）。快照携带下一次触发时间与动作（`nextScheduleAtUnix`/`nextScheduleAction`，无调度为零值）。边界诚实：schedule 是 agent 本地配置意图，与 `autoRestart`/开机自启同信任根，**不受 `allowRestart` 门禁**（`allowRestart` 门的是远程操作面）——要禁用定时动作请删除 `schedule` 配置。**暂停/恢复调度**（§10.2 暂停与恢复，暂停对象=调度）：`POST /ops/agents/:agentId/processes/:name/schedule/pause`（或 `resume`）冻结/解冻该进程全部 cron 触发，进程运行态不动，暂停期间错过的槽位不补跑（crond 语义）；**它是远程运维动作，走 `allowRestart` 双闸**（与上面「触发本身不受门禁」不矛盾——门禁的是远程开关，不是本地配置意图），无独立配置键；暂停意图落盘 `logs/supervisor/schedule_pause_state.json`（跨 agent 重启保留，损坏 fail-open 空映射），快照上行 `scheduleSuspended`，事件留痕 `schedule_paused`/`schedule_resumed`。

### ops.allowExec（远程命令执行，§10.7 边界三件套）

**agentcore 设计面，sidecar 二进制不解析此块（死配置）**——与 managedProcesses 同。`allowExec: true` 开启远程命令执行（`POST /ops/agents/:agentId/exec`），三条前置缺一不可：

1. **双闸**（既有口径）：`ops.enabled: true` + `allowExec: true` 都缺省 false，须显式同开。
2. **白名单 fail-closed**：`execAllowedCommands` 必须非空——空列表一律拒绝一切命令，**不再等价「放行一切」**（2026-10-15 物化前为 fail-open，已纠正）。`OpsConfig.Validate()` 同步拦配置面。
3. **审计**：每次尝试（成功 / 业务失败 / 客户端不可用）写 `execution_logs`（`source=agent_exec`），操作人、命令、参数、退出码、耗时入审计载荷；审计面缺失不阻断执行主路径（fail-open）。

> 注意：面板不设通用 Shell 入口——程序操作一律走托管进程的显式动作（start/stop/restart），这是 §10.7「程序操作=明确受授权动作」的既有口径。

**管理状态**：运维手动 stop 的进程期望状态落盘 supervisor 日志目录 `desired_state.json`——`autoRestart: true` 只恢复**崩溃**的进程，agent 重启后开机自启不误启运维主动停掉的进程；手动 start/restart 恢复期望运行。文件缺失或损坏时 fail-open（等同无记录，沿用 autoRestart 既有语义）。

| 键                                        | 默认      | 说明                                                                                                                                                                                                                                                                                                         |
| ----------------------------------------- | --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `command` / `args` / `workingDir` / `env` | —         | 进程启动参数                                                                                                                                                                                                                                                                                                 |
| `kind`                                    | `program` | 受管对象类别（闭集 `program` / `probe-adapter`，缺省 program）。`probe-adapter`=外部探测适配器，走与 program 同一套生命周期（§10.1）                                                                                                                                                                         |
| `adapterId` / `adapterVersion`            | 空        | 适配器身份与版本（清单 metadata.id / version）。`kind=probe-adapter` 时 `adapterId` **必填**，缺失配置校验失败——它是探测面引用与授权校验的锚点                                                                                                                                                               |
| `adapterProtocol` / `adapterCapabilities` | —         | 适配器 IPC 协议名（缺省 `croupier-probe-jsonl-v1`）与声明能力集（如 `login` / `handshake`）。**声明≠已验证**：平台仍以实际探测结果判定目标状态；游戏地址/频率/授权/凭证不进此清单（留探测面配置）                                                                                                            |
| `autoRestart`                             | `false`   | 崩溃自动拉起开关（指数退避 + 熔断，S2 已交付）                                                                                                                                                                                                                                                               |
| `memThresholdBytes`                       | `0`（关） | RSS 达到该值打 `mem_over_limit` 标记                                                                                                                                                                                                                                                                         |
| `cpuThresholdPercent`                     | `0`（关） | CPU% 达到该值打 `cpu_over_limit` 标记                                                                                                                                                                                                                                                                        |
| `snapshotProfile`                         | `none`    | S3 崩溃快照档，闭集 `none/go/node/python/jvm`（显式声明，不做语言自动探测）：`go` 注入 `GOTRACEBACK=crash`；`node` 前置 `--report-on-fatalerror --report-exclude-env`；`python` 注入 `PYTHONFAULTHANDLER=1`；`jvm` 注入 `JAVA_TOOL_OPTIONS`（`-XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=<快照目录>`） |
| `schedule[]`                              | 空        | 定时动作列表（§10.6 调度状态）：`action` 闭集 `start/stop/restart` + `cron` 五字段（分 时 日 月 周，与 server 侧任务调度同一解析器）。到点走与面板显式动作同一漏斗（期望状态落盘 + `schedule_fire` 事件随流）；30s 分钟槽扫描去重，无补跑（agent 停机错过的槽位不补触发）                                    |

```yaml
ops:
  enabled: true
  metricsInterval: 30s
  # S3：崩溃快照产物根目录（每进程一子目录，权限 0700——产物含进程内存）。
  snapshotDir: logs/snapshots
  managedProcesses:
    demo-app:
      command: /usr/local/bin/demo-app
      args: ["--port", "8080"]
      workingDir: /srv/demo-app
      memThresholdBytes: 2147483648 # 2GiB
      cpuThresholdPercent: 90
      snapshotProfile: go # 崩溃时全 goroutine 栈转储并尝试 core dump
      schedule: # 定时动作（§10.6）：凌晨停、工作日早晨起
        - action: stop
          cron: "0 3 * * *"
        - action: start
          cron: "30 7 * * 1-5"
```

面板入口：节点维护页「监管」列 → 进程监管抽屉（状态灯/PID/运行时长/内存/CPU/超限标记/快照档）。崩溃时 `detect_down` 事件登记快照目录与产物文件名；宿主机 `core_pattern` 非管道模式时 agent 发一次 `snapshot_hint` 只读提示（含建议命令，agent 不写宿主配置）。保留期/配额/产物下载权限见 [Agent Supervisor 设计简档](../design/agent-supervisor-design.md)。

### ops.allowUpdate（受管程序二进制更新，§10.4）

**agentcore 设计面，sidecar 二进制不解析此块（死配置）**——与 managedProcesses 同。`allowUpdate: true` 开启远程换包（`POST /ops/agents/:agentId/processes/:name/update`，入参 `stagedPath` + `expectedSha256`），与 allowExec 同属双闸家族（`ops.enabled` + `allowUpdate` 都缺省 false，须显式同开），独立键控。

**更新状态**（§10.4）：流程=内容哈希钉扎（staged 文件 sha256 ≠ `expectedSha256` 一律拒绝，fail-closed 不碰运行态）→ 先停后换（静默核心，不发 `manual_*` 中间事件）→ 旧二进制备份 `<command>.croupier-bak`（**成功后保留**为 last-known-good，滚动覆盖，供人工回滚）→ 拉起 → 稳定性窗口（缺省 5s）内退出即自动回滚旧版。更新意图落 desired running（与手动 restart 一致）。指纹留痕三处：supervisor 事件 `update_applied` / `update_rolled_back`（message 带旧→新 sha256）、server 侧 execution_logs（`source=process_update`，持久查询主入口，按目标+时间+指纹检索）、进程快照 `fingerprint` 字段随 metrics 上行。

边界诚实（v1）：签名校验未实现（只做内容哈希——哈希是稳定可查锚点，来源可信性由分发通道保证）；数据库/配置兼容性检查未实现（属游戏侧知识，由调用方把关）；游戏进程维护窗口/在线阈值/流量导入闸未核实，随首个真实游戏接入定；首次安装（无旧版）即失败无事件（留痕在 execution_logs）。

### probe（声明式探针面）

Agent 侧探针配置（R2）。`targets[]` 只声明**探什么**；阈值、语义、采样间隔与上行口径由 `core/healthprobe` 给显式默认。**`probe:` 段存在即参与探测面**（R4）：`targets` 为空是合法声明（空静态 + 动态可增删——server 按 DB 源归属派生的 db 探针目标会下发进来）；整段缺省 = 不装配探针面（零行为差异）。任一本地目标非法（重名/类型未知/缺必填参数）**启动即失败**，不做静默跳过——重名会让上行路由串台。

| 键                          | 默认        | 说明                                                                   |
| --------------------------- | ----------- | ---------------------------------------------------------------------- |
| `probe.targets`             | 空→仅动态面 | 本地目标列表（见下表）；空 = 只接 server 派生的 db 目标（R4）          |
| `probe.maxDetailsPerWindow` | `10`        | 每个故障窗口每目标的失败明细条数上限（上限只裁明细，不裁失败总数事实） |

**单目标键位**（`core/healthprobe.TargetConfig`，键一律小驼峰）

| 键                     | 默认        | 说明                                                                               |
| ---------------------- | ----------- | ---------------------------------------------------------------------------------- |
| `name`                 | 必填        | 目标标识（同一 agent 内唯一；告警行标识与指标路径 `probe.<target>.*` 都用它）      |
| `type`                 | 必填        | 闭集 `http` / `tcp` / `db`（`process` 不进工厂，由 supervisor 侧负责）             |
| `url`                  | type=http   | HTTP 探测地址                                                                      |
| `address`              | type=tcp    | `host:port`                                                                        |
| `dsn`/`driver`/`query` | type=db     | 数据库连接串 / 驱动名（由二进制依赖注册的驱动）/ 断言 SQL                          |
| `timeout`              | 类型内默认  | 单次探测超时                                                                       |
| `semantics`            | `readiness` | 视角声明，闭集 `readiness`（就绪：功能是否可用）/ `liveness`（存活：进程是否活着） |
| `interval`             | `30s`       | 本地基础采样频率——**下发策略未覆盖该目标时的兜底值**                               |
| `failThreshold`        | `3`         | 连续失败多少次开故障窗口                                                           |
| `recoverThreshold`     | `1`         | 连续成功多少次闭故障窗口                                                           |

```yaml
probe:
  maxDetailsPerWindow: 10
  targets:
    - name: game-db-primary
      type: db
      dsn: "user=app@tcp(127.0.0.1:3306)/demo"
      driver: mysql
      query: "SELECT 1"
      semantics: readiness
      interval: 30s
      failThreshold: 3
    - name: platform-api
      type: http
      url: "http://127.0.0.1:8080/healthz"
      semantics: liveness
      timeout: 3s
    - name: redis
      type: tcp
      address: "127.0.0.1:6379"
```

**采样频率的唯一真值在 Server**（配置与执行分离）：面板发布 `probe.policy` 采样策略后经 wire 下发、agent 热生效（基础/按目标/时段/特殊时点四粒度叠加），此时**策略优先于本表 `interval`**，本表 `interval` 仅作未覆盖目标的兜底；策略未发布时用本表值。详见 [API 文档 probe 端点](../api/ops.md#probe-端点) 与 [评审简档 §13.3](../design/agent-health-recovery-review.md)。

**派生 db 探针目标（R4）**：`probe:` 段存在的 agent 会周期拉取 server 按 `db_sources.owner_agent_id` 派生的 db 目标集（30s 轮询，目标名 `db:<源名>`、重名源 `#<id>` 后缀消歧、driver 映射 postgres→pgx、探测断言 `SELECT 1`）。拉取失败或「空版本+空正文」（服务端不可派生）时保持现有目标；**空集**是真实状态（解除全部归属），agent 会卸掉全部动态探针；版本为目标集内容 hash，集不变则跳过。派生目标与本地目标同名时整体拒绝下发（下轮重试），不静默串台。DSN 明文只在 control 连接回给归属 agent，与本地 `probe.targets` 同信任边界。

探针结果两处可见：节点维护页「探针汇总」卡（状态矩阵，异常行前置），以及告警中心（窗口开/合转 firing/resolved 生命周期，告警 ID `probe:<agentId>:<target>`）。

## 双 Agent / 多 Agent 部署

同一宿主多 Agent（多游戏或多环境）：复制配置差异点只有 `agent.id`、`agent.gameId/env`、`agent.localAddr`、`agent.httpAddr` 四项（`configs/agent2.yaml` 不入库，由运维复制 `configs/agent.yaml` 改上述四项生成——`docker/docker-compose.deploy.yml` 的 `agent2` 服务按该路径挂载）。上游 `server.addr` 共用同一 LB 入口。

## 环境变量

本二进制无「env 覆盖 YAML」层——下表为运行时环境变量（不映射配置键）：

| 变量                                                               | 默认              | 说明                                                   |
| ------------------------------------------------------------------ | ----------------- | ------------------------------------------------------ |
| `CROUPIER_CONFIG_DIR`                                              | —                 | 配置目录（优先于 `--config` 指向）                     |
| `CROUPIER_AGENT_LOCAL_ADDR`                                        | 空→host:port 兜底 | 本地 TCP 监听地址（覆盖 `agent.localAddr` 的兜底路径） |
| `CROUPIER_AGENT_VERSION`                                           | —                 | 版本号覆盖（注册/上报用）                              |
| `CROUPIER_AGENTLOCAL_PROBE_INTERVAL`                               | 5s                | provider keepalive 探针间隔                            |
| `CROUPIER_AGENTLOCAL_PRUNE_INTERVAL`                               | 30s               | 会话清理周期                                           |
| `CROUPIER_AGENTLOCAL_MAX_AGE`                                      | 2m                | 会话最大存活                                           |
| `CROUPIER_AGENTLOCAL_TASKRESULT_MAX_AGE`                           | 10m               | 任务结果保留时长                                       |
| `CROUPIER_EXTENSION_SYNC_API` / `CROUPIER_EXTENSION_SYNC_INTERVAL` | 30s               | 扩展同步地址与周期                                     |

## 验证

1. Agent 日志出现 session 建立与注册成功
2. Dashboard「运维中心 → 节点维护」看到 agent 在线、标签正确
3. 从 Dashboard 触发一次该 Agent 名下函数调用，验证回调链路（`httpAddr` 正确性）
