---
title: 插件机制正式设计——扩展点全景、外部服务接入规范与 herald 对接（设计简档，待拍板）
---

# 插件机制正式设计（插件 × 接入 × 告警出口）

## 状态

- 状态: **Proposed（2026-10-10 设计补单令落档，零代码过目；同日补单令并入 LAN 资产接入——§3.8）**；拍板后按 §8 分期，与六件套同款节奏。
- 补单令三项合写一份：①插件机制正式设计（§2/§3/§6/§7）②外部服务接入规范（§4 + 附录 A）③herald 对接规范（§5 + 附录 B）；另并入 ④devops-agent LAN 资产接入方式（§3.8，cockpit 首选 + provider 兜底）。
- 设计输入: [插件架构调研](../research/plugin-architecture-survey.md)（Grafana/VS Code/WordPress/VitePress 六问口径）、[Provider 插件设计](provider-plugin-design.md)（#66，P0 已落地）、[扩展安装模型](../architecture/extension-installation-model.md)（五表 4 态）、[官方扩展统一模式](../architecture/official-extension-unified-pattern.md)、[herald 对接简档](agent-herald-integration.md)、[Agent 清单](agents-inventory.md)。
- 决策摘要（按推荐方案定稿，供拍板后审）:
  1. **一套机制不两套**：插件机制 = 既有 extension 五表（catalog/release/installation/binding/events）+ #66 driver 层，本设计只做全景归位与增量契约，**不新建注册中心、不另立 manifest 格式**。
  2. **四层信任分层**（§2）：编译期内置 → 官方扩展包 → 第三方扩展包 → 外部服务（零代码）。第三方不受信代码**不进进程**——以外部服务形态接入，不引入动态代码加载（沿 #66 裁决①）。
  3. **无子进程/gRPC 沙箱**：进程内 + RBAC 三层 capability + secguard 出站守卫 + secrets 引用是隔离边界（survey §7.3 威胁模型不成立）。
  4. **扩展点闭集七类起步**（§3）：外部平台调用 / CDC 数据源 / 检测器规则 / 告警出口 / 告警入口 / 面板页面与卡片 / agent 能力；新扩展点必须过 §7.2「六问」检查清单才能入表。
  5. **检测器 v1 内置参数化，不开放第三方规则插件**：三道闸/devops 规则是模板+参数，规则引擎插件化留位（§3.3）。
  6. **告警出口抽象 `Outlet` 接口位**（§5.1）：事件信封（§5.2）与出口解耦；herald 为第一内置出口，generic webhook 为第二出口留位——供应商可插拔，不绑 herald 实现。
  7. **告警入口 webhook v1 闭集两格式**（§5.3）：Alertmanager 兼容 + generic JSON；HMAC 签名 + 时间戳防重放 + `event_id` 幂等，落现有告警页（静默/认领/升级全复用）。
  8. **配置热更双轨**（§6.4）：server 侧 installation config PUT + reconcile 重建实例；agent 侧 core/configsync 版本号轮询（extension_sync_puller 先例）。
  9. **LAN 资产接入 cockpit 优先**（§3.8）：devops-agent 接局域网 PVE/BMC/vSphere 资产，首选把 cockpit 当外部服务适配（L4 配置实例，统一 REST；croupier 只做客户端，不重复封装 bmclib/bpg-proxmox-api/govmomi）；cockpit 未部署/不可达时走 provider 插件位本地薄封装同款库直连（L1 driver，设计同源代码解耦——操作契约单一真值）。
  10. **破坏性基础设施动作全部走审批+审计**（§3.8）：BMC 拉起/电源、PVE 快照回滚、克隆测试环境等动作步强制二次确认（复用审批流）+ execlog 审计。
  11. **接口命名对齐 cockpit 设计简档**（§3.8）：操作闭集 `<asset>.<action>`（如 `bmc.power_on` / `pve.snapshot_rollback` / `vsphere.vm_clone`），croupier 侧不另造第二套命名。

## 1. 定位：给「插件」下一个全仓定义

