# Croupier 重构主计划

更新时间：2026-03-15
状态：进行中
目标：将 Croupier 重构为“精简核心 + 官方扩展 + 商店安装 + Agent 执行”的架构。

---

## 0. 重构原则

本计划覆盖并替代此前所有零散 todo。旧计划、旧拆分、旧阶段目标一律不再作为约束，避免继续被历史设计绑住。

重构只遵循下面几条原则：

1. 核心必须小而稳定。
2. 非核心业务尽量扩展化、商店化。
3. 扩展先走声明式安装，不先做任意代码插件。
4. Server 负责控制与编排，Agent 负责执行与上报。
5. 所有扩展统一走权限、审计、配置、任务、健康检查、UI 渲染底座。
6. 迁移按阶段推进，每个阶段结束后都必须能停下、验证、继续。

---

## 1. 最终目标架构

### 1.1 Core Kernel

核心只保留这些能力：

- auth
- permission / RBAC
- audit
- workspace / game / env / node 基础模型
- agent 注册、心跳、节点管理
- function registry / invocation pipeline
- config / secret 管理
- task / job 基础设施
- base ops
- extension catalog / installation / runtime
- UI schema 渲染底座

### 1.2 Extension Runtime

扩展系统负责：

- 扩展目录
- 安装 / 卸载 / 启用 / 停用 / 升级 / 回滚
- 配置管理
- 生命周期管理
- capability 注册
- function / provider / page / workflow 绑定
- 健康检查
- Agent 分发与同步

### 1.3 Driver Layer

驱动层只保留少量稳定驱动：

- openapi-driver
- webhook-driver
- grpc-driver
- workflow-driver
- internal-ui-driver

暂不引入动态 Go 插件作为主架构。后续若确有必要，再单独评估 `go-plugin-driver`。

### 1.4 Official Extensions

优先规划为官方扩展的模块：

- analytics
- alerting
- notification
- external-platform
- approval
- backup-advanced
- monitoring-advanced

---

## 2. 边界划分

### 2.1 必须保留在核心

- 认证与权限
- 审计
- Agent 协议
- 节点与基础运维
- 函数注册与调用链路
- 配置与密钥
- 扩展运行时本身
- 商店与安装实例

### 2.2 应逐步迁移为扩展

- 数据分析
- 报表与看板
- 第三方平台集成
- 审批流
- 通知中心
- 反馈 / 工单
- 备份增强能力
- 高级监控与告警

### 2.3 禁止继续发生的情况

- 在核心继续新增重业务 API
- 让业务模块绕过权限和审计
- 继续以 YAML 作为正式主数据源
- 把“安装扩展”和“写死代码”混用

---

## 3. 扩展模型

统一抽象：

- Extension：安装单元
- Capability：能力分类
- Operation：具体动作

示例：

- Extension：`official.analytics`
- Capability：`analytics.query`
- Operation：`run`

不要直接把底层 method 名称暴露给产品、权限和 UI。

---

## 4. 扩展包标准

目标结构：

```text
extension/
  manifest.yaml
  icon.png
  README.md
  config.schema.json
  secrets.schema.json
  capabilities.yaml
  navigation.yaml
  pages/
  functions/
  providers/
  workflows/
  openapi/
```

`manifest.yaml` 至少包含：

- id
- name
- display_name
- version
- vendor
- kind
- driver
- targets
- install_mode
- min_core_version
- dependencies
- permissions
- capabilities
- ui
- healthcheck
- lifecycle
- upgrade
- visibility

---

## 5. 数据模型目标

需要新增或重构的核心表：

- `extension_catalog`
- `extension_release`
- `extension_installation`
- `extension_capability`
- `extension_runtime_binding`
- `extension_health`
- `extension_event`
- `secret_binding`

要求：

- 正式安装状态必须入库
- Agent 本地只做缓存与运行时副本
- 文件配置仅保留开发 / 本地调试用途

---

## 6. API 目标

核心新增 API 分组：

### 6.1 Catalog

- `GET /api/v1/extensions/catalog`
- `GET /api/v1/extensions/catalog/:id`
- `GET /api/v1/extensions/catalog/:id/releases`

### 6.2 Installation

