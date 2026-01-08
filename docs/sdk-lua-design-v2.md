# Croupier Lua SDK 设计文档（集成到 croupier-sdk-cpp）

## 概述

Croupier Lua SDK 通过在 `croupier-sdk-cpp` 中添加 **Lua 绑定层** 实现，复用 C++ SDK 的核心功能。Lua 绑定作为可选构建选项，启用后生成包含 Lua API 的共享库。

## 架构设计

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              Skynet 服务节点                                     │
├─────────────────────────────────────────────────────────────────────────────────┤
│                                                                                 │
│   ┌─────────────┐                                                               │
│   │ Lua Service │  业务逻辑服务                                                  │
│   └──────┬──────┘                                                               │
│          │                                                                       │
│          │ Lua API 调用                                                          │
│          ▼                                                                       │
│   ┌──────────────────────────────────────────────────────────────────────────┐  │
│   │                       Lua SDK (croupier.lua)                               │  │
│   │  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐                 │  │
│   │  │ Client Module │  │ Invoker Module│  │ Logger Module │                 │  │
│   │  └───────────────┘  └───────────────┘  └───────────────┘                 │  │
│   └──────────────────────────────────────────────────────────────────────────┘  │
│                            │                                                     │
│                            │ Lua C API                                          │
│                            ▼                                                     │
│   ┌──────────────────────────────────────────────────────────────────────────┐  │
│   │               croupier-sdk-cpp (WITH LUA BINDING)                        │  │
│   │  ┌────────────────────────────────────────────────────────────────────┐   │  │
│   │  │                    Lua C API Binding                               │   │  │
│   │  │  (src/bindings/lua_binding.cpp)                                   │   │  │
│   │  └────────────────────────────────────────────────────────────────────┘   │  │
│   │                              │                                          │   │  │
│   │  ┌────────────────────────────────────────────────────────────────────┐   │  │
│   │  │                    C++ SDK Core                                  │   │  │
│   │  │  - CroupierClient                                              │   │  │
│   │  │  - GrpcManager                                                 │   │  │
│   │  │  - FunctionHandler                                             │   │  │
│   │  └────────────────────────────────────────────────────────────────────┘   │  │
│   │                              │                                          │   │  │
│   │  ┌────────────────────────────────────────────────────────────────────┐   │  │
│   │  │                    gRPC C++ Library                               │   │  │
│   │  └────────────────────────────────────────────────────────────────────┘   │  │
│   └──────────────────────────────────────────────────────────────────────────┘  │
│                            │                                                     │
│                            │ gRPC                                                │
│                            ▼                                                     │
│                    ┌─────────────┐                                              │
│                    │ Croupier    │                                              │
│                    │   Agent     │                                              │
│                    └─────────────┘                                              │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## 设计优势

| 方案 | 优势 |
|------|------|
| **集成到 croupier-sdk-cpp** | ✅ 复用 C++ 核心，无重复代码 |
| **CMake 可选构建** | ✅ `ENABLE_LUA_BINDING=ON` 才编译 Lua 绑定 |
| **统一版本管理** | ✅ C++ SDK 更新时 Lua 同步更新 |
| **共享库输出** | ✅ `libcroupier-sdk-lua.so` 包含所有功能 |
| **Skynet 开箱即用** | ✅ 编译后直接复制到 Skynet 使用 |

## croupier-sdk-cpp 集成方案

### 1. 目录结构（在 croupier-sdk-cpp 中）

```
croupier-sdk-cpp/
├── CMakeLists.txt                    # 添加 ENABLE_LUA_BINDING 选项
├── include/
│   └── croupier/
│       └── sdk/
│           └── *.h                   # 现有的 C++ 头文件
│
├── src/
│   ├── croupier_client.cpp           # 现有 C++ 实现
│   ├── grpc_service.cpp
│   └── bindings/                     # 新增：语言绑定
│       ├── lua_binding.cpp           # Lua C API 绑定
│       └── lua_binding.h
│
├── lua/                              # 新增：Lua SDK 模块
│   ├── croupier/
│   │   ├── init.lua
│   │   ├── client.lua
│   │   ├── invoker.lua
│   │   └── utils.lua
│   └── examples/
│       └── skynet_service.lua
│
└── skynet/                           # 新增：Skynet 集成
    ├── service/
    │   └── croupier_service.lua
    └── examples/
        └── config/
            └── croupier.conf
```

