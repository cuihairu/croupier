# Croupier Python SDK

[![PyPI Version](https://img.shields.io/pypi/v/croupier-sdk)](https://pypi.org/project/croupier-sdk/)
[![Python Version](https://img.shields.io/pypi/pyversions/croupier-sdk)](https://pypi.org/project/croupier-sdk/)
[![License](https://img.shields.io/github/license/cuihairu/croupier-sdk-python)](https://github.com/cuihairu/croupier-sdk-python/blob/main/LICENSE)

🎯 **Croupier Python SDK** - 适用于游戏功能注册的高性能异步 Python SDK，支持文件传输为服务器端热重载提供基础。

## ✨ 核心特性

- 📡 **文件传输** - 支持文件上传传输，为服务器热重载提供基础
- ⚡ **异步架构** - 基于 asyncio 的完全异步实现
- 🛠️ **工具集成** - 无缝集成 Uvicorn、Gunicorn、FastAPI
- 🐍 **类型安全** - 完整的类型提示支持
- 🔄 **gRPC 集成** - 原生支持 Croupier gRPC 协议
- 📊 **轻量级设计** - 最小化依赖，专注核心功能

## 🚀 快速开始

### 安装

```bash
pip install croupier-sdk
```

### 基础使用

```python
import asyncio
from croupier import create_client

# 创建客户端配置
config = {
    "agent_addr": "127.0.0.1:19090",
    "timeout": 30000,
    "retry_attempts": 3
}

# 创建客户端
client = create_client(config)

async def main():
    # 基础客户端功能正在开发中
    # 目前请直接使用 gRPC 客户端
    print("📡 File transfer capabilities coming soon!")
    print("🔧 Use gRPC client directly for now")

if __name__ == "__main__":
    asyncio.run(main())
```

### 与 FastAPI 集成

```python
from fastapi import FastAPI
from croupier import create_client

app = FastAPI()

@app.on_event("startup")
async def startup():
    config = {
        "agent_addr": "127.0.0.1:19090",
        "timeout": 30000
    }

    # 客户端功能开发中
    print("Croupier SDK ready for server hot reload support")
```

## 🛠️ 开发状态

当前 SDK 处于开发阶段，提供基础接口定义：

- ✅ 接口定义完成
- ✅ 类型提示支持
- 🚧 文件传输功能（开发中）
- 🚧 基础客户端实现（规划中）

## 📖 未来功能

### 文件上传接口

```python
# 计划中的文件上传 API
await client.upload_file({
    "file_path": "./functions/player_ban.py",
    "content": file_content,
    "metadata": {"version": "1.0.0"}
})
```

### 函数注册

```python
# 计划中的函数注册 API
await client.register_function({
    "id": "player.ban",
    "version": "1.0.0",
    "handler": ban_handler
})
```

## 🧪 示例

查看 `examples/` 目录获取示例：

- **基础示例** - 简单的接口使用示例
- **FastAPI 集成** - 与 Web 框架集成示例

## 📝 更新日志

### v1.0.0 (开发中)

- 🚧 SDK 架构设计
- 📡 文件传输接口定义
- ⚡ 异步架构支持
- 🐍 类型提示支持

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