- `GET /api/v1/extensions/installations`
- `POST /api/v1/extensions/install`
- `POST /api/v1/extensions/:id/enable`
- `POST /api/v1/extensions/:id/disable`
- `POST /api/v1/extensions/:id/upgrade`
- `DELETE /api/v1/extensions/:id/uninstall`

### 6.3 Config

- `GET /api/v1/extensions/:id/config-schema`
- `GET /api/v1/extensions/:id/config`
- `PUT /api/v1/extensions/:id/config`
- `POST /api/v1/extensions/:id/test-connection`

### 6.4 Runtime

- `GET /api/v1/extensions/:id/capabilities`
- `POST /api/v1/extensions/:id/health-check`
- `POST /api/v1/extensions/:id/reconcile`
- `GET /api/v1/extensions/:id/events`

### 6.5 Agent

- `GET /api/v1/agents/:id/extensions`
- `POST /api/v1/agents/:id/extensions/sync`

---

## 7. Dashboard 目标

Dashboard 分三层：

- 核心页面
- 扩展管理页面
- 扩展渲染页面

必须新增的页面：

- 商店列表页
- 扩展详情页
- 安装向导页
- 配置页
- 健康状态页
- 升级页
- 生命周期事件页

扩展页面应尽量由 schema 驱动，不继续大量手写业务页面。

---

## 8. Agent 目标

Agent 需要新增 `ExtensionRuntime`，负责：

- 拉取分配给本节点的安装实例
- 加载 config / secret 引用
- 初始化 driver
- 发现 capability
- 注册 function
- 执行调用
- 上报健康、错误、版本、能力清单

说明：

- Agent 是执行节点，不是商店主控
- 安装主数据源仍在 Server

---

## 9. 分阶段执行计划

以下阶段必须顺序推进。每个阶段都要求：

- 有明确产出
- 有可验证检查项
- 有中断恢复说明

### Phase 0：冻结边界与建立主线

目标：

- 停止继续按旧路径往核心塞业务
- 明确核心与扩展边界
- 为后续重构建立统一术语

任务：

- [x] 输出核心模块清单（见 `docs/architecture/core-extension-mapping.md`）
- [x] 输出扩展候选模块清单（见 `docs/architecture/core-extension-mapping.md`）
- [x] 输出现有目录到未来架构的映射表（见 `docs/architecture/core-extension-mapping.md`）
- [x] 确定扩展模型术语：Extension / Capability / Operation（见本计划第 3 节）
- [x] 确定不采用 `HashiCorp/go-plugin` 作为主扩展机制（见本计划第 11 节）
- [x] 清理旧的重构计划、过时路线说明、误导性文档（已清理 docs 首页与 third-party/architecture 文档中的 pack 与 YAML 主路径误导表述）

产出：

- 新版架构主文档
- 新版迁移边界说明

检查项：

- [x] 团队可以用一句话说清核心和扩展的边界（见 `docs/architecture/core-extension-mapping.md` 顶部边界语句）
- [x] 新业务不再默认进入核心（见 `docs/development/new-business-extension-policy.md` 准入规则）

中断恢复：

- 从文档和目录映射表继续
- 若边界尚未冻结，禁止进入下一阶段

### Phase 1：定义扩展规范

目标：

- 定义 manifest、capability、installation、binding 等基础标准

任务：

- [x] 设计 `manifest.yaml` 规范（见 `docs/architecture/extension-manifest-draft.md`）
- [x] 设计 `capabilities.yaml` 规范（见 `docs/architecture/extension-capabilities-draft.md`）
- [x] 设计 config schema 和 secrets schema 约定（见 `docs/architecture/extension-manifest-draft.md`）
- [x] 设计 runtime binding 规范（见 `docs/architecture/extension-runtime-service-draft.md`）
- [x] 定义扩展生命周期状态机（见 `docs/architecture/extension-installation-model.md`）
- [x] 定义安装实例 scope 与 target 模型（见 `docs/architecture/extension-installation-model.md`）
- [x] 输出示例扩展包（见 `docs/architecture/extension-package-layout-draft.md`）

产出：

- 扩展规范文档
- 示例 manifest

检查项：