### 2. CMake 配置

```cmake
# croupier-sdk-cpp/CMakeLists.txt（添加部分）

# ========== Lua Binding Option ==========
option(ENABLE_LUA_BINDING "Enable Lua language binding" OFF)

if(ENABLE_LUA_BINDING)
    # 查找 Lua
    find_package(LUA REQUIRED)

    # 检查 Lua 版本 (5.3+)
    if(NOT LUA_VERSION_MAJOR EQUAL 5)
        message(WARNING "Lua 5.3+ is recommended, found ${LUA_VERSION_MAJOR}.${LUA_VERSION_MINOR}")
    endif()

    message(STATUS "Lua binding enabled")
    message(STATUS "  LUA_VERSION: ${LUA_VERSION_MAJOR}.${LUA_VERSION_MINOR}")
    message(STATUS "  LUA_INCLUDE_DIR: ${LUA_INCLUDE_DIR}")
    message(STATUS "  LUA_LIBRARIES: ${LUA_LIBRARIES}")
endif()
```

### 3. Lua 绑定目标

```cmake
# croupier-sdk-cpp/CMakeLists.txt（添加目标）

# ========== Lua Binding Target ==========
if(ENABLE_LUA_BINDING)
    # Lua 绑定源文件
    set(LUA_BINDING_SOURCES
        src/bindings/lua_binding.cpp
    )

    # 创建带 Lua 绑定的共享库
    add_library(croupier-sdk-lua SHARED
        ${SDK_SOURCES}
        ${LUA_BINDING_SOURCES}
        ${GENERATED_PROTO_SOURCES}
    )

    # 设置输出名称
    set_target_properties(croupier-sdk-lua PROPERTIES
        OUTPUT_NAME "croupier"
        VERSION ${PROJECT_VERSION}
        SOVERSION ${PROJECT_VERSION_MAJOR}
        PREFIX "lib"                    # Linux/macOS: libcroupier.so
                                    # Windows: croupier.dll (由 PREFIX 处理)
    )

    # 包含目录
    target_include_directories(croupier-sdk-lua PRIVATE
        ${CMAKE_CURRENT_SOURCE_DIR}/include
        ${PROTO_GENERATED_DIR}
        ${LUA_INCLUDE_DIR}
    )

    # 链接库
    target_link_libraries(croupier-sdk-lua PRIVATE
        Threads::Threads
        ${LUA_LIBRARIES}
    )

    if(ENABLE_GRPC)
        target_link_libraries(croupier-sdk-lua PRIVATE
            ${GRPC_LIBRARIES}
            ZLIB::ZLIB
        )
        target_compile_definitions(croupier-sdk-lua
            PRIVATE
                CROUPIER_SDK_ENABLE_GRPC
        )
    endif()

    # 导出符号（Lua 需要动态链接）
    if(WIN32)
        # Windows: 导出所有符号
        set_target_properties(croupier-sdk-lua PROPERTIES
            WINDOWS_EXPORT_ALL_SYMBOLS ON
        )
    else()
        # Linux/macOS: 导出符号
        target_link_options(croupier-sdk-lua PRIVATE
            -Wl,--export-dynamic
        )
    endif()

    # 安装 Lua 模块
    install(DIRECTORY "${CMAKE_CURRENT_SOURCE_DIR}/lua/"
        DESTINATION "${CMAKE_INSTALL_DATAROOTDIR}/lua/${LUA_VERSION_MAJOR}.${LUA_VERSION_MINOR}"
        FILES_MATCHING PATTERN "*.lua"
    )

    # 安装共享库
    install(TARGETS croupier-sdk-lua
        RUNTIME DESTINATION "${CMAKE_INSTALL_BINDIR}"
        LIBRARY DESTINATION "${CMAKE_INSTALL_LIBDIR}"
    )

    message(STATUS "Lua binding target configured: croupier-sdk-lua")
endif()
```

### 4. Lua C API 绑定实现

