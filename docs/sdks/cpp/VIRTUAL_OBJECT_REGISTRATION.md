# Croupier C++ SDK：虚拟对象注册机制

## 🎯 概述

Croupier采用创新的**四层组件化架构**实现虚拟对象管理，通过**ID引用模式**优雅地解决了对象参数传递的性能问题。本文档详细介绍了虚拟对象的注册机制和C++ SDK的扩展方案。

## 📋 核心架构

### 四层抽象模型

```
Function Level    ← 单个原子操作 (wallet.transfer)
     ↓
Entity Level      ← 业务对象模型 (wallet.entity)
     ↓
Resource Level    ← UI资源组织 (钱包管理面板)
     ↓
Component Level   ← 可分发模块 (economy-system)
```

### 设计理念

#### ✅ **ID引用模式** - 解决性能问题

```cpp
// ❌ 避免笨重的对象参数传递
invoke("wallet.transfer", {object: wallet_instance, params: {...}})

// ✅ 优雅的ID引用设计
invoke("wallet.transfer", {
  from_player_id: "player123",  // 直接使用ID引用
  to_player_id: "player456",
  currency_code: "gold",
  amount: "100.0"
})
```

#### ✅ **声明式配置** - 配置驱动开发

```json
// wallet.entity.json
{
  "id": "wallet.entity",
  "schema": {
    /* JSON Schema定义对象结构 */
  },
  "operations": {
    "read": "wallet.get",
    "transfer": "wallet.transfer"
  },
  "relationships": {
    "currency": { "type": "many-to-one", "entity": "currency" }
  }
}
```

#### ✅ **无状态函数** - 易于扩展

- 每个函数是纯函数，通过ID查找对象
- 支持水平扩展，无状态共享问题
- Repository模式管理对象生命周期

## 🏗️ C++ SDK扩展方案

### 核心数据结构

```cpp
namespace croupier::sdk {

// 虚拟对象描述符
struct VirtualObjectDescriptor {
    std::string id;                              // e.g. "wallet.entity"
    std::string version;                         // 版本号
    std::string name;                            // 显示名称
    std::string description;                     // 描述信息
    std::map<std::string, std::string> schema;   // JSON Schema定义
    std::map<std::string, std::string> operations; // 操作映射
    std::map<std::string, RelationshipDef> relationships; // 关系定义
};

// 关系定义
struct RelationshipDef {
    std::string type;        // "one-to-many", "many-to-one", "many-to-many"
    std::string entity;      // 关联实体ID
    std::string foreign_key; // 外键字段名
};

// 组件描述符（完整模块）
struct ComponentDescriptor {
    std::string id;                             // e.g. "economy-system"
    std::string version;                        // 组件版本
    std::string name;                           // 组件名称
    std::vector<VirtualObjectDescriptor> entities;  // 包含的实体
    std::vector<FunctionDescriptor> functions;      // 包含的函数
    std::map<std::string, std::string> resources;   // UI资源定义
    std::map<std::string, std::string> config;      // 组件配置
};

} // namespace croupier::sdk
```

### 扩展的CroupierClient接口

```cpp
class CroupierClient {
public:
    // ========== 现有接口（保持兼容） ==========
    bool RegisterFunction(const FunctionDescriptor& desc, FunctionHandler handler);
    bool Connect();
    void Serve();
    void Stop();
    void Close();

    // ========== 新增：虚拟对象注册 ==========

    // 注册单个虚拟对象及其关联函数
    bool RegisterVirtualObject(
        const VirtualObjectDescriptor& desc,
        const std::map<std::string, FunctionHandler>& handlers
    );

    // 批量注册组件（推荐方式）
    bool RegisterComponent(const ComponentDescriptor& comp);

    // 从JSON配置文件加载并注册组件
    bool LoadComponentFromFile(const std::string& config_file);

    // ========== 新增：管理接口 ==========

    // 获取已注册的虚拟对象列表
    std::vector<VirtualObjectDescriptor> GetRegisteredObjects() const;

    // 获取已注册的组件列表
    std::vector<ComponentDescriptor> GetRegisteredComponents() const;

    // 取消注册虚拟对象
    bool UnregisterVirtualObject(const std::string& object_id);

    // 取消注册组件
    bool UnregisterComponent(const std::string& component_id);
};
```

### 工具函数

