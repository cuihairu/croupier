# 三类 Agent 选型：sidecar / devops / capture

Croupier 有三类 agent 二进制，面向三种互不重叠的场景。本文回答三个问题：
**什么时候用哪一个、部署在哪、权限边界是什么**。逐字段配置说明见
[agent-config.md](./agent-config)，一键安装见 [agent-install.md](./agent-install)。

## 总览对比

|                | sidecar（croupier-agent）                                                      | devops-agent                                                                              | capture-agent                                                            |
| -------------- | ------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| **一句话定位** | 业务旁挂：函数注册/调用 + 健康上报                                             | 内网堡垒机式远程接入                                                                      | 游戏库变更捕获（CDC）+ 三闸风控                                          |
| **什么时候用** | 业务进程要被 GM 平台调用函数                                                   | 运维/客服要远程进内网机器排障                                                             | 要监控游戏库数据变更并告警                                               |
| **部署在哪**   | 与业务进程同主机/同网段                                                        | 内网运维堡垒机上（能 reach 目标机器）                                                     | 能直连源库 binlog 的旁路机上                                             |
| **监听端口**   | 本地网关 19091（SDK 注册入口）                                                 | 无监听（纯出站）                                                                          | 无监听（纯出站）                                                         |
| **出站连接**   | → Server 控制面 19090                                                          | → Server 控制面 19090                                                                     | → Server 控制面 19090                                                    |
| **权限边界**   | 只代理已注册的函数调用；进程管控 RPC 恒拒绝（`allowRestart/allowExec` 不解析） | 主机白名单（host+ports+protocols）+ 会话级审计留痕；`devops.enabled=false` 时不接任何会话 | 只读 binlog（REPLICATION SLAVE 最小权限账号）；不写源库                  |
| **能否多开**   | 每业务网段一台，多台可负载均衡                                                 | 每内网区一台                                                                              | 每源库一台（server_id 不得冲突）                                         |
| **注册标签**   | `role=sidecar`（随系统标签）                                                   | `role=devops`                                                                             | `role=capture`                                                           |
| **当前能力面** | 完整可用                                                                       | 骨架：注册/心跳/审计链已通，**会话协议通道未接线**（见下方边界）                          | 骨架→可用：MySQL binlog 采集 + 三闸判定 + 面板热下发已通（lineage 降级） |

## sidecar（croupier-agent）——业务旁挂 agent

**什么时候用**：游戏业务进程（或任何要接 GM 平台的服务）需要把函数注册到
Croupier、被 Web/审批流调用。SDK（Go/Python/Java/JS/C++/C#）经本地网关
`agent.localAddr`（默认 19091）连到 sidecar 注册 Provider；Server 下发
Invoke/Task 到 sidecar，sidecar 转发给已注册的业务会话执行。

**部署在哪**：与业务进程同主机或同网段（SDK 连 19091 即可）；每台 sidecar
以 `agent.gameId` + `agent.env` 绑定唯一 (game, env) 作用域。

**权限边界**：

- 只转发已注册函数的调用；同步调用预算默认 15s、上限 60s。
- ops 模块（`ops.enabled`）只开系统信息/指标/进程/定时任务**只读可见**；
  重启/停止/执行命令 RPC 在此二进制恒拒绝（结构体里那些字段不解析）。
- 无远程接入面：不提供 SSH/RDP/会话能力。

**组合关系**：与 capture 无交互；与 devops 是「业务面 vs 运维面」的分工，
可装在同一台机器（端口与进程互不影响）。

## devops-agent——内网堡垒机式远程接入

**什么时候用**：运维/客服需要从 GM 平台远程进入内网机器（打包机、CI 节点、
客服工位背后的游戏服）做排障操作，且要求**会话级审计留痕**与主机白名单。
参考形态：SSH/RDP/VNC 远程会话经 agent 代理（Web 终端参考 cockpit）。

