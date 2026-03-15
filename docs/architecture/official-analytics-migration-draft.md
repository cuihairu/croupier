# official.analytics 迁移边界草案

更新时间：2026-03-15  
状态：Draft（用于 Phase 6 第一步边界冻结）

## 1. 目标

把 `analytics` 从核心中剥离为 `official.analytics` 扩展，核心只保留稳定基础设施：

- 鉴权、权限、审计
- 扩展安装与运行时
- 函数注册与调用链路
- 任务基础设施（不含 analytics 业务逻辑）

## 2. 当前现状映射

### 2.1 API 层

- `internal/api/analytics/*`：行为分析、留存、支付、概览、实时、filters 读写
- `internal/api/agent/*`：`analytics-filters` 读取
- `internal/api/function/*`：`function analytics` 统计视图

结论：

- `internal/api/analytics/*` 归类为 `official.analytics` 扩展 API。
- `internal/api/agent` 的 `analytics-filters` 需迁移为扩展能力接口或通过扩展同步 payload 下发，不应留在核心路由。
- `function analytics` 统计口保留兼容读接口，数据来源改为扩展上报/聚合结果。

### 2.2 任务与处理链路

- `cmd/ingest`
- `cmd/analytics-worker`
- `internal/analytics/mq/*`
- `internal/analytics/worker/*`

结论：

- 作为 `official.analytics` 的 runtime/worker 组件，不再视为核心控制面逻辑。
- 核心只保留 worker 生命周期编排入口（可选）和健康状态聚合。

### 2.3 存储与配置

- `registry.analytics_filters_path`（文件态 filters）
- Redis Streams / Kafka（事件队列）
- ClickHouse（分析存储）

结论：

- 文件态 `analytics_filters` 从核心 registry 配置移除，迁移为 installation config/runtime binding。
- Redis/Kafka/ClickHouse 连接配置迁移至扩展 `config_schema`。

### 2.4 页面与路由

- Dashboard `analytics` 页面（overview/realtime/payments/retention/behavior）

结论：

- 迁移为 `official.analytics` 扩展页面（schema 驱动）。
- 核心只保留扩展容器页，不再维护 analytics 业务页。

## 3. 核心保留 / 扩展迁移清单

### 3.1 核心保留

- extension catalog/install/runtime/sync
- audit + permission + operator identity
- dispatcher + agent registry（通用能力）
- 通用 ops 指标和健康聚合

### 3.2 扩展迁移

- analytics API（含 filters 配置）
- ingest + worker 任务逻辑
- analytics 存储写入与查询
- analytics 页面与图表

## 4. 分步执行建议（Phase 6）

1. 冻结 analytics 领域边界（本稿）
2. 定义 `official.analytics` manifest/capabilities/config schema
3. 把 filters 从 `registry` 文件迁移为 installation config
4. 将 `internal/api/analytics` 改为扩展路由（保留短期兼容转发）
5. 把 Dashboard analytics 页面挂载到扩展页面容器
6. 回归验证：未安装扩展时核心可用；安装后 analytics 能力恢复

## 5. 风险与约束

- 现有脚本和运维可能仍依赖 `registry.analytics_filters_path` 文件，迁移需兼容窗口。
- Dashboard analytics 页面可能存在隐式耦合（菜单权限、接口聚合），需要逐页拆分。
- 历史统计口径要保持一致，迁移中需建立对比校验（旧链路 vs 扩展链路）。