- [x] 一个扩展无需写宿主代码即可描述安装信息（已通过 manifest/capabilities/install schema 约束）
- [x] capability 和 operation 命名规则固定（见本计划第 3 节与 `extension-capabilities-draft.md`）

中断恢复：

- 从 schema 和 manifest 设计继续
- 若规范未稳定，不得开始建表和写 API

### Phase 2：实现核心扩展运行时骨架

目标：

- 在核心中落地 catalog / installation / runtime 的最小可运行骨架

任务：

- [x] 新增 `extension_catalog` 等核心表
- [x] 新增 manifest 解析器
- [x] 新增 installation service
- [x] 新增 catalog service
- [x] 新增 runtime reconciler
- [x] 新增 health / event 记录结构
- [x] 接入权限与审计

产出：

- 后端扩展运行时最小骨架

检查项：

- [x] 可以写入 catalog
- [x] 可以创建 installation
- [x] 可以完成 enable / disable / uninstall 状态流转

中断恢复：

- 从数据表和 service 层继续
- 若 installation 还只是内存态，不能进入下一阶段

### Phase 3：实现 Dashboard 扩展管理壳

目标：

- 先有“商店与安装壳”，再迁业务

任务：

- [x] 新增商店列表页
- [x] 新增扩展详情页
- [x] 新增安装向导页
- [x] 新增安装实例列表页
- [x] 新增配置页
- [x] 新增状态与事件页
- [x] 新增 schema renderer 最小版本

产出：

- 扩展管理基础 UI

检查项：

- [x] 能浏览 catalog
- [x] 能安装扩展
- [x] 能启停扩展
- [x] 能查看安装状态和错误

中断恢复：

- 从安装与详情页继续
- 若 UI 还依赖临时 mock，不得进入正式迁移

### Phase 4：实现 Agent ExtensionRuntime

目标：

- 让 Agent 成为真正的扩展执行节点

任务：

- [x] Agent 拉取 installation 分配（最小实现：可选 HTTP 拉取 `/api/v1/agents/:id/extensions` 并应用到本地 ExtensionRuntime）
- [x] Agent 初始化 driver（新增 `ExtensionDriverRuntime`，支持按 binding 解析 driver 并执行 init/reload/stop 生命周期）
- [x] Agent 本地缓存 runtime 状态
- [x] Agent capability discover（从 extension runtime bindings 发现 capability/function/operation）
- [x] Agent 注册 function（将发现结果以 `extension:<installation_id>` provider 注册到 LocalStore 并参与上游注册）
- [x] Agent 上报健康与错误（通过 Register 动态 labels 上报 runtime 安装数/状态/错误摘要与时间）
- [x] Agent 支持 reconcile / reload

产出：

- Agent 侧扩展运行时

检查项：

- [x] Server 安装的扩展可同步到 Agent（已具备 `agents/:id/extensions` 拉取 + 应用能力，并有拉取与应用测试）
- [x] Agent 可上报能力列表（扩展 bindings 发现的 function/capability 已参与上游 Register，上游注册测试覆盖）
- [x] 调用链路能走通（扩展函数可经 Agent NNG invoke 入口命中 extension driver 并返回响应，端到端测试覆盖）

中断恢复：

- 从 Agent 同步和 capability 注册继续
- 若 Agent 还依赖本地 YAML 为主配置，不得进入下一阶段

### Phase 5：迁移 external-platform 为第一个官方扩展

目标：

- 用第三方平台接入作为第一个完整迁移样板

任务：

- [x] 抽离现有 platform/provider 逻辑为 `official.external-platform`（核心侧保留 legacy gateway 兼容层，主路径已 extension-first）
- [x] 把 YAML 驱动配置迁移为 installation 驱动（最小实现：Agent 可从 extension bindings 同步动态 provider，替换 extension 管理域内配置）
- [x] 兼容现有 `external.v1` 协议（最小兼容：`external.<provider>.<method>` 支持 `CallPlatformRequest/Response` proto）
- [x] 统一 capability / operation 映射
- [x] 接入 openapi-driver（Agent `openapi-driver` 由 no-op 升级为真实转发，复用 external bridge 调用 ProviderManager）
- [x] 支持扩展页面展示平台能力（`/extensions/:id/capabilities` 增加结构化 `details`，可直接渲染 provider/capability/operations）

