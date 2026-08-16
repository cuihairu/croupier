# Croupier SDK for Skynet

本目录提供 Croupier C++ SDK 的 Skynet 集成。Skynet 集成只包装函数 Provider 生命周期，不引入对象管理、菜单或 UI 配置。

## 边界

- `croupier_service.lua` 只负责创建 Provider Client、注册函数、连接 Agent、停止客户端。
- 函数 ID、资源、操作、风险等能力契约由 SDK 层 `FunctionDescriptor` 表达。
- Skynet 示例不注册页面、菜单、分类或对象管理配置。

## 构建

```bash
cd sdks/cpp
cmake -B build \
  -DCMAKE_TOOLCHAIN_FILE=./cmake/vcpkg-bootstrap.cmake \
  -DENABLE_LUA_BINDING=ON \
  -DBUILD_SHARED_LIBS=ON
cmake --build build --parallel
```

## 安装到 Skynet

```bash
export SKYNET_PATH=/tmp/skynet
cd sdks/cpp/skynet/examples
./start.sh
```

脚本会把动态库、Lua 服务和示例复制到 `$SKYNET_PATH/croupier-sdk`。

## 服务 API

| 命令                | 参数                         | 返回值           | 说明                             |
| ------------------- | ---------------------------- | ---------------- | -------------------------------- |
| `start`             | `address, auth_token`        | `boolean, error` | 创建 Croupier Provider Client    |
| `register_function` | `function_id, response_json` | `boolean, error` | 注册一个返回固定 JSON 的示例函数 |
| `connect`           | 无                           | `boolean, error` | 连接 Agent 并提交已注册函数      |
| `serve`             | 无                           | `boolean, error` | 阻塞服务循环                     |
| `status`            | 无                           | `table`          | 返回启动状态和注册函数数量       |
| `stop`              | 无                           | `boolean, error` | 关闭客户端                       |

## 示例

```lua
local skynet = require "skynet"
local croupier_service = skynet.newservice("croupier_service")

local ok, err = skynet.call(croupier_service, "lua", "start", "127.0.0.1:19091")
if not ok then
    error(err)
end

skynet.call(
    croupier_service,
    "lua",
    "register_function",
    "player.profile.get",
    [[{"player_id":"p1","level":10}]]
)

skynet.call(croupier_service, "lua", "connect")
```

## 示例文件

| 文件                          | 说明                         |
| ----------------------------- | ---------------------------- |
| `examples/simple_example.lua` | 最小函数注册示例             |
| `examples/main.lua`           | 多函数注册和状态查询示例     |
| `examples/config.lua`         | Skynet 配置示例              |
| `examples/start.sh`           | 安装到本地 Skynet 的辅助脚本 |
