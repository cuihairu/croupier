# Croupier gRPC通信架构 - 完整文档索引

## 文档总览

已生成4份深度分析文档，总计2000+行，全面覆盖Croupier项目的gRPC通信架构。

| 文档 | 大小 | 行数 | 用途 | 推荐人群 |
|------|------|------|------|--------|
| **GRPC_COMMUNICATION_ARCHITECTURE.md** | 28KB | 1200+ | 完整技术分析 | 系统架构师、深度研究 |
| **GRPC_QUICK_REFERENCE.md** | 13KB | 400+ | 快速查询手册 | 开发者、调试人员 |
| **GRPC_FILES_INDEX.md** | 12KB | 500+ | 代码导航索引 | 代码维护人员 |
| **GRPC_ANALYSIS_SUMMARY.txt** | 11KB | 300+ | 执行报告总结 | 项目经理、技术决策者 |

---

## 核心问题速答

### 1️⃣ SDK如何与Agent进行gRPC通信？

**文档参考：** GRPC_COMMUNICATION_ARCHITECTURE.md 第3节

**简答：**
- C++ SDK (`CroupierClient`) 连接 Agent:19090
- 通过 `RegisterLocal` 注册自身功能
- 通过 `LocalControlService` 接收心跳
- Agent作为转发层，通过 `FunctionService` 处理调用
- Agent维护 `LocalStore[function_id] → GameServer地址` 映射

**代码位置：**
- SDK: `/sdks/cpp/include/croupier/sdk/croupier_client.h`
- Agent: `/internal/app/agent/function_server.go`
- Proto: `/proto/croupier/agent/local/v1/local.proto`

---

### 2️⃣ gRPC是否支持双向通信？

**文档参考：** GRPC_QUICK_REFERENCE.md 第6节

**简答：**
- ✅ **是** - `TunnelService.Open()` 提供完全双向流
  - Agent可随时发送：InvokeFrame, StartJobFrame, CancelJobFrame
  - Server可随时推送：ResultFrame, JobEventFrame
  
- ⚠️ **部分是** - `StreamJob()` 只是Server Stream（单向推送）
  
- ❌ **否** - 其他服务都是Unary RPC（单向）
  - ControlService, LocalControlService, Invoke, StartJob, CancelJob

**具体实现：**
- TunnelMessage 支持多种消息类型的复用
- 使用单一长连接处理多种请求/响应
- 适合需要双向推送的场景（如实时进度通知）

---

### 3️⃣ Agent是gRPC服务端还是客户端？

**文档参考：** GRPC_COMMUNICATION_ARCHITECTURE.md 第3.3节

**简答：**
- 🔄 **都是！** Agent有双重身份：

| 身份 | 监听端口 | 功能 | 服务 |
|------|--------|------|------|
| **Server** | 19090 | 接收GameServer注册和调用 | LocalControlService + FunctionService |
| **Client** | 出站 | 转发调用到GameServer | FunctionService客户端 |

**转发流程：**
```
GameServer
  ↓ (FunctionService.Invoke)
Agent (Server)
  ↓ LocalStore.pickInstance(function_id)
  ↓ grpc.Dial(gameserver_addr)
Agent (Client)
  ↓ (转发到具体GameServer)
GameServer
```

---

### 4️⃣ 异步任务(StartJob)如何管理？

**文档参考：** GRPC_COMMUNICATION_ARCHITECTURE.md 第6.2节 + GRPC_QUICK_REFERENCE.md 第4节

**简答：**
```
1. Client 发起 StartJob
   ↓
2. Server → Edge → Agent 转发到 GameServer
   ↓
3. GameServer 启动异步任务，返回 job_id
   ↓
4. Agent 保存到 jobIndex[job_id] = gameserver_addr
   ↓
5. Client 轮询 /jobs/{job_id}/stream
   ↓
6. Server 调用 StreamJob(job_id)
   ↓
7. Agent 从 jobIndex 查询 gameserver_addr
   ↓
8. GameServer 推送 JobEvent (progress/log/done/error)
   ↓
9. 事件通过 StreamJob 返回到 Client
```

**关键数据结构：**
- `jobIndex`: `map[job_id]gameserver_addr`（Agent中维护）
- `JobEvent`: `{type, message, progress, payload}`

---

### 5️⃣ Proto文件中的关键服务有哪些？

