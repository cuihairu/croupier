---
title: herald 对接简档——croupier 告警出口对接通知编排平台（立项待批）
---

# herald 对接简档（告警投递出口）

## 状态

- 状态: **M2 已落地（2026-10-10，Outlet 接口位 + herald 出口适配器，见 §8）**，六件套之一（[Agent 清单](agents-inventory.md) 顶部有索引）。
- 报警出口令（用户令，2026-10-09）：capture 告警 / healthprobe 故障窗口 / devops 构建失败 → 走 **herald**（用户自研通知编排与投递平台，仓 `~/workspaces/herald`）的 REST API/webhook 投递；**croupier 这边当接入方，courier 式接法**——herald 只提供能力，通道/收件人由 herald 侧配置；接入方式写进本简档。
- 依据: herald `docs/api/rest.md`、`docs/guide/integration.md`（应用接入/集成者 API，2026-10 时点）。

## 1. 定位（谁管什么）

| 职责                                           | 归属       | 说明                                                                                  |
| ---------------------------------------------- | ---------- | ------------------------------------------------------------------------------------- |
| 告警产生与真值存储                             | croupier   | 三类告警先落 croupier 告警页（现有体系，含静默/认领/升级），**这是 Record**           |
| 投递编排（渠道选择/收件人/群组/值班表/升级链） | **herald** | croupier 只报事件，**不配渠道不配收件人**（courier 式：把包裹放门口，路线 herald 定） |
| 投递执行与结果记录                             | herald     | Provider 池 + 投递日志/审计流，croupier 可查自己命名空间的投递记录                    |
| 告警状态展示                                   | croupier   | 告警页仍是排查入口；herald 投递状态 v1 不回写 croupier（边界见 §6）                   |

## 2. 接入方式（apps 集成者 API）

按 herald「应用接入」模型，croupier 在 herald 侧播种 app 命名空间 **`croupier`**，持分级 token（herald 配置文件播种，凭证与品类同其他应用互相隔离）：

- **trigger** token（必配）：触发投递；
- query / config token（v1 不申请）：v1 不做投递状态拉取与品类自助管理，品类注册一次性走 herald 侧配置完成。

品类（category，触发面分类键，注册时定默认紧急度）：

| 品类           | 默认紧急度 | 承载内容                                           |
| -------------- | ---------- | -------------------------------------------------- |
| `agent-alerts` | urgent     | capture 规则命中、devops 构建失败、supervisor 熔断 |
| `availability` | urgent     | healthprobe 故障窗口开始/恢复                      |

## 3. 触发面：事件适配（`POST /api/v1/apps/croupier/events`）

croupier 是「事件语义在自己词汇里的整合方」（告警源：kind/severity/target + 自有事件主键），走 herald 的**事件接入适配面**而非裸 dispatch——herald 侧完成 kind→品类、severity→紧急度、target→受众映射，croupier 自有事件主键进 `event_id`（进幂等、投递结果回调回带）：

| croupier 事件源                                                              | kind                                    | severity 映射                                         | event_id                                                                        |
| ---------------------------------------------------------------------------- | --------------------------------------- | ----------------------------------------------------- | ------------------------------------------------------------------------------- |
| capture 规则命中（§4.4 命中详情）                                            | `capture.rule_hit`                      | 告警 severity 直映（critical→critical，warn→warning） | 告警 id                                                                         |
| devops 规则命中（build_failed/build_timeout/stale_green/consecutive_failed） | `devops.build_*` / `devops.stale_green` | warning（critical 规则可配）                          | 告警 id                                                                         |
| supervisor 熔断（breaker_tripped）                                           | `supervisor.breaker_tripped`            | critical                                              | `{agentId}:{seq}`（seq 是 agent 本地游标重启归零，裸 seq 跨 agent 必撞，见 §8） |
| healthprobe 故障窗口开始                                                     | `probe.unavailable`                     | warning（Liveness critical）                          | `{target}:{window_start_ts}`                                                    |
| healthprobe 窗口恢复                                                         | `probe.recovered`                       | info                                                  | 同上（state 翻转重发）                                                          |

要点：

- **severity 词汇**：croupier 三档（critical/warning/info）→ herald 紧急度（critical/urgent/normal），herald 侧映射；
- **target/受众**：v1 不逐事件指定受众——事件不带 target，由品类默认路由到 herald 侧受众/群组（收件人关系、订阅、群组、值班表全部 herald 侧维护，croupier 零配置）；后续按 scope 分流受众时再启用 target（`group:gm-ops@{game_id}` 类 ref），届时仍由 herald 侧解析；
- **去重（dedup）**：healthprobe 窗口的二态（unavailable→recovered）与 herald dedup 状态折叠天然对齐——同 `dedup_key` 同态折叠不重发，状态翻转自动重发，恢复通知不丢单；其余事件 `dedup_key = {kind}:{告警 id}`；
- **内容**：v1 用 `title`+`body` 直投（摘要文本），不用模板——croupier 不依赖 herald 命名空间模板表；需要卡片化时再注册模板。

## 4. SDK 与客户端（server 侧出口适配器）

