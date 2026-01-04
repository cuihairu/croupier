# Croupier C++ SDK 目录索引与文件映射

## 📦 完整目录结构

```
sdks/cpp/
│
├── 📄 CMakeLists.txt (355 行)
│   ├─ 项目配置：C++17, vcpkg 集成
│   ├─ 多平台支持：Windows/Linux/macOS (x64/x86/arm64)
│   ├─ 依赖配置：gRPC, Protobuf, nlohmann-json
│   ├─ 库目标：shared + static 并行构建
│   ├─ 示例程序：croupier-example, virtual-object-demo
│   └─ 安装配置：CMake config + CPack 打包
│
├── 📂 include/croupier/sdk/
│   └─ croupier_client.h (270 行) ⭐ 核心公开接口
│       ├─ FunctionHandler 类型定义 (L:18)
│       ├─ FunctionDescriptor 结构 (L:21-25)
│       ├─ VirtualObjectDescriptor 结构 (L:35-43)
│       ├─ RelationshipDef 结构 (L:28-32)
│       ├─ ComponentDescriptor 结构 (L:46-55)
│       ├─ ClientConfig 结构 (L:57-83) 🎮 game_id/env
│       ├─ InvokerConfig 结构 (L:86-106) 🎮 game_id/env
│       ├─ InvokeOptions 结构 (L:109-116)
│       ├─ JobEvent 结构 (L:119-125)
│       ├─ CroupierClient 类 (L:128-186)
│       │  ├─ RegisterFunction() (L:136)
│       │  ├─ RegisterVirtualObject() (L:141-144) ⭐
│       │  ├─ RegisterComponent() (L:147)
│       │  ├─ LoadComponentFromFile() (L:150)
│       │  ├─ GetRegisteredObjects() (L:155)
│       │  ├─ GetRegisteredComponents() (L:158)
│       │  ├─ UnregisterVirtualObject() (L:161)
│       │  ├─ UnregisterComponent() (L:164)
│       │  ├─ Connect() (L:169)
│       │  ├─ Serve() (L:172)
│       │  ├─ Stop() (L:175)
│       │  ├─ Close() (L:178)
│       │  └─ GetLocalAddress() (L:181)
│       ├─ CroupierInvoker 类 (L:189-220)
│       │  ├─ Connect() (L:195)
│       │  ├─ Invoke() (L:198-199)
│       │  ├─ StartJob() (L:202-203)
│       │  ├─ StreamJob() (L:206)
│       │  ├─ CancelJob() (L:209)
│       │  ├─ SetSchema() (L:212)
│       │  └─ Close() (L:215)
│       └─ utils 命名空间 (L:223-267) 🛠️ 工具函数
│          ├─ NewIdempotencyKey()
│          ├─ ValidateJSON()
│          ├─ ParseJSON()
│          ├─ ToJSON()
│          ├─ LoadObjectDescriptor() ⭐
│          ├─ LoadComponentDescriptor() ⭐
│          ├─ ValidateObjectDescriptor() ⭐
│          ├─ ValidateComponentDescriptor() ⭐
│          ├─ GenerateObjectTemplate()
│          ├─ GenerateComponentTemplate()
│          ├─ ObjectDescriptorToJSON()
│          └─ ComponentDescriptorToJSON()
│
├── 📂 src/
│   └─ croupier_client.cpp (898 行) ⭐ 核心实现
│       ├─ Utils 工具函数实现 (L:15-99)
│       │  ├─ NewIdempotencyKey() 生成 UUID (L:17-27)
│       │  ├─ ValidateJSON() JSON 语法验证 (L:29-65)
│       │  ├─ ParseJSON() JSON 解析 (L:67-85)
│       │  └─ ToJSON() JSON 序列化 (L:87-98)
│       │
│       ├─ CroupierClient::Impl 类实现 (L:102-407) ⭐
│       │  ├─ 成员变量 (L:104-115)
│       │  │  ├─ config_ 配置存储
│       │  │  ├─ handlers_ 函数映射表
│       │  │  ├─ descriptors_ 元数据
│       │  │  ├─ objects_ 虚拟对象表
│       │  │  └─ components_ 组件表
│       │  │
│       │  ├─ Impl() 构造函数 (L:117-131) 🎮 game_id/env 验证
│       │  │  ├─ game_id 空检查 (L:119-121)
│       │  │  ├─ env 有效性验证 (L:123-127)
│       │  │  └─ 日志记录 (L:129-130)
│       │  │
│       │  ├─ RegisterFunction() (L:137-148)
│       │  ├─ RegisterVirtualObject() (L:151-193) ⭐
│       │  ├─ RegisterComponent() (L:196-234) ⭐
│       │  ├─ LoadComponentFromFile() (L:237-246)
│       │  ├─ GetRegisteredObjects() (L:249-255)
│       │  ├─ GetRegisteredComponents() (L:258-264)
│       │  ├─ UnregisterVirtualObject() (L:267-287)
│       │  ├─ UnregisterComponent() (L:290-315)
│       │  ├─ Connect() (L:317-337) 📡 Agent 连接
│       │  ├─ Serve() (L:339-353) 🔄 主服务循环
│       │  ├─ Stop() (L:355-364)
│       │  ├─ Close() (L:366-370)
│       │  ├─ GetLocalAddress() (L:372-374)
│       │  └─ StartLocalServer() (L:377-406) 🖧 本地 gRPC
│       │
│       ├─ CroupierInvoker::Impl 类实现 (L:410-543)
│       │  ├─ Connect() (L:418-429) 📡 连接
│       │  ├─ Invoke() (L:431-455) 📨 同步调用
│       │  ├─ StartJob() (L:457-472) 🚀 异步任务
│       │  ├─ StreamJob() (L:474-517) 📊 流式传输
│       │  ├─ CancelJob() (L:519-531) ⏹️ 取消任务
│       │  ├─ SetSchema() (L:533-536)
│       │  └─ Close() (L:538-542)
│       │
│       ├─ CroupierClient 公开接口转发 (L:546-609)
│       ├─ CroupierInvoker 公开接口转发 (L:611-645)
│       └─ Utils 工具函数实现 (L:648-896)
│          ├─ LoadObjectDescriptor() (L:651-661)
│          ├─ LoadComponentDescriptor() (L:664-674)
│          ├─ ValidateObjectDescriptor() (L:677-718) ⭐ 验证逻辑
│          ├─ ValidateComponentDescriptor() (L:721-750) ⭐
│          ├─ GenerateObjectTemplate() (L:753-777)
│          ├─ GenerateComponentTemplate() (L:780-793)
│          ├─ ParseObjectDescriptor() (L:796-804)
│          ├─ ParseComponentDescriptor() (L:807-815)
│          ├─ ObjectDescriptorToJSON() (L:818-859)
│          └─ ComponentDescriptorToJSON() (L:862-893)
│
├── 📂 examples/
│   └─ virtual_object_demo.cpp (334 行) 📚 完整示例
│       ├─ Wallet 处理器 (L:8-60)
│       │  ├─ WalletGetHandler() (L:10-24)
│       │  ├─ WalletTransferHandler() (L:26-45)
│       │  └─ WalletDepositHandler() (L:47-60)
│       ├─ Currency 处理器 (L:62-93)
│       │  ├─ CurrencyGetHandler() (L:64-78)
│       │  └─ CurrencyCreateHandler() (L:80-92)
│       ├─ Demo 1: 单函数注册 (L:96-119)
│       ├─ Demo 2: 虚拟对象 (L:121-175) ⭐
│       ├─ Demo 3: 组件注册 (L:177-244) ⭐
│       ├─ Demo 4: 模板生成 (L:246-260)
│       ├─ Demo 5: 序列化 (L:262-284)
│       ├─ Demo 6: 验证 (L:286-306)
│       └─ main() 启动 (L:308-334)
│
├── 📂 .github/workflows/
│   └─ cpp-sdk-build.yml (483 行) 🤖 CI/CD 自动化
│       ├─ 版本管理任务 (L:44-118)
│       ├─ 多平台构建矩阵 (L:120-165)
│       ├─ 构建步骤 (L:168-237)
│       ├─ 测试执行 (L:231-236)
│       ├��� 分离打包 (L:239-284)
│       ├─ 发布流程 (L:303-462)
│       └─ 通知系统 (L:465-483)
│
├── 📄 vcpkg.json (40 行) 📦 依赖声明
│   ├─ grpc (含 codegen)
│   ├─ protobuf (含 zlib)
│   ├─ nlohmann-json
│   └─ gtest (可选)
│
├── 📄 README.md (569 行) 📖 用户文档
│   ├─ 核心特性说明
│   ├─ 快速开始指南
│   ├─ 使用示例 (4 个)
│   ├─ 架构设计说明
│   ├─ API 参考
│   ├─ 部署和分发
│   ├─ 开发环境搭建
│   ├─ 进阶主题
│   └─ 贡献指南
│
└─ VIRTUAL_OBJECT_REGISTRATION.md (441 行) 🏗️ 架构深度文档
    ├─ 四层抽象模型
    ├─ 设计理念说明
    ├─ C++ SDK 扩展方案
    ├─ 4 个使用示例
    ├─ 实现指南
    ├─ 架构优势分析
    └─ 后续规划
```

