---
title: 配置工作流全景分析——项目组流程分类与平台适配模型
---

# 配置工作流全景分析（项目组流程分类 × 平台适配模型）

## 状态

Proposed（分析稿）。前置文档：[配置热更新](./config-hot-reload-design.md)（通道与下发）、[Excel 在线配置](./excel-online-design.md)（双源版本模型）。

> **落地注记（2026-08）**：本文 §5 的「在线查看」侧已实现为 `/dev/config-explorer` 配置浏览器
> （`internal/platform/configsource` + `internal/api/configexplorer`）：
> 五种数据源适配器（git 只读 / redis 可写 / nacos 可写 / db 只读 / croupier 可写），
> 目录树懒加载 + 按格式渲染（xlsx 表格 / json·yaml·lua·python Monaco 高亮 / csv 表格），
> 可写源应急编辑（reason 必填 + 审计 `config.emergency_edit`，写回配置中心本身）。
> 平台不参与各项目配置流程；source/delivery 全量适配器与 `configWorkflow` 绑定模型仍按 §7 阶段推进。

## 1. 问题与立场

**平台不定义项目组的配置流程。** 不同项目组的既有工作流不同：

- 有的项目组用 **Git 管理 Excel**（策划提交 → CI/Luban 导出）；
- 有的项目组**数据库直管**（业务表即配置，运营后台直改）；
- 有的项目组沿用 **skynet/Redis 配置总线**惯例；
- 有的项目组已经用了 **Nacos/Apollo** 配置中心；
- 也有小团队没有任何管线，需要**在线编辑**兜底。

Croupier 的职责是**适配**这些流程，把它们接入统一的治理面（版本化 / 审批 / 审计 / 灰度 / 回滚 / 变更通知），而不是用一套流程取代它们。本文先把工作流分类整理清楚，再定义适配模型。

## 2. 工作流分类（按项目组现状）

### 2.1 六类主流工作流

| #   | 工作流                         | 谁写                          | 写在哪                           | 怎么生效                                                   | 版本化                   | 审批/审计           | 典型团队                                 |
| --- | ------------------------------ | ----------------------------- | -------------------------------- | ---------------------------------------------------------- | ------------------------ | ------------------- | ---------------------------------------- |
| A   | **Git 管理 Excel + CI 导出**   | 策划提交 .xlsx 到 Git/SVN     | Git 仓库                         | CI（Luban/xlsx2lua）导出产物 → 随发布或热更下发            | Git 历史                 | MR review（仓库侧） | 大团队、MMO、双端共享数值表              |
| B   | **Git 直管产物**               | 程序直改 JSON/Lua/YAML        | Git 仓库                         | 构建发布或自研 reload                                      | Git 历史                 | MR review           | 程序主导的小配置                         |
| C   | **数据库直管**                 | 策划/运营在既有后台或直接改表 | 业务数据库表                     | 游戏服轮询/重启加载                                        | 无（或表内版本字段自研） | 无                  | 运营活动、商城、邮件模板等"业务表即配置" |
| D   | **skynet/Redis 配置总线**      | 运维/工具脚本写 Redis         | Redis key（配置哈希 + 版本 key） | 键空间通知 `__keyspace@0__:*` 或轮询版本号 → 逻辑服 reload | 无                       | 无                  | skynet 社区惯例，零额外组件              |
| E   | **Nacos/Apollo 配置中心**      | 配置中心控制台                | Nacos dataId / Apollo namespace  | 长轮询实时推送，自带灰度（Beta 按 IP / 按集群）            | 配置中心自带             | 配置中心自带        | 已引入配置中心的 Java/Go 团队            |
| F   | **Web 在线编辑**（无管线兜底） | 策划在管理台改                | 平台 DB（ConfigVersion）         | SSE 通知 + 拉取                                            | ConfigVersion            | 平台 approvals      | 无 CI 的小团队                           |

### 2.2 各工作流的治理缺口

平台的价值不在"再提供一种写法"，而在补齐各工作流**缺的治理能力**：