```cpp
namespace croupier::sdk::utils {

// 从JSON文件加载虚拟对象描述符
VirtualObjectDescriptor LoadObjectDescriptor(const std::string& file_path);

// 从JSON文件加载组件描述符
ComponentDescriptor LoadComponentDescriptor(const std::string& file_path);

// 验证虚拟对象定义的完整性
bool ValidateObjectDescriptor(const VirtualObjectDescriptor& desc);

// 验证组件定义的完整性
bool ValidateComponentDescriptor(const ComponentDescriptor& comp);

// 生成默认的对象配置模板
std::string GenerateObjectTemplate(const std::string& object_id);

// 生成默认的组件配置模板
std::string GenerateComponentTemplate(const std::string& component_id);

} // namespace croupier::sdk::utils
```

## 💡 使用示例

### 示例1：单个函数注册（现有方式）

```cpp
#include "croupier/sdk/croupier_client.h"
using namespace croupier::sdk;

// 函数处理器实现
std::string WalletTransferHandler(const std::string& context, const std::string& payload) {
    auto data = utils::ParseJSON(payload);

    // 通过ID获取源钱包和目标钱包
    std::string from_player = data["from_player_id"];
    std::string to_player = data["to_player_id"];
    std::string amount = data["amount"];

    // 执行转账业务逻辑
    TransferResult result = WalletService::Transfer(from_player, to_player, amount);

    // 返回结果
    std::map<std::string, std::string> response;
    response["transfer_id"] = result.transfer_id;
    response["status"] = result.status;
    return utils::ToJSON(response);
}

int main() {
    ClientConfig config;
    config.service_id = "wallet-service";

    CroupierClient client(config);

    // 注册单个函数
    FunctionDescriptor desc;
    desc.id = "wallet.transfer";
    desc.version = "1.0.0";

    client.RegisterFunction(desc, WalletTransferHandler);
    client.Connect();
    client.Serve();
}
```

### 示例2：虚拟对象注册（推荐方式）

```cpp
#include "croupier/sdk/croupier_client.h"
using namespace croupier::sdk;

int main() {
    ClientConfig config;
    config.service_id = "economy-service";

    CroupierClient client(config);

    // 定义钱包实体
    VirtualObjectDescriptor wallet_desc;
    wallet_desc.id = "wallet.entity";
    wallet_desc.version = "1.0.0";
    wallet_desc.name = "钱包实体";
    wallet_desc.description = "玩家钱包管理实体";

    // 定义Schema
    wallet_desc.schema["type"] = "object";
    wallet_desc.schema["properties"] = R"({
        "wallet_id": {"type": "string"},
        "player_id": {"type": "string"},
        "currency_id": {"type": "string"},
        "balance": {"type": "string", "pattern": "^[0-9]+\\.?[0-9]*$"}
    })";

    // 定义操作映射
    wallet_desc.operations["read"] = "wallet.get";
    wallet_desc.operations["transfer"] = "wallet.transfer";
    wallet_desc.operations["deposit"] = "wallet.deposit";
    wallet_desc.operations["withdraw"] = "wallet.withdraw";

    // 定义关系
    RelationshipDef currency_rel;
    currency_rel.type = "many-to-one";
    currency_rel.entity = "currency";
    currency_rel.foreign_key = "currency_id";
    wallet_desc.relationships["currency"] = currency_rel;

    // 准备函数处理器
    std::map<std::string, FunctionHandler> handlers;
    handlers["wallet.get"] = WalletGetHandler;
    handlers["wallet.transfer"] = WalletTransferHandler;
    handlers["wallet.deposit"] = WalletDepositHandler;
    handlers["wallet.withdraw"] = WalletWithdrawHandler;

    // 注册虚拟对象
    if (!client.RegisterVirtualObject(wallet_desc, handlers)) {
        std::cerr << "Failed to register wallet entity" << std::endl;
        return 1;
    }

    client.Connect();
    client.Serve();

    return 0;
}
```

### 示例3：组件级注册（最优雅）

