# Croupier C++ SDK 集成指南

本文档说明如何在Windows项目中集成Croupier C++ SDK。

## 版本说明

SDK提供MT版本（静态运行时），使用vcpkg的 `x64-windows-static` triplet编译。

---

## MT 版本（静态运行时 /MT）

### 文件结构

```
contrib/croupier/
├── include/croupier/sdk/       # SDK 头文件
└── lib/                       # Release 库
    └── croupier-sdk.lib       # SDK 主库（不含依赖）
└── debug/lib/                 # Debug 库
    └── croupier-sdk.lib       # SDK 主库（不含依赖）
```

### 集成步骤

#### 1. 添加包含目录
- 项目属性 → C/C++ → 常规 → 附加包含目录
- 添加：`../../contrib/croupier/include`

#### 2. 添加库目录
- 项目属性 → 链接器 → 常规 → 附加库目录
- Release: `../../contrib/croupier/lib`
- Debug: `../../contrib/croupier/debug/lib`

#### 3. 添加依赖库
- 项目属性 → 链接器 → 输入 → 附加依赖项
- 添加以下库：

**Release 配置：**
```
croupier-sdk.lib
grpc++.lib
grpc.lib
gpr.lib
libprotobuf.lib
ws2_32.lib
```

**Debug 配置：**
```
croupier-sdk.lib
grpc++.lib
grpc.lib
gpr.lib
libprotobufd.lib
ws2_32.lib
```

#### 4. 设置运行时类型
- 项目属性 → C/C++ → 代码生成 → 运行时库
- Release: **多线程 (/MT)**
- Debug: **多线程调试 (/MTd)**

### ⚠️ 重要

- **必须使用 `/MT` 或 `/MTd` 编译选项**，否则会产生链接错误
- 需要同时链接 gRPC 相关库（见上方依赖列表）
- Debug 版本的 protobuf 库名为 `libprotobufd.lib`（带 d 后缀）

---

## 完整依赖列表

### Release 依赖
```
croupier-sdk.lib      # SDK 主库
grpc++.lib            # gRPC C++
grpc.lib              # gRPC 核心库
gpr.lib               # gPR 基础运行时
libprotobuf.lib       # Protocol Buffers
ws2_32.lib            # Windows Socket
```

### Debug 依赖
```
croupier-sdk.lib      # SDK 主库
grpc++.lib            # gRPC C++ (Debug)
grpc.lib              # gRPC 核心库 (Debug)
gpr.lib               # gPR 基础运行时 (Debug)
libprotobufd.lib      # Protocol Buffers (Debug)
ws2_32.lib            # Windows Socket
```

---

## 快速示例代码

```cpp
#include <croupier/sdk/croupier_client.h>

using namespace croupier::sdk;

int main() {
    // 创建客户端配置
    ClientConfig config;
    config.game_id = "my-game";
    config.service_id = "my-service";
    config.agent_addr = "127.0.0.1:19090";
    config.insecure = true;

    // 创建客户端
    CroupierClient client(config);

    // 连接并注册
    if (client.Connect()) {
        // 注册虚拟对象
        VirtualObjectDescriptor desc;
        desc.id = "player.entity";
        desc.name = "Player";

        std::map<std::string, FunctionHandler> handlers;
        handlers["get"] = [](const std::string& ctx, const std::string& payload) {
            return "{\"data\": {}}";
        };

        client.RegisterVirtualObject(desc, handlers);
        client.Serve();
    }

    return 0;
}
```

---

## 常见问题

### Q: 链接时出现 LNK2038 运行时库不匹配错误
**A**: 检查你的项目运行时库设置。Debug 用 `/MTd`，Release 用 `/MT`。必须与SDK使用相同的运行时类型。

### Q: Debug 版本链接 libprotobuf.lib 失败
**A**: Debug 版本需要链接 `libprotobufd.lib`（带 d 后缀），而不是 `libprotobuf.lib`。

### Q: 是否需要链接 abseil 库？
**A**: 不需要。vcpkg 的 gRPC 库已经包含了所需的 abseil 符号。

### Q: 库文件很大怎么办？
**A**: gRPC 库本身较大。如果只需要最小依赖，可以考虑只使用必要的 gRPC 组件。
