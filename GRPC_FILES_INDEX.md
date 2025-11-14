# Croupier gRPC通信相关文件索引

## Proto定义文件

### 核心服务定义

| 文件路径 | 服务 | 功能 | 关键类型 |
|--------|------|------|--------|
| `/proto/croupier/control/v1/control.proto` | ControlService | Agent向Server注册 | RegisterRequest, RegisterResponse, HeartbeatRequest, RegisterCapabilitiesRequest |
| `/proto/croupier/agent/local/v1/local.proto` | LocalControlService | Game Server向Agent注册 | RegisterLocalRequest, RegisterLocalResponse, ListLocalRequest |
| `/proto/croupier/function/v1/function.proto` | FunctionService | 函数调用（全系统通用） | InvokeRequest, InvokeResponse, StartJobResponse, JobEvent, JobStreamRequest |
| `/proto/croupier/tunnel/v1/tunnel.proto` | TunnelService | Agent与Server双向通信 | TunnelMessage, InvokeFrame, StartJobFrame, ResultFrame, JobEventFrame |
| `/proto/croupier/edge/job/v1/job.proto` | JobService | 任务管理（暂未实现） | - |

### 选项和扩展

| 文件路径 | 内容 | 用途 |
|--------|------|------|
| `/proto/croupier/options/function.proto` | 函数描述选项 | proto中的custom options |
| `/proto/croupier/options/ui.proto` | UI渲染选项 | UI自动生成相关 |

### 示例定义

| 文件路径 | 示例 |
|--------|------|
| `/proto/examples/games/player/v1/player.proto` | 玩家函数示例 |
| `/proto/examples/integrations/prom/v1/prom.proto` | Prometheus集成示例 |

---

## Go实现文件

### Agent实现

| 文件路径 | 核心内容 | 关键功能 |
|--------|---------|--------|
| `/internal/app/agent/app.go` | Agent应用启动 | RegisterGRPC()注册gRPC服务 |
| `/internal/app/agent/function_server.go` | FunctionServer实现 | Invoke/StartJob/StreamJob/CancelJob转发逻辑 |
| `/internal/app/agent/job_index.go` | jobIndex | 维护job_id -> instance_addr映射 |

### Platform服务实现

| 文件路径 | 核心内容 | 关键功能 |
|--------|---------|--------|
| `/internal/platform/control/server.go` | ControlService实现 | Register/Heartbeat/RegisterCapabilities |
| `/internal/platform/agentlocal/local_control.go` | LocalControlService实现 | RegisterLocal/Heartbeat/ListLocal/GetJobResult |
| `/internal/platform/agentlocal/store.go` | LocalStore实现 | 维护function_id -> []instance映射 |
| `/internal/platform/registry/store.go` | Registry存储 | 维护Agent会话、Provider能力 |

### Server实现

| 文件路径 | 核心内容 | 关键功能 |
|--------|---------|--------|
| `/internal/app/server/http/server.go` | HTTP服务器 | HTTP REST API + FunctionInvoker转发 |
| `/cmd/server/main.go` | Server主程序 | grpc.NewServer启动，edgeForwarder创建 |

### Edge实现

| 文件路径 | 核心内容 | 关键功能 |
|--------|---------|--------|
| `/internal/app/edge/app.go` | Edge应用 | TunnelService/FunctionService/JobService注册 |
| `/cmd/edge/main.go` | Edge主程序 | gRPC Server启动监听 |

### 通信工具

| 文件路径 | 核心内容 | 关键功能 |
|--------|---------|--------|
| `/internal/transport/interceptors/client.go` | gRPC Client拦截器 | 重试、超时管理 |
| `/internal/transport/jsoncodec/jsoncodec.go` | JSON编码器 | gRPC支持JSON编码 |
| `/internal/platform/tlsutil/tlsutil.go` | TLS工具 | mTLS证书配置 |

---

## C++ SDK实现

| 文件路径 | 核心内容 | 关键类/函数 |
|--------|---------|-----------|
| `/sdks/cpp/include/croupier/sdk/croupier_client.h` | SDK头文件 | CroupierClient, CroupierInvoker, ClientConfig, InvokerConfig |
| `/sdks/cpp/src/croupier_client.cpp` | SDK实现 | 连接、注册、调用逻辑 |
| `/sdks/cpp/examples/example.cpp` | 基础示例 | 函数注册和远程调用示例 |
| `/sdks/cpp/examples/virtual_object_demo.cpp` | 虚拟对象示例 | 虚拟对象和组件注册示例 |

---

## 命令行程序入口