**现状事实**（2026-10-10 时点）：扩展体系已运转（五表 + `official.<domain>` 统一模式 + pack 导入 + Agent 同步）；#66 driver 层 P0 已落地（`internal/drivers` 契约 + Registry 处置 + manifest provider 块定稿）；cicd/identity 两处内置工厂注册表；herald 出口简档已落档。**缺的不是机制，是全景地图与接入规范**——本设计补这个，并约束后续扩展点不再各自发明。

| 术语                      | 定义                                                                              |
| ------------------------- | --------------------------------------------------------------------------------- |
| 宿主（host）              | 加载插件的进程：croupier server / sidecar-agent / capture-agent                   |
| 扩展点（extension point） | 宿主留出的可插拔位：一份稳定契约 + 注册方式 + 失败语义（§3 七类）                 |
| 插件（plugin）            | 挂到扩展点上的能力单元，四种形态之一（§2 分层）                                   |
| driver                    | 一种调用协议的编译期内置实现（openapi/webhook，#66 §3.1）                         |
| provider                  | 一个 driver 的配置实例（installation + runtime binding 承载）                     |
| binding                   | `extension_runtime_bindings` 行：安装产生的运行时挂载（function/provider/page/…） |

## 2. 插件形态四层分层（内置 vs 第三方）

判据是**信任边界**，不是技术花活——信任层级决定准入方式与契约强度：

| 层              | 形态                                 | 准入                          | 契约                                               | 隔离边界                                           | 版本协商                            | 先例                                                              |
| --------------- | ------------------------------------ | ----------------------------- | -------------------------------------------------- | -------------------------------------------------- | ----------------------------------- | ----------------------------------------------------------------- |
| L1 编译期内置   | Go 代码进二进制                      | 代码审查（PR）                | Go 接口 + `init()` 自注册工厂表                    | 同进程，RBAC 门控调用面                            | 随平台发版（driver 闭集无独立版本） | `internal/cicd`、`internal/drivers`、`internal/security/identity` |
| L2 官方扩展包   | `official.*` seed（manifest 入五表） | 仓库内 seed 审核              | release manifest + 统一模式三层权限 + configSchema | 进程内 + secguard + RBAC                           | `min_core_version` + 依赖图         | notification/alerting/approval/backup-advanced                    |
| L3 第三方扩展包 | pack `.tgz` 导入                     | 管理员导入 + 安装审批         | 同 L2 契约（同构校验零豁免）                       | 同 L2；checksum + release 查重                     | 同 L2                               | pack 导入链（契约基线 §7.3）                                      |
| L4 外部服务     | 零代码，配置实例化                   | 管理员配置（capability 门控） | §4 接入面（端点/凭据/协议闭集）                    | 网络边界：secguard 守卫出站 + secrets 引用不落明文 | 服务方可自治发版（契约=HTTP 语义）  | provider 配置实例（#66 §3.1 Grafana datasource 同构）             |

**核心裁决**：插件 = 「配置实例」优先于「可执行包」。第三方不受信代码没有进进程的通道（无 `.so`/子进程/gRPC 插件加载），能力扩展靠 L1 平台发版 + L2/L3 声明式扩展 + L4 外部服务。与 Grafana 后端插件（子进程 gRPC）、VS Code（共享 host 进程）对照：我们取 WordPress 的信任模型（「管理员能装什么」即边界）+ Grafana 的契约纪律（能力声明+版本协商），不取两者的运行时加载。

## 3. 扩展点全景（哪些能插）

每个扩展点五要素：**契约面 / 注册方式 / 生命周期归属 / 配置热更 / 失败留位**。七类起步：

### 3.1 外部平台调用（driver/provider）——#66 领地

- 契约面：`drivers.Driver`（Kind/Call/Close，JSON 透传）+ manifest provider 块（type/operations/permissions，P0 定稿）。
- 注册：编译期 `drivers.Register` + 运行时 binding（provider 型）。生命周期/热更/失败语义全按 #66 §6，本设计不重复定义。
- 状态：**P0 已落地**，openapi driver 随 P1 迁入。

### 3.2 CDC 数据源（capture source）