```cpp
// croupier-sdk-cpp/src/bindings/lua_binding.cpp

#include "croupier/sdk/croupier_client.h"
#include <lua.hpp>
#include <memory>
#include <string>

namespace croupier {
namespace lua {

// Lua 用户数据结构
struct LuaClient {
    CroupierClient* client;
    int callback_ref;
    bool serving;
};

// ===== 辅助函数 =====

// 获取 LuaClient 从栈
static LuaClient* check_lua_client(lua_State* L) {
    void* ud = luaL_checkudata(L, 1, "CroupierClient");
    luaL_argcheck(L, ud != nullptr, 1, "'CroupierClient' expected");
    return static_cast<LuaClient*>(ud);
}

// 从表获取字符串字段
static std::string get_table_string(lua_State* L, int index, const char* key, const char* default_val = "") {
    lua_getfield(L, index, key);
    if (lua_isnil(L, -1)) {
        lua_pop(L, 1);
        return default_val;
    }
    const char* value = lua_tostring(L, -1);
    std::string result = value ? value : "";
    lua_pop(L, 1);
    return result;
}

// 从表获取整数字段
static int get_table_int(lua_State* L, int index, const char* key, int default_val = 0) {
    lua_getfield(L, index, key);
    if (lua_isnil(L, -1)) {
        lua_pop(L, 1);
        return default_val;
    }
    int value = (int)lua_tointeger(L, -1);
    lua_pop(L, 1);
    return value;
}

// 从表获取布尔字段
static bool get_table_bool(lua_State* L, int index, const char* key, bool default_val = false) {
    lua_getfield(L, index, key);
    if (lua_isnil(L, -1)) {
        lua_pop(L, 1);
        return default_val;
    }
    bool value = lua_toboolean(L, -1) != 0;
    lua_pop(L, 1);
    return value;
}

// 推送结果到 Lua
static int push_result(lua_State* L, const std::string& result) {
    lua_pushstring(L, result.c_str());
    return 1;
}

// 推送错误到 Lua
static int push_error(lua_State* L, const std::string& error) {
    lua_pushnil(L);
    lua_pushstring(L, error.c_str());
    return 2;
}

// ===== C API 函数 =====

// 创建客户端
// croupier.create_client(config_table)
static int create_client(lua_State* L) {
    luaL_checktype(L, 1, LUA_TTABLE);

    // 从配置表读取参数
    ClientConfig config;
    config.agent_addr = get_table_string(L, 1, "agent_addr", "127.0.0.1:19090");
    config.service_id = get_table_string(L, 1, "service_id", "lua-service");
    config.service_version = get_table_string(L, 1, "service_version", "1.0.0");
    config.game_id = get_table_string(L, 1, "game_id", "default-game");
    config.env = get_table_string(L, 1, "env", "dev");
    config.local_addr = get_table_string(L, 1, "local_addr", "0.0.0.0:0");
    config.insecure = get_table_bool(L, 1, "insecure", false);
    config.cert_file = get_table_string(L, 1, "cert_file", "");
    config.key_file = get_table_string(L, 1, "key_file", "");
    config.ca_file = get_table_string(L, 1, "ca_file", "");
    config.server_name = get_table_string(L, 1, "server_name", "");
    config.timeout_seconds = get_table_int(L, 1, "timeout", 30);
    config.heartbeat_interval = get_table_int(L, 1, "heartbeat_interval", 30);
    config.auto_reconnect = get_table_bool(L, 1, "auto_reconnect", true);
    config.reconnect_interval_seconds = get_table_int(L, 1, "reconnect_interval", 5);
    config.reconnect_max_attempts = get_table_int(L, 1, "reconnect_max_attempts", 0);

    // 创建 C++ 客户端
    auto* client = new CroupierClient(config);

    // 创建 Lua 用户数据
    auto* lc = static_cast<LuaClient*>(lua_newuserdata(L, sizeof(LuaClient)));
    lc->client = client;
    lc->callback_ref = LUA_NOREF;
    lc->serving = false;

    // 设置元表
    luaL_getmetatable(L, "CroupierClient");
    lua_setmetatable(L, -2);

    return 1;
}

// 注册函数
// client:register_function(descriptor_table)
static int register_function(lua_State* L) {
    auto* lc = check_lua_client(L);

    luaL_checktype(L, 2, LUA_TTABLE);

    // 从描述符表读取参数
    FunctionDescriptor descriptor;
    descriptor.id = get_table_string(L, 2, "id");
    descriptor.version = get_table_string(L, 2, "version");
    descriptor.category = get_table_string(L, 2, "category");
    descriptor.risk = get_table_string(L, 2, "risk");
    descriptor.entity = get_table_string(L, 2, "entity");
    descriptor.operation = get_table_string(L, 2, "operation");
    descriptor.enabled = get_table_bool(L, 2, "enabled", true);

    // 验证描述符
    if (descriptor.id.empty()) {
        return push_error(L, "function id is required");
    }
    if (descriptor.version.empty()) {
        return push_error(L, "function version is required");
    }
    if (descriptor.category.empty()) {
        return push_error(L, "function category is required");
    }

    // 注册到 C++ 客户端
    try {
        lc->client->RegisterFunction(descriptor,
            [L, lc](const FunctionContext& ctx, const std::string& payload) -> std::string {
                // 调用 Lua 回调
                lua_rawgeti(L, LUA_REGISTRYINDEX, lc->callback_ref);

                // 推送参数
                lua_pushstring(L, ctx.function_id.c_str());
                lua_pushstring(L, ctx.call_id.c_str());
                lua_pushstring(L, payload.c_str());

                // 调用 Lua 处理器
                int result = lua_pcall(L, 3, 2, 0);

                if (result != LUA_OK) {
                    const char* err = lua_tostring(L, -1);
                    lua_pop(L, 1);
                    throw std::runtime_error(err);
                }

                // 获取结果
                if (lua_isnil(L, -2)) {
                    // 第二个值是错误
                    const char* err = lua_tostring(L, -1);
                    lua_pop(L, 2);
                    throw std::runtime_error(err);
                }

                const char* ret = lua_tostring(L, -2);
                std::string result = ret ? ret : "";
                lua_pop(L, 2);
                return result;
            }
        );

        lua_pushboolean(L, true);
        return 1;
    } catch (const std::exception& e) {
        return push_error(L, e.what());
    }
}

// 连接
// client:connect() -> ok, err
static int connect(lua_State* L) {
    auto* lc = check_lua_client(L);

    try {
        lc->client->Connect();
        lua_pushboolean(L, true);
        return 1;
    } catch (const std::exception& e) {
        return push_error(L, e.what());
    }
}

// 断开连接
// client:disconnect()
static int disconnect(lua_State* L) {
    auto* lc = check_lua_client(L);

    lc->client->Disconnect();

    return 0;
}

// 开始服务
// client:serve(callback_function)
static int serve(lua_State* L) {
    auto* lc = check_lua_client(L);

    luaL_checktype(L, 2, LUA_TFUNCTION);

    // 保存回调引用
    lua_pushvalue(L, 2);
    int callback_ref = luaL_ref(L, LUA_REGISTRYINDEX);
    lc->callback_ref = callback_ref;

    lc->serving = true;

    // 启动服务（阻塞）
    try {
        lc->client->Serve();
        lc->serving = false;
        return 0;
    } catch (const std::exception& e) {
        lc->serving = false;
        luaL_unref(L, LUA_REGISTRYINDEX, lc->callback_ref);
        lc->callback_ref = LUA_NOREF;
        return push_error(L, e.what());
    }
}

// 停止服务
// client:stop()
static int stop(lua_State* L) {
    auto* lc = check_lua_client(L);

    lc->client->Stop();
    lc->serving = false;

    // 清理回调引用
    if (lc->callback_ref != LUA_NOREF) {
        luaL_unref(L, LUA_REGISTRYINDEX, lc->callback_ref);
        lc->callback_ref = LUA_NOREF;
    }

    return 0;
}

// 同步调用
// client:invoke(function_id, payload, options_table) -> result, err
static int invoke(lua_State* L) {
    auto* lc = check_lua_client(L);

    const char* function_id = luaL_checkstring(L, 2);
    const char* payload = lua_tostring(L, 3);  // 可选

    InvokeOptions options;
    options.game_id = _config.game_id;
    options.env = _config.env;
    options.timeout_seconds = 30;

    if (lua_istable(L, 4)) {
        options.game_id = get_table_string(L, 4, "game_id", options.game_id.c_str());
        options.env = get_table_string(L, 4, "env", options.env.c_str());
        options.timeout_seconds = get_table_int(L, 4, "timeout", 30);
        options.idempotency_key = get_table_string(L, 4, "idempotency_key", "");
    }

    try {
        std::string result = lc->client->Invoke(function_id, payload ? payload : "", options);
        lua_pushstring(L, result.c_str());
        return 1;
    } catch (const std::exception& e) {
        return push_error(L, e.what());
    }
}

// 启动任务
// client:start_job(function_id, payload, options_table) -> job_id, err
static int start_job(lua_State* L) {
    auto* lc = check_lua_client(L);

    const char* function_id = luaL_checkstring(L, 2);
    const char* payload = lua_tostring(L, 3);

    InvokeOptions options;
    options.game_id = _config.game_id;
    options.env = _config.env;
    options.timeout_seconds = 30;

    if (lua_istable(L, 4)) {
        options.game_id = get_table_string(L, 4, "game_id", options.game_id.c_str());
        options.env = get_table_string(L, 4, "env", options.env.c_str());
        options.timeout_seconds = get_table_int(L, 4, "timeout", 30);
        options.idempotency_key = get_table_string(L, 4, "idempotency_key", "");
    }

    try {
        std::string job_id = lc->client->StartJob(function_id, payload ? payload : "", options);
        lua_pushstring(L, job_id.c_str());
        return 1;
    } catch (const std::exception& e) {
        return push_error(L, e.what());
    }
}

// 取消任务
// client:cancel_job(job_id) -> ok, err
static int cancel_job(lua_State* L) {
    auto* lc = check_lua_client(L);

    const char* job_id = luaL_checkstring(L, 2);

    try {
        bool cancelled = lc->client->CancelJob(job_id);
        lua_pushboolean(L, cancelled);
        return 1;
    } catch (const std::exception& e) {
        return push_error(L, e.what());
    }
}

// GC 清理
static int gc(lua_State* L) {
    auto* lc = check_lua_client(L);

    if (lc->callback_ref != LUA_NOREF) {
        luaL_unref(L, LUA_REGISTRYINDEX, lc->callback_ref);
    }

    if (lc->client) {
        delete lc->client;
        lc->client = nullptr;
    }

    return 0;
}

// ===== 库初始化 =====

static const struct luaL_Reg croupier_lib[] = {
    {"create_client", create_client},
    {NULL, NULL}
};

static const struct luaL_Reg client_methods[] = {
    {"register_function", register_function},
    {"connect", connect},
    {"disconnect", disconnect},
    {"serve", serve},
    {"stop", stop},
    {"invoke", invoke},
    {"start_job", start_job},
    {"cancel_job", cancel_job},
    {"is_connected", [](lua_State* L) {
        auto* lc = check_lua_client(L);
        lua_pushboolean(L, lc->client->IsConnected());
        return 1;
    }},
    {"get_local_address", [](lua_State* L) {
        auto* lc = check_lua_client(L);
        lua_pushstring(L, lc->client->GetLocalAddress().c_str());
        return 1;
    }},
    {NULL, NULL}
};

static const struct luaL_Reg client_metamethods[] = {
    {"__gc", gc},
    {"__close", stop},  // 支持 to-be-closed 协议
    {NULL, NULL}
};

// 注册元方法
extern "C" {
    int luaopen_croupier_core(lua_State* L) {
        // 创建客户端元表
        luaL_newmetatable(L, "CroupierClient");
        lua_pushvalue(L, -1);
        lua_setfield(L, -2, "__index");
        luaL_setfuncs(L, client_methods, 0);
        luaL_setfuncs(L, client_metamethods, 0);
        lua_pop(L, 1);

        // 创建库
        luaL_newlib(L, croupier_lib);
        return 1;
    }
}

} // namespace lua
} // namespace croupier
```