产出：

- 第一个可安装、可启停、可同步的官方扩展

检查项：

- [x] 新平台不需要改核心代码即可通过安装实例接入（bindings 驱动 provider/method 发现，平台接口自动识别）
- [x] Agent 可以发现并注册对应 function（`official.external-platform` bindings 可自动产出并注册 `external.*` function）

中断恢复：

- 从 provider 配置迁移继续
- 若协议兼容未完成，不得删除旧接口

### Phase 6：迁移 analytics 为官方扩展

目标：

- 把最重的业务模块从核心剥离

任务：

- [x] 梳理 analytics API、任务、存储、页面边界（见 `docs/architecture/official-analytics-migration-draft.md`）
- [x] 抽象核心仍需保留的基础能力（见 `docs/architecture/official-analytics-migration-draft.md` 的“核心保留 / 扩展迁移清单”）
- [x] 把 analytics 页面转为扩展页面（`official.analytics` runtime 已生成 `page` bindings，扩展接口新增 `GET /extensions/:id/pages` / `GET /extensions/installations/:id/pages`）
- [x] 把 analytics 配置转为 installation 配置（analytics filters 已改为优先读写 `official.analytics` 安装配置，文件路径仅兜底）
- [x] 把分析任务接入扩展 runtime（`runtime.Reconcile` 已为 `official.analytics` 生成 filters/ingest 相关 binding 骨架）
- [x] 保持已有数据模型兼容迁移（安装配置读写兼容 `filters` 与 `analytics_filters`，并保留文件路径兜底）

产出：

- `official.analytics`

检查项：

- [x] 不安装 analytics 时核心仍可正常运行（`TestFiltersGetFallsBackToFileWhenExtensionNotInstalled`）
- [x] 安装 analytics 后功能可恢复（`TestFiltersGetPrefersExtensionInstallationConfig`）

中断恢复：

- 从 API 和页面边界拆分继续
- 若基础依赖还没抽干净，不得开始删除旧代码

### Phase 7：迁移 alerts / approval / backup / notification

目标：

- 逐步把非核心业务迁出核心

任务：

- [ ] alerts 迁移
- [ ] notification 迁移
- [ ] approval 迁移
- [ ] backup advanced 迁移
- [ ] 统一这些扩展的权限、菜单、配置方式

产出：

- 第二批官方扩展

检查项：

- [ ] 核心目录显著收敛
- [ ] 业务模块安装与升级路径统一

中断恢复：

- 每个模块独立推进
- 未完成一个模块的边界切割，不要并行清理过多旧代码

### Phase 8：核心收敛与兼容层清理

目标：

- 清理历史残留，完成架构切换

任务：

- [ ] 删除废弃 YAML 主配置路径
- [ ] 删除旧 provider 直连入口
- [ ] 删除过时 API
- [ ] 删除仅为旧模块存在的核心耦合代码
- [ ] 清理旧文档与旧示例
- [ ] 更新 README、部署说明、架构文档

产出：

- 收敛后的核心代码库

检查项：

- [ ] 核心不再承载重业务
- [ ] 扩展安装成为默认工作方式
- [ ] 新增业务默认走扩展流程

中断恢复：

- 从兼容层清理继续
- 任一兼容入口删除前必须有替代路径

---

## 10. 当前优先级

当前只做以下事情，其他先不扩散：

1. 冻结边界
2. 定义扩展规范
3. 建立扩展运行时骨架
4. 先迁 external-platform
5. 再迁 analytics

当前已补充的 Phase 1 / 2 设计文档：

- `docs/architecture/core-extension-mapping.md`
- `docs/architecture/extension-manifest-draft.md`
- `docs/architecture/extension-capabilities-draft.md`
- `docs/architecture/extension-installation-model.md`
- `docs/architecture/extension-gorm-model-draft.md`
- `docs/architecture/extension-runtime-service-draft.md`
- `docs/architecture/dashboard-extension-ui-draft.md`

---

## 11. 当前明确不做