- 契约面：`agents/capture/source.Provider`（Open/Bookmark/Close）+ ChangeEvent 统一事件契约（11 字段）。
- 注册：编译期内置（L1）——MySQL 已落地，PG 随 C3；源闭集随 capture 批次扩展。**不开放第三方源插件**：CDC 源要 REPLICATION 权限与位点语义，误配置代价是静默丢事件，不进 L3/L4。
- 热更：表白名单变更 = 重开源（位点续传不丢）；连接参数变更 = 进程重启（supervisor 拉起）。
- 失败留位：连接失败退避重开；位点损坏 fail-safe 退出交运维（已实现）。

### 3.3 检测器/规则（capture 三道闸、devops 规则）

- 契约面（C2 落地时定型）：规则 = 模板闭集（`lineage.*`/`reconcile.*`/`cep.*`）+ 参数面（面板配置）；逐事件评估，输出命中详情进告警管线。
- **v1 内置参数化，不开放第三方规则插件**（决策⑤）：规则正确性直接决定误报率，第三方规则没有审计与测试抓手；引擎插件化（如允许自定义 CEP 表达式）留位——触发条件=出现 ≥2 个游戏方要求自带规则。
- 状态：设计定稿（capture 简档 §4），实现随 C2。

### 3.4 告警出口（alert outlet）

- 契约面：`Outlet` 接口位（§5.1）——`Deliver(ctx, AlertEvent) error`，AlertEvent 为 §5.2 统一信封。
- 注册：编译期工厂 + server 侧配置选择启用；herald 适配器为第一内置实现。
- 状态：**已落地（2026-10-10，M2）**——`internal/platform/outlet`（Outlet + Manager + HeraldOutlet），事件源 hook 走 `MetricsStore.SetOnSupervisorEvent`，配置段 `herald:`（herald 简档 §8 有实现与简档差异清单；扇出多出口仍留位）。

### 3.5 告警入口（inbound alerts，webhook 形式）

- 契约面：§5.3 入站 webhook 契约（HMAC + 幂等 + 映射表）。
- 注册：配置驱动来源映射（v1 闭集 Alertmanager + generic JSON），落现有告警页。
- 状态：**已落地（2026-10-10，M3）**——`internal/api/alert` inbound handler（`POST /api/v1/alerts/inbound/{source}`，公开路由同 cicd webhook 先例；HMAC-SHA256 + ±5 分钟防重放 + eventId 幂等 upsert 落 `alerts` 表）+ `alertInbound:` 配置段（来源闭集 × secretEnv 环境变量引用 × labels 映射配置驱动）。实现与设计差异见 todo.md M3 交付记录。

### 3.6 面板页面与卡片（dashboard）

- 契约面：**既有体系，不新增机制**——`binding_type=page/capability`（runtime binding）+ PageSpec/ComponentTemplate 发布链（dashboard-page-model）。扩展页 = 安装时派生 page binding；卡片组件 = ComponentTemplate 层。
- 边界：第三方 pack 提供页面声明（声明式 PageSpec），**不提供前端可执行代码**——页面渲染宿主是 croupier web，组件闭集受编辑器支持面约束（composite-editor v3 能力域）。
- 状态：已落地（五批次链），本设计仅归位标注。

### 3.7 agent 能力（函数面 / 托管进程）

- 契约面：函数注册链（FunctionDescriptor + pack）+ `managedProcesses` 配置（supervisor S1/S2 已交付）。
- 边界：core 系 agent（devops/capture）**没有函数面是定位而非缺陷**（agent-core §9）；函数面属于 sidecar-agent 承载的游戏服 SDK 生态。
- 状态：已落地，归位标注。

### 3.8 局域网基础设施资产（cockpit 外部服务 + provider 兜底）

devops-agent 要能操作局域网内的 PVE/BMC/vSphere 资产（部署流水线的基础设施动作步）。接入方式两级，**cockpit 优先、provider 兜底**：

