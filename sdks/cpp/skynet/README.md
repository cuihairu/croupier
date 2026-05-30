# Croupier SDK for Skynet

本目录包含 Croupier C++ SDK 与 [Skynet](https://github.com/cloudwu/skynet) 框架的集成代码和示例。

## 什么是 Skynet？

Skynet 是一个轻量级的在线游戏服务框架，使用 C + Lua 编写。它被广泛用于 MMO 游戏、社交游戏等领域。

## Croupier SDK Skynet 集成

本集成提供了：
- **C 服务** (`libcroupier-sdk.so`): Skynet C 服务，封装了 Croupier C++ SDK
- **Lua 服务** (`croupier_service.lua`): Skynet Lua 服务，提供易于使用的 Lua API
- **完整示例**: 展示如何在 Skynet 中使用 Croupier SDK

## 目录结构

```
skynet/
├── README.md                   # 本文档
├── service/
│   └── croupier_service.lua    # Skynet Lua 服务
└── examples/
    ├── README.md               # 详细文档和示例说明
    ├── config.lua              # Skynet 配置
    ├── main.lua                # 完整示例
    ├── simple_example.lua      # 简单入门示例
    └── start.sh                # 自动设置脚本
```

## 快速开始

### 方法 1: 使用自动设置脚本（推荐）

```bash
cd croupier-sdk-cpp/skynet/examples
./start.sh
```

这个脚本会自动：
1. 检查编译好的库文件
2. 复制所有必需文件到 Skynet 目录
3. 创建配置文件
4. 提供运行脚本

### 方法 2: 手动设置

#### 1. 编译 Croupier SDK with Lua 支持

```bash
cd croupier-sdk-cpp
cmake -B build -DCMAKE_TOOLCHAIN_FILE=./cmake/vcpkg-bootstrap.cmake \
  -DENABLE_LUA_BINDING=ON \
  -DBUILD_SHARED_LIBS=ON
cmake --build build
```

#### 2. 安装 Skynet

```bash
cd /tmp
git clone https://github.com/cloudwu/skynet.git
cd skynet
make
```

#### 3. 复制文件到 Skynet

```bash
# 设置变量
SDK_ROOT=/path/to/croupier-sdk-cpp
SKYNET_PATH=/tmp/skynet

# 复制 .so 文件
mkdir -p $SKYNET_PATH/cservice
cp $SDK_ROOT/build/lib/libcroupier-sdk.so $SKYNET_PATH/cservice/

# 复制 Lua 模块
mkdir -p $SKYNET_PATH/luaclib
cp -r $SDK_ROOT/lua/* $SKYNET_PATH/luaclib/

# 复制服务文件
cp $SDK_ROOT/skynet/service/croupier_service.lua $SKYNET_PATH/service/
```

#### 4. 运行示例

```bash
cd $SKYNET_PATH

# 复制示例配置
cp $SDK_ROOT/skynet/examples/config.lua .

# 运行简单示例
./skynet $SDK_ROOT/skynet/examples/simple_example.lua

# 或运行完整示例
./skynet $SDK_ROOT/skynet/examples/main.lua
```

## 核心 API

### 启动 Croupier 服务

```lua
local skynet = require "skynet"

-- 启动 Croupier 服务
local croupier_service = ".croupier"
skynet.newservice("croupier_service")

-- 连接到 Croupier Server
local ok = skynet.call(croupier_service, "lua", "start", "localhost:50051")
```

### 注册虚拟对象

```lua
local vo_id = "player:1001"
local class_id = "player"
local properties = {
    level = 50,
    vip_level = 3,
    coins = 10000
}

local ok, err = skynet.call(croupier_service, "lua", "register_vo",
    vo_id, class_id, properties)
```

### 调用函数

```lua
-- 单次调用
local ok, result = skynet.call(croupier_service, "lua", "invoke",
    vo_id, "get_balance", {currency = "coins"})

-- 批量调用
local requests = {
    {vo_id = "player:1001", function_name = "get_balance", args = {currency = "coins"}},
    {vo_id = "player:1001", function_name = "get_level", args = {}}
}
local ok, results = skynet.call(croupier_service, "lua", "batch_invoke", requests)
```

### 查询虚拟对象

```lua
-- 获取单个虚拟对象
local ok, vo = skynet.call(croupier_service, "lua", "get_vo", "player:1001")

-- 列出虚拟对象
local ok, vos = skynet.call(croupier_service, "lua", "list_vos", "player")
```

### 服务状态和关闭

```lua
-- 查看状态
local status = skynet.call(croupier_service, "lua", "status")
print("Requests:", status.request_count)

-- 关闭服务
skynet.call(croupier_service, "lua", "stop")
```

## 完整 API 参考

### 服务命令

| 命令 | 参数 | 返回值 | 说明 |
|------|------|--------|------|
| `start` | `address, auth_token` | `boolean` | 启动 Croupier 客户端 |
| `stop` | 无 | `boolean` | 停止客户端 |
| `status` | 无 | `table` | 获取服务状态 |
| `register_vo` | `vo_id, class_id, properties` | `boolean, error` | 注册虚拟对象 |
| `unregister_vo` | `vo_id` | `boolean, error` | 注销虚拟对象 |
| `invoke` | `vo_id, function_name, args` | `boolean, result` | 调用函数 |
| `batch_invoke` | `requests` | `boolean, results` | 批量调用 |
| `get_vo` | `vo_id` | `boolean, vo` | 获取虚拟对象 |
| `list_vos` | `class_id` | `boolean, vos` | 列出虚拟对象 |

## 使用场景

### 场景 1: 游戏服务器

```lua
-- 游戏服务器启动时
skynet.start(function()
    -- 连接到 Croupier Server
    skynet.call(".croupier", "lua", "start", "croupier-server:50051")

    -- 注册游戏服务器
    skynet.call(".croupier", "lua", "register_vo",
        "game-server-001", "game-server", {
            max_players = 1000,
            current_players = 0
        })
end)

-- 玩家登录时
function on_player_login(player_id)
    -- 注册玩家虚拟对象
    skynet.call(".croupier", "lua", "register_vo",
        "player:"..player_id, "player", {
            login_time = os.time(),
            level = 1
        })
end
```

### 场景 2: 跨服通信

```lua
-- 从其他服务器调用函数
local function call_remote_server(server_id, func, args)
    local ok, result = skynet.call(".croupier", "lua", "invoke",
        "game-server:"..server_id, func, args)

    if ok then
        return result
    else
        skynet.error("Remote call failed:", result)
        return nil
    end
end

-- 获取其他服务器的玩家数量
local player_count = call_remote_server("server-001", "get_player_count", {})
```

### 场景 3: 玩家数据同步

```lua
-- 批量同步玩家数据到多个系统
local function sync_player_data(player_id)
    local requests = {
        {vo_id = "player:"..player_id, function_name = "get_inventory", args = {}},
        {vo_id = "player:"..player_id, function_name = "get_friends", args = {}},
        {vo_id = "player:"..player_id, function_name = "get_achievements", args = {}}
    }

    local ok, results = skynet.call(".croupier", "lua", "batch_invoke", requests)

    if ok then
        -- 处理结果
        for i, result in ipairs(results) do
            process_data(result)
        end
    end
end
```

## 性能考虑

### 1. 连接复用

```lua
-- 推荐：全局唯一服务
skynet.newservice("croupier_service")

-- 所有游戏服务共享同一个连接
```

### 2. 批量操作

```lua
-- 推荐：批量调用
local requests = {}
for i, player_id in ipairs(player_ids) do
    table.insert(requests, {
        vo_id = "player:"..player_id,
        function_name = "save_data",
        args = {}
    })
end
skynet.call(".croupier", "lua", "batch_invoke", requests)
```

### 3. 异步处理

```lua
-- 推荐：使用 skynet.fork 进行异步调用
skynet.fork(function()
    local ok, result = skynet.call(".croupier", "lua", "invoke", ...)
    -- 处理结果
end)
```

## 配置

### Skynet 配置 (config.lua)

```lua
-- C 服务路径
cpath = root.."cservice/?.so;"..root.."../build/bin/?.so"

-- Lua 服务路径
luaservice = root.."service/?.lua;"..root.."examples/?.lua"

-- Lua 模块路径
lua_path = root.."luaclib/?.lua;"..root.."luaclib/?/init.lua"
lua_cpath = root.."luaclib/?.so"

-- Croupier 服务器（可选，也可通过环境变量）
croupier_server = "localhost:50051"
auth_token = nil
```

### 环境变量

```bash
# 设置 Croupier 服务器地址
export CROUPIER_SERVER=croupier.example.com:50051

# 设置认证令牌
export AUTH_TOKEN=your-auth-token
```

## 故障排除

### 问题: 找不到 libcroupier-sdk.so

```bash
# 检查文件是否存在
ls -la $SKYNET_PATH/cservice/libcroupier-sdk.so

# 检查配置文件中的 cpath
grep cpath config.lua
```

### 问题: Lua 模块加载失败

```bash
# 检查 Lua 模块路径
ls -la $SKYNET_PATH/luaclib/croupier/

# 确保 lua_cpath 配置正确
```

### 问题: 连接 Croupier Server 失败

```bash
# 检查服务器是否运行
telnet localhost 50051

# 检查防火墙设置
sudo iptables -L -n | grep 50051
```

## 更多资源

- [示例详细文档](examples/README.md)
- [Skynet 官方文档](https://github.com/cloudwu/skynet/wiki)
- [Croupier SDK 文档](../README.md)

## 贡献

欢迎提交问题和改进建议！

## 许可证

Copyright 2025 Croupier Authors
