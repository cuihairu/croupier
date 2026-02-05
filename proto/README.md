# Proto Definitions

## 目录结构

```
proto/
├── component/                     # 组件定义元数据（非通信协议）
│   └── v1/
│       ├── function_options.proto   # 方法级 protobuf 扩展选项
│       ├── ui_options.proto         # 字段级 protobuf 扩展选项
│       └── dashboard_ui.proto       # UI/i18n/菜单/权限定义
│
├── sdk/                           # Game SDK → Agent 通信协议
│   └── v1/
│       ├── provider.proto          # 函数注册（游戏服务器向 Agent 注册函数）
│       └── invocation.proto        # 函数调用（调用者请求执行函数）
│
├── agent/                         # Agent → Server 通信协议
│   └── v1/
│       ├── register.proto          # Agent 注册
│       ├── ops.proto               # 运维操作
│       └── job.proto               # 任务类型（JobStatus, JobEvent）
│
├── external/                      # Server → 第三方服务通信
│   └── v1/
│       └── platform.proto         # 第三方平台调用统一接口
│
└── examples/                      # 函数定义示例
    ├── games/                     # 游戏相关示例
    └── integrations/              # 第三方集成示例
```

## 通信场景

### component/ - 组件元数据
**不是通信协议**，用于定义函数时的注解/装饰器。

| 文件 | 用途 |
|-----|------|
| `function_options.proto` | 方法级元信息：function_id, risk, timeout, two_person_rule 等 |
| `ui_options.proto` | 字段级 UI 元信息：widget, label, placeholder, sensitive 等 |
| `dashboard_ui.proto` | 共享 UI 类型：I18nText, Menu, PermissionSpec |

**使用示例**：
```proto
import "croupier/component/v1/function_options.proto";
import "croupier/component/v1/ui_options.proto";

service PlayerService {
  rpc Ban(BanRequest) returns (BanResponse) {
    option (croupier.component.v1.function) = {
      function_id: "player.ban"
      risk: "high"
      timeout: "30s"
    };
  }
}

message BanRequest {
  string player_id = 1 [(croupier.component.v1.ui) = { label: "玩家ID", widget: "input" }];
}
```

### sdk/ - Game SDK → Agent
游戏服务器 SDK 与本地 Agent 的通信协议。

| 文件 | 用途 |
|-----|------|
| `provider.proto` | 游戏服务器向 Agent 注册函数、心跳、查询本地实例 |
| `invocation.proto` | 调用者请求执行函数（同步/异步）、任务流、取消任务 |

**通信方向**：
```
Game SDK --[NNG]--> Agent
Invoker  --[NNG]--> Agent
```

### agent/ - Agent → Server
Agent 与中央 Server 的通信协议。

| 文件 | 用途 |
|-----|------|
| `register.proto` | Agent 向 Server 注册、心跳、能力注册 |
| `ops.proto` | 运维操作：指标上报、进程管理、系统信息查询 |
| `job.proto` | 任务类型定义（JobStatus, JobEvent） |

**通信方向**：
```
Agent --[NNG]--> Server
```

### external/ - Server → 第三方服务
Server 调用第三方平台（如 QuickSDK）的统一接口。

| 文件 | 用途 |
|-----|------|
| `platform.proto` | 调用第三方平台 API、查询平台列表、重载配置 |

**通信方向**：
```
Server --> 第三方平台 (QuickSDK, etc.)
```

### examples/ - 示例
函数定义示例，展示如何使用 `component` 中的元数据注解。

## 通信协议说明

本系统使用 **NNG (nanomsg-next-gen)** 作为通信协议，不再使用 gRPC。

Proto 文件中的 `message` 定义用于 NNG 消息的序列化/反序列化，`service` 定义仅作为接口文档说明，不生成 gRPC 代码。

## 通信流程图

```
┌─────────────┐
│  Game SDK   │
└──────┬──────┘
       │ NNG (注册函数)
       ↓
┌─────────────┐           ┌─────────────┐
│    Agent    │──────────→│   Server    │
│             │ NNG       │             │
└──────┬──────┘           └──────┬──────┘
       │                         │
       │ NNG                     │ HTTP
       ↑                         │ (第三方平台)
┌──────┴──────┐                  ↓
│  Invoker    │          ┌─────────────┐
└─────────────┘          │ 第三方平台   │
                         └─────────────┘
```

## 目录命名规范

- **按通信场景分类**，不是按组件名分类
- **服务提供方**作为目录名（sdk 提供 Game SDK 接口，agent 提供 Agent 接口）
- **非通信协议**放在独立的目录（component）
