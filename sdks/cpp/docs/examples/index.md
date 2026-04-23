# 使用示例

本章节提供 Croupier C++ SDK 的各种使用示例。

## 示例列表

- [基础函数](./basic-function.md) - 最简单的函数注册示例
- [虚拟对象](./virtual-object.md) - 钱包虚拟对象完整示例
- [插件](./plugin.md) - 动态加载插件示例
- [综合示例](./comprehensive.md) - 完整的游戏服务端示例

## 运行示例

### 构建示例

```bash
# 克隆仓库
git clone https://github.com/cuihairu/croupier-sdk-cpp.git
cd croupier-sdk-cpp

# 构建（包含示例）
./scripts/build.sh --examples ON

# 或者使用 CMake
cmake -B build -DBUILD_EXAMPLES=ON
cmake --build build
```

### 运行示例

```bash
# 确保 Agent 已启动
./build/bin/croupier-example
./build/bin/croupier-virtual-object-demo
./build/bin/croupier-plugin-demo
./build/bin/croupier-comprehensive-demo
```

## 前置准备

在运行示例前，需要：

1. **启动 Croupier Server**
```bash
cd /path/to/croupier/server
go run ./services/server -f services/server/etc/server.yaml
```

2. **启动 Croupier Agent**
```bash
cd /path/to/croupier/server
go run ./services/agent -f services/agent/etc/agent.yaml
```

3. **验证连接**
```bash
# Agent 默认监听 127.0.0.1:19090
curl http://127.0.0.1:19090/health
```

## 示例配置

所有示例使用以下默认配置：

```json
{
  "game_id": "demo-game",
  "env": "development",
  "service_id": "demo-service",
  "agent_addr": "127.0.0.1:19090",
  "insecure": true
}
```

可以通过环境变量覆盖：

```bash
export CROUPIER_GAME_ID=my-game
export CROUPIER_ENV=production
export CROUPIER_AGENT_ADDR=10.0.0.5:19090

./build/bin/croupier-example
```

## 下一步

查看具体示例了解详细用法：
- [基础函数示例](./basic-function.md)
- [虚拟对象示例](./virtual-object.md)
- [插件示例](./plugin.md)
- [综合示例](./comprehensive.md)