- 不把 `HashiCorp/go-plugin` 作为主扩展机制
- 不恢复旧 `pack` 路线
- 不先做任意二进制插件执行
- 不继续把 analytics 等重模块堆进核心
- 不让 Dashboard 为每个扩展继续深度写死页面

---

## 12. 下一步直接执行项

下一轮开始时，直接从以下任务进入：

- [x] 输出当前代码目录到目标架构的映射表
- [x] 编写 `manifest.yaml` 草案
- [x] 编写 `capabilities.yaml` 草案
- [x] 编写 installation 数据表草案
- [x] 编写 GORM model 草案
- [x] 编写 extension runtime service 草案
- [x] 设计 Dashboard 商店页和安装页最小信息结构

后续优先继续：

- [x] 编写 extension API DTO 草案
- [x] 编写 extension SQL migration 草案
- [x] 编写 Agent sync payload 详细规范
- [x] 编写 `official.external-platform` 迁移设计稿
- [x] 编写 extension repo / service 包结构落地方案
- [x] 编写 `svc.ServiceContext` 扩展挂载草案
- [x] 编写 extension 路由注册草案
- [x] 开始落第一批后端骨架代码
- [x] 为 extension 骨架补 repo 层与更严格的 service 分层
- [x] 为 extension API 补分页过滤、基础错误语义和权限守卫
- [x] 为 extension runtime 补 binding / event / sync 的最小真实实现
- [x] 开始接入 Dashboard 的 `/extensions` 基础页面
- [x] 扩展事件接口支持服务端分页/筛选，安装详情返回真实 runtime bindings
- [x] Agent sync payload 支持 `agent_group/default` 与全局目标匹配下发
- [x] 安装/更新配置接口增加基于 `config_schema` 的服务端校验（required/type/enum）
- [x] 安装前冲突检测（同扩展+同 scope/target），并统一映射 duplicated key 为 conflict
- [x] 升级前校验目标版本存在性、重复升级冲突、目标 schema 兼容性
- [x] 卸载时清理 runtime bindings，并在 Agent sync 中排除 uninstalled 实例
- [x] 为扩展配置 schema 校验逻辑补单元测试（required/type/enum/integer）
- [x] 安装/升级增加 `manifest.dependencies` 依赖校验（含基础版本匹配）并补测试
- [x] 依赖版本匹配升级为 semver 约束（支持 `>=`, `<`, `^`, `~`, 逗号组合）
- [x] 依赖校验升级为递归依赖树检查，并拦截循环依赖
- [x] 卸载前增加“被依赖保护”，阻止卸载仍被活动扩展依赖的实例
- [x] 卸载拦截错误增强：返回完整阻塞依赖扩展列表（`extension@version`）
- [x] 冲突错误结构化：返回 `details.code=dependency_blocked` 与 `details.blockers`
- [x] 对齐 Agent 扩展同步兼容路由：`GET /api/v1/agents/:id/extensions`、`POST /api/v1/agents/:id/extensions/sync`
- [x] Dashboard 卸载交互接入 `dependency_blocked` 结构化错误并展示 blockers 列表
- [x] 补齐安装实例配置能力接口：`config-schema/config/test-connection`
- [x] Dashboard 安装详情接入测试连接按钮与新配置接口
- [x] 补齐 Catalog 版本接口：`GET /api/v1/extensions/catalog/:id/releases`（含前端 API 封装）
- [x] 补齐 Runtime 能力接口：`capabilities/health-check` 并接入 Dashboard 详情操作
- [x] Catalog/Runtime 能力字段改为真实 manifest 解析（含测试覆盖）
- [x] 补齐 `/api/v1/extensions/:id/*` 兼容路由（config/config-schema/test-connection/capabilities/health-check/reconcile/events）
- [x] 补齐 `/api/v1/extensions/:id/*` 动作路由（enable/disable/upgrade/uninstall）
- [x] `/api/v1/extensions/:id/*` 兼容路由支持 `id=安装ID` 或 `id=扩展ID`
- [x] Catalog `installed` 字段改为真实安装态（列表 + 详情）
- [x] Dashboard 商店页基于 `installed` 状态禁用重复安装入口
- [x] Catalog 元数据补齐并在 Dashboard 展示：`default_install`、`tags`
- [x] 依赖校验错误结构化（missing/version_mismatch/cycle）并经统一响应层透出 `details`
- [x] Dashboard 安装向导接入依赖结构化错误提示（missing/version/cycle）
- [x] 安装冲突错误结构化为 `extension_already_installed`（含安装实例与范围详情），Dashboard 商店安装弹窗可直接提示已存在实例信息
- [x] 安装冲突检测改为数据库精确查询活动安装（不再依赖分页列表），补充单元测试覆盖“命中活动实例/忽略已卸载实例”
- [x] Agent 新增 `ExtensionRuntime` 最小骨架：支持 `ApplyPayload`、本地快照缓存、`Reconcile`、`Reload`，并在 `App` 暴露入口（含单元测试）
- [x] Agent 增加扩展同步拉取器 `ExtensionSyncPuller`（支持周期拉取与手动 `PullOnce`），与 `App.Run` 通过环境变量联动
- [x] Agent 在扩展同步后自动同步 LocalStore 函数视图：支持从 `binding_type=function/capability/operation` 生成可注册 function id（含测试）
- [x] Agent 扩展运行时增加状态机字段（ok/degraded/error）与最后错误记录，并接入 Upstream 动态 labels 上报
- [x] Agent 引入可插拔 driver 运行时层（内置 openapi/webhook/grpc/workflow/internal-ui no-op driver），并把 driver 协调结果纳入 runtime labels
- [x] 扩展函数调用路由接入 Agent invoke 主链路：`providerManagerWrapper` 优先命中 extension function route 并转发到 `ExtensionDriverRuntime.Invoke`
- [x] Phase 5 预备：`official.external-platform` 兼容发现层（`provider/openapi` bindings 自动产出 `external.<provider>.<operation>` function id）
- [x] Phase 5 预备：`external.<provider>.<method>` 调用兼容 `croupier.external.v1`（支持 `CallPlatformRequest/Response` proto 透传与错误封装）
- [x] Phase 5 预备：ProviderManager 支持 `SyncExtensionProviders`（按 installation/binding 动态注册/替换/清理 extension providers）
- [x] Phase 5 预备：`internal/api/platform` 改为“扩展函数优先、PlatformLoader 兜底”，并支持从 registry 发现 `external.*` 平台/方法
- [x] Phase 5 预备：ProviderManager 增加迁移开关（`CROUPIER_EXTENSION_PROVIDERS_ONLY`、`CROUPIER_EXTENSION_PROVIDER_OVERRIDE_STATIC`）支持按环境切换到 extension-first
- [x] Phase 5 预备：`internal/api/platform` 支持从 extension installation bindings 直接发现平台与方法（agent 未同步前也可展示声明能力）
- [x] Phase 5 预备：Dashboard 平台列表增加 `source` 展示（extension/legacy），用于观察迁移切流状态
- [x] Phase 5 预备：抽取 `external.<provider>.<method>` 共享标识工具（构造/解析/规范化）并复用到 agent+platform，避免语义漂移
- [x] Phase 5 预备：平台方法发现支持从 `function` bindings 的 `external.*` 直接反推 provider/method（兼容不同 binding 风格）
- [x] Phase 5 预备：平台接口支持 `CROUPIER_PLATFORM_EXTENSION_ONLY`（扩展专用模式下禁用 legacy 回退，实现可控下线）
- [x] Phase 5 预备：平台接口新增 `CROUPIER_PLATFORM_LEGACY_DISABLED`（可在非 extension-only 下强制关闭 legacy 回退，便于灰度切流）
- [x] Phase 5 预备：`svc` 初始化阶段支持 `CROUPIER_PLATFORM_LEGACY_DISABLED`，legacy loader 可按环境完全不加载
- [x] Phase 5 预备：平台调用新增 `CROUPIER_PLATFORM_LEGACY_FALLBACK_ON_EXTENSION_ERROR`（控制扩展报错后是否回退 legacy，支持严格切流）
- [x] Phase 5 预备：统一 external capability/operation 映射（`external.<provider>` + `<method>`），并写入 agent function 描述元数据
- [x] Phase 5 预备：补齐 legacy 弃用可观测与说明（`svc` 启动告警、README 迁移开关说明、`configs/platforms.yaml` 标记 legacy）
- [x] Phase 5 预备：默认配置文案去 legacy 化（`PlatformConfig` 注释与 `configs/server.yaml` 平台段明确 extension-first）
- [x] Phase 5 预备：`svc` 初始化在 `CROUPIER_PLATFORM_EXTENSION_ONLY` 下跳过 legacy loader（防误配置回流）
- [x] Phase 5 预备：`/api/v1/platform/call` 增加 `fallback/fallback_reason` 可观测字段（区分 legacy 主路径与扩展报错后的回退路径）
- [x] Phase 5 预备：平台迁移开关逻辑下沉到共享模块（`internal/platform/migrationflags`），统一 `api/platform` 与 `svc` 判定语义
- [x] Phase 5 预备：统一 provider/openapi binding 解析器（抽到 `internal/core/extension/externalfunc` 并复用到 Agent + Platform，消除重复解析语义）
- [x] Phase 5 预备：external 平台 provider/method 发现器下沉到 core（`DiscoverProviderOperations`），Agent 与 Platform 共用同一发现语义
- [x] Phase 5 预备：Platform API 引入 legacy gateway 抽象，隔离对 `PlatformLoader` 的直接依赖，便于后续整体移除 legacy 实现
- [x] Phase 6 预备：`/api/v1/agent/analytics-filters` 改为 extension-first（优先读取 `official.analytics` installation config，文件路径兜底）
- [x] Phase 7 预备：编写 `official.alerting` 迁移草案并落 runtime binding 骨架（`docs/architecture/official-alerting-migration-draft.md`）
- [x] Dashboard 升级流程接入依赖结构化错误提示（missing/version/cycle）
- [x] `test-connection` / `health-check` 写入扩展事件流，提升可观测性
- [x] 扩展事件透出 `payload` 字段，并在 Dashboard 事件列表展示
- [x] Dashboard 安装详情接入 `config-schema` 可视化预览（字段/类型/必填/描述）
- [x] 扩展操作事件记录真实操作者（来自鉴权上下文 username）
- [x] Dashboard 升级流程改为版本下拉选择（基于 catalog releases）
- [x] Dashboard 安装弹窗改用 `catalog/:id/releases`，补无版本提示

