---
title: Agent 配置指南
icon: settings
order: 11
category:
  - 运维手册
tag:
  - 配置
  - agent
---

# Agent 配置指南

对应 `configs/agent.yaml`（仓库内模板）。该二进制**没有环境变量展开层**：配置只从 YAML 读取；运行时环境变量仅 `CROUPIER_CONFIG_DIR` / `CROUPIER_AGENTLOCAL_*` 等（见 agent-config.md 的环境变量表），CLI 参数只有 `--config` 指定配置文件路径。

## 全量键参考

### server（上游连接）

| 键                          | 默认    | 说明                                                                           |
| --------------------------- | ------- | ------------------------------------------------------------------------------ |
| `server.addr`               | 必填    | Server transport 入口。生产指向 L4 LB（HAProxy/NLB/Service），不要指向具体实例 |
| `server.insecure`           | `false` | 默认走 TLS；`true` 为明文连接（仅开发排障）                                    |
| `server.serverName`         | `""`    | TLS SNI/校验名（与证书 CN 不一致时使用）                                       |
| `server.insecureSkipVerify` | `false` | 跳过证书校验（仅排障；生产禁用）                                               |

### agent（身份与可达性）

| 键                           | 默认                                | 说明                                                                                              |
| ---------------------------- | ----------------------------------- | ------------------------------------------------------------------------------------------------- |
| `agent.id`                   | `""`                                | 留空自动生成；多 agent 部署建议显式指定（拓扑页可读性）                                           |
| `agent.gameId` / `agent.env` | 必填                                | 归属绑定；留空启动失败（一个 agent 只属于一个 game+env）                                          |
| `agent.localAddr`            | 空 → host:port 回落，再 `0.0.0.0:0` | 本地 TCP 监听（游戏服函数注册的入口）                                                             |
| `agent.httpAddr`             | 必填                                | **Server→Agent 回调地址**；必须是 Server 视角可达的地址（容器网络用服务名，裸金属用 IP）          |
| `agent.labels`               | `{}`                                | 自定义标签（机房/机型等，节点页过滤用）                                                           |
| `agent.invokeTimeoutMs`      | `15000`                             | Agent→游戏服同步调用的默认预算（ms）；请求 `metadata["timeoutMs"]` 声明更小值时取小者；上限 60000 |

> `httpAddr` 是最常见的部署错误：写成 `0.0.0.0` 或 `localhost` 会导致 Server 回调失败——任务派发/文件传输都不通。
>
> 注册标签包含 agent 自报的系统标签（`os`/`arch`/`hostname`/`ip` 等）与运行二进制指纹 `agentBinarySha256`（agent-supervisor-design §10.5 的持久化锚点：每次注册自报，随注册标签一起落库；agent 掉线后系统信息用它回答「最后已知指纹」）。该指纹在配置/动态标签合并之后注入——在 `agent.labels` 里显式写同名键无效（计算事实不可覆盖）。

### upstream（连接韧性）

| 键                           | 默认            | 说明                                                                           |
| ---------------------------- | --------------- | ------------------------------------------------------------------------------ |
| `upstream.heartbeatInterval` | `0`→内核默认 3s | 心跳秒数；Server 按 3 倍间隔判定 agent 失联                                    |
| `upstream.retryInterval`     | `5`             | **DEAD**：解析后无人读取；重连退避固定 5s→60s×1.5（core/register/register.go） |
| `upstream.maxRetries`        | `3`             | **DEAD**：解析后无人读取（同上）                                               |
| `upstream.timeout`           | `10000`         | 请求超时（ms）                                                                 |

### tls（本地 `:19091` 监听的入站 TLS）

`tls.enabled / certFile / keyFile / caFile / insecureSkipVerify`——加密 SDK Provider 拨入 Agent 本地网关的连接；TLS 位于分帧之下，线上协议不变。`enabled: true` 时 `certFile` + `keyFile` 必填（缺失 → 启动即报错）；配置 `caFile` 即启用双向 TLS（Provider 必须出示 CA 签发的客户端证书）。单连接握手失败只断该连接（记 warning），不影响 Accept 循环与其他会话。SDK 侧需要 TLS 拨号选项（六语言 SDK 正在逐一跟进；在那之前保持 `enabled: false`）。

