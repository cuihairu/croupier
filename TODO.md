# Croupier TODO（优化版 v2）

目标：两周内打通“Proto-first → Pack → Server/Agent → RBAC/审批 → 动态 UI”的最小竖切；将“可视化/多域扩展”放到 P1/P2。

## P0（两周，最小竖切）
- X‑Render 集成（基础）
  - 前端引入 form-render 与 @ant-design/icons，保留现有 GmFunctions 渲染为 fallback；
  - 在 GmFunctions 中按 descriptors/ui schema 渲染参数表单，先替换“函数表单”路径；
  - 成功标准：不改后端即可通过 JSON Schema 自动渲染并提交调用，基础字段/数组/对象/Map 可用。
- 实体管理页 MVP
  - 列表 + 新建/校验/预览，直接使用 /api/entities、/api/entities/:id、/api/entities/validate、/api/entities/:id/preview；
  - 表格用 ProTable，表单用 form-render；不做拖拽与复杂筛选；
  - 成功标准：能新增一个实体定义并通过预览看到 UI。
- 道具域闭环补齐
  - 新增 descriptors：components/item-management/descriptors/item.update.json、item.delete.json；
  - 成功标准：item.create/get/list/update/delete 完整可调用（前端表单渲染正常）。
- 注册中心增强（最小版）
  - internal/server/registry/registry.go 增加 FunctionMeta{Entity,Operation,Enabled}，将 AgentSession.Functions 从 map[string]bool 升级为 map[string]FunctionMeta；
  - 增加查询：GetFunctionsForEntity(entity)、GetEntitiesWithOperation(op)、GetFunctionByEntityOp(entity, op)；
  - 成功标准：可按实体/操作检索可用函数（用于前端联动或调试页）。
- 打包与文档小步
  - 沿用 Makefile pack；README 增补“前端 X‑Render 启动与使用”简节；
  - 成功标准：开发者按 README 启动即可演示“函数表单 + 实体管理 + 道具 CRUD”。

## P1（强化易用性与工程化）
- X 组件沉淀
  - 抽象 XResourceTable、XEntityForm，在 EntityManager 与其他模块复用；
  - 完善 UI schema 能力：show_if/required_if/枚举 label/日期时间等控件。
- RBAC 与审批
  - allow_if 增强：has_permission/is_owner/数值比较/时间窗口；
  - 基于 descriptors.auth.risk / auth.two_person_rule 的统一策略入口。
- 幂等与 assignments
  - 命令类幂等：唯一键 + 软 TTL（最小实现）；
  - assignments 落 GORM（替代 assignments.json），HTTP 查询保持兼容。
- 工具与可观测
  - cmd/schema-validator：校验 descriptors/ui/manifest/pack；
  - cmd/pack-builder：一键打包 + 校验；
  - /metrics 暴露 Prometheus 规范指标（当前为 JSON）。
- 经济域竖切
  - currency.create/get/list + wallet.get，演示跨实体操作。

## P2（可视化与生态）
- 可视化 Builder（拖拽 + 实时预览），优先服务“函数参数 Schema”的所见即所得；
- 连接器与渲染：REST/GraphQL/SQL 适配与 outputs.views 渲染器插件；
- 多租户/计费（如有）：租户隔离/配额/审计增强。

## 验收清单（关键交付）
- P0：
  - 通过 X‑Render 渲染并成功调用任一函数（无手写表单）；
  - 实体管理页可新增/校验/预览实体；
  - item.* CRUD 函数齐全且可调用；
  - 注册中心可按实体/操作检索（返回非空）。
- P1：
  - 统一的 X 组件在两处以上复用；
  - allow_if 新语义单测覆盖通过；
  - schema-validator/pack-builder 在 CI 运行并拦截不规范包；
  - assignments 使用 DB 存储后支持分页/筛选。
- P2：
  - 表单拖拽能产出可用 JSON Schema（并可回填渲染）。

## 涉及文件（分阶段执行）
- P0：
  - web/package.json：新增 form-render；
  - web/src/pages/GmFunctions/index.tsx：接入 form-render 渲染；
  - web/src/pages/Entities/index.tsx：新增 EntityManager MVP 页面；
  - components/item-management/descriptors/item.update.json、item.delete.json：新增；
  - internal/server/registry/registry.go：引入 FunctionMeta 与查询函数；必要时在 Agent 注册路径携带元数据。
- P1：
  - web/src/components/XResourceTable.tsx、web/src/components/XEntityForm.tsx：新增；
  - internal/server/http/server.go：allow_if 解析增强；/metrics Prometheus 导出；
  - 新增 cmd/schema-validator、cmd/pack-builder；Makefile 增加目标；
  - assignments GORM 落库与 API 兼容。