| 工作流         | 缺口                                                  | croupier 补什么                                                                     |
| -------------- | ----------------------------------------------------- | ----------------------------------------------------------------------------------- |
| A Git+CI       | 产物下发无平台视角的灰度/回滚/审计；CI 挂了没法应急改 | 产物注入 → 版本化 → 灰度/回滚/审计/通知（**不动 Git 流程**）                        |
| B Git 产物     | 同上                                                  | 同上（产物即 Git 里的文件，走同一注入端点）                                         |
| C 数据库直管   | **无版本、无审批、无审计、改错无法回滚**——风险最高    | DB 表 → 快照版本化（导出为 ConfigVersion）；可选接管写入（平台表单写表 + 版本快照） |
| D Redis 总线   | 无版本/审计/灰度；pub/sub 非可靠                      | 平台作为**写入侧**：版本化值写 Redis + publish 版本号；游戏服消费方式不变           |
| E Nacos/Apollo | 与 GM 审批/审计割裂                                   | 桥接：平台审批通过后同步写入 Nacos（旁路，不替换控制台）                            |
| F 在线编辑     | 已内建版本/审批                                       | 无需补（兜底路径）                                                                  |

**结论**：没有一种工作流需要被消灭；C、D 两类是治理能力最缺、最需要平台接入的。

## 3. 适配模型：平台只统一中间段

```
┌─ 创作侧（项目自选，平台适配）─┐    ┌─ 平台统一治理段 ─┐    ┌─ 生效侧（项目自选，平台适配）─┐
│ A Git+CI 产物上传            │    │ ConfigVersion    │    │ ① croupier 拉取/SSE（默认）   │
│ B Git 产物上传               │ ─► │ 版本化/审批/审计  │ ─► │ ② Redis 发布（skynet 惯例）  │
│ C 数据库表快照               │    │ 灰度/回滚/通知    │    │ ③ Nacos/Apollo 桥接          │
│ F Web 在线编辑               │    │                  │    │ ④ 保持现状（仅登记版本）      │
└──────────────────────────────┘    └──────────────────┘    └───────────────────────────────┘
```

**铁律（与 excel-online-design 一致，扩展到全工作流）：ConfigVersion 是唯一发布事实源。**

- 创作侧无论哪种工作流，进入平台即注册为 ConfigVersion（namespace + key + value + 来源标记）；
- 生效侧无论哪种通道，数据都从 ConfigVersion 出（Redis/Nacos 只是**镜像**，不是源头）；
- 版本号单调递增、回滚=新版本指向旧值、审批复用 approvals、审计记来源（manual/pipeline/db-snapshot）。

## 4. 绑定模型：按项目（game/env）声明工作流

```yaml
# 每个 game（可选 env 级覆盖）声明自己的配置工作流，平台按绑定呈现入口
configWorkflow:
  gameId: demo
  sources:                      # 创作侧（可多个并存）
    - type: git-artifact        # A/B：CI 上传端点 + （可选）webhook 触发
    - type: db-table            # C：声明受管表清单
        tables: [activity, shop_item]
        mode: snapshot          # snapshot(只读快照版本化) | managed(平台接管写入)
    - type: web-editor          # F：在线编辑入口（有则显示 /dev/excel-config）
  delivery:                     # 生效侧（每 namespace 可不同，先做单选）
    type: redis                 # croupier-pull(默认) | redis | nacos | none
    redis: { keyPrefix: "cfg:", versionKey: "cfg:__version__" }
```

- **未声明 = 平台不展示任何创作入口**，只有 ConfigVersion 只读列表（纯适配，不引导）；
- UI 按绑定渲染：`web-editor` 才显示在线编辑器；`db-table` 显示表快照与（managed 时）编辑表单；`git-artifact` 显示上传/最近注入记录；
- 生效侧 `redis`/`nacos` 时，发布动作附带镜像写入，失败不影响 ConfigVersion 注册（镜像可重试，事实源不丢）。

## 5. 适配器清单（来源 × 生效 两端的实现矩阵）

### 5.1 创作侧 Source Adapters