Dashboard 已完成的最小接入：

- 新增 `src/services/api/extensions.ts` API 客户端
- 新增运营菜单路由：
  - `/operations/extensions/store`
  - `/operations/extensions/installations`
- 新增页面：
  - `src/pages/Extensions/Store/index.tsx`
  - `src/pages/Extensions/Installations/index.tsx`
- 接入权限键：
  - `canExtensionsRead`
  - `canExtensionsManage`

下一步（Dashboard）：

- [x] 扩展安装详情页（含 bindings / config schema / secret refs）
- [x] 安装向导从“自由输入”升级为基于 manifest/schema 的动态表单
- [x] 扩展事件页增加分页与筛选
- [x] Agent sync payload 可视化调试页

新增完成文档：

- `docs/architecture/extension-service-context-draft.md`
- `docs/architecture/extension-route-registration-draft.md`

当前已落地代码骨架：

- `internal/model/extension_*.go`
- `internal/core/extension/*`
- `internal/repo/gorm/extension/*`
- `internal/api/extension/*`
- `internal/svc/service_context.go`
- `internal/handler/routes.go`

新增完成文档：

- `docs/architecture/extension-api-dto-draft.md`
- `docs/architecture/extension-sql-migration-draft.md`
- `docs/architecture/agent-extension-sync-payload-draft.md`
- `docs/architecture/official-external-platform-migration-draft.md`
- `docs/architecture/extension-package-layout-draft.md`

新增完成文档：

- `docs/architecture/extension-runtime-service-draft.md`
- `docs/architecture/dashboard-extension-ui-draft.md`

已完成文档：

- `docs/architecture/core-extension-mapping.md`
- `docs/architecture/extension-manifest-draft.md`
- `docs/architecture/extension-installation-model.md`

---

## 13. 备注

本文件是当前唯一有效的重构主计划。

如果后续方向发生变化，直接更新本文件，不要重新恢复旧计划内容。
