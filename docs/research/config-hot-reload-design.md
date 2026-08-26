---
title: 游戏配置热更新调研与设计（Excel/配置中心/Redis/代码生成）
---

# 游戏配置热更新调研与设计

## 状态

Proposed（调研完成，设计定稿）。P0 复用 ConfigVersion 模型 + 版本化下发 API 已落地；Excel 管线与推送通道分阶段。

## 1. 问题

游戏配置（数值表/开关/活动时间/兑换码/IAP 商店）变更频率远高于代码，且要**不重启生效**。两条根本不同的路线：

- **编译期路线**：Excel → 导出源码/资源 → 走**客户端资源包**更新（release 系统）；
- **运行期路线**：配置独立于包，服务端（和客户端）**运行时拉取/推送**生效。

混淆两者是常见设计错误：数值表走运行期会造成客户端体积膨胀与版本撕裂；开关类走编译期则失去热更意义。

## 2. 市面方案全面对比

### 2.1 编译期（Excel → 产物）

| 方案                                                                     | 产物         | 工具链                                             | 热更途径                                  | 评价                                                                       |
| ------------------------------------------------------------------------ | ------------ | -------------------------------------------------- | ----------------------------------------- | -------------------------------------------------------------------------- |
| **Excel → 源码文件**（C++ header / Go struct / Java class）              | 类型安全代码 | 自研导出器 + Excel/WPS + SVN/Git                   | 随服务端构建发布                          | 老牌 MMO 标配；类型安全最高；但**每次改数值要走构建+部署**，运行期不可热更 |
| **Excel → 二进制/JSON/Lua/Table（protobuf/flatbuffers/json/lua table）** | 数据文件     | ExcelExporter 类工具（如 xlsx2lua、TabToy、Luban） | 服务端：重启或自研 reload；客户端：热更包 | 手游主流（**Luban** 国内事实标准：Excel→json/lua/bin+多语言 schema）       |
| **Google Sheets + CI 导出**                                              | 同上         | Sheets API + CI                                    | 同上                                      | 策划协作友好（实时协同+修订历史）；国内访问不稳                            |
| **Airtable/Notion 数据库当配置源**                                       | JSON API     | 自研同步                                           | 运行期拉取                                | 轻量后台配置；游戏数值表不适合（列数/类型弱）                              |

### 2.2 运行期（配置中心）

| 方案                                              | 机制                                          | 推送                    | 灰度                       | 游戏适配 | 评价                                                                                                                                |
| ------------------------------------------------- | --------------------------------------------- | ----------------------- | -------------------------- | -------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| **etcd**                                          | KV + watch                                    | 长连接 watch            | 按 key 前缀                | ★★★      | 运维标准件；游戏服每节点一个 watch；自建                                                                                            |
| **Nacos**                                         | 配置中心+注册中心                             | 长轮询/UDT              | Beta 发布（按 IP 灰度！）  | ★★★★     | 国内游戏公司常用；自带灰度发布与回滚，操作台中文                                                                                    |
| **Apollo（携程）                                  | 配置中心                                      | 长轮询+实时             | 按集群/namespace 灰度发布  | ★★★★     | 权限/审计完善；发布记录/回滚一键化                                                                                                  |
| **Consul KV**                                     | KV + watch                                    | 长连接                  | 无内置                     | ★★       | 常伴随服务发现引入                                                                                                                  |
| **Spring Cloud Config**                           | git 后端                                      | bus 刷新                | 无                         | ★        | JVM 系绑定                                                                                                                          |
| **Redis 作为配置总线**（含 skynet 惯例）          | 发布订阅 `__keyspace@0__:notify` 或版本号轮询 | pub/sub（非可靠）或轮询 | 自研                       | ★★★      | **skynet 社区惯例**：配置写 Redis，逻辑服订阅键空间通知或轮询版本 key；零额外组件，游戏服已有 Redis；无版本化/审计/灰度，需自研补齐 |
| **croupier 自有 ConfigVersion**（本仓库已有雏形） | DB 版本表 + REST                              | 拉取（可加 SSE 推送）   | 复用 assignment/game scope | ★★★★     | 已在模型清单中；缺下发/订阅协议——本文补齐                                                                                           |
| Firebase Remote Config / LaunchDarkly             | SaaS                                          | SDK 轮询                | 按 用户属性 targeting      | ★★       | 国内不可达/贵；客户端向                                                                                                             |

### 2.3 选型结论

分层混用，**一类配置一条通道**：

| 配置类型                                | 例                         | 通道                                                                                     | 理由                                                       |
| --------------------------------------- | -------------------------- | ---------------------------------------------------------------------------------------- | ---------------------------------------------------------- |
| 数值表（大、强 schema、双端共享）       | 角色/道具/关卡数值         | **Excel → Luban 导出 → 版本化对象存储**；服务端 adapter reload + 客户端走 release 热更包 | 类型安全；双端一致性靠同一导出产物；平台只管产物托管与版本 |
| 运行开关/规则（小、服务端独占、高频改） | 功能开关/风控规则/活动时间 | **croupier ConfigVersion（本文强化）**：版本化 + 按游戏/环境 scope + 审批 + 订阅推送     | 正是 GM 平台本职；复用 assignment 灰度与审计               |
| 客户端动态文案/开关                     | 公告/礼包开关              | 同上通道 + 客户端 SDK 拉取                                                               | 无需 Lua 热更的轻量级                                      |