### outboundTLS（Agent → 本地 Provider 出站调用链的 TLS）

Agent 出站调用游戏侧 Provider（本地网关 `callLocalProvider` 拨号）时的客户端 TLS：`enabled / certFile / keyFile / caFile / serverName / insecureSkipVerify`。`enabled: false`（默认）时该链路保持明文（兼容回落）。注意与 `server:` 块的分工：**agent → server 上行链路的 TLS 由 `server:` 块驱动**（`server.insecure: false` 即启用；证书字段也在 `server` 块里），`outboundTLS` 只作用于 Agent→本地 Provider 出站链路。生产建议内部 CA，见 TLS 与证书。

### ops（指标上报与进程监管）

| 键                    | 默认                                        | 说明                               |
| --------------------- | ------------------------------------------- | ---------------------------------- |
| `ops.enabled`         | `false`                                     | 系统指标采集与上报（必须显式开启） |
| `ops.metricsInterval` | `30s`                                       | 采集间隔（也是受管进程的采样间隔） |
| `ops.metricsEnabled`  | `false`（YAML 默认 false，必须显式置 true） | 指标开关（停采集但保留上报通道）   |

> 注意：更老的部署用过 snake_case 旧键 `ops.metrics_interval` / `ops.metrics_enabled`，为兼容仍会解析；新配置一律用上表的 lowerCamelCase 键。
> 其余 `ops` 键（`allowRestart` / `allowExec` / `allowUpdate`、`managedProcesses`、
> `execAllowedCommands`、`supervisorLog`、`snapshotDir`）由 sidecar CLI 直接解析并透传
> 给 agentcore 的 `Validate()`；非法闭集、cron、allowlist 配置启动即失败退出。

### ops.managedProcesses（受管进程监管，supervisor）

> `autoRestart: true` 的进程随 sidecar 正常启动路径拉起；被运维手工停掉的进程，其期望状态在 sidecar 重启后依然保持。

每个条目以逻辑名为键，声明一个 agent 托管进程。S1（只读监控）已交付：每 `ops.metricsInterval` 采样一次 RSS/CPU；超阈值在面板打旗（不自动杀进程），快照随指标上报捎带。

**受管状态**：被运维手工停掉的进程，其期望状态持久化在 supervisor 日志目录的 `desired_state.json`——`autoRestart: true` 只复活**崩溃**的进程，agent 重启后的开机自启不会误拉运维有意停掉的进程；手工 start/restart 会恢复期望的运行态。文件缺失或损坏时 fail-open（视为无记录，沿用既有 autoRestart 语义）。

**调度状态**（§10.6）：`schedule[]` 声明定时启停/重启。定时触发与恢复循环是两条执行路径——定时停止与手工停止用同一信号（stopCh），恢复循环不会误拉 schedule 停掉的进程；触发与显式操作走同一漏斗（期望状态落盘 + 捎带 `schedule_fire` 事件，与 `manual_start`/`manual_stop` 事件分开定型）。快照携带下次触发时间与动作（`nextScheduleAtUnix`/`nextScheduleAction`；无 schedule 时为零值）。诚实边界：schedule 是 agent 本地的配置意图，信任根与 `autoRestart`/开机自启相同，**不受 `allowRestart` 门禁**（`allowRestart` 门禁的是远程操作面）——要停用定时动作，删掉 `schedule` 配置即可。**暂停/恢复 schedule**（§10.2 暂停与恢复；暂停对象 = 调度）：`POST /ops/agents/:agentId/processes/:name/schedule/pause`（或 `resume`）冻结/解冻该进程的全部 cron 触发，进程运行态不受影响，暂停期间错过的槽位不补跑（crond 语义）；**它是远程运维动作，受 `allowRestart` 双闸约束**（这与上文「触发本身不受门禁」不矛盾——门禁覆盖的是远程开关，不是本地配置意图）；无专用配置键，暂停意图持久化到 `logs/supervisor/schedule_pause_state.json`（agent 重启不丢；损坏 → fail-open 空表）；快照向上报告 `scheduleSuspended`，事件留痕 `schedule_paused`/`schedule_resumed`。

### ops.allowExec（远程命令执行，§10.7 三段边界）

`allowExec: true` 启用远程命令执行（`POST /ops/agents/:agentId/exec`）；三个前置条件缺一不可：