### 5. Lua SDK 模块

```lua
-- croupier-sdk-cpp/lua/croupier/init.lua
local croupier = {}

-- 加载 C 核心库
local ok, core = pcall(require, "croupier.core")
if not ok then
    error("Failed to load croupier.core: " .. core)
end

croupier._core = core

-- 导出模块
croupier.Client = require "croupier.client"
croupier.Invoker = require "croupier.invoker"
croupier.types = require "croupier.types"

return croupier
```

```lua
-- croupier-sdk-cpp/lua/croupier/client.lua
local core = require "croupier.core"

local Client = {}
Client.__index = Client

function Client.new(config)
    local self = setmetatable({}, Client)

    -- 创建核心客户端
    self._handle = core.create_client({
        agent_addr = config.agent_addr or "127.0.0.1:19090",
        service_id = config.service_id or "lua-service",
        service_version = config.service_version or "1.0.0",
        game_id = config.game_id or "default-game",
        env = config.env or "dev",
        local_addr = config.local_addr or "0.0.0.0:0",
        insecure = config.insecure or false,
        cert_file = config.cert_file,
        key_file = config.key_file,
        ca_file = config.ca_file,
        timeout = config.timeout or 30,
        heartbeat_interval = config.heartbeat_interval or 30,
        auto_reconnect = config.auto_reconnect,
        reconnect_interval = config.reconnect_interval,
        reconnect_max_attempts = config.reconnect_max_attempts,
    })

    self._functions = {}
    self._connected = false
    self._serving = false

    return self
end

function Client:register_function(descriptor, handler)
    if type(descriptor) ~= "table" then
        error("descriptor must be a table")
    end

    -- 验证必需字段
    local required = {"id", "version", "category", "risk"}
    for _, field in ipairs(required) do
        if not descriptor[field] then
            error(string.format("descriptor.%s is required", field))
        end
    end

    -- 存储处理器
    self._functions[descriptor.id] = handler

    -- 通过 C API 注册
    local ok, err = self._handle:register_function(descriptor)
    if not ok then
        self._functions[descriptor.id] = nil
        error(string.format("register function failed: %s", err))
    end

    -- 设置回调
    self._handle:set_function_handler(descriptor.id, function(context, payload)
        -- 解析 payload
        local payload_obj
        if payload and payload ~= "" then
            local ok, decoded = pcall(cjson.decode, payload)
            if ok then
                payload_obj = decoded
            else
                payload_obj = payload
            end
        end

        -- 调用处理器
        local ok, result = pcall(handler, context, payload_obj)

        if not ok then
            return nil, result  -- 错误信息
        end

        -- 序列化结果
        if type(result) == "table" then
            return cjson.encode(result)
        end

        return tostring(result)
    end)

    return true
end

function Client:connect()
    if self._connected then
        return true
    end

    local ok, err = self._handle:connect()
    if not ok then
        return false, err
    end

    self._connected = true
    return true
end

function Client:serve()
    if not self._connected then
        local ok, err = self:connect()
        if not ok then
            error(err)
        end
    end

    self._serving = true

    -- 启动服务（阻塞）
    local ok, err = self._handle:serve()
    self._serving = false

    if not ok then
        error(err)
    end
end

function Client:stop()
    if not self._serving then
        return
    end

    self._serving = false
    self._handle:stop()
end

function Client:disconnect()
    if not self._connected then
        return
    end

    self:stop()
    self._handle:disconnect()
    self._connected = false
end

function Client:is_connected()
    return self._connected
end

return Client
```