```cpp
#include "croupier/sdk/croupier_client.h"
using namespace croupier::sdk;

int main() {
    ClientConfig config;
    config.service_id = "economy-system";

    CroupierClient client(config);

    // 方式A：从配置文件加载
    if (!client.LoadComponentFromFile("economy-system.component.json")) {
        std::cerr << "Failed to load economy component" << std::endl;
        return 1;
    }

    // 方式B：程序化定义组件
    ComponentDescriptor economy_comp;
    economy_comp.id = "economy-system";
    economy_comp.version = "1.0.0";
    economy_comp.name = "经济系统";

    // 添加钱包实体
    VirtualObjectDescriptor wallet_entity = BuildWalletEntity();
    economy_comp.entities.push_back(wallet_entity);

    // 添加货币实体
    VirtualObjectDescriptor currency_entity = BuildCurrencyEntity();
    economy_comp.entities.push_back(currency_entity);

    // 添加跨实体函数
    FunctionDescriptor market_trade;
    market_trade.id = "market.trade";
    market_trade.version = "1.0.0";
    economy_comp.functions.push_back(market_trade);

    // 注册整个组件
    if (!client.RegisterComponent(economy_comp)) {
        std::cerr << "Failed to register economy component" << std::endl;
        return 1;
    }

    client.Connect();
    client.Serve();

    return 0;
}
```

### 示例4：配置文件驱动（生产推荐）

```cpp
// economy-system.component.json
{
  "id": "economy-system",
  "version": "1.0.0",
  "name": "经济系统组件",
  "entities": [
    {
      "id": "wallet.entity",
      "schema": { /* ... */ },
      "operations": {
        "read": "wallet.get",
        "transfer": "wallet.transfer"
      },
      "relationships": { /* ... */ }
    },
    {
      "id": "currency.entity",
      "schema": { /* ... */ },
      "operations": {
        "create": "currency.create",
        "read": "currency.get"
      }
    }
  ],
  "functions": [
    {
      "id": "wallet.transfer",
      "params": { /* JSON Schema */ },
      "result": { /* JSON Schema */ }
    }
  ]
}
```

```cpp
// 简洁的主程序
int main() {
    CroupierClient client(config);

    // 一行代码完成整个组件注册
    client.LoadComponentFromFile("economy-system.component.json");

    client.Connect();
    client.Serve();
    return 0;
}
```

## 🔧 实现指南

### 阶段1：扩展现有SDK

1. **扩展头文件** (`croupier_client.h`)
   - 添加新的数据结构定义
   - 扩展CroupierClient类接口
   - 保持向后兼容性

2. **实现核心逻辑** (`croupier_client.cpp`)
   - 实现虚拟对象注册逻辑
   - 添加配置文件解析功能
   - 扩展现有的注册机制

3. **添加工具函数** (`utils.cpp`)
   - JSON配置解析和验证
   - 模板生成功能
   - 错误处理和日志

### 阶段2：生产化增强

4. **配置验证系统**
   - JSON Schema验证
   - 关系一致性检查
   - 循环依赖检测

5. **开发工具支持**
   - 配置文件生成器
   - 可视化编辑器集成
   - 调试和诊断工具

6. **性能优化**
   - 配置缓存机制
   - 懒加载和热重载
   - 批量操作优化

## 🎯 架构优势

### 性能优势

- ✅ **轻量参数**：只传递ID字符串，网络开销极小
- ✅ **无状态设计**：函数可水平扩展，无状态共享问题
- ✅ **缓存友好**：多层级缓存对象数据

### 开发体验

- ✅ **渐进增强**：从简单函数逐步演进到复杂对象
- ✅ **声明式配置**：JSON驱动，易于理解和维护
- ✅ **工具友好**：配置可生成UI、文档、测试用例

### 架构设计

- ✅ **职责清晰**：函数专注业务逻辑，Repository管理对象
- ✅ **类型安全**：JSON Schema确保参数类型正确
- ✅ **关系明确**：通过Entity定义明确对象间关系

## 📚 参考模式

该设计借鉴了多个成熟的架构模式：

- **DDD (Domain-Driven Design)**：Entity概念映射到业务领域对象
- **Repository Pattern**：通过ID获取对象，分离业务逻辑和数据访问
- **Microservice Architecture**：无状态函数，易于分布式部署
- **GraphQL思想**：声明式查询，类型安全的API设计

## 🚀 后续规划

1. **立即实施**：扩展C++ SDK，添加虚拟对象注册接口
2. **短期目标**：完善配置验证和开发工具支持
3. **中期目标**：实现代码生成和可视化编辑
4. **长期目标**：性能优化和多语言SDK统一

---

**通过这套架构，您可以优雅地管理复杂的游戏业务对象，同时保持高性能和良好的开发体验！**
