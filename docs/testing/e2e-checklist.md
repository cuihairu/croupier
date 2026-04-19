# 函数注册到 Dashboard 展示 - 端到端验收清单

本文档给出当前 session 设计下的联调验收清单，覆盖 `SDK -> Agent -> Server -> Dashboard` 主路径。

## 前置条件

- [ ] Server 已启动，REST 端口默认 `18780`
- [ ] Agent 已启动，并已连接到 Server 的 session/control 端口 `19090`
- [ ] Agent 本地 gateway 已监听，默认 `19091`
- [ ] Dashboard 已启动（默认 `8000`）
- [ ] 已准备一个测试 provider / SDK

## 基础健康检查

```bash
# Server 健康状态
curl http://localhost:18780/healthz

# Dashboard
curl http://localhost:8000

# Agent 是否已出现在 Server 视图
curl http://localhost:18780/api/v1/agents
```

## 阶段一：SDK -> Agent 建立 provider session

### 验收目标

- SDK 主动连接 Agent `19091`
- 首帧为 `ProviderConnectRequest`
- Agent 返回 `session_id`
- 函数描述符被 Agent 接受

### 检查项

| 步骤 | 预期结果 |
| --- | --- |
| SDK 建连到 Agent 本地 gateway | 连接成功 |
| Agent 识别首帧 | `ProviderConnectRequest` 合法 |
| Agent 返回响应 | 收到 `ProviderConnectResponse(session_id)` |
| SDK 启动心跳 | 心跳持续成功 |

### 反向校验

- [ ] SDK 没有开启本地监听端口
- [ ] 配置中没有 `rpc_addr`
- [ ] 没有使用历史 `LocalControlService`

## 阶段二：Agent -> Server 同步函数摘要

### 验收目标

- Agent 将 provider session 摘要同步到 Server
- Server Registry 中可见新函数
- Dashboard 可读取描述符

### 检查项

| 步骤 | 预期结果 |
| --- | --- |
| Agent 与 Server session 正常 | 心跳正常 |
| Server 收到函数摘要 | Registry 更新成功 |
| 查询函数列表 | 新函数可见 |

### 验证命令

```bash
curl http://localhost:18780/api/v1/functions
curl http://localhost:18780/api/v1/functions/descriptors
```

## 阶段三：Dashboard 展示

### 验收目标

- 菜单动态渲染正确
- 函数详情页可见 schema 与元数据

### UI 检查项

- [ ] 函数出现在正确的菜单分组
- [ ] 名称、描述、分类展示正确
- [ ] `input_schema` / `output_schema` 正常渲染
- [ ] 风险级别、标签等元数据展示正确

## 阶段四：调用链路

### 验收目标

- Server / Dashboard 发起调用后，请求经 Agent 下发给 SDK
- SDK 在同一条 provider session 上返回结果

### 检查项

| 步骤 | 预期结果 |
| --- | --- |
| 发起 `InvokeRequest` | SDK handler 被命中 |
| 返回 `InvokeResponse` | 调用成功 |
| 发起 `StartTaskRequest` | 任务创建成功 |
| 收到 `TaskEvent` | 事件流正常 |

## 阶段五：重连与摘流

### 验收目标

- 断线后自动重连
- 旧 session 失效
- 新 session 重新注册
- drain 时不再分配新请求

### 检查项

- [ ] 停掉 Agent 或 SDK 后，另一端能感知断线
- [ ] 重连使用指数退避
- [ ] 达到上限后进入固定廉价重试周期
- [ ] 重连成功后拿到新的 `session_id`
- [ ] `ProviderDrainRequest` 生效后不再接新流量

## 阶段六：TLS 变体

### 验收目标

- `Agent <-> Server` 可启用 TLS / mTLS
- `SDK <-> Agent` 默认可明文，也可按需启用 TLS

### 检查项

- [ ] 内网明文场景可正常工作
- [ ] 启用 TLS 后证书校验正常
- [ ] 启用 mTLS 后双向证书校验正常

## 失败判定

出现以下任一情况，应视为未通过：

- SDK 仍要求本地监听端口
- 文档或实现仍依赖 `rpc_addr`
- 首帧不是 `ProviderConnectRequest`
- 断线后复用旧 `session_id`
- 业务 payload 默认不是 JSON
- Dashboard 仍依赖历史 `gRPC` / `旧传输` 描述理解链路