### 6. Skynet 服务示例

```lua
-- croupier-sdk-cpp/skynet/service/croupier_service.lua
local skynet = require "skynet"
local croupier = require "croupier"

local M = {}
M.__index = M

function M.new()
    local self = setmetatable({}, M)

    -- 从环境变量读取配置
    self.config = {
        agent_addr = skynet.getenv("CROUPIER_AGENT_ADDR") or "127.0.0.1:19090",
        service_id = skynet.getenv("SERVICE_ID") or "skynet-service",
        game_id = skynet.getenv("GAME_ID") or "default-game",
        env = skynet.getenv("ENV") or "dev",
        insecure = skynet.getenv("CROUPIER_INSECURE") == "true",
    }

    -- 创建客户端
    self.client = croupier.Client.new(self.config)

    return self
end

function M:register(descriptor, handler)
    return self.client:register_function(descriptor, function(context, payload)
        -- 在 Skynet 上下文中执行
        return handler(context, payload)
    end)
end

function M:start()
    local ok, err = self.client:connect()
    if not ok then
        skynet.error("croupier connect failed:", err)
        return false
    end

    skynet.error("croupier connected")

    -- 启动服务（在独立协程中）
    skynet.fork(function()
        self.client:serve()
    end)

    return true
end

function M:invoke(function_id, payload)
    return self.client:invoke(function_id, payload)
end

-- 启动服务
skynet.start(function()
    local service = M.new()

    -- 注册示例函数
    service:register({
        id = "player.get",
        version = "1.0.0",
        category = "player",
        risk = "low",
    }, function(context, payload)
        skynet.error("player.get called:", payload)

        -- 调用其他 Skynet 服务
        local player = skynet.call(".player_mgr", "lua", "get_player", payload.player_id)

        return {
            status = "success",
            player = player
        }
    end)

    -- 启动服务
    service:start()

    skynet.info("croupier service started")
end)

return M
```