- **① 首选：cockpit 当外部服务适配**（L4 配置实例，零代码）。能力提供方是 cockpit 的 hypervisor/BMC provider 层（统一 REST）；croupier 只做客户端插件（openapi driver + provider binding），**不重复封装** bmclib / bpg-proxmox-api / govmomi——这些库的运维（版本、凭证轮换、BMC 固件差异）归 cockpit 管。croupier 侧零新增依赖。
- **② 断路兜底：provider 插件位本地薄封装**（L1 driver）。cockpit 未部署/不可达时直连资产：driver 实现为同款库的薄封装（bmclib/govmomi/proxmox-api），与 cockpit 客户端**设计同源代码解耦**——两实现独立演进，但**操作契约单一真值**（同一 operations 闭集，见下），cockpit 可达时配置切回 L4 路径，行为不变。
- **③ 场景与破坏性纪律**：动作步=物理机 BMC 拉起/电源（`bmc.*`）、PVE 快照回滚（`pve.*`）、克隆测试环境（`vsphere.*`/`pve.*`）。全部破坏性操作**强制二次确认**（复用审批流，approval 批次先例）+ **execlog 审计**（操作留痕走既有 execlog 链，操作人/时间/参数/结果全量）。
- **④ 接口命名对齐**：操作命名以 cockpit 设计简档为准，闭集 `<asset>.<action>`（起步 `bmc.power_on` / `bmc.power_off` / `bmc.power_cycle` / `pve.snapshot_rollback` / `pve.vm_clone` / `vsphere.vm_clone` 等）；croupier 侧（manifest provider 块 operations、面板按钮、审批单）一律引用该命名，不另造第二套。
- 契约面：走 §4.1 标准接入面（端点=cockpit REST baseURL、凭据=cockpit 令牌 secret 引用、协议=http-rest、权限=`infrastructure.operate` 起步）。
- 状态：本设计定稿（接入方式与两级路径）；cockpit 简档落档与 provider 兜底 driver 为实现批次（§8）。
- 边界：cockpit 设计简档**尚未落在本仓**（并行/待立项）——§3.8 命名对齐以该简档落档后双向校准为准（§9）。

## 4. 外部服务接入规范（供应商自助接入面）

### 4.1 标准接入面五要素

外部服务（短信网关/风控 API/数据平台/工单系统…）被 croupier 适配，一律按同一张表回答：

| 要素       | 契约                                                                                           | 说明                                              |
| ---------- | ---------------------------------------------------------------------------------------------- | ------------------------------------------------- |
| 端点       | `baseURL`（http/https，secguard 出站守卫白名单内）                                             | 每服务一个 base，操作路径由 driver 协议推导       |
| 凭据       | `token` → secret_refs 引用（**不落明文**，config/secrets 拆分既有模型）                        | 轮换 = 改 secret 不动配置                         |
| 协议       | 闭集三选一：`http-rest`（openapi driver）/ `webhook-out`（出站推送）/ `webhook-in`（入站接收） | 自定义协议 = 写 L1 driver（平台发版），不给配置面 |
| 超时与重试 | driver 统一默认（连接 5s/收发 30s），可按 provider 覆盖；幂等键由调用方携带                    | 失败语义标准化（§6.6）                            |
| 权限       | 三层 capability `<domain>.read/operate/admin` + 调用面 `external.<provider>.<method>`          | 管理面与调用面分离（#66 §3.3 现状）               |

### 4.2 三种协议形态的适配方式

1. **http-rest**：供应商给 REST API → 走 openapi driver。供应商只需提供「操作清单」（方法名 + 路径 + 请求/响应 JSON 形态），manifest provider 块声明 operations，binding config 携带端点与凭据。**不需要供应商写任何代码**。（规范样例：局域网基础设施资产经 cockpit 统一 REST 接入——§3.8 ①；其断路兜底=本规范「自定义协议=L1 driver」的资产版形态——§3.8 ②）
2. **webhook-out**（croupier→供应商，告警出口类）：§5 Outlet 的 generic webhook 实现——固定信封（附录 B）POST 到供应商 URL，HMAC 签名头，供应商侧按信封字段路由。
3. **webhook-in**（供应商→croupier，告警入口类）：§5.3 入站契约——供应商按映射模板把自家告警字段映射到 croupier 信封。

### 4.3 接入文档模板（附录 A 全文）

`docs/templates/` 落一份自助 checklist（M4）：供应商按 10 步填空即完成接入——能力清单 → 协议形态 → manifest provider 块 JSON → binding config → secrets 登记 → capability 声明 → 超时/重试参数 → 拨测（test-connection）→ 面板可见性验收 → 升级/下线约定。首个真实供应商接入时走查修订。

## 5. herald 对接规范（告警出口双向）

### 5.1 出口抽象（供应商可插拔，不绑实现）