| 文件 | 功能 | 监听端口 | 启动命令 |
|------|------|--------|---------|
| `/cmd/server/main.go` | Server主程序 | 8443(gRPC), 8080(HTTP) | `./croupier server --config server.yaml` |
| `/cmd/agent/main.go` | Agent主程序 | 19090(gRPC), 8081(HTTP) | `./croupier agent --config agent.yaml` |
| `/cmd/edge/main.go` | Edge主程序 | 8443(gRPC), 8081(HTTP) | `./croupier edge --config edge.yaml` |
| `/cmd/demo/main.go` | Demo程序 | 动态 | 演示游戏服实现示例 |

---

## 配置和示例文件

### 配置模板

| 文件路径 | 用途 |
|--------|------|
| `/configs/server.example.yaml` | Server配置示例 |
| `/configs/agent.example.yaml` | Agent配置示例 |
| `/configs/edge.example.yaml` | Edge配置示例 |

### 文档

| 文件路径 | 内容 |
|--------|------|
| `/CLAUDE.md` | 项目开发指南（本文件）|
| `/README.md` | 项目总体说明 |
| `/ARCHITECTURE.md` | 架构设计文档 |
| `/FUNCTION_ARCHITECTURE.md` | 函数管理架构 |
| `/docs/FUNCTION_MANAGEMENT_ARCHITECTURE_ANALYSIS.md` | 函数管理详细分析 |
| `/docs/providers-manifest.schema.json` | Provider manifest JSON Schema |

---

## 数据模型和类型

### Go类型定义

| 类型 | 定义位置 | 用途 |
|------|---------|------|
| `AgentSession` | `/internal/platform/registry/store.go` | Agent会话信息 |
| `LocalInstance` | `/internal/platform/agentlocal/store.go` | 本地Game Server实例 |
| `LocalStore` | `/internal/platform/agentlocal/store.go` | 本地实例存储 |
| `FunctionServer` | `/internal/app/agent/function_server.go` | Agent的FunctionService实现 |
| `edgeForwarder` | `/cmd/server/main.go` | Server到Edge的转发客户端 |

### C++类型定义

| 类型 | 定义位置 | 用途 |
|------|---------|------|
| `CroupierClient` | `/sdks/cpp/include/croupier/sdk/croupier_client.h` | SDK客户端（提供者） |
| `CroupierInvoker` | `/sdks/cpp/include/croupier/sdk/croupier_client.h` | SDK调用器（消费者） |
| `ClientConfig` | `/sdks/cpp/include/croupier/sdk/croupier_client.h` | SDK连接配置 |
| `InvokerConfig` | `/sdks/cpp/include/croupier/sdk/croupier_client.h` | 调用器配置 |
| `VirtualObjectDescriptor` | `/sdks/cpp/include/croupier/sdk/croupier_client.h` | 虚拟对象定义 |
| `ComponentDescriptor` | `/sdks/cpp/include/croupier/sdk/croupier_client.h` | 组件定义 |

---

## 通信关键路径文件

### 同步调用路径

```
Web UI (HTTP)
  → internal/app/server/http/server.go (routes.go中的/invoke)
    → cmd/server/main.go (edgeForwarder)
      → internal/app/edge/app.go (FunctionService)
        → internal/app/agent/function_server.go (Invoke)
          → Game Server (via grpc.Dial)
```

### 异步调用路径

```
Web UI (HTTP)
  → internal/app/server/http/server.go (routes.go中的/start-job和/stream-job)
    → cmd/server/main.go (edgeForwarder)
      → internal/app/edge/app.go (FunctionService/TunnelService)
        → internal/app/agent/function_server.go (StartJob/StreamJob)
          → internal/app/agent/job_index.go (记录/查询job)
            → Game Server (via grpc.Dial)
```

### Agent注册路径

```
Game Server (SDK)
  → internal/platform/agentlocal/local_control.go (RegisterLocal)
    ← internal/platform/agentlocal/store.go (记录实例)
      
Agent
  → internal/platform/control/server.go (Register via ControlService)
    ← internal/platform/registry/store.go (记录Agent会话)
```

---

## HTTP路由相关文件

### 函数调用路由

| 文件 | 路由 | 方法 | 处理函数 |
|------|------|------|---------|
| `/internal/app/server/http/server.go` | `/games/{gameId}/functions/{functionId}` | POST | httpInvoke |
| `/internal/app/server/http/server.go` | `/games/{gameId}/jobs` | POST | httpStartJob |
| `/internal/app/server/http/server.go` | `/games/{gameId}/jobs/{jobId}` | GET | httpGetJob |
| `/internal/app/server/http/server.go` | `/games/{gameId}/jobs/{jobId}/stream` | GET | httpStreamJob |
| `/internal/app/server/http/server.go` | `/games/{gameId}/jobs/{jobId}` | DELETE | httpCancelJob |

---

## 生成的代码位置