**部署在哪**：内网运维区一台堡垒机式主机上——它必须能网络直达目标机器
（allowlist 里的 host:ports），同时能出站连 Server 控制面。不要求公网可达。

**权限边界**：

- `devops.allowlist` 是硬边界：`{host, ports, protocols}` 三元组，不在白名单
  的连接目标直接拒绝；`enabled: true` 时白名单必填非空。
- `devops.execlogDir` 逐会话执行审计（lumberjack 轮转，默认 100MB×10×30 天）。
- `devops.enabled=false`（默认）= 只注册上线、不接任何远程会话——装好即上线，
  但没有会话能力，是刻意的安全缺省。
- **已知边界（诚实声明）**：远程会话协议通道（SSH/RDP/VNC 经 guacd 一类网关）
  **尚未接线**——`devops.enabled=true` 时会话建立会得到「not implemented」
  拒绝。当前安装的意义是把 agent 纳入注册/心跳/审计监管面，等通道批落地。

**组合关系**：与 sidecar 可同机；与 capture 无交互。

## capture-agent——CDC 数据变更捕获

**什么时候用**：要监控游戏库（MySQL binlog）的数据变更并做风控判定——
对账闸（道具变更必须对上流水）、CEP 闸（非白名单时段/数量突变/连续小额
试探）、血缘闸（非白名单账号写入）。命中产生 `capture.rule_hit` 告警进
告警中心，规则经面板 `capture.gates` 热下发。

**部署在哪**：能直连源库（3306，binlog 订阅）的旁路机上；不与业务同机更佳
（binlog 消费有持续 CPU/内存开销）。上游 Server 地址可配。

**权限边界**：

- 源库账号最小权限：`REPLICATION SLAVE` + `REPLICATION CLIENT` + 订阅表
  SELECT；只读 binlog，不写源库。
- 订阅白名单 `capture.mysql.tables`（`"库.表": true`）控制带宽与噪声；
  `capture.enabled=false`（默认）= 只注册上线、不碰任何数据库。
- 前置条件：源库 `binlog_format=ROW`、`binlog_row_image=FULL`、`gtid_mode=ON`、
  `serverId` 不与源实例及其他订阅者冲突。
- **已知边界（诚实声明）**：血缘闸在 binlog 模式下降级——binlog 不携带会话
  账号，命中判定恒跳过并计数（注册标签 `captureLineage=degraded`），源侧
  补账号后零配置生效。

**组合关系**：与另两类无交集。告警经 Server 落告警中心（Alerts 页），
规则经风控页发布热生效。

## 「我该用哪个」决策树

1. 需求是「GM 平台要调用我的游戏函数」→ **sidecar**
2. 需求是「人要远程进内网机器」→ **devops-agent**（注意当前会话通道未接线，
   先装作监管纳管）
3. 需求是「数据库变更要被监控、风控、告警」→ **capture-agent**
4. 三种需求都有 → 三个都装（同一台机器可以共存，配置文件与 systemd 服务
   相互独立；一键安装脚本跑三次、每次 `--agent` 不同即可）

## 安装与下一步

```bash
# 三类各一条命令（干净机实跑记录见 agent-install.md）
curl -fsSL https://raw.githubusercontent.com/cuihairu/croupier/main/scripts/install.sh \
  | sudo bash -s -- --agent sidecar  --server gm.example.com:19090 --game-id demo --with-service
curl -fsSL https://raw.githubusercontent.com/cuihairu/croupier/main/scripts/install.sh \
  | sudo bash -s -- --agent devops   --server gm.example.com:19090 --game-id demo --with-service
curl -fsSL https://raw.githubusercontent.com/cuihairu/croupier/main/scripts/install.sh \
  | sudo bash -s -- --agent capture  --server gm.example.com:19090 --game-id demo --with-service
```

装好后确认上线：Server 管理端「运维 → 节点」应出现对应 `role` 标签的节点。
逐字段配置与生产建议值见 [agent-config.md](./agent-config)。