---

## 🎯 按功能查找代码

### 🔴 SPI (Service Provider Interface)

| 功能 | 文件 | 行号 |
|------|------|------|
| FunctionHandler 类型 | croupier_client.h | 18 |
| RegisterFunction() | croupier_client.h | 136 |
| RegisterVirtualObject() | croupier_client.h | 141-144 |
| RegisterComponent() | croupier_client.h | 147 |
| Handler 执行 | croupier_client.cpp | 431-455 |
| Handler 存储 | croupier_client.cpp | 104-106 |

**关键代码片段**：
```cpp
// 定义 (line 18)
using FunctionHandler = std::function<std::string(const std::string&, const std::string&)>;

// 注册 (line 136)
bool RegisterFunction(const FunctionDescriptor& desc, FunctionHandler handler);

// 调用 (line 431-455)
std::string Invoke(...) {
    auto handler = schemas_[function_id];
    return handler(context, payload);
}
```

---

### 🟢 game_id 和 env

| 功能 | 文件 | 行号 |
|------|------|------|
| ClientConfig 定义 | croupier_client.h | 57-83 |
| game_id/env 字段 | croupier_client.h | 65-66, 90-91 |
| 初始化验证 | croupier_client.cpp | 117-131 |
| 日志记录 | croupier_client.cpp | 129-130 |
| Proto 定义 | control.proto | 22-23 |

