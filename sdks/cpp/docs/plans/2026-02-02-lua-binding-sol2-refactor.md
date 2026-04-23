# Lua 绑定 sol2 重构设计

**日期**: 2026-02-02
**状态**: 设计阶段
**作者**: AI Assistant (Claude)
**审批**: 待定

## 目录

- [1. 概述](#1-概述)
- [2. 当前问题](#2-当前问题)
- [3. 设计目标](#3-设计目标)
- [4. 架构设计](#4-架构设计)
- [5. Lua API 设计](#5-lua-api-设计)
- [6. 数据流与类型转换](#6-数据流与类型转换)
- [7. 错误处理](#7-错误处理)
- [8. 生命周期管理](#8-生命周期管理)
- [9. 测试策略](#9-测试策略)
- [10. 迁移计划](#10-迁移计划)
- [11. 性能优化](#11-性能优化)
- [12. 风险评估](#12-风险评估)

---

## 1. 概述

### 1.1 背景

Croupier SDK 当前使用传统 Lua C API 实现 Lua 绑定，代码冗长（~380 行），维护成本高。sol2 是一个现代 C++17 Lua 绑定库，可以大幅简化代码并提升类型安全。

### 1.2 目标

使用 sol2 完全重写 Lua 绑定，实现：
- **代码量减少 70%**: 380 行 → ~100-150 行
- **更 Lua 化的 API**: 使用 Lua table 替代 JSON 字符串
- **类型安全**: 编译时类型检查，运行时自动转换
- **更好的错误处理**: C++ 异常自动传播到 Lua

---

## 2. 当前问题

### 2.1 代码复杂度

```cpp
// 当前实现：注册虚拟对象 (~50 行)
int LuaBinding::client_register_virtual_object(lua_State* L) {
    CroupierClient* client = get_client(L, 1);
    if (!client) return push_error(L, "invalid client");

    const char* vo_id = luaL_checkstring(L, 2);
    const char* class_id = luaL_checkstring(L, 3);

    // 大量手动栈操作和 JSON 解析代码...
    if (lua_istable(L, 4)) {
        lua_getglobal(L, "cjson");
        lua_getfield(L, -1, "encode");
        lua_pushvalue(L, 4);
        lua_call(L, 1, 1);
        // ... 20+ 行代码
    }
}
```

### 2.2 Lua API 不自然

```lua
-- 当前：必须使用 JSON 字符串
local props = cjson.encode({level = 10, name = "Player1"})
client:register_virtual_object("player001", "Player", props)
```

### 2.3 维护成本

- 重复的错误处理代码
- 手动类型转换容易出错
- 栈操作难以调试

---

## 3. 设计目标

### 3.1 重构方式

**选择**: **完全重写，替换现有实现**

- ❌ 不保留旧 Lua C API 代码
- ✅ 删除所有手动栈操作
- ✅ 使用 sol2 完全重写

### 3.2 API 兼容性

**选择**: **重新设计，更 Lua 化**

- ❌ 不受旧 API 约束
- ✅ 使用 Lua table 直接传递参数
- ✅ 利用 sol2 的自动类型转换

### 3.3 依赖管理

**选择**: **完全移除，纯 sol2**

- ❌ 不直接使用 lua.h/lauxlib.h
- ✅ 只包含 sol2/sol.hpp（header-only）
- ✅ sol2 底层处理 Lua C API

### 3.4 目标场景

**选择**: **通用 Lua 模块**

- ✅ 同时支持 Skynet 和独立 Lua 应用
- ✅ 一个二进制适配所有场景

---

## 4. 架构设计

### 4.1 整体架构

```
┌─────────────────────────────────────────┐
│         Lua Application Layer          │
│  (Skynet / Standalone Lua Scripts)      │
└─────────────────┬───────────────────────┘
                  │ Lua API calls
                  │
┌─────────────────▼───────────────────────┐
│         sol2 Binding Layer              │
│  (lua_binding_sol2.cpp ~100-150 lines)  │
│  - sol::state_view                      │
│  - sol::usertype<CroupierClient>        │
│  - Automatic type conversion            │
└─────────────────┬───────────────────────┘
                  │ Direct C++ calls
                  │
┌─────────────────▼───────────────────────┐
│      Croupier SDK C++ Layer             │
│  (croupier_client.h/cpp)                │
│  - CroupierClient class                 │
│  - gRPC communication                   │
└─────────────────────────────────────────┘
```

### 4.2 核心组件

#### 4.2.1 lua_binding_sol2.cpp（入口点）

**职责**:
- 实现 `luaopen_croupier(lua_State* L)` 函数
- 使用 `sol::state_view` 包装现有 Lua state
- 注册 `CroupierClient` usertype 到 Lua
- 注册辅助函数（如版本信息）

**代码量**: ~100-150 行

#### 4.2.2 CroupierClient 包装

**方式**: 直接暴露 C++ 类方法到 Lua

**sol2 自动处理**:
- ✅ 类型转换（string ↔ std::string, table ↔ map）
- ✅ 参数验证
- ✅ 错误传播（C++ exception → Lua error）
- ✅ 生命周期管理（shared_ptr）

#### 4.2.3 类型转换策略

| Lua 类型 | C++ 类型 | 说明 |
|---------|---------|------|
| string | std::string | 自动转换 |
| number | int, double | 自动转换 |
| boolean | bool | 自动转换 |
| table | std::map, sol::table | 自动转换 |
| function | std::function | 回调支持（可选） |

### 4.3 文件结构

```
src/bindings/
  ├── lua_binding_sol2.cpp    ← 新文件（~100行）
  └── lua_binding_sol2.h      ← 新文件（~30行）

include/croupier/sdk/bindings/
  └── lua_binding_sol2.h      ← 公共头文件（可选）

旧文件（标记为 deprecated）:
  ├── lua_binding.cpp         ← 将被删除
  └── lua_binding.h           ← 将被删除
```

### 4.4 CMakeLists.txt 变更

```cmake
# 添加 sol2 依赖
find_package(sol2 CONFIG REQUIRED)

# 编译新的 sol2 绑定
target_sources(croupier-sdk-shared PRIVATE
    src/bindings/lua_binding_sol2.cpp
)

target_link_libraries(croupier-sdk-shared PRIVATE sol2::sol2)

# 移除旧的 Lua C API 链接
# target_link_libraries(... ${LUA_LIBRARIES})  ← 删除
```

---

## 5. Lua API 设计

### 5.1 客户端构造

```lua
local croupier = require("croupier")

-- 方式 1: 简洁构造
local client = croupier.Client("localhost:50051")

-- 方式 2: 带选项构造
local client = croupier.Client {
    server = "localhost:50051",
    timeout = 5,
    credentials = "token-string"
}
```

### 5.2 虚拟对象注册

**旧 API** (使用 JSON):
```lua
local props = cjson.encode({level = 10, name = "Player1"})
client:register_virtual_object("player001", "Player", props)
```

**新 API** (直接 Lua table):
```lua
client:register_vo("player001", "Player", {
    level = 10,
    name = "Player1",
    position = {x = 100, y = 200}
})
```

### 5.3 函数调用

**旧 API**:
```lua
local args = cjson.encode({target_id = "enemy001", damage = 50})
local result = client:invoke_function("player001", "attack", args)
local data = cjson.decode(result)
```

**新 API**:
```lua
local result = client:invoke("player001", "attack", {
    target_id = "enemy001",
    damage = 50
})
-- result 自动转换为 Lua table
print(result.damage_dealt)  -- → 50
```

### 5.4 批量操作

```lua
local results = client:batch_invoke {
    {vo = "player001", func = "move", args = {x = 10, y = 20}},
    {vo = "player002", func = "attack", args = {target = "enemy001"}},
    {vo = "player001", func = "get_stats", args = {}}
}

-- results 是 Lua table 数组
for i, result in ipairs(results) do
    print(result.status)
end
```

### 5.5 查询操作

```lua
-- 获取单个 VO（自动解析为 Lua table）
local player = client:get_vo("player001")
print(player.name)
print(player.position.x)

-- 列出所有 VOs（可选过滤）
local all_players = client:list_vos("Player")
for id, vo in pairs(all_players) do
    print(id, vo.name)
end
```

### 5.6 错误处理

```lua
-- sol2 自动将 C++ 异常转换为 Lua 错误
local ok, err = pcall(function()
    client:register_vo("player001", "Player", props)
end)

if not ok then
    print("Error:", err)  -- C++ 异常消息
end
```

### 5.7 完整 API 列表

```lua
croupier.Client(address | options)
    :register_vo(id, class_id, properties_table) → boolean
    :unregister_vo(id) → boolean
    :invoke(vo_id, function_name, args_table) → result_table
    :batch_invoke({requests...}) → {results...}
    :get_vo(id) → vo_table | nil
    :list_vos(class_filter) → {id: vo, ...}
    :set_credentials(token_string)
    :close()
    :__tostring() → string

croupier.version  → SDK version string
```

---

## 6. 数据流与类型转换

### 6.1 基本类型转换

| Lua 类型 | C++ 类型 | 转换方式 |
|---------|---------|---------|
| string | std::string | 自动转换 |
| number | int, double | 自动转换 |
| boolean | bool | 自动转换 |
| nil | nullptr_t / optional | 自动转换 |

### 6.2 复杂类型转换

| Lua 类型 | C++ 类型 | 转换方式 |
|---------|---------|---------|
| table | std::map\<string, T\> | sol2 自动解析 |
| table | std::vector\<T\> | 数组自动转换 |
| table | sol::table | 延迟求值 |
| function | std::function\<R(Args...)\> | 自动包装 |

### 6.3 典型数据流示例

#### 场景：注册虚拟对象

**Lua 侧**:
```lua
client:register_vo("player001", "Player", {
    level = 10,
    stats = {hp = 100, mp = 50}
})
```

**C++ 侧**:
```cpp
bool RegisterVirtualObject(const std::string& vo_id,
                           const std::string& class_id,
                           const sol::table& properties) {
    // sol2 自动解析 Lua table
    int level = properties["level"];        // → 10
    sol::table stats = properties["stats"]; // → 嵌套 table
    int hp = stats["hp"];                   // → 100

    // 内部序列化为 JSON 发送给 gRPC
    std::string json = serialize_table(properties);
    return original_impl(vo_id, class_id, json);
}
```

### 6.4 CroupierClient 接口调整

#### 当前接口
```cpp
bool RegisterVirtualObject(const std::string& vo_id,
                           const std::string& class_id,
                           const std::string& properties_json);
```

#### 调整后接口
```cpp
bool RegisterVirtualObject(const std::string& vo_id,
                           const std::string& class_id,
                           const sol::object& properties);
```

#### 实现示例
```cpp
bool CroupierClient::RegisterVirtualObject(
    const std::string& vo_id,
    const std::string& class_id,
    const sol::object& properties
) {
    std::string props_json;
    if (properties.valid()) {
        props_json = serialize_sol_object(properties);
    }
    // 调用原有实现...
}
```

---

## 7. 错误处理

### 7.1 三层错误传播

```
┌─────────────────────────────────────┐
│  Layer 1: C++ Exception Layer       │
│  throw std::runtime_error("...")     │
└──────────────┬──────────────────────┘
               │ sol2 自动捕获
┌──────────────▼──────────────────────┐
│  Layer 2: sol2 Conversion Layer     │
│  catch (exception)                   │
│  throw sol::error(msg)               │
└──────────────┬──────────────────────┘
               │ 自动传递
┌──────────────▼──────────────────────┐
│  Layer 3: Lua Error Handling        │
│  local ok, err = pcall(...)          │
└─────────────────────────────────────┘
```

### 7.2 错误类型映射

| C++ 异常类型 | Lua 处理方式 |
|-------------|-------------|
| std::runtime_error | `error("msg")` |
| std::invalid_argument | `error("bad argument #x: ...")` |
| std::out_of_range | `error("index out of range")` |
| sol::error | 直接传递（Lua 侧错误） |

### 7.3 错误处理示例

#### C++ 侧（抛出异常）
```cpp
bool CroupierClient::RegisterVirtualObject(...) {
    if (vo_id.empty()) {
        throw std::invalid_argument("vo_id cannot be empty");
    }
    // ...
}
```

#### Lua 侧（捕获错误）
```lua
local ok, err = pcall(function()
    return client:register_vo("", "Player", {})
end)

if not ok then
    print("Error:", err)  -- → "bad argument #1: vo_id cannot be empty"
end
```

### 7.4 自定义错误处理器（可选）

```cpp
lua.new_usertype<CroupierClient>("Client",
    sol::meta_function::error_handler, &CroupierClient::handle_error
);

// C++ 侧实现
void CroupierClient::handle_error(lua_State* L, const std::exception& e) {
    logger->error("Lua error: {}", e.what());
    throw sol::error(e.what());
}
```

---

## 8. 生命周期管理

### 8.1 shared_ptr 自动管理

```cpp
// C++ 侧返回 shared_ptr
auto client = std::make_shared<CroupierClient>(addr);

// Lua 侧持有引用
local client = croupier.Client("localhost:50051")

-- Lua GC 自动清理（当 client 变量超出作用域）
-- sol2 设置 __gc 元方法，调用 shared_ptr::~shared_ptr
```

### 8.2 循环引用处理

```cpp
// 避免循环引用：使用 weak_ptr
class LuaHandler {
    std::weak_ptr<CroupierClient> client_;  // 弱引用

public:
    void operator()(const std::string& msg) {
        if (auto c = client_.lock()) {  // 安全访问
            c->Send(msg);
        }
    }
};
```

### 8.3 资源清理顺序

```
1. Lua 变量超出作用域
   ↓
2. Lua GC 触发 __gc
   ↓
3. sol2 调用 shared_ptr::~shared_ptr
   ↓
4. CroupierClient::~CroupierClient()
   ↓
5. gRPC channel/shutdown()
```

---

## 9. 测试策略

### 9.1 单元测试（C++ 侧）

**文件**: `tests/test_lua_binding_sol2.cpp`

```cpp
#include <gtest/gtest.h>
#include <sol2/sol.hpp>

class LuaBindingSol2Test : public ::testing::Test {
protected:
    sol::state lua;

    void SetUp() override {
        lua.open_libraries(sol::lib::base, sol::lib::table);
        luaopen_croupier(lua.lua_state());
    }
};

TEST_F(LuaBindingSol2Test, CreateClient) {
    auto client = lua["croupier"]["Client"].new("localhost:50051");
    ASSERT_TRUE(client.valid());
}

TEST_F(LuaBindingSol2Test, RegisterVirtualObject) {
    auto client = lua["croupier"]["Client"].new("localhost:50051");

    bool success = client["register_vo"](
        "test001", "TestClass",
        sol::table{{x = 10, y = 20}}
    );

    EXPECT_TRUE(success);
}

TEST_F(LuaBindingSol2Test, TableTypeConversion) {
    auto client = lua["croupier"]["Client"].new("localhost:50051");

    sol::table props = lua.create_table();
    props["position"] = lua.create_table_with("x", 10, "y", 20);
    props["stats"] = lua.create_table_with("hp", 100, "mp", 50);

    bool success = client["register_vo"]("player001", "Player", props);
    EXPECT_TRUE(success);
}

TEST_F(LuaBindingSol2Test, ErrorHandling) {
    auto client = lua["croupier"]["Client"].new("invalid:address");

    EXPECT_THROW(
        client["register_vo"]("test", "Class", {}),
        sol::error
    );
}

TEST_F(LuaBindingSol2Test, BatchInvoke) {
    auto client = lua["croupier"]["Client"].new("localhost:50051");

    sol::table requests = lua.create_table_with(
        1, lua.create_table_with("vo", "p1", "func", "move", "args", lua.create_table_with("x", 10)),
        2, lua.create_table_with("vo", "p2", "func", "attack")
    );

    auto results = client["batch_invoke"](requests);
    ASSERT_TRUE(results.valid());
}
```

### 9.2 集成测试（Lua 侧）

**文件**: `lua/tests/test_sol2_binding.lua`

```lua
local croupier = require("croupier")

-- 测试 1: 客户端创建
local client = croupier.Client("localhost:50051")
assert(client ~= nil, "Failed to create client")

-- 测试 2: 虚拟对象注册
local props = {
    level = 10,
    name = "TestPlayer",
    position = {x = 100, y = 200}
}

local ok, err = pcall(function()
    return client:register_vo("lua_test_001", "Player", props)
end)

if ok then
    print("✓ Virtual object registered")
else
    print("✗ Registration failed:", err)
end

-- 测试 3: 函数调用
local result = client:invoke("lua_test_001", "get_stats", {})
assert(result.level == 10, "Stats mismatch")

-- 测试 4: 批量操作
local results = client:batch_invoke {
    {vo = "lua_test_001", func = "move", args = {x = 50}},
    {vo = "lua_test_001", func = "attack", args = {target = "enemy"}}
}
assert(#results == 2, "Batch invoke failed")

-- 测试 5: 查询
local vo = client:get_vo("lua_test_001")
assert(vo.name == "TestPlayer", "VO query failed")

-- 清理
client:unregister_vo("lua_test_001")
client:close()

print("All Lua tests passed!")
```

### 9.3 Skynet 集成测试

**文件**: `skynet/tests/test_sol2_integration.lua`

```lua
local skynet = require "skynet"
local croupier = require "croupier"

skynet.start(function()
    -- 在 Skynet 环境中测试
    local client = croupier.Client "localhost:50051"

    -- 测试并发调用
    for i = 1, 10 do
        skynet.fork(function()
            local ok = client:register_vo("skynet_test_"..i, "Test", {})
            assert(ok, "Skynet registration failed")
        end)
    end

    skynet.timeout(100, function()
        client:close()
        skynet.exit()
    end)
end)
```

---

## 10. 迁移计划

### 10.1 阶段 1: 并行开发（1-2周）

**目标**: 新旧实现共存

**任务**:
- [ ] 创建 `src/bindings/lua_binding_sol2.cpp`
- [ ] 创建 `include/croupier/sdk/bindings/lua_binding_sol2.h`
- [ ] 在 CMakeLists.txt 添加新 target: `croupier-sdk-sol2`
- [ ] 实现核心功能（register_vo, invoke, batch_invoke）
- [ ] 编写单元测试

**验收标准**:
- 新绑定可以独立编译
- 单元测试通过
- 旧绑定不受影响

### 10.2 阶段 2: 测试验证（1周）

**目标**: 验证功能完整性和性能

**任务**:
- [ ] 运行完整测试套件
- [ ] 性能基准测试（对比旧实现）
- [ ] Skynet 集成测试
- [ ] 内存泄漏检测

**验收标准**:
- 所有测试通过
- 性能不低于旧实现
- 无内存泄漏

### 10.3 阶段 3: 切换主分支（1周）

**目标**: 将 sol2 设为默认实现

**任务**:
- [ ] 替换 `ENABLE_LUA_BINDING` 为使用 sol2
- [ ] 更新所有文档
- [ ] 更新示例代码
- [ ] 更新 CI/CD 配置
- [ ] 发布 migration guide

**验收标准**:
- 默认使用 sol2 绑定
- 文档更新完成
- CI 全部通过

### 10.4 阶段 4: 清理（1天）

**目标**: 移除旧代码

**任务**:
- [ ] 标记旧文件为 `deprecated`
- [ ] 添加迁移提示注释
- [ ] 下一个版本删除旧文件

**验收标准**:
- 代码库干净
- 无遗留警告

### 10.5 回滚计划

如果 sol2 实现出现问题：

**CMake 标志**:
```cmake
option(USE_LEGACY_LUA_BINDING "Use legacy Lua C API implementation" OFF)

if(USE_LEGACY_LUA_BINDING)
    target_sources(croupier-sdk-shared PRIVATE
        src/bindings/lua_binding.cpp  # 旧实现
    )
else()
    target_sources(croupier-sdk-shared PRIVATE
        src/bindings/lua_binding_sol2.cpp  # sol2 实现
    )
endif()
```

**优势**:
- 零风险切换
- 可以快速回滚
- 两个实现可以共存

---

## 11. 性能优化

### 11.1 性能目标

| 指标 | 当前实现 | sol2 目标 | 说明 |
|-----|---------|----------|------|
| 函数调用延迟 | \< 1ms | \< 1ms | 保持不变 |
| 类型转换 | 手动 | 自动 | 可接受的开销 |
| 内存占用 | 基准 | +5% | sol2 少量开销 |
| 代码大小 | 380 行 | ~150 行 | 减少 60% |

### 11.2 优化策略

#### 11.2.1 避免频繁序列化

```cpp
// 缓存常用 Lua table 的 JSON 表示
class CroupierClient {
    std::unordered_map<std::string, std::string> json_cache_;

    std::string get_cached_json(const sol::table& table) {
        // 生成 hash key
        std::string key = generate_table_hash(table);

        auto it = json_cache_.find(key);
        if (it != json_cache_.end()) {
            return it->second;
        }

        // 序列化并缓存
        std::string json = serialize_table(table);
        json_cache_[key] = json;
        return json;
    }
};
```

#### 11.2.2 批量操作优化

```lua
-- 减少 IPC 次数
local results = client:batch_invoke {
    {vo = "p1", func = "move", args = {x = 10}},
    {vo = "p2", func = "attack", args = {target = "e1"}},
    -- ... 100 个操作
}
-- 而不是循环调用 100 次 client:invoke
```

#### 11.2.3 使用 string_view

```cpp
// 避免字符串拷贝（C++17）
bool RegisterVirtualObject(std::string_view vo_id,
                           std::string_view class_id,
                           const sol::table& properties);
```

#### 11.2.4 延迟加载

```lua
-- 使用 Lua 迭代器模式，按需获取数据
for id, vo in client:list_vos("Player") do
    -- vo 数据按需加载
end
```

### 11.3 性能基准测试

**测试脚本**: `tests/benchmark/lua_binding_bench.cpp`

```cpp
#include <benchmark/benchmark.h>
#include <sol2/sol.hpp>

static void BM_RegisterVO_Sol2(benchmark::State& state) {
    sol::state lua;
    luaopen_croupier(lua.lua_state());
    auto client = lua["croupier"]["Client"].new("localhost:50051");

    for (auto _ : state) {
        client["register_vo"]("test", "Class", sol::table{{x = 10}});
    }
}

BENCHMARK(BM_RegisterVO_Sol2)->Iterations(1000);
BENCHMARK_MAIN();
```

---

## 12. 风险评估

### 12.1 技术风险

| 风险 | 影响 | 概率 | 缓解措施 |
|-----|------|------|---------|
| sol2 与 Lua 5.5 兼容性问题 | 高 | 低 | vcpkg 已有 sol2 + Lua 5.5 patch |
| 性能下降 | 中 | 低 | 基准测试验证 |
| C++ 异常传播失败 | 高 | 低 | sol2 成熟，已验证 |
| Skynet 集成问题 | 中 | 中 | 专门测试 Skynet 场景 |

### 12.2 项目风险

| 风险 | 影响 | 概率 | 缓解措施 |
|-----|------|------|---------|
| 开发时间超期 | 中 | 低 | 并行开发，不影响旧代码 |
| 现有 Lua 代码需要修改 | 高 | 中 | 提供迁移脚本和文档 |
| 学习曲线 | 低 | 高 | sol2 代码更简单，易学 |

### 12.3 依赖风险

| 依赖项 | 风险 | 缓解措施 |
|-------|------|---------|
| sol2 | 维护活跃 | ✅ 低风险 |
| Lua 5.5/5.4 | 主流版本 | ✅ 低风险 |
| vcpkg sol2 port | 已存在 | ✅ 低风险 |

---

## 13. 附录

### 13.1 参考资料

- [sol2 官方文档](https://sol2.readthedocs.io/)
- [Lua 5.5 参考手册](https://www.lua.org/manual/5.5/)
- [vcpkg sol2 port](https://github.com/microsoft/vcpkg/tree/master/ports/sol2)

### 13.2 代码示例

#### 完整的 lua_binding_sol2.cpp 示例

```cpp
#include "croupier/sdk/croupier_client.h"
#include <sol2/sol.hpp>

namespace croupier::sdk::lua {

extern "C" {

int luaopen_croupier(lua_State* L) {
    // 包装现有 Lua state
    sol::state_view lua(L);

    // 注册 CroupierClient usertype
    lua.new_usertype<CroupierClient>("CroupierClient",
        // 构造函数
        sol::constructors<
            CroupierClient(const std::string&),
            CroupierClient()
        >(),

        // 虚拟对象方法
        "register_vo", &CroupierClient::RegisterVirtualObject,
        "unregister_vo", &CroupierClient::UnregisterVirtualObject,
        "get_vo", &CroupierClient::GetVirtualObject,
        "list_vos", &CroupierClient::ListVirtualObjects,

        // 函数调用
        "invoke", &CroupierClient::InvokeFunction,
        "batch_invoke", &CroupierClient::BatchInvoke,

        // 连接管理
        "set_credentials", &CroupierClient::SetCredentials,
        "close", &CroupierClient::Close,

        // 元方法
        sol::meta_function::to_string, [](const CroupierClient& c) {
            return "croupier.Client: " + std::to_string(reinterpret_cast<uintptr_t>(&c));
        }
    );

    // 创建模块表
    sol::table module = lua.create_table();

    // 添加 Client 构造函数
    module["Client"] = lua["CroupierClient"];

    // 添加版本信息
    module["_VERSION"] = CROUPIER_SDK_VERSION;

    // 返回模块
    return sol::stack::push(lua, module);
}

} // extern "C"

} // namespace croupier::sdk::lua
```

### 13.3 迁移检查清单

- [ ] 代码审查完成
- [ ] 单元测试全部通过
- [ ] 集成测试全部通过
- [ ] 性能基准测试通过
- [ ] Skynet 集成测试通过
- [ ] 文档更新完成
- [ ] 示例代码更新完成
- [ ] CI/CD 配置更新完成
- [ ] Migration guide 发布
- [ ] 回滚计划就绪

---

**文档版本**: 1.0
**最后更新**: 2026-02-02
