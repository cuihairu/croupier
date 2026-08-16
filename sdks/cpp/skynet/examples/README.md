# Croupier SDK Skynet Examples

这些示例展示 Skynet 服务如何通过 Croupier C++ SDK 注册函数。示例不包含对象注册、对象查询或 UI 配置。

## 前置要求

```bash
cd sdks/cpp
cmake -B build \
  -DCMAKE_TOOLCHAIN_FILE=./cmake/vcpkg-bootstrap.cmake \
  -DENABLE_LUA_BINDING=ON \
  -DBUILD_SHARED_LIBS=ON
cmake --build build --parallel
```

## 安装示例

```bash
export SKYNET_PATH=/tmp/skynet
cd sdks/cpp/skynet/examples
./start.sh
```

## 运行

```bash
cd /tmp/skynet/croupier-sdk
./run.sh
```

## 文件

| 文件                 | 说明                                         |
| -------------------- | -------------------------------------------- |
| `simple_example.lua` | 启动服务、注册一个函数、连接 Agent、查询状态 |
| `main.lua`           | 注册多个游戏后台函数并暴露 Skynet 控制命令   |
| `config.lua`         | Skynet 配置                                  |
| `start.sh`           | 安装脚本                                     |

## 服务命令

| 命令                | 参数                         | 说明                          |
| ------------------- | ---------------------------- | ----------------------------- |
| `start`             | `address, auth_token`        | 创建 Croupier Provider Client |
| `register_function` | `function_id, response_json` | 注册示例函数                  |
| `connect`           | 无                           | 连接 Croupier Agent           |
| `status`            | 无                           | 查看服务状态                  |
| `stop`              | 无                           | 关闭客户端                    |

## 函数注册示例

```lua
local croupier_service = skynet.newservice("croupier_service")

skynet.call(croupier_service, "lua", "start", "127.0.0.1:19091")
skynet.call(
    croupier_service,
    "lua",
    "register_function",
    "mail.send",
    [[{"mail_id":"mail_demo_001","status":"queued"}]]
)
skynet.call(croupier_service, "lua", "connect")
```
