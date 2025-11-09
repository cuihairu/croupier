# Croupier Roadmap (Proto‑first)

目标：让使用者“只写 proto（附少量注解）即可获得 GM 界面 + 校验 + 鉴权 + 调用链”，并可通过插件/适配器无侵入接入第三方系统（Prometheus/HTTP 等）。协议层统一使用 Protobuf；JSON 仅作为前端提交与静态元数据的载体（proto 的标准 JSON 映射）。

—

## 原则
- Proto 作为单一事实来源（契约、类型、演进、跨语言）。
- 由生成器从 proto 派生：Descriptors、JSON Schema、UI Schema、Manifest、FDS（FileDescriptorSet）。
- Server/Agent 动态加载类型（TypeRegistry + dynamicpb），按 descriptor.transport 编解码。
- 界面“视图驱动”，可插拔 Renderer（表格/图表/JSON/第三方库），由 Descriptor 下发视图规范。
- 适配器（Adapter）支持无 SDK 系统（Prom/HTTP/K8s/SQL 等），以 pack 形式导入/下发。

## 关键交付物
- protoc 插件：`protoc-gen-croupier`（或 Buf 插件），输入用户 proto → 输出 pack.tgz。
- Pack 规范：`fds.pb` + `descriptors/*.json` + `ui/*.json` + `manifest.json` + 可选 `web-plugin/*`。
- Server：Pack 导入、动态类型注册、编解码桥接、审计/权限、视图规范下发。
- Web：Schema-Form 渲染、Renderer 注册与插件加载、视图布局与数据变换。
- Agent/SDK：注册能力上报、pack 下发与适配器装载、流式/长任务支持。
- 适配器：`prom-adapter`（专用）与 `http-adapter`（通用配置）。

—

## 目录/结构（新增/变更）
- `proto/croupier/options/function.proto`：方法级元信息（function_id/version/risk/route/timeout/approval/labels/...）。
- `proto/croupier/options/ui.proto`：字段级 UI 元信息（widget/label/placeholder/sensitive/enum/datasource/show_if/...）。
- `tools/protoc-gen-croupier/`：生成器实现（Go）。
- `internal/pack/`：pack 解析/校验/落库与 TypeRegistry 装载。
- `internal/core/descriptor/`：transport/outputs/views/transform 支持。
- `web/src/plugin/registry.ts`：Renderer 注册与动态装载。
- `tools/adapters/prom/`、`tools/adapters/http/`：适配器实现与示例。

—

## Descriptor 变更（核心）
- 新增：`transport`（统一走 protobuf）
  ```json
  {
    "transport": {
      "request_type": "proto",
      "proto": {
        "request_fqn": "<pkg.Message>",
        "response_fqn": "<pkg.Message>",
        "encoding": "pb-json"  // UI 与 HTTP 层 JSON，Server 内部统一转 pb-bin
      }
    }
  }
  ```
- 新增：`outputs.views[]`（视图描述）与可选 `transform`
  ```json
  {
    "outputs": {
      "views": [
        {
          "id": "main",
          "type": "chart|table|json",
          "renderer": "echarts.line|vega-lite|table.basic",
          "transform": { "lang": "cel", "expr": "<JSON 映射>" },
          "options": { "smooth": true }
        }
      ],
      "layout": { "type": "grid", "cols": 2 }
    }
  }
  ```
- 可选：`placement`（adapter 运行位置）：`core|agent`。

—

## 里程碑与任务拆分