### 7. 编译和使用

```bash
# 编译（启用 Lua 绑定）
cd croupier-sdk-cpp
mkdir build && cd build
cmake .. -DENABLE_LUA_BINDING=ON -DCMAKE_BUILD_TYPE=Release
make -j$(nproc)

# 输出文件：
# Linux: libcroupier.so
# Windows: croupier.dll

# 安装到 Skynet
cp libcroupier.so /path/to/skynet/cservice/
cp -r lua/croupier /path/to/skynet/lua/
```

## 配置选项对比

| 选项 | 说明 |
|------|------|
| `ENABLE_LUA_BINDING=ON` | 启用 Lua 绑定，生成 `libcroupier.so` |
| `LUA_INCLUDE_DIR` | Lua 头文件路径（自动查找） |
| `LUA_LIBRARIES` | Lua 库名（自动查找） |
| `LUA_VERSION_MAJOR` | Lua 主版本号（需要 5.3+） |

## 优势总结

与独立 `croupier-sdk-lua` 仓库相比，集成方案优势明显：

| 对比项 | 独立仓库 | 集成到 croupier-sdk-cpp |
|--------|----------|---------------------|
| 代码复用 | ❌ 需要复制 C++ 代码 | ✅ 直接复用 |
| 维护成本 | ❌ 需要同步更新 | ✅ 统一维护 |
| 构建复杂度 | ❌ 需要 C++ + Lua 两套构建系统 | ✅ 一个 CMake 系统 |
| 版本一致性 | ❌ 容易出现版本不同步 | ✅ 自动保持一致 |
| 发布流程 | ❌ 需要分别发布 | ✅ 一次发布，多语言可用 |