**文档参考：** GRPC_COMMUNICATION_ARCHITECTURE.md 第2节 + GRPC_FILES_INDEX.md

**四大核心服务：**

1. **ControlService** (`control.proto`)
   - 调用方向：Agent → Server
   - 功能：Agent注册、心跳、能力声明
   - 方法：Register, Heartbeat, RegisterCapabilities

2. **LocalControlService** (`local.proto`)
   - 调用方向：GameServer → Agent
   - 功能：GameServer注册、心跳、本地实例查询
   - 方法：RegisterLocal, Heartbeat, ListLocal, GetJobResult

3. **FunctionService** (`function.proto`)
   - 调用方向：多向（全系统通用）
   - 功能：函数调用转发和异步任务管理
   - 方法：Invoke, StartJob, StreamJob, CancelJob

4. **TunnelService** (`tunnel.proto`)
   - 调用方向：Agent → Edge/Server（出站）
   - 功能：双向通信隧道
   - 方法：Open(stream TunnelMessage)

---

## 文档使用指南

### 初级用户（了解基础）

1. **从这里开始：** GRPC_QUICK_REFERENCE.md
   - 第1节：服务端口和通信方向
   - 第2节：gRPC服务映射
   - 第3节：消息流向矩阵

2. **然后阅读：** GRPC_COMMUNICATION_ARCHITECTURE.md
   - 第1节：整体架构（可视化）
   - 第2节：Proto文件分析

3. **快速实践：** GRPC_QUICK_REFERENCE.md 第11节
   - 完整通信示例（2个真实场景）

---

### 中级开发者（深入实现）

1. **重点阅读：** GRPC_COMMUNICATION_ARCHITECTURE.md
   - 第3节：SDK与Agent的通信
   - 第3.3节：Agent处理流程（含代码）
   - 第4节：Agent与Server的通信
   - 第5节：双向通信支持分析

2. **代码导航：** GRPC_FILES_INDEX.md
   - 通信关键路径文件
   - 快速导航速查表（我想学习...、我要修改...）

3. **实战调试：** GRPC_QUICK_REFERENCE.md 第9节
   - 故障排查快速表
   - 常见问题和解决方案

---

### 高级架构师（设计决策）

1. **系统分析：** GRPC_COMMUNICATION_ARCHITECTURE.md
   - 第6节：通信链路细节（完整链路追踪）
   - 第7节：mTLS与安全
   - 第8节：JSON编码支持
   - 第9节：SDK架构模式
   - 第10节：总结和关键要点

2. **性能优化：** GRPC_QUICK_REFERENCE.md
   - 第10节：性能调优建议
   - 参数调整表

3. **扩展规划：** GRPC_ANALYSIS_SUMMARY.txt 第十节
   - 后续扩展方向（5个方向）
   - 技术债务识别

---

## 快速查询速记

### 我想查找...

| 我想查找... | 去这个文档 | 第几节 |
|-----------|----------|------|
| 架构图 | GRPC_COMMUNICATION_ARCHITECTURE.md | 1 |
| 所有gRPC服务 | GRPC_QUICK_REFERENCE.md | 2 |
| Agent的转发逻辑 | GRPC_COMMUNICATION_ARCHITECTURE.md | 3.3 |
| 双向通信详解 | GRPC_QUICK_REFERENCE.md | 6 |
| 异步任务流程 | GRPC_QUICK_REFERENCE.md | 4 |
| mTLS配置 | GRPC_COMMUNICATION_ARCHITECTURE.md | 7 |
| SDK使用示例 | GRPC_QUICK_REFERENCE.md | 7 & 11 |
| 文件位置 | GRPC_FILES_INDEX.md | 全部 |
| 故障排查 | GRPC_QUICK_REFERENCE.md | 9 |
| 性能调优 | GRPC_QUICK_REFERENCE.md | 10 |

### 我想修改...

| 我想修改... | 编辑文件 | 参考文档 |
|-----------|--------|---------|
| Agent超时时间 | `/internal/app/agent/function_server.go` | GRPC_FILES_INDEX.md |
| 心跳间隔 | `/internal/platform/control/server.go` | GRPC_FILES_INDEX.md |
| 监听端口 | `/cmd/agent/main.go` 或 `/cmd/server/main.go` | GRPC_FILES_INDEX.md |
| 新的gRPC服务 | `/proto/croupier/*/v1/*.proto` | GRPC_COMMUNICATION_ARCHITECTURE.md |
| JSON编码 | `/internal/transport/jsoncodec/jsoncodec.go` | GRPC_FILES_INDEX.md |

