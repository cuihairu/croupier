# Croupier SDK Skynet 集成示例

本目录展示如何在 Skynet 框架中使用 Croupier C++ SDK。

## 目录结构

```
skynet/
├── examples/
│   ├── README.md           # 本文档
│   ├── config.lua          # Skynet 配置文件
│   ├── main.lua            # 完整示例（多场景演示）
│   ├── simple_example.lua  # 简单入门示例
│   └── start.sh            # 启动脚本
├── service/
│   └── croupier_service.lua # Croupier Skynet 服务
└── README.md               # 总体文档
```

## 前置要求

1. **编译 Croupier SDK with Lua 绑定**

```bash
cd croupier-sdk-cpp
cmake -B build -DCMAKE_TOOLCHAIN_FILE=./cmake/vcpkg-bootstrap.cmake \
  -DENABLE_LUA_BINDING=ON \
  -DBUILD_SHARED_LIBS=ON
cmake --build build
```

2. **安装 Skynet**

```bash
cd /tmp
git clone https://github.com/cloudwu/skynet.git
cd skynet
make
```

## 快速开始

### 1. 设置环境

```bash
# 复制编译好的 .so 文件到 Skynet cservice 目录
cp build/lib/libcroupier-sdk.so /path/to/skynet/cservice/

# 复制 Croupier Lua 模块到 Skynet luaclib 目录
mkdir -p /path/to/skynet/luaclib/
cp -r lua/* /path/to/skynet/luaclib/

# 复制服务文件
cp skynet/service/croupier_service.lua /path/to/skynet/service/
```

### 2. 运行简单示例

```bash
cd /path/to/skynet

# 使用 Croupier SDK 的配置
cp /path/to/croupier-sdk-cpp/skynet/examples/config.lua ./

# 运行简单示例
./skynet /path/to/croupier-sdk-cpp/skynet/examples/simple_example.lua
```

### 3. 运行完整示例

```bash
./skynet /path/to/croupier-sdk-cpp/skynet/examples/main.lua
```

## 示例说明

### simple_example.lua - 入门示例

展示最基本的用法：
- 启动 Croupier 服务
- 注册一个虚拟对象
- 调用函数
- 关闭连接

**适用场景**：第一次使用，快速验证集成是否正确

### main.lua - 完整示例

展示所有功能：
- 服务生命周期管理
- 虚拟对象注册/注销
- 单次和批量函数调用
- 查询虚拟对象
- 控制台命令接口
- 错误处理

**适用场景**：了解所有 API 和生产环境集成

## 核心 API

### 启动服务

```lua
local croupier_service = ".croupier"
skynet.newservice("croupier_service")

-- 启动客户端
local ok = skynet.call(croupier_service, "lua", "start", "localhost:50051", "auth_token")
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

local ok, err = skynet.call(croupier_service, "lua", "register_vo", vo_id, class_id, properties)
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

-- 列出所有虚拟对象
local ok, vos = skynet.call(croupier_service, "lua", "list_vos", "player")
```

### 服务状态

```lua
local status = skynet.call(croupier_service, "lua", "status")
-- status.started: boolean
-- status.server_address: string
-- status.request_count: number
```

### 关闭服务

```lua
skynet.call(croupier_service, "lua", "stop")
```

## 配置选项

在 `config.lua` 中可以配置：

```lua
-- Croupier 服务器地址
croupier_server = "localhost:50051"

-- 认证令牌（可选）
auth_token = "your-auth-token"

-- C 服务路径（包含 libcroupier-sdk.so）
cpath = root.."cservice/?.so;"..root.."../build/bin/?.so"

-- Lua 服务路径
luaservice = root.."service/?.lua;"..root.."examples/?.lua"
```

## 最佳实践

### 1. 服务单例模式

```lua
-- 在全局唯一启动 Croupier 服务
local croupier_service = ".croupier"
skynet.newservice("croupier_service")

-- 其他服务通过已知的服务名调用
local ok = skynet.call(croupier_service, "lua", "start", server_address)
```

### 2. 错误处理

```lua
local ok, result = pcall(function()
    return skynet.call(croupier_service, "lua", "invoke", vo_id, func, args)
end)

if not ok then
    skynet.error("Invoke failed:", result)
    -- 处理错误
end
```

### 3. 异步调用

```lua
-- 使用 skynet.fork 进行异步调用
skynet.fork(function()
    local ok, result = skynet.call(croupier_service, "lua", "register_vo", ...)
    if ok then
        skynet.error("Registration successful")
    end
end)
```

### 4. 连接管理

```lua
-- 在服务启动时连接
skynet.start(function()
    local ok = skynet.call(croupier_service, "lua", "start", server_address)
    if not ok then
        skynet.error("Failed to connect to Croupier server")
        -- 重连逻辑
    end
end)

-- 在服务退出时断开
skynet.dispatch("exit", function()
    skynet.call(croupier_service, "lua", "stop")
end)
```

## 故障排除

### 问题 1: 找不到 libcroupier-sdk.so

**错误信息**: `error loading module 'croupier' from file './cservice/?.so'`

**解决方案**:
```bash
# 确保 .so 文件在正确的路径
ls -la /path/to/skynet/cservice/libcroupier-sdk.so

# 检查 config.lua 中的 cpath 配置
```

### 问题 2: Lua 模块加载失败

**错误信息**: `module 'croupier' not found`

**解决方案**:
```bash
# 检查 Lua 模块路径
ls -la /path/to/skynet/luaclib/croupier/

# 确保 lua_cpath 包含正确路径
```

### 问题 3: 连接服务器失败

**错误信息**: `Failed to connect to localhost:50051`

**解决方案**:
```bash
# 检查 Croupier Server 是否运行
netstat -an | grep 50051

# 验证配置中的服务器地址
# 在 config.lua 中设置正确的 croupier_server
```

### 问题 4: 函数调用超时

**错误信息**: `timeout`

**解决方案**:
```lua
-- 增加超时时间（默认为 200ms）
skynet.call(croupier_service, "lua", "invoke", vo_id, func, args, 5000) -- 5秒超时
```

## 性能优化

### 1. 批量调用

```lua
-- 不推荐：多次单独调用
for i, vo_id in ipairs(vo_ids) do
    skynet.call(croupier_service, "lua", "invoke", vo_id, "get_info", {})
end

-- 推荐：使用批量调用
local requests = {}
for i, vo_id in ipairs(vo_ids) do
    table.insert(requests, {vo_id = vo_id, function_name = "get_info", args = {}})
end
skynet.call(croupier_service, "lua", "batch_invoke", requests)
```

### 2. 连接复用

```lua
-- 不推荐：每次调用都创建新连接
local function handle_request()
    local service = skynet.newservice("croupier_service")
    skynet.call(service, "lua", "start", server)
    -- ...
end

-- 推荐：全局复用连接
skynet.newservice("croupier_service")
-- 所有服务共享同一个连接
```

### 3. 异步处理

```lua
-- 不推荐：阻塞等待
local ok, result = skynet.call(croupier_service, "lua", "invoke", ...)
process_result(result)

-- 推荐：异步回调
skynet.fork(function()
    local ok, result = skynet.call(croupier_service, "lua", "invoke", ...)
    if ok then
        process_result(result)
    end
end)
-- 继续处理其他任务...
```

## 更多资源

- [Skynet 官方文档](https://github.com/cloudwu/skynet/wiki)
- [Croupier SDK C++ 文档](../../README.md)
- [Lua 绑定 API 参考](../../lua/README.md)

## 贡献

欢迎提交问题和改进建议！

## 许可证

Copyright 2025 Croupier Authors