| 适配器                 | 端点/机制                                                                            | 现状                                                      |
| ---------------------- | ------------------------------------------------------------------------------------ | --------------------------------------------------------- |
| `git-artifact`         | `POST /api/v1/configs/artifact`（CI 上传产物，checksum 校验）+ 可选 Git webhook 接收 | 端点已在 P1 规划（config-hot-reload-design §3.4），待实现 |
| `web-editor`           | `POST /api/v1/configs/excel/import                                                   | compile`                                                  | **已落地** |
| `db-table`             | 快照任务（定时/手动）导出受管表 → ConfigVersion；managed 模式生成表单写回            | 待实现                                                    |
| `external-passthrough` | 仅登记外部版本号（E 类项目已在 Nacos 发布，平台只记录审批与审计）                    | 待实现                                                    |

### 5.2 生效侧 Delivery Adapters

| 适配器            | 机制                                                                                                                              | 现状                                      |
| ----------------- | --------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------- |
| `croupier-pull`   | SSE watch + 增量拉取（默认）                                                                                                      | **已落地**（config-hot-reload-design P0） |
| `redis-publisher` | 发布时写 `cfg:<ns>:<key>` 全量值 + `cfg:__version__` 版本号 + `PUBLISH cfg:__notify__ <ns>:<version>`；游戏服 skynet 惯例消费不变 | 待实现                                    |
| `nacos-bridge`    | 发布时调 Nacos OpenAPI 写 dataId（值来自 ConfigVersion，灰度用 Nacos Beta）                                                       | 待实现                                    |
| `none`            | 只版本化+审计，下发项目自理                                                                                                       | 配置项即可                                |

### 5.3 skynet/Redis 惯例的最小接入（D 类重点）

```
发布动作（ConfigVersion published）
  → redis-publisher:
      HSET cfg:runtime <key> <value-json>
      SET  cfg:__version__ <n>
      PUBLISH cfg:__notify__ "runtime:<n>"
游戏服（skynet 惯例，无需改 SDK）:
  订阅 __keyspace 通知或订阅 cfg:__notify__ → 按 key 重读 → 框架 reload
```

- 通知可丢、数据必拉：游戏服拿到版本号后与本地比对，落后则全量/增量重读 Redis；
- Redis 里永远是**最新值**；历史与回滚在 ConfigVersion（回滚=再发布旧值为新版本→再写 Redis）。

## 6. 边界（与既有文档对齐）

| 系统                     | 边界                                                                                                                                   |
| ------------------------ | -------------------------------------------------------------------------------------------------------------------------------------- |
| config-hot-reload-design | 定义**默认通道**（croupier-pull/SSE）与 ConfigVersion 模型强化；本文定义**工作流适配层**，Redis/Nacos 桥接在其下发协议之上作为可选镜像 |
| excel-online-design      | 其"双源模型"是本文 F + A 的特例；本文把来源推广到六类                                                                                  |
| release-management       | 客户端资源/配置表仍走 release；本文只覆盖服务端配置                                                                                    |
| hot-patch-design         | reload 动作仍归 hot-patch adapter；适配器只负责数据到达                                                                                |

## 7. 阶段

| 阶段       | 内容                                                                                 |
| ---------- | ------------------------------------------------------------------------------------ |
| P0（已有） | ConfigVersion + croupier-pull/SSE + web-editor（在线编辑/上传编译）                  |
| P1         | `configWorkflow` 绑定模型（game/env 级）+ UI 按绑定渲染；`git-artifact` 上传端点落地 |
| P2         | `redis-publisher`（skynet 惯例最小接入）+ `db-table` snapshot 模式                   |
| P3         | `nacos-bridge` + `db-table` managed 模式 + `external-passthrough` 登记               |

## 8. Review Checklist

- 平台任何文档/UI 不得把某一种工作流表述为"标准流程"——只有"平台统一治理段"是标准的；
- 新增 source adapter 必须落到 ConfigVersion（禁止旁路直发生效侧）；
- delivery adapter 失败不得阻塞 ConfigVersion 注册（镜像可重试）；
- `configWorkflow` 未声明的项目，UI 不出现创作入口（不引导、不预设）；
- db-table managed 模式写入必须同时产生 ConfigVersion 快照（版本单调）。