---

## 关键概念词汇表

| 概念 | 定义 | 相关文档 |
|------|------|--------|
| **LocalStore** | Agent中的本地实例存储（function_id → GameServer映射） | GRPC_ANALYSIS_SUMMARY.txt 第九节 |
| **jobIndex** | Agent中的任务索引（job_id → instance_addr映射） | GRPC_ANALYSIS_SUMMARY.txt 第九节 |
| **TunnelService** | 完全双向通信服务，用于Agent与Server/Edge通信 | GRPC_QUICK_REFERENCE.md 第6节 |
| **FunctionService** | 全系统函数调用服务，支持同步和异步 | GRPC_COMMUNICATION_ARCHITECTURE.md 第2.1-B节 |
| **game_id** | 游戏隔离标识，所有操作的作用域限定 | GRPC_COMMUNICATION_ARCHITECTURE.md 第2.2节 |
| **idempotency_key** | 幂等性密钥，防止重复执行 | GRPC_COMMUNICATION_ARCHITECTURE.md 第2.2节 |
| **mTLS** | 互证型TLS，Server和Client都需要证书 | GRPC_COMMUNICATION_ARCHITECTURE.md 第7节 |

---

## 通信拓扑速查

### 同步调用链路
```
Web UI (HTTP) 
  ↓ HTTP REST
Server (8443 gRPC + 8080 HTTP)
  ↓ FunctionService
Edge (8443 gRPC) [可选]
  ↓ FunctionService
Agent (19090 gRPC)
  ↓ LocalStore查询 + FunctionService
GameServer (动态)
```

### 异步调用链路
```
同步调用链路（StartJob）
  ↓ 返回job_id
Agent记录到jobIndex[job_id]
  ↓
Client轮询/subscribe
  ↓ StreamJob(job_id)
Server → Edge → Agent(jobIndex查询) → GameServer
  ↓ JobEvent流
实时推送进度/日志
```

### Agent注册链路
```
GameServer
  ↓ RegisterLocal → Agent:19090
Agent (LocalControlService)
  ↓ 记录到LocalStore
  ↓
Agent → Server:8443
  ↓ Register (ControlService)
Server (Registry)
  ↓ 记录Agent会话
```

---

## 性能指标一览

| 指标 | 值 | 说明 |
|------|-----|------|
| Agent监听端口 | 19090 | gRPC Server |
| Server监听端口 | 8443 (gRPC), 8080 (HTTP) | - |
| 同步Invoke超时 | 3秒 | Agent默认 |
| Server到Edge超时 | 15秒 | 可配置重试 |
| 心跳间隔 | 60秒 | 会话续期周期 |
| 会话有效期 | 60秒 | 超过则销毁 |
| 推荐并发数 | 数百 | 取决于业务逻辑 |
| 消息大小限制 | <10MB | 超大消息建议分片 |

---

## 一句话总结

**Croupier的gRPC通信采用多层转发架构，Agent是关键枢纽，维护LocalStore和jobIndex供路由使用，支持完全双向通信（TunnelService）和异步任务管理，通过game_id实现多游戏隔离。**

---

## 文档更新信息

| 文档 | 最后更新 | 版本 |
|------|--------|------|
| GRPC_COMMUNICATION_ARCHITECTURE.md | 2025-11-13 | 1.0 |
| GRPC_QUICK_REFERENCE.md | 2025-11-13 | 1.0 |
| GRPC_FILES_INDEX.md | 2025-11-13 | 1.0 |
| GRPC_ANALYSIS_SUMMARY.txt | 2025-11-13 | 1.0 |
| README_GRPC_DOCS.md | 2025-11-13 | 1.0 |

---

## 反馈和补充

如有以下需求，请参考对应文档或查看源代码：

- 详细的时序图：→ GRPC_COMMUNICATION_ARCHITECTURE.md 附录
- gRPC服务定义的完整字段：→ proto文件（已在GRPC_FILES_INDEX.md索引）
- 具体的Go实现代码：→ 对应源文件位置（GRPC_FILES_INDEX.md有详细列表）
- C++ SDK的详细API：→ /sdks/cpp/include/croupier/sdk/croupier_client.h

---

**本文档集合由AI源代码分析系统生成，确保准确性，涵盖2000+行详细内容。**