- **`Outlet` 接口位**（server 侧，小接口）：

```go
// Deliver 投递一条告警事件；返回错误即投递失败（异步重试由出口管理器负责）。
// 实现必须非阻塞友好（内部队列或快速返回）。
type Outlet interface {
    Name() string // 配置选择键（"herald" / "webhook"）
    Deliver(ctx context.Context, ev AlertEvent) error
}
```

- 事件源（capture 规则命中 / devops 规则 / supervisor 熔断 / healthprobe 窗口）只产 §5.2 信封，**不感知出口**；出口管理器按配置扇出（v1 单出口，多出口扇出留位）。
- **herald = 第一内置出口**：对接方式以 [herald 简档](agent-herald-integration.md)为权威（apps 集成者 API / 事件适配面 / trigger token / 品类映射 / courier 式接法），本设计不重复；差异仅在实现落点上把「herald 适配器」泛化为「Outlet 接口的第一实现」。
- **generic webhook = 第二出口留位**（§4.2 形态 2）：同一信封 POST 出去，供应商侧自建映射；无 herald 的部署用 webhook 出口对接钉钉/飞书/企业微信机器人（M2 之后的按需批）。
- 供应商可插拔原则落点：**新增出口 = 新增一个 Outlet 实现（L1 内置）+ 配置一行**，事件词汇与告警管线零改动。

### 5.2 统一告警事件信封（出站 wire）

出口间共享的**事件词汇**（闭集随批次扩展，新增 kind 走对应简档）：

| 字段               | 类型         | 说明                                                                                                                                                    |
| ------------------ | ------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `kind`             | string，闭集 | `capture.rule_hit` / `devops.build_failed` / `supervisor.breaker_tripped` / `probe.unavailable` / `probe.recovered` / `alert.inbound`（入口转出口链路） |
| `severity`         | 闭集         | `critical` / `warning` / `info`（出口自行映射紧急度）                                                                                                   |
| `eventId`          | string       | croupier 自有事件主键（告警 id / 事件 seq / `{target}:{window_start_ts}`），进幂等                                                                      |
| `dedupKey`         | string       | 出口侧去重键（同键同态折叠，状态翻转重发）                                                                                                              |
| `title` / `body`   | string       | 摘要文本（v1 直投，不用模板）                                                                                                                           |
| `occurredAtUnixMs` | int64        | 事件时间                                                                                                                                                |
| `scope`            | object       | `gameId` / `env` / `agentId`（可空的 scope 三元组）                                                                                                     |

herald 映射（kind→品类、severity→紧急度、event_id→幂等）沿 herald 简档 §3 表；webhook 出口直接透传信封 + `X-Croupier-Signature`（HMAC-SHA256，共享密钥配置引用）。

### 5.3 告警入口（第三方告警系统 → croupier，webhook 形式）

- **端点**：`POST /api/v1/alerts/inbound/{source}`（source=来源注册名，闭集起步 `alertmanager` / `generic`；capability `alerts.operate` + 来源级共享密钥门控）。
- **安全**：`X-Croupier-Signature`（HMAC-SHA256 over raw body）+ `X-Croupier-Timestamp`（±5 分钟防重放）；密钥走 secret_refs。
- **payload 契约（generic）**：直接使用 §5.2 信封（`kind` 固定 `alert.inbound` + `labels.source` 携带来源语义）；**Alertmanager 兼容**：接受 `{alerts:[{status,labels,annotations,startsAt,endsAt,generatorURL}]}`，映射配置把 labels 键映射到信封字段（配置驱动，不写死供应商字段名）。
- **落点**：入站告警落**现有告警存储/告警页**——静默、认领、升级链路全复用（与 capture 告警同桶，scope 按 gameId/env 归库）；`eventId` 幂等去重（同 id 重推不重复落库）。
- **不做的**：croupier 不做跨系统告警收敛/降噪引擎（那是出口侧/herald 侧职责）；入口不回写确认状态到来源系统（v1 单向）。

### 5.4 边界（诚实清单）

- 出口失败不阻塞告警链：异步投递 + 有限重试 + 失败计数（herald 简档 §6 口径，全出口通用）。
- 入口签名校验失败 = 400 + 审计事件；不落库（防注水）。
- 静默/认领语义 croupier 侧处置状态，不传出口停投递（herald 简档 §6 同款边界，全出口通用）。
- generic webhook 出口的模板化 payload（钉钉/飞书格式适配）留位，v1 只投标准信封。

