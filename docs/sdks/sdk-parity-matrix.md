---
title: SDK 能力矩阵
---

# SDK 能力矩阵

本页定义跨语言统一基线。实现状态和语言映射见根目录 `sdks/SDK_FEATURE_MATRIX.md`；协议细节见 [SDK Wire Protocol](../zh/architecture/sdk-wire-protocol.md)。

## 分层

| 层级 | 定位                  | 统一要求                                                 |
| ---- | --------------------- | -------------------------------------------------------- |
| L1   | Provider 被调用最小集 | 所有官方 SDK 必备：注册、调用、心跳、重连、TLS、drain。  |
| L2   | Provider 治理扩展     | 实现即须对齐：schema、manifest、文件传输、OpenAPI 导入。 |
| L3   | Invoker 独立调用方    | 仅 Server HTTP；禁止复用 Provider TCP session。          |
| L4   | 语言/引擎扩展         | 不做跨语言对齐，但必须声明平台版本与语言运行时限制。     |

## 必备与可选

| 能力             | 分层 | 状态 | 接入要求                                             |
| ---------------- | ---- | ---- | ---------------------------------------------------- |
| 注册与会话       | L1   | 必备 | 非空 `gameId/env/agentId/serviceId/sdkVersion`。     |
| 函数目录         | L1   | 必备 | 稳定函数 ID、输入/输出 schema、风险与权限。          |
| 心跳/重连/drain  | L1   | 必备 | 在途调用先收尾，drain 后不再接新请求。               |
| OpenAPI Provider | L2   | 可选 | `operationId` 稳定，schema 与实现一致。              |
| 任务             | L3   | 可选 | 启动、状态、事件、取消与幂等结果查询。               |
| 外部函数插件     | L2   | 可选 | 声明契约、进程生命周期、超时取消和故障隔离。         |
| 文件下发         | L2   | 可选 | 校验 SHA256、大小和落盘边界。                        |
| 探测             | L2   | 可选 | 明确协议语义与网络位置，未实现必须返回 unsupported。 |

## 版本与兼容

| 对象     | 必填契约                                             | 兼容规则                                     |
| -------- | ---------------------------------------------------- | -------------------------------------------- |
| SDK      | 语言、`sdkVersion`、`gameId/env`、wire 能力闭集      | 低于配置下限拒绝注册；低于高水位接受但告警。 |
| Agent    | `agentVersion`、承载能力、wire 版本、`gameId/env`    | 能力缺失报 unsupported，不静默降级。         |
| Function | `functionId`、版本、输入/输出 schema、权限、审批策略 | 低版本可设拒绝门槛；旧绑定按摘要判 stale。   |
| OpenAPI  | OpenAPI 3、稳定 `operationId`、schema、风险标注      | 导入时校验；schema 或 ID 变更是新契约。      |
| 插件     | manifest、依赖、ABI/IPC 版本、能力、权限             | 未声明版本或不满足依赖拒绝安装/启用。        |
| PageSpec | Page、binding、审批策略、renderer ABI、schema 摘要   | 发布快照冻结；stale 绑定阻断执行。           |

升级顺序：协议与平台 → Server → Agent → SDK/插件 → PageSpec 发布。混合版本窗口必须保持旧字段可读；删除字段或改变语义属于破坏性变更，先发布可兼容过渡契约。

## 错误闭集

| 状态                     | 触发                             | 消费方动作                             |
| ------------------------ | -------------------------------- | -------------------------------------- |
| `MISSING_SCOPE`          | 缺 `gameId/env`                  | 修正身份后重连；不得本地默认。         |
| `SCOPE_MISMATCH`         | 会话与函数/页面目标 scope 不一致 | 停止执行并检查注册来源。               |
| `SCHEMA_INVALID`         | payload 不满足输入/输出契约      | 不调用业务实现。                       |
| `VERSION_TOO_LOW`        | SDK/函数低于拒绝门槛             | 升级后重新注册。                       |
| `CONTRACT_STALE`         | 页面快照与当前函数契约不一致     | 重新发布页面或更新函数。               |
| `CAPABILITY_UNSUPPORTED` | 能力未实现但被请求               | 如实报错；不能把 metadata 当成功。     |
| `RESULT_UNKNOWN`         | 超时、断线或业务响应丢失         | 只能查询原 operationId，禁止替代重试。 |

## 接入链验收

1. 注册：非空 scope、agent/service 身份、SDK 版本和函数契约成功入库；错误 scope 被拒绝。
2. Schema：合法最小 payload 通过；缺字段、类型错误、越权字段失败且错误定位到字段。
3. 授权：无权限调用被拒；审批函数先审批后执行，批准后契约变化被阻断。
4. 受控写：同 `operationId` 重放返回原凭据或幂等结果；同 ID 改参数冲突。
5. 结果未知：模拟响应丢失后只允许查询原 ID，不得发放替代副作用。
6. 断线恢复：重建 session 后函数目录一致，重连风暴有退避。
7. 任务取消/状态：运行任务收到取消后停止在途工作；终态与事件可查询。
8. 探测故障：探测不可用产生可定位错误，不伪装业务成功。
9. 可观测：SDK/OpenAPI/插件路径记录耗时、跳转数和失败分类。

## 验收工具与参考

- 参考游戏服务：`examples/openapi-provider` 提供 `/players` 查询、CRUD 和 `player.kick`，`operationId` 与 OpenAPI 3 契约一致，默认 `default/dev` 可覆盖。
- 跨语言工具：扩展 `scripts/check-sdk-matrix.sh` 为集成验收 runner，按语言执行 L1/L3 契约测试、错误闭集和混合版本用例；Go/Python 参考插件按实际实现补充。
- 本仓已有矩阵校验只证明 API 表面存在；本页验收链要求调用行为、授权、幂等和结果未知也通过。