### Protocol Buffers生成文件

```
生成路径：/pkg/pb/

结构：
  pkg/pb/croupier/control/v1/
    ├── control.pb.go
    └── control_grpc.pb.go
  
  pkg/pb/croupier/function/v1/
    ├── function.pb.go
    └── function_grpc.pb.go
  
  pkg/pb/croupier/agent/local/v1/
    ├── local.pb.go
    └── local_grpc.pb.go
  
  pkg/pb/croupier/tunnel/v1/
    ├── tunnel.pb.go
    └── tunnel_grpc.pb.go
```

### 生成命令

```bash
# 生成所有Proto代码
make proto

# 或使用buf
buf generate

# 详见：/buf.gen.yaml 和 /proto/buf.yaml
```

---

## 测试文件

### 单元测试

| 文件 | 测试内容 |
|------|---------|
| `/internal/app/server/http/server_simple_tests.go` | HTTP服务器基础测试 |
| `/internal/app/server/http/server_games_e2e_test.go` | 游戏端对端测试 |
| `/internal/app/server/http/server_new_apis_test.go` | 新API测试 |
| `/internal/app/server/http/server_mask_test.go` | 敏感字段遮蔽测试 |

### Mock实现

```go
// 常见Mock接口
- FunctionInvoker (interface)
  - Invoke()
  - StartJob()
  - StreamJob()
  - CancelJob()

// 文件：internal/app/server/http/server.go
```

---

## 构建和编译相关

| 文件 | 用途 |
|------|------|
| `/Makefile` | 构建自动化 |
| `/cmd/server/main.go` | Server编译入口 |
| `/cmd/agent/main.go` | Agent编译入口 |
| `/cmd/edge/main.go` | Edge编译入口 |
| `/sdks/cpp/CMakeLists.txt` | C++ SDK编译配置 |
| `/sdks/cpp/vcpkg.json` | C++ 依赖管理 |

---

## 性能和监控相关文件

| 文件 | 内容 |
|------|------|
| `/internal/transport/interceptors/client.go` | gRPC拦截器（重试、超时） |
| `/cmd/server/main.go` (edgeForwarder) | 性能指标收集 |
| `/internal/telemetry/` | OpenTelemetry集成 |

---

## 快速导航速查表

### 我想学习...

| 我想学习... | 看这些文件 |
|-----------|----------|
| gRPC服务定义 | `/proto/croupier/*/v1/*.proto` |
| Agent如何转发调用 | `/internal/app/agent/function_server.go` |
| 异步任务如何工作 | `/internal/app/agent/job_index.go` + `local.proto` |
| HTTP REST API | `/internal/app/server/http/server.go` |
| SDK使用 | `/sdks/cpp/include/croupier/sdk/croupier_client.h` + examples |
| 双向通信 | `/proto/croupier/tunnel/v1/tunnel.proto` |
| 多游戏隔离 | 搜索 "game_id" 在 `control.proto` 和 `local.proto` |
| 安全认证 | `/internal/platform/tlsutil/` |

### 我要修改...

| 我要修改... | 编辑这些文件 |
|-----------|----------|
| Agent的超时时间 | `/internal/app/agent/function_server.go` (3*time.Second) |
| 心跳间隔 | `/internal/platform/control/server.go` (60 * time.Second) |
| Agent监听端口 | `/cmd/agent/main.go` (viper "local_addr") |
| Server监听端口 | `/cmd/server/main.go` (viper "server_grpc_addr") |
| gRPC编码格式 | `/internal/transport/jsoncodec/jsoncodec.go` |
| 新的gRPC服务 | 在 `/proto/croupier/*/v1/*.proto` 中定义，然后 `make proto` |

### 调试技巧

| 问题 | 调试方法 |
|------|--------|
| Agent无法注册到Server | 查看 Agent 日志是否有 Register 错误；检查 control.proto 中 RegisterRequest |
| 函数调用失败 | 检查 Agent 的 LocalStore 是否有此函数；查看 function_server.go 的 pickInstance |
| StreamJob 收不到事件 | 检查 jobIndex 是否在 StartJob 时被填充；查看 agent/job_index.go |
| gRPC连接经常断开 | 增加心跳间隔或检查网络；查看 control.proto 的 Heartbeat |
| TLS握手失败 | 检查证书配置；查看 platform/tlsutil/tlsutil.go |

---

## 生成这些文档的数据来源

本系列文档基于以下源代码分析生成：

- **主要分析工具**：直接代码阅读和结构化分析
- **分析深度**：完整遍历了 `proto/`, `internal/`, `cmd/`, `sdks/` 目录
- **验证方法**：通过cross-reference验证通信路径的完整性
- **更新时间**：2025年11月13日