## 6. 插件接口契约（机制面，全扩展点通用）

### 6.1 注册与发现：两表怎么选

| 注册方式                   | 适用判据                                       | 先例                                                |
| -------------------------- | ---------------------------------------------- | --------------------------------------------------- |
| 编译期 `init()` 自注册工厂 | 代码进二进制（L1）；单一实例语义；无需多 scope | `internal/drivers`、`internal/cicd`、capture source |
| 运行时五表 binding         | 需要安装生命周期/多实例/多 scope/面板可见      | extension 全体系                                    |

判据一句话：**要安装面的走五表，不要的走编译期**；两者经 manifest provider 块衔接（#66 §3.2 同构约束）。

### 6.2 manifest 契约

唯一 manifest = release `manifest_json`（既有字段 + provider 块 P0 定稿 + 统一模式三层权限 + configSchema）。**禁止**为新扩展点另立 manifest 格式或第二个声明文件；扩展点的新声明一律以 manifest 新增块的形式走「字段闭集 + 守卫测试 + binding 同构」三件套（provider 块先例）。

### 6.3 生命周期

- 五表件：4 态同步状态机（installed/enabled/disabled/uninstalled，§4 安装模型实际落地版）+ reconcile 全量替换 bindings；升级 = 版本切换 + config 对新 schema 校验；回滚 = upgrade 旧版本。
- 编译期内置件（L1）：无安装面，生命周期 = 配置 `enabled` 开关 + 进程重启（drivers 实例随 installation 状态启停是 P1 批次的唯一例外——provider 是 L4 配置实例，经五表驱动）。

### 6.4 配置热更（双轨）

| 宿主   | 机制                                  | 语义                                                                                                                    |
| ------ | ------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| server | installation config `PUT` + reconcile | 配置变更 → 实例重建（driver Close + 新 config 构造）；热更粒度 = provider 实例，不动进程                                |
| agent  | `core/configsync.Puller` 版本号轮询   | 版本推进 → apply 热生效；apply 失败游标不推进（保留旧配置继续跑）；断连降级用最后已知配置（extension_sync_puller 先例） |

必须显式声明的热更语义表（每个扩展点的 configSchema 标注 `hotReload: true|false`，默认 false = 需重启生效——false 是安全缺省）。

### 6.5 隔离与权限

- 进程内执行，无子进程/gRPC 沙箱（决策③）；隔离边界 = RBAC 三层 capability（管理面）+ 调用面 `external.<provider>.<method>` + secguard 出站守卫（SSRF/白名单）+ secrets 引用。
- 失败隔离：每扩展点调用面 panic recovery（driver Call 包 recover→error）+ 超时上限 + 连续失败计数（达阈值标 failed binding，面板可见；自动熔断复用 supervisor 熔断语义留位）。

### 6.6 失败留位（每扩展点的降级语义，禁静默丢弃）

| 扩展点               | 失败语义                                                 |
| -------------------- | -------------------------------------------------------- |
| driver/provider 调用 | 错误透传调用方（HTTP 语义），binding 标 failed 计数      |
| CDC 源               | 退避重开；位点损坏 fail-safe 退出（§3.2）                |
| 检测器               | 单规则 panic 不拖垮事件流（recover+规则级禁用+事件记账） |
| 告警出口             | 异步重试 + 失败计数；告警页落库不受影响                  |
| 告警入口             | 400 + 审计，不落库                                       |
| 面板 binding         | 组件占位 + 错误态，页面其余部分可用                      |
| agent 能力           | supervisor 退避拉起/熔断（S2 已交付）                    |

### 6.7 版本协商

`min_core_version`（既有）+ driver 闭集随平台发版（无独立版本）+ manifest 不另立版本字段（#66 裁决⑤）。签名/分发可信层维持远期留位（survey §7.3：单公司自托管，优先级低）。

## 7. 与 #66 的关系（归位一体，不两套）

### 7.1 映射表