**不自建通用配置中心**（etcd/Nacos 已覆盖超大规模场景）：单公司多游戏形态下，croupier 的 DB 版本表 + 订阅推送在千级节点内足够；若某游戏节点规模上千再引入 Nacos 做旁路，二者不冲突（croupier 仍是审批与审计入口）。

## 3. 设计（croupier ConfigVersion 强化）

### 3.1 现状与补齐点

已有 `ConfigVersion` 模型（game-scoped）。补齐三件事：**结构化命名空间、版本订阅协议、与 Excel 管线的衔接**。

### 3.2 数据模型扩展

```
ConfigVersion（已有，强化）
├── GameID/Env（已有）
├── Namespace   受控分层: gameplay(数值) | runtime(开关) | activity(活动) | iap | ops
├── Key         如 runtime/login.captchaEnabled
├── Value       JSON（schema 由 SchemaRoutes 校验，已有 schema 服务！）
├── Version     单调递增（每 namespace+key）
├── Status      draft → published → rolled_back（审批复用 approvals）
├── GrayTarget  可选灰度：区服标签集合（复用 assignment 选择器）
└── Audit       操作者/来源(manual|pipeline)/审批单号
```

### 3.3 下发与订阅协议

```
服务端游戏服 / agent:
  GET /api/v1/configs?namespace=runtime&version>=N   （增量拉取，已有 configs 路由基础）
  SSE /api/v1/configs/watch?namespaces=...            （复用既有 SSE 基础设施，变更即推）
客户端:
  GET /api/v1/public/configs?ns=iap&clientVersion=..  （公开只读端点，走 release 同风格公开层）
```

游戏服侧消费模式（框架无关）：

- **拉模式**：启动全量 + 定时(30s)比对 version；
- **推模式**：SSE 订阅变更通知 → 触发拉取（防丢：通知只带版本号，数据必走拉取）；
- 应用回调由游戏侧注册（`onConfigChange(ns, key)`），croupier 不关心框架。

### 3.4 Excel 管线衔接（数值表通道）

```
策划 Excel (Git/SVN)
  → CI 导出（推荐 Luban: Excel → lua/json/bin + schema）
  → 产物上传: POST /api/v1/configs/artifact (复用 objstore+checksum，走 release 的 artifact 同款通道)
  → 平台侧注册为 namespace=gameplay 的一个 ConfigVersion（value={manifest, downloadUrl, checksum}）
  → 游戏服收到通知 → 拉清单 → 按需下载 → 框架 reload（skynet: 重新 require 配置模块；KBE: 重读表；
     Node: 清 require.cache）——reload 细节归 hot-patch-design.md 的 adapter 体系
```

平台不解析 Excel（导出属 CI 职责），只做：产物托管、版本化、按区服灰度、审计、通知。

### 3.5 与相关系统的边界

| 系统               | 边界                                                                                |
| ------------------ | ----------------------------------------------------------------------------------- |
| release-management | 客户端资源包（含客户端配置表）走 release；服务端配置走本文                          |
| hot-patch          | 配置「应用」需要 reload 时，由 hot-patch adapter 执行；本文只负责数据通道           |
| feature-flags      | 平台自身域的开关走 featureFlags（部署级）；游戏内业务开关走 ConfigVersion（运行级） |
| SchemaRoutes       | ConfigVersion 的 value 校验复用 schema 服务（JSON Schema 已有）                     |

## 4. 阶段

| 阶段             | 内容                                                                                               |
| ---------------- | -------------------------------------------------------------------------------------------------- |
| **P0（已落地）** | Namespace/灰度字段 + 版本订阅 SSE watch + 公开客户端拉取端点 + 测试                                |
| P1               | Excel 产物通道（artifact 上传→ConfigVersion 注册→清单下发）；agent 侧拉取+回调 SDK（Go）           |
| P2               | 灰度按区服标签（复用 assignment）；配置 diff 预览与一键回滚 UI                                     |
| P3               | 多语言 SDK（lua/python/js）订阅库；配置变更影响面分析（引用该 key 的函数清单，联动函数注册元数据） |

## 5. Review Checklist

- namespace 是受控枚举（前端分组与校验依赖），新增值前后端同步；
- value 必须过 JSON Schema 校验（复用 schemas 服务），未注册 schema 的 key 拒绝发布；
- 订阅通知只携带版本号，不携带数据（通知可丢，数据必拉）；
- rollback 不是删除：产生新版本指向旧值（版本单调）；
- 公开端点不得暴露 draft/rolled_back 版本内容。
