# Croupier Python SDK

[![PyPI Version](https://img.shields.io/pypi/v/croupier-sdk)](https://pypi.org/project/croupier-sdk/)
[![Python Version](https://img.shields.io/pypi/pyversions/croupier-sdk)](https://pypi.org/project/croupier-sdk/)
[![License](https://img.shields.io/github/license/cuihairu/croupier-sdk-python)](https://github.com/cuihairu/croupier-sdk-python/blob/main/LICENSE)

🎯 **Croupier Python SDK** - 适用于游戏功能注册的高性能异步 Python SDK，支持热重载、自动重连和无缝集成。

## ✨ 核心特性

- 🔥 **热重载支持** - 文件变更自动重载，无需重启服务
- 🔄 **自动重连** - 网络断开自动重连，确保服务稳定性
- ⚡ **异步架构** - 基于 asyncio 的完全异步实现
- 🛠️ **工具集成** - 无缝集成 Uvicorn、Gunicorn、FastAPI
- 📊 **监控指标** - 内置性能指标和健康检查
- 🐍 **类型安全** - 完整的 TypeScript 类型定义

## 🚀 快速开始

### 安装

```bash
pip install croupier-sdk[web,monitoring]
```

### 基础使用

```python
import asyncio
from croupier import create_hotreload_client

# 创建客户端配置
config = {
    "enabled": True,
    "auto_reconnect": True,
    "file_watching": {
        "enabled": True,
        "watch_dir": "./functions"
    }
}

# 创建热重载客户端
client = create_hotreload_client(config)

# 定义游戏函数
async def player_ban(context: str, payload: str) -> str:
    # 实现玩家封禁逻辑
    return f'{{"status": "success", "player_id": "{payload}"}}'

async def wallet_transfer(context: str, payload: str) -> str:
    # 实现钱包转账逻辑
    return f'{{"status": "success", "transaction_id": "tx_12345"}}'

async def main():
    # 注册函数
    client.register_function("player.ban", "1.0.0", player_ban)
    client.register_function("wallet.transfer", "1.0.0", wallet_transfer)

    # 连接到 Agent
    await client.connect()

    # 保持运行
    await client.shutdown_event.wait()

if __name__ == "__main__":
    asyncio.run(main())
```

### 与 FastAPI 集成

```python
from fastapi import FastAPI
from croupier import hotreload_client

app = FastAPI()

@app.on_event("startup")
async def startup():
    config = {
        "enabled": True,
        "auto_reconnect": True,
        "tools": {
            "uvicorn": True,
            "watchdog": True
        }
    }

    async with hotreload_client(config) as client:
        # 注册游戏函数
        await setup_game_functions(client)

async def setup_game_functions(client):
    """设置游戏函数"""
    client.register_function("shop.buy", "1.0.0", shop_buy_handler)
    client.register_function("player.create", "1.0.0", player_create_handler)
```

## 🛠️ 热重载开发模式

### 启用文件监听

```python
config = {
    "file_watching": {
        "enabled": True,
        "watch_dir": "./game_functions",
        "patterns": ["*.py", "*.json", "*.yaml"]
    },
    "tools": {
        "uvicorn": True,        # Uvicorn 开发服务器集成
        "watchdog": True,       # 文件监听
        "importlib_reload": True # 模块热重载
    }
}
```

### 开发服务器启动

```bash
# 使用 Uvicorn + 热重载
uvicorn main:app --reload --host 0.0.0.0 --port 8000

# 使用 Gunicorn (生产环境)
gunicorn -w 4 -k uvicorn.workers.UvicornWorker main:app
```

## 📊 监控与指标

```python
# 获取热重载状态
status = client.get_reload_status()

print(f"连接状态: {status.connection_status}")
print(f"重连次数: {status.reconnect_count}")
print(f"函数重载次数: {status.function_reloads}")
print(f"运行时间: {status.uptime:.2f}s")
```

## 🔧 高级配置

### 完整配置示例

```python
from croupier import HotReloadConfig

config = HotReloadConfig(
    enabled=True,
    auto_reconnect=True,
    reconnect_delay=5.0,
    max_retry_attempts=10,
    health_check_interval=30.0,
    graceful_shutdown_timeout=30.0,
    file_watching={
        "enabled": True,
        "watch_dir": "./functions",
        "patterns": ["*.py", "*.json", "*.yaml"]
    },
    tools={
        "uvicorn": True,
        "watchdog": True,
        "importlib_reload": True
    }
)
```

### 环境变量配置

```bash
# 基础配置
CROUPIER_AGENT_ADDR=127.0.0.1:19090
CROUPIER_GAME_ID=my-game
CROUPIER_ENV=development

# 热重载配置
CROUPIER_HOT_RELOAD_ENABLED=true
CROUPIER_AUTO_RECONNECT=true
CROUPIER_WATCH_DIR=./functions
```

## 📖 API 文档

### 核心类

#### `HotReloadableClient`

异步热重载客户端主类。

**方法:**
- `register_function(function_id, version, handler)` - 注册函数
- `connect()` - 连接到 Agent
- `reload_function(function_id, version, handler)` - 重载单个函数
- `reload_functions(functions)` - 批量重载函数
- `graceful_shutdown(timeout)` - 优雅关闭

#### `HotReloadConfig`

热重载配置类。

**属性:**
- `enabled: bool` - 是否启用热重载
- `auto_reconnect: bool` - 是否自动重连
- `file_watching: dict` - 文件监听配置
- `tools: dict` - 工具集成配置

## 🎮 示例项目

查看 `examples/` 目录获取完整示例：

- **基础示例** - 简单的函数注册和热重载
- **FastAPI 集成** - 与 Web 框架集成
- **监控示例** - 指标收集和监控
- **生产部署** - 生产环境配置示例

## 🧪 测试

```bash
# 运行测试
python -m pytest tests/ -v

# 测试覆盖率
python -m pytest tests/ --cov=croupier --cov-report=html

# 类型检查
mypy croupier/

# 代码格式化
black croupier/ tests/
```

## 📝 更新日志

### v1.0.0 (2024-11-15)

- ✨ 初始发布
- 🔥 热重载支持
- ⚡ 异步架构
- 🛠️ 工具集成
- 📊 监控指标

## 🤝 贡献

欢迎贡献代码！请查看 [CONTRIBUTING.md](CONTRIBUTING.md) 了解详情。

## 📄 许可证

MIT License - 查看 [LICENSE](LICENSE) 文件了解详情。

## 🔗 相关链接

- [Croupier 主项目](https://github.com/cuihairu/croupier)
- [文档](https://docs.croupier.io)
- [API 参考](https://docs.croupier.io/api/python)
- [问题反馈](https://github.com/cuihairu/croupier-sdk-python/issues)

---

Made with ❤️ by the Croupier Team