- 用 herald **apps-sdk/go**（`github.com/cuihairu/herald/apps-sdk/go`）的 trigger Client，不自研 REST 封装；herald Error（拒绝，重试无意义）与 Transport 错误（网络层，可重试）两类分别处理。
- 出口适配器在 **croupier server 侧**（不是 agent 侧）：agent 告警事件经既有上行通道到 server 告警管线后，由适配器转发 herald——agent 不直连 herald（少一个出网点、herald 凭证不下发到 agent 主机）。

## 5. 配置（server 侧，lowerCamelCase 契约）

```yaml
herald:
  enabled: false # 缺省关；关=纯 croupier 告警页，行为与现状一致
  baseUrl: http://herald:8080 # 空则降级告警日志、不注册出口
  app: croupier # 空取默认 croupier
  tokenEnv: HERALD_TRIGGER_TOKEN # 凭证走环境变量引用，不落配置文件；环境变量缺失降级不注册
  target: group:gm-ops # 默认受众 ref，空取默认 group:gm-ops（dispatch face 要求受众非空，见 §8）
```

kind→品类映射（`probe.*` 前缀→`availability`，其余闭集→`agent-alerts`）与 severity→紧急度（critical/warning/info→critical/urgent/normal）是代码内闭集，不进配置；需要运行时可调时再加映射配置段。

## 6. 降级与边界（诚实清单）

- **herald 不可达不阻塞告警链**：适配器异步投递+有限重试，失败落 server 日志与投递失败计数；告警页照常落库——croupier 的告警能力不依赖 herald 存活。
- **herald 幂等是进程内的**（服务重启即清）：croupier 重试按自有 `event_id` 语义重复发送即可，由 herald 幂等+dedup 兜底折叠，croupier 不自建投递去重表。
- **投递结果回调（delivery_result/unsubscribe + HMAC 验签）v1 不接**：herald 侧查投递记录已够排查；接回调时 croupier 需要公网可达的回调端点或内网互通路径，留待批。
- **静默/认领语义分离**：croupier 告警页的静默是 croupier 内的处置状态，不会传给 herald 停投递；herald 的静默（值班表/规则抑制）是投递侧的。两侧语义独立，文档如实标注，不做双向同步。
- 事件词汇（kind 闭集）随 agent 批次扩展；新增 kind 属 wire 闭集变更，随对应 agent 简档走。

## 7. 实施落点

- server：告警管线后挂 herald 出口适配器（独立小包，随 K3 告警通道批次交付）；配置契约 §5 进 `configs/server.yaml` 示例。→ **已落地为 `internal/platform/outlet`，见 §8。**
- 依赖方向：`internal/server` 适配器 → `apps-sdk/go`（跨仓依赖，版本钉死；herald 仓同主人，升级节奏自控）。
- 测试：适配器单测（事件→请求映射/severity 映射/重试分类）+ herald 侧联调用例（本地起 heraldd 冒烟，CI 不依赖外网）。

## 8. 落地记录（M2，2026-10-10）

实现落点：

- **`internal/platform/outlet`**：`Outlet` 接口位（plugin-mechanism §5.1）+ `Manager`（扇出留位，v1 单出口；传输错误有限重试、`Permanent` 拒绝短路、失败计数）+ `HeraldOutlet`（apps-sdk/go `Dispatch`，`IsTransport` 分类：传输错误可重试、herald 拒绝 `Permanent` 不重试）。
- **事件源 hook**：`MetricsStore.SetOnSupervisorEvent`（去重后新事件锁外异步回调）→ `internal/svc` 布线 `registerHeraldOutlet`；首个真实事件源 = supervisor `breaker_tripped`（critical）。capture/devops/probe 源随各自批次接入（kind 闭集已在 outlet 包定义）。
- **配置**：`herald:` 段（§5），缺省关；`configs/server.yaml` 注释示例。
- **信封**：EventID/DedupKey = `{agentId}:{seq}` / `supervisor.breaker_tripped:{agentId}:{seq}`；Scope.AgentID 携 agent。

与简档基线的差异（诚实清单）：

1. **走 dispatch face 而非事件适配面**：herald `/events` 适配面要求逐事件 target 非空，与「事件不带 target」相抵；实现走 `POST /api/v1/apps/{app}/dispatch`，Audiences = 配置的默认受众 ref（`herald.target`，默认 `group:gm-ops`）。逐事件 target 分流留待 scope 分流批次。
2. **event_id 携 agentId 前缀**（§3 表内已更新）：简档原文写裸 seq，seq 是 agent 本地游标、重启归零，裸值跨 agent 重启必撞幂等。
3. **品类映射在代码闭集**：§5 原拟 `categories` 配置表未实装（映射面稳定，配置化留位）。
4. **herald 侧播种清单**（部署口径，品类注册一次性走 herald 侧配置完成的具体含义）：app `croupier` token（trigger+config）、groups 受众播种、`delivery.category_urgency`（缺省 normal 走 Inbox 面，过不了 IM 强度通道）、`dedup.enabled: true`（缺省关=闸门直通不折叠）。

验收门（plugin-mechanism §8 M2）：适配器单测（映射/重试分类）✅ + 本地 heraldd 冒烟（品类注册→dispatch 受理→同 EventID dedup 折叠，`internal/platform/outlet/heraldd_smoke_test.go`，env 门控 CI 不依赖外网）✅ + `herald.enabled` 缺省关 ✅。