## 实现状态

### 已实现文件

| 文件路径 | 状态 | 说明 |
|---------|------|------|
| `croupier-sdk-cpp/CMakeLists.txt` | ✅ 已修改 | 添加 `ENABLE_LUA_BINDING` 选项 |
| `croupier-sdk-cpp/src/bindings/lua_binding.h` | ✅ 已创建 | Lua 绑定头文件 |
| `croupier-sdk-cpp/src/bindings/lua_binding.cpp` | ✅ 已创建 | Lua C API 绑定实现 |
| `croupier-sdk-cpp/lua/croupier/init.lua` | ✅ 已创建 | Lua 模块入口 |
| `croupier-sdk-cpp/skynet/service/croupier_service.lua` | ✅ 已创建 | Skynet 服务封装 |
| `croupier-sdk-cpp/skynet/examples/config.lua` | ✅ 已创建 | Skynet 配置示例 |
| `croupier-sdk-cpp/skynet/examples/main.lua` | ✅ 已创建 | Skynet 主服务示例 |
| `croupier-sdk-cpp/lua/examples/standalone_example.lua` | ✅ 已创建 | 独立 Lua 示例 |

### 编译命令

```bash
# 启用 Lua 绑定编译
cd croupier-sdk-cpp
mkdir build && cd build
cmake .. -DENABLE_LUA_BINDING=ON -DBUILD_SHARED_LIBS=ON
make -j$(nproc)

# 输出: bin/libcroupier-sdk.so (包含 Lua API)
```

### 使用示例

```lua
-- 独立 Lua 使用
local croupier = require "croupier"
local client = croupier.Client.new("localhost:50051")
client:register_virtual_object("player:1001", "player", {level = 50})

-- Skynet 中使用
local croupier_service = skynet.call(".croupier", "lua", "start", "localhost:50051")
skynet.call(".croupier", "lua", "register_vo", "player:1001", "player", {level = 50})
```