**关键代码片段**：
```cpp
// 配置 (lines 65-66, 90-91)
struct ClientConfig {
    std::string game_id;           // Required
    std::string env = "development";
};

// 验证 (lines 117-131)
if (config_.game_id.empty()) {
    std::cerr << "Warning: game_id is required for proper backend separation" << std::endl;
}
if (config_.env != "development" && config_.env != "staging" && config_.env != "production") {
    std::cerr << "Warning: Unknown environment '" << config_.env << "'" << std::endl;
}
```

---

### 🔵 与 Agent 交互

| 功能 | 文件 | 行号 |
|------|------|------|
| Connect() | croupier_client.h | 169 |
| Connect() 实现 | croupier_client.cpp | 317-337 |
| StartLocalServer() | croupier_client.cpp | 377-406 |
| Serve() | croupier_client.h | 172 |
| Heartbeat | croupier_client.cpp | （待实现）|
| Proto 协议 | local.proto | 1-40 |

**关键代码片段**：
```cpp
// 步骤1：连接 Agent (line 317)
bool Connect() {
    // 连接到 agent_addr (127.0.0.1:19090)
    // 调用 LocalControlService::RegisterLocal()

    // 步骤2：启动本地服务器
    if (!StartLocalServer()) { /* ... */ }

    // 步骤3：注册会话
    // NOTE: Agent gRPC 注册功能待实现
}

// 步骤2：本地服务器 (line 377)
bool StartLocalServer() {
    // 解析 local_listen 配置
    // 分配端口（port=0 时自动分配）
    // 保存 local_address_
    return true;
}
```

**Proto 消息** (local.proto):
```protobuf
message RegisterLocalRequest {
    string service_id = 1;
    string version = 2;
    string rpc_addr = 3;                    // 本地服务地址
    repeated LocalFunctionDescriptor functions = 4;
}

message RegisterLocalResponse {
    string session_id = 1;  // 后续用于心跳
}
```

---

### 🟣 权限接口

| 功能 | 文件 | 行号 |
|------|------|------|
| auth_token | croupier_client.h | 77 |
| TLS 配置 | croupier_client.h | 70-74 |
| InvokeOptions | croupier_client.h | 109-116 |
| metadata | croupier_client.h | 115 |
| 认证示例 | README.md | 490-506 |

