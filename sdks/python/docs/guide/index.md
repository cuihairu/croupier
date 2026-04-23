# 入门指南

## 系统要求

- Python ≥ 3.12

## 安装

```bash
pip install croupier-sdk
```

## 快速开始

```python
from croupier_sdk import CroupierClient, ClientConfig

config = ClientConfig(
    agent_addr="localhost:19090",
    game_id="demo-game",
    env="development",
    insecure=True,
)

client = CroupierClient(config)

def handler(ctx, payload):
    return {"success": True}

client.register_function({
    "id": "test.function",
    "version": "0.1.0",
}, handler)

client.serve()
```

## 下一步

- [主线程调度器](./threading.md) - gRPC 回调线程安全处理
- [API 参考](../api/) - 完整的 API 文档