1. **双闸**（既有基线）：`ops.enabled: true` + `allowExec: true` 两者默认 false，必须一起显式打开。
2. **Allowlist fail-closed**：`execAllowedCommands` 必须非空——空列表拒绝一切命令，**不再等价于「全放行」**（2026-10-15 物化之前是 fail-open，已修正）。`OpsConfig.Validate()` 在配置面同步拦截。
3. **审计**：每次尝试（成功 / 业务失败 / 客户端不可用）写 `execution_logs`（`source=agent_exec`）；操作者、命令、参数、退出码、耗时进入审计载荷；审计面缺失不阻塞主执行路径（fail-open）。

> 注意：面板没有通用 Shell 入口——程序操作一律走受管进程的显式动作（start/stop/restart）；这就是 §10.7「程序操作 = 显式授权动作」的既有基线。

**受管状态**：被运维手工停掉的进程，其期望状态持久化在 supervisor 日志目录的 `desired_state.json`——`autoRestart: true` 只复活**崩溃**的进程，agent 重启后的开机自启不会误拉运维有意停掉的进程；手工 start/restart 会恢复期望的运行态。文件缺失或损坏时 fail-open（视为无记录，沿用既有 autoRestart 语义）。

| 键                                        | 默认      | 说明                                                                                                                                                                                                                                                                                                           |
| ----------------------------------------- | --------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `command` / `args` / `workingDir` / `env` | —         | 进程启动参数                                                                                                                                                                                                                                                                                                   |
| `kind`                                    | `program` | 受管对象类别（闭集 `program` / `probe-adapter`，默认 program）。`probe-adapter` = 外部探测适配器，与 program 同一生命周期（§10.1）                                                                                                                                                                             |
| `adapterId` / `adapterVersion`            | 空        | 适配器身份与版本（manifest 的 metadata.id / version）。`kind=probe-adapter` 时 `adapterId` **必填**，缺失 → 配置校验失败——它是探测面引用与鉴权检查的锚点                                                                                                                                                       |
| `adapterProtocol` / `adapterCapabilities` | —         | 适配器 IPC 协议名（默认 `croupier-probe-jsonl-v1`）与声明能力集（如 `login` / `handshake`）。**声明 ≠ 校验**：平台仍以实际探测结果判定目标状态；游戏地址/频率/授权/凭据不进 manifest（留给探测面配置）                                                                                                         |
| `autoRestart`                             | `false`   | 崩溃自动重启与 sidecar 启动拉起的开关（指数退避 + 熔断，S2 已交付）                                                                                                                                                                                                                                            |
| `memThresholdBytes`                       | `0`（关） | RSS 达到该值时打 `mem_over_limit` 旗                                                                                                                                                                                                                                                                           |
| `cpuThresholdPercent`                     | `0`（关） | CPU% 达到该值时打 `cpu_over_limit` 旗                                                                                                                                                                                                                                                                          |
| `snapshotProfile`                         | `none`    | S3 崩溃快照档位，闭集 `none/go/node/python/jvm`（显式声明，不做语言自动探测）：`go` 注入 `GOTRACEBACK=crash`；`node` 前置 `--report-on-fatalerror --report-exclude-env`；`python` 注入 `PYTHONFAULTHANDLER=1`；`jvm` 注入 `JAVA_TOOL_OPTIONS`（`-XX:+HeapDumpOnOutOfMemoryError -XX:HeapDumpPath=<快照目录>`） |
| `schedule[]`                              | 空        | 定时动作列表（§10.6 调度状态）：`action` 闭集 `start/stop/restart` + 五字段 `cron`（分 时 日 月 周，与 server 侧任务调度同一解析器）。触发与面板显式操作走同一漏斗（期望状态落盘 + 捎带 `schedule_fire` 事件）；30s 分钟槽扫描去重，不补跑（agent 宕机期间错过的槽位不再触发）                                 |

```yaml
ops:
  enabled: true
  metricsInterval: 30s
  # S3: root directory for crash snapshot artifacts (one subdirectory per process, permission 0700 — artifacts contain process memory).
  snapshotDir: logs/snapshots
  managedProcesses:
    demo-app:
      command: /usr/local/bin/demo-app
      args: ["--port", "8080"]
      workingDir: /srv/demo-app
      memThresholdBytes: 2147483648 # 2GiB
      cpuThresholdPercent: 90
      snapshotProfile: go # dump full goroutine stacks on crash and attempt a core dump
      schedule: # timed actions (§10.6): stop at night, start on weekday mornings
        - action: stop
          cron: "0 3 * * *"
        - action: start
          cron: "30 7 * * 1-5"
```

