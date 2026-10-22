---
title: 游戏接入与兼容性验收
---

# 游戏接入与兼容性验收

## 状态与范围

- 状态：规范待评审；`docs/sdks/sdk-parity-matrix.md` 是跨语言接入矩阵，`sdks/SDK_FEATURE_MATRIX.md` 仍是实现状态主表。
- 本页定义“游戏业务已接入”的判定标准：不是注册元数据、不是编译通过、也不是 README 声明，而是查询与受控写完整链路验收通过。

## 适用路径

| 路径           | 适用场景                             | 最小要求                                         |
| -------------- | ------------------------------------ | ------------------------------------------------ |
| SDK Provider   | 游戏服务内注册函数                   | L1 必备能力 + 输入/输出 schema + scope 授权。    |
| OpenAPI        | 既有 REST 网关低成本接入             | OpenAPI 3、稳定 operationId、schema 与实现一致。 |
| 外部函数插件   | 非游戏进程、多语言或独立生命周期业务 | manifest、权限、进程监督、超时取消和崩溃恢复。   |
| Server Invoker | 运营控制台、自动化或外部系统调用     | Server HTTP、任务状态/取消/事件、授权与幂等。    |
| PageSpec       | 运营页面编排                         | 发布快照、审批策略、schema 摘要和 renderer ABI。 |

能力缺失必须返回 `CAPABILITY_UNSUPPORTED`；页面存在、菜单存在或 Descriptor 存在都不等于业务接入。

## 最小能力

1. 身份：`gameId/env` 非空，`agentId/serviceId/sdkVersion` 与实际一致；跨 scope 会话或函数引用失败。
2. 查询：至少一个稳定只读函数，schema 可校验，错误定位到字段，无权限时拒绝。
3. 受控写：使用 `operationId` + 函数契约指纹；高风险函数先审批后执行。
4. 幂等：游戏侧持久化 `(scope, operationId, requestFingerprint)`；同 ID 重放返回原结果或凭据。
5. 结果未知：超时/断线只能按原 `operationId` 查询；禁止新的随机重试键产生副作用。
6. 断线与取消：SDK/插件支持重连、drain 在途收尾、任务取消和终态查询。
7. 探测：只读健康/业务探针可独立失败；探测不可用不得替代业务验证。
8. 可观测：注册、调用、审批、任务、探测和断线产生可关联 ID 与错误分类。

## 兼容矩阵

| 组件      | 版本/契约门槛                                 | 未满足动作                       |
| --------- | --------------------------------------------- | -------------------------------- |
| 平台 wire | 协议版本与能力闭集                            | 拒绝连接或声明能力缺失。         |
| SDK       | `sdkVersion` 低于配置下限拒绝；低于高水位告警 | 升级 SDK。                       |
| Agent     | `agentVersion`、承载能力、wire 版本           | 禁用不满足的目标并给出升级建议。 |
| Function  | 函数版本、schema 摘要、审批策略               | 低版本可配置拒绝；旧绑定 stale。 |
| 插件      | manifest、依赖、ABI/IPC 版本、权限            | 拒绝安装或启用。                 |
| OpenAPI   | OpenAPI 3、稳定 `operationId`、schema         | 导入失败，保留旧契约。           |
| PageSpec  | renderer ABI、binding 快照、审批策略          | 阻断发布或执行。                 |

升级顺序为协议/平台 → Server → Agent → SDK/插件 → PageSpec。混合窗口只允许追加字段；删除或改语义必须双版本过渡。

## 接入步骤

1. 选择路径：游戏内函数走 SDK；既有 REST 走 OpenAPI；独立进程/多语言走插件。
2. 声明身份与能力：scope、服务名、语言版本、能力闭集和缺失能力的 unsupported 语义。
3. 定义契约：稳定函数 ID、schema、风险、权限、审批与幂等要求。
4. 导入参考实现：至少覆盖 `examples/openapi-provider` 的查询、创建、更新、删除和 `player.kick`。
5. 接入页面：PageSpec 绑定函数契约，高风险操作开启审批与确认文案。
6. 跑验收工具：先 API/矩阵静态门禁，再启动真链路跑 `docs/sdks/sdk-parity-matrix.md` 九条接入链。
7. 出验收报告：记录版本、契约指纹、用例结果、已知 unsupported 和性能跳转数。

## 验收场景

1. 参考游戏查询与受控写：`player.list/player.get` 查询通过，`player.create/player.update/player.kick` 在授权、审批和幂等规则下执行。
2. 同 ID 多端注册：不同 SDK 版本可共存；低于下限的实例被拒，低于高水位的实例告警。
3. 旧契约：修改函数 schema 后旧 PageSpec 绑定判定 stale，执行阻断。
4. 错 scope：用 `game-a/dev` 的会话调用 `game-b/prod` 目标，返回 `SCOPE_MISMATCH` 并留审计。
5. 不支持能力：请求未实现插件或探测，返回 `CAPABILITY_UNSUPPORTED`，不出现空成功。
6. 结果未知：HTTP 响应丢失后同 `operationId` 查询收敛原凭据；替代重试被拒绝。
7. 断线恢复：provider 重连后目录一致，drain 后新请求拒绝、在途请求有终态。
8. 任务取消：cancel 后函数停止副作用，事件显示取消原因；结果可查询。
9. 探测故障：端口可达但业务函数失败时，集成验收不通过。
10. 可观测：SDK 路径与外部 OpenAPI 路径的耗时、跳转和失败分类都能对比。