### M1 生成器与注解（Proto‑first 基座）
目标：只写 proto → 产出 pack.tgz（含 descriptors/ui/fds/manifest）。
- [ ] 设计自定义 options：function/ui 注解（proto/croupier/options/*）。
- [ ] `protoc-gen-croupier`：解析 FileDescriptor，生成：
  - [ ] `fds.pb`（FileDescriptorSet）
  - [ ] `descriptors/*.json`（含 transport/auth/semantics/placement）
  - [ ] `ui/*.json`（JSON Schema + UI Schema）
  - [ ] `manifest.json`（函数清单、版本、labels、依赖）
  - [ ] 打包 `pack.tgz`
- [ ] `buf.gen.yaml` 集成插件（可选 Buf 插件发布）。
- [ ] 示例：`games.player.v1` 的 `Ban` 生成完整 pack。

验收：`buf generate` 一键生成 pack；用 CLI `croupier pack inspect` 可查看清单。

### M2 Server：Pack 导入与动态类型桥接
目标：导入 pack → UI/调用可用。
- [x] CLI/API：`/api/packs/import`、`croupier packs import pack.tgz`。
- [x] TypeRegistry：从 `fds.pb` 载入（目录加载 `*.pb`），动态编解码。
- [x] Invoke 编解码桥：UI JSON → dynamicpb（pb-json → pb-bin），响应反向转换。
- [x] StartJob：与 Invoke 一致的路由校验与 protobuf 编码（新增）。
- [x] 审计：入参脱敏（按 UI `sensitive` + 常见字段），记录摘要。
- [x] 权限：descriptor.auth → RBAC；two_person_rule：服务端已实现审批队列与 API；支持 Postgres 持久化（需 -tags pg）。
  - [x] 审批 UI 打通（前端列表/详情/同意/拒绝、筛选/分页、二次确认/MFA）。
- [x] 兼容现有调用路径（不破坏已有接口）。
- [x] Packs ETag/version：`/api/packs/export` 返回 `ETag`，`/api/packs/list` 返回 `etag`（Agent 用于下发激活校验）；可选导出 RBAC（`PACKS_EXPORT_REQUIRE_AUTH=true` 时校验 `packs:export`）。
- [x] API RBAC 门禁补充：`/api/registry`（`registry:read`）与 `/api/audit`（`audit:read`）。

验收：示例 pack 导入后，前端可看到函数、填写表单、完成一次调用（回环或回声）。

### M3 Web：Schema‑Form 与 Renderer 插件
目标：通用表单 + 视图渲染 + 插件注册。
- [x] Schema‑Form 渲染（支持 grid/group/tabs/array/map、校验、显隐/联动）。
- [x] Renderer Registry：`registerRenderer(id, component)` 与调用管线。
  - [x] 内置 renderer：`echarts.line`（或 `vega-lite`）。
- [x] 视图数据流：执行 transform（前端 CEL-lite），渲染多视图+布局（expr+template/forEach/filter）。
- [x] 插件装载：从 pack manifest 动态 import 前端插件（ESM），支持 sandbox 选项（基础）。
- [x] GM Pages：Assignments/Packs/Registry/Audit 门禁与可用性（筛选/分页/导出/覆盖健康、Only Uncovered/Partial/分组）。
- [x] Transform 单测补充（map/pluck/sum/avg、数组下标、toFixed/iso*）。

验收：用示例函数展示表格/折线图；可动态加载 echarts 插件渲染。

### M4 适配器：Prometheus（专用）与 HTTP（通用）
目标：无 SDK 系统接入。
- [ ] prom-adapter（Agent 侧，placement=agent）：
  - [ ] Proto：Query/QueryRange/Timeseries；UI 注解；Descriptor + 视图（折线）。
  - [x] 实现雏形：QueryRange 调用 `/api/v1/query_range`（基础，无缓存/限流）。
  - [x] Pack：发布 prom 示例 pack。
- [ ] http-adapter（通用配置）：
  - [ ] 固定 proto：GenericHttpInvoke{request,response}
  - [ ] 映射：JMESPath/CEL 把 JSON → 目标 pb（或标准契约）。
  - [x] Pack：示例（Alertmanager/Grafana 简单查询）。

验收：导入 prom pack，在 UI 选择区间并绘图；导入 http pack，完成一次 REST → 表格展示。

### M5 Agent/SDK 能力上报与 pack 下发
目标：能力发现、热更新与分发。
- [ ] Agent 注册上报：函数 id/版本、request/response FQN、stream 支持、标签（game/env/region）。
- [x] Server → Agent pack 下发（最小演示）：按作用域（game/env）分发，Agent 轮询 assignments 与 pack export，支持目录落地与 Server reload/import 触发；ListLocal 过滤实例。
- [x] Adapter 管理（基础）：健康检查、优雅退出、指数退避重启、日志滚动、指标导出（running/healthy/last_health_ts/last_start_ts/health_failures_total），可选“连续失败阈值自动重启”。
- [x] 下发激活校验：Agent 导入/重载后对比 `/api/packs/list` 的 `etag`，确认生效。
- [ ] Go SDK：简化 Handler（ctx+req→resp），自动注册辅助；示例工程。
- [ ] 作业流：日志/进度流式透传（UI 接收）。

验收：Server 控制某 game/env 下发/撤回某适配器；SDK 示例正常注册与调用。

### M6 安全与可观测
- [x] RBAC/ABAC 表达式（基于上下文如 actor/game/env）：`auth.allow_if`（Lite）。
- [x] 两人规则：审批持久化、幂等与并发控制、UI 审批页。
- [x] 速率/并发/熔断与重试策略（函数级）（基础：rate_limit、concurrency）。
- [x] 指标：Server/Agent/Edge 统一 metrics（JSON + Prometheus 文本）；支持 per_function 与 per_game_denies 开关。
- [x] 追踪：trace_id 贯通（HTTP 适配器向下游透传 X-Trace-Id/Game/Env），后续接入 OTLP。
- [x] 兼容策略：函数版本协商、灰度/回滚（基础：Agent 侧 prefer version 路由）。

验收：关键函数开启审批与限流，指标在 Prom/Grafana 可见，链路可追踪。

—

## 时间预估（可滚动调整）
- M1：1–2 周（注解设计 + 生成器骨架 + 示例）。
- M2：1–2 周（导入 + 动态类型 + 编解码 + 审计/权限骨架）。
- M3：1 周（表单/renderer/插件）。
- M4：1–2 周（prom + http 适配器）。
- M5：1 周（能力上报、下发、SDK 示例）。
- M6：1–2 周（安全与可观测强化）。

—

## 风险与决策
- Renderer 选型：`echarts`（工程落地快） vs `vega-lite`（规范统一）。先选 echarts，保留 vega-lite 适配。
- 变换语言：`CEL`（服务端/前端可复用）优先；大 JSON 场景考虑 `JSONata/JMESPath`。
- 安全：插件隔离（iframe + postMessage）默认关闭网络；manifest 签名/哈希校验。
- 兼容：保持现有 FunctionService 与 descriptors 逐步过渡（提供迁移脚本）。

—

## 验收样例（端到端）
- 用户写 `games.player.v1` 的 `Ban` proto + 注解 → 生成 pack → 在 Server 导入 → UI 自动出现表单 → 提交 → Server 编解码为 pb → Agent → 业务服返回 → UI 以 table/json 展示。
- 导入 `prom-adapter` pack → 选择表达式与时间区间 → UI 折线图展示，审计记录查询参数。

—

## 近期迭代计划（滚动两周）
- 审批持久化落地：嵌入式 SQLite（或 BoltDB）实现 `/api/approvals*` 存储、分页、筛选；补充并发与幂等保护；补齐 CLI 辅助命令。
- TypeRegistry/编解码测试：为 `LoadFDS/JSONToProtoBin/ProtoBinToJSON` 与 HTTP `/api/invoke|/api/start_job` 路径补充单测（`internal/pack/testdata`）。
- Web 表单与 Renderer 骨架：Schema-Form 渲染基础能力；Renderer Registry + 内置 `json.view/table.basic/echarts.line`；打通 `outputs.views`。
- 适配器雏形：`tools/adapters/prom` 与 `tools/adapters/http` PoC，生成示例 pack 并通过 `/api/packs/import` 导入。
- 配置与观测：确认 metrics 默认值透传；整理 `docs/metrics.md` 与 `docs/config.md` 的样例配置；补充 `/metrics.prom` per-function 开关说明。
- 清理与对齐：README 统一使用 unified CLI（`croupier server/agent/edge`），保留历史参数别名说明；移除残留 `core` 叙述。