面板入口：节点维护页「监管」列 → 进程监管抽屉（状态灯/PID/uptime/内存/CPU/超限旗/快照档位）。崩溃时 `detect_down` 事件记录快照目录与产物文件名；宿主机 `core_pattern` 不是管道模式时，agent 发一次性只读 `snapshot_hint` 提示（含建议命令；agent 不写宿主机配置）。保留/配额/产物下载权限：见 Agent Supervisor 设计简报（本节仅中文）。

### ops.allowUpdate（受管程序二进制更新，§10.4）

`allowUpdate: true` 启用远程换包（`POST /ops/agents/:agentId/processes/:name/update`，输入 `stagedPath` + `expectedSha256`）；与 allowExec 同属双闸家族（`ops.enabled` + `allowUpdate` 都默认 false，必须一起显式打开），键控相互独立。

**更新状态**（§10.4）：流程 = 内容 hash 钉扎（staged 文件 sha256 ≠ `expectedSha256` 一律拒绝，fail-closed，运行态不动）→ 先停后换（安静核心，无 `manual_*` 中间事件）→ 旧二进制备份为 `<command>.croupier-bak`（**成功后保留**，作为 last-known-good，每次滚动覆盖，供手工回滚）→ 拉起 → 稳定窗口（默认 5s）内退出即自动回滚旧版。更新意图落为期望运行（与手工 restart 相同）。指纹三处留痕：supervisor 事件 `update_applied` / `update_rolled_back`（message 带旧→新 sha256）、server 侧 execution_logs（`source=process_update`，主持久查询入口，可按 目标+时间+指纹 检索）、进程快照 `fingerprint` 字段随指标上报。

诚实边界（v1）：签名校验未实现（仅内容 hash——hash 是稳定可查的锚点，来源可信由分发渠道保证）；数据库/配置兼容性检查未实现（游戏侧知识，由调用方把关）；游戏进程维护窗口/在线阈值/流量接收闸未验证，待首个真实游戏接入时收口；首次安装（无旧版）失败无事件（trace 留在 execution_logs）。

### probe（声明式探测面）

Agent 侧探测配置（R2）。`targets[]` 只声明**探测什么**；阈值、语义、采样间隔、上报语义由 `core/healthprobe` 给显式默认。**出现 `probe:` 块即编入探测面**（R4）：空 `targets` 是合法声明（空静态 + 动态目标可增删——server 依 DB 源归属派生的 db 探测目标会推进来）；整块省略 = 不装配探测面（零行为差异）。任何非法本地目标（重名/未知类型/缺必填参数）**启动即失败**——没有静默跳过：重名会导致上行路由串线。

| 键                          | 默认        | 说明                                                                 |
| --------------------------- | ----------- | -------------------------------------------------------------------- |
| `probe.targets`             | 空 → 仅动态 | 本地目标列表（见下表）；空 = 只有 server 派生的 db 目标（R4）        |
| `probe.maxDetailsPerWindow` | `10`        | 每目标每故障窗口的失败明细条数上限（只裁明细，不裁失败次数这一事实） |

**单目标键**（`core/healthprobe.TargetConfig`，键全为 lowerCamelCase）

| 键                     | 默认        | 说明                                                                               |
| ---------------------- | ----------- | ---------------------------------------------------------------------------------- |
| `name`                 | 必填        | 目标标识（同一 agent 内唯一；告警线标识与指标路径 `probe.<target>.*` 都用它）      |
| `type`                 | 必填        | 闭集 `http` / `tcp` / `icmp` / `db`（`process` 不走工厂，由 supervisor 侧处理）    |
| `url`                  | type=http   | HTTP 探测地址                                                                      |
| `address`              | type=tcp    | `host:port`                                                                        |
| `address`              | type=icmp   | 主机名或 IP；ICMP 还支持 `count`（默认 1，需 `CAP_NET_RAW`）                       |
| `dsn`/`driver`/`query` | type=db     | 数据库连接串 / 驱动名（二进制依赖注册的驱动）/ 断言 SQL                            |
| `timeout`              | 按类型默认  | 单次探测超时                                                                       |
| `semantics`            | `readiness` | 视角声明，闭集 `readiness`（就绪：功能是否可用）/ `liveness`（存活：进程是否在跑） |
| `interval`             | `30s`       | 本地基础采样频率——**推送策略未覆盖该目标时的回落值**                               |
| `failThreshold`        | `3`         | 连续失败多少次开故障窗                                                             |
| `recoverThreshold`     | `1`         | 连续成功多少次关故障窗                                                             |

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
    - name: gateway
      type: icmp
      address: "192.0.2.1"
      timeout: 2s