**关键代码片段**：
```cpp
// 认证配置 (lines 70-78)
struct ClientConfig {
    // ========== Authentication ==========
    std::string auth_token;
    std::map<std::string, std::string> headers;
    
    // ========== TLS Configuration ==========
    bool insecure = true;
    std::string cert_file;
    std::string key_file;
    std::string ca_file;
    std::string server_name;
};

// 调用时权限 (lines 109-116)
struct InvokeOptions {
    std::string idempotency_key;
    std::string trace_id;                   // 审计追踪
    std::map<std::string, std::string> metadata;  // 权限元数据
};
```

---

## 📍 关键类和方法定位

### CroupierClient 核心方法

```
文件: croupier_client.h/cpp

CroupierClient
  ├─ 构造函数 (h:130, cpp:546-547)
  ├─ RegisterFunction() (h:136, cpp:552-554)
  ├─ RegisterVirtualObject() (h:141-144, cpp:557-561) ⭐
  ├─ RegisterComponent() (h:147, cpp:563-565) ⭐
  ├─ LoadComponentFromFile() (h:150, cpp:567-569)
  ├─ GetRegisteredObjects() (h:155, cpp:572-574)
  ├─ GetRegisteredComponents() (h:158, cpp:576-578)
  ├─ UnregisterVirtualObject() (h:161, cpp:580-582)
  ├─ UnregisterComponent() (h:164, cpp:584-586)
  ├─ Connect() (h:169, cpp:590-592)
  ├─ Serve() (h:172, cpp:594-596)
  ���─ Stop() (h:175, cpp:598-600)
  ├─ Close() (h:178, cpp:602-604)
  └─ GetLocalAddress() (h:181, cpp:606-608)
```

### Impl 内部实现

```
文件: croupier_client.cpp

CroupierClient::Impl (L:102-407)
  ├─ 成员变量 (L:104-115)
  ├─ 构造函数 (L:117-131)
  ├─ 虚拟对象注册 (L:151-193)
  ├─ 组件注册 (L:196-234)
  ├─ 连接和服务 (L:317-406)
  └─ Handler 存储和验证
```

---

## 🔧 常用代码片段位置

| 用途 | 文件 | 行号 | 说明 |
|------|------|------|------|
| 生成幂等性 ID | croupier_client.cpp | 17-27 | NewIdempotencyKey() |
| JSON 验证 | croupier_client.cpp | 29-65 | ValidateJSON() |
| JSON 解析 | croupier_client.cpp | 67-85 | ParseJSON() |
| JSON 序列化 | croupier_client.cpp | 87-98 | ToJSON() |
| 对象验证 | croupier_client.cpp | 677-718 | ValidateObjectDescriptor() |
| 组件验证 | croupier_client.cpp | 721-750 | ValidateComponentDescriptor() |
| 模板生成 | croupier_client.cpp | 753-793 | GenerateXxxTemplate() |

---

## 📚 文档导航

### 用户文档
- **README.md**: 快速开始、示例、API 参考
- **VIRTUAL_OBJECT_REGISTRATION.md**: 架构设计、四层模型、DDD 模式

### 技术分析
- **CPP_SDK_DEEP_ANALYSIS.md**: 深度分析（新）
- **CPP_SDK_QUICK_REFERENCE.md**: 快速参考表（新）
- **CPP_SDK_DIRECTORY_INDEX.md**: 本文档（目录索引）

### 示例代码
- **virtual_object_demo.cpp**: 6 个完整演示

### 配置
- **CMakeLists.txt**: 构建系统
- **vcpkg.json**: 依赖管理
- **.github/workflows/cpp-sdk-build.yml**: CI/CD 自动化

---

## 🔍 快速查询表

### "我要..."

| 需求 | 查看 | 关键行号 |
|------|------|--------|
| 注册一个函数 | croupier_client.h | 136 |
| 注册虚拟对象 | croupier_client.h | 141-144 |
| 注册完整组件 | croupier_client.h | 147 |
| 配置 game_id | croupier_client.h | 65-66 |
| 配置生产环境 | croupier_client.h | 70-74 |
| 实现 handler | virtual_object_demo.cpp | 8-93 |
| 验证对象 | croupier_client.cpp | 677-718 |
| 生成模板 | croupier_client.cpp | 753-793 |
| 连接 Agent | croupier_client.cpp | 317-337 |
| 启动服务 | croupier_client.cpp | 339-353 |