| #66 内容                              | 在本设计中的位置                               |
| ------------------------------------- | ---------------------------------------------- |
| 三层模型（extension/driver/provider） | §1 术语表引用，不重定义                        |
| driver 接口 + 注册表（P0 已落地）     | §3.1 扩展点第 1 类                             |
| manifest provider 块（P0 已定稿）     | §6.2 唯一 manifest 的第一个「新增块」先例      |
| Registry 归档处置（P0 已落地）        | §6.1「两表怎么选」的反例注脚（双注册中心教训） |
| P1 external-platform 迁移             | §3.1 内的既定批次，不受本设计影响              |
| P2 pack descriptor / webhook driver   | §3.1 留位，触发条件不变                        |
| P3 out-of-process connector           | §2 核心裁决的远期复核项，不承诺                |

### 7.2 新扩展点准入「六问」检查清单（进 §3 表的前置门）

①契约面是什么（接口/manifest 块）？②注册走编译期还是五表（§6.1 判据）？③生命周期归属（4 态 or enabled 开关）？④配置热更语义（hotReload 标注）？⑤权限面（三层 capability + 调用面命名）？⑥失败留位（§6.6 表加行）？——六问答不全不许入表，防止扩展点各自发明契约。

## 8. 分期计划（拍板后动工）

| 期                             | 内容                                                                                                                                                                                                                           | 验收门                                                                      |
| ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------- |
| M1 归位标注（零代码）          | 本设计落档；§3 表各扩展点标注状态锚点；#66 P0 产出归位                                                                                                                                                                         | 文档三层同步 + docs build                                                   |
| M2 Outlet 接口位 + herald 出口 | Outlet 小接口 + herald 适配器（herald 简档 §7 原落点，随 K3 告警通道批次）——**已交付（2026-10-10）**，实现与简档差异见 herald 简档 §8                                                                                          | 适配器单测（映射/重试分类）+ 本地 heraldd 冒烟 + `herald.enabled` 缺省关 ✅ |
| M3 告警入口 webhook            | inbound 端点 + Alertmanager/generic 映射 + HMAC/防重放/幂等——**已交付（2026-10-10）**                                                                                                                                          | 单测（签名/重放/映射/幂等）+ 告警页落库回归 ✅                              |
| M4 接入模板落地                | `docs/templates/external-service-integration-template.md` + 首个真实供应商走查修订——**已交付 Draft（2026-10-10）**：模板 10 步 + 路径判据 + herald/Alertmanager 双走查记录 + 修订项 4 条；http-rest 分支待首个 REST 供应商回填 | 模板过目（**待用户过目定稿**）+ 走查记录 ✅                                 |
| M5 cockpit 接入批次            | cockpit 简档落档后：L4 provider 配置实例接 cockpit REST（命名对齐 §3.8 ④）；触发后评估 ②provider 兜底 driver（薄封装同款库）                                                                                                   | 端到端：面板发起 BMC 拉起 → 审批 → execlog 留痕 → 资产动作完成              |
| 远期留位                       | 检测器插件化、多出口扇出、webhook 出口模板化（钉钉/飞书）、签名分发                                                                                                                                                            | 各自触发条件见 §3.3/§5.4                                                    |

## 9. 已知边界（诚实清单）

- 本设计是**归位与规范**，不带来即时新能力：M2-M4 之外的条目全部留位，触发条件明示（§8）。
- 检测器/源不开放第三方插件是**定位**：规则正确性与数据完整性不值得为生态性让步；若未来游戏方强诉求，走「六问」重新评审（§7.2）。
- 入站告警 v1 两格式闭集：新供应商格式 = 新映射配置（理想）或新增映射模板（L1 改动随批）；**不承诺任意格式零代码接入**。
- 多出口扇出、出口侧模板化 payload 未做（§5.4）；告警收敛/降噪不进 croupier。
- 面板第三方页面受组件闭集约束（§3.6）：PageSpec 能表达的才可插，任意自定义渲染不在能力域。
- cockpit 设计简档**尚未落在本仓**（§3.8）：§3.8 的 `<asset>.<action>` 命名是 croupier 侧建议形，最终以 cockpit 简档落档后双向校准为准；cockpit 未就绪前 M5 不启动。
- hotReload 缺省 false：未标注的配置变更一律需重启生效，文档不得夸成热更。