```

**采样频率的单一事实源在 Server**（配置与执行分离）：面板发布 `probe.policy` 采样策略后经线上推送、agent 热生效（四种叠加粒度：基础/按目标/时间窗/特殊时点）；届时**策略优先于本表 `interval`**，后者只作未覆盖目标的回落；未发布策略时按本表取值。见 API 文档探测端点与 Review 简报 §13.3。

**派生 db 探测目标（R4）**：带 `probe:` 块的 agent 周期拉取 server 从 `db_sources.owner_agent_id` 派生的 db 目标集（30s 轮询，目标名 `db:<源名>`，源重名时以 `#<id>` 后缀消歧，driver 映射 postgres→pgx，探测断言 `SELECT 1`）。拉取失败或「空版本 + 空体」（server 派生不出任何东西）时保留现有目标；**空集**是真实状态（归属全部移除），agent 拆除全部动态探测；版本是目标集的内容 hash——集合未变则跳过。派生目标与本地目标重名时整批拒绝推送（下轮重试）——绝不静默串线。DSN 明文只经控制连接下发给属主 agent，信任边界与本地 `probe.targets` 相同。

探测结果两处可见：节点维护页「探测汇总」卡片（状态矩阵，异常行置前）与告警中心（开/关窗驱动 firing/resolved 生命周期；告警 ID `probe:<agentId>:<target>`）。

## 双 Agent / 多 Agent 部署

一台宿主机跑多个 agent（多游戏或多环境）：配置复制后只需改四处——`agent.id`、`agent.gameId/env`、`agent.localAddr`、`agent.httpAddr`（`configs/agent2.yaml` 不入库；运维复制 `configs/agent.yaml` 改这四项即可——`docker/docker-compose.deploy.yml` 的 `agent2` 服务挂载该路径）。上游 `server.addr` 共用同一 LB 入口。

## 环境变量

该二进制没有「env 覆盖 YAML」层——下表是运行时环境变量（不映射配置键）：

| 变量                                                               | 默认                | 说明                                                   |
| ------------------------------------------------------------------ | ------------------- | ------------------------------------------------------ |
| `CROUPIER_CONFIG_DIR`                                              | —                   | 配置目录（优先于 `--config` 路径）                     |
| `CROUPIER_AGENT_LOCAL_ADDR`                                        | 空 → host:port 回落 | 本地 TCP 监听地址（覆盖 `agent.localAddr` 的回落路径） |
| `CROUPIER_AGENT_VERSION`                                           | —                   | 版本覆盖（用于注册/上报）                              |
| `CROUPIER_AGENTLOCAL_PROBE_INTERVAL`                               | 5s                  | Provider 保活探测间隔                                  |
| `CROUPIER_AGENTLOCAL_PRUNE_INTERVAL`                               | 30s                 | 会话修剪间隔                                           |
| `CROUPIER_AGENTLOCAL_MAX_AGE`                                      | 2m                  | 会话最大寿命                                           |
| `CROUPIER_AGENTLOCAL_TASKRESULT_MAX_AGE`                           | 10m                 | 任务结果保留时长                                       |
| `CROUPIER_EXTENSION_SYNC_API` / `CROUPIER_EXTENSION_SYNC_INTERVAL` | 30s                 | 扩展同步地址与间隔                                     |

## 验证

1. agent 日志显示会话建立与注册成功
2. Dashboard「运营中心 → 节点维护」看到 agent 在线且标签正确
3. 从 Dashboard 触发一次该 agent 的函数调用，验证回调链路（`httpAddr` 正确性）
