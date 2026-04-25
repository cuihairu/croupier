---
title: Python SDK
---

# Python SDK

Python SDK 是 Croupier 的官方 Python 客户端，面向函数注册、调用处理和 Agent 会话管理。

## 代码位置

- `sdks/python`

## 特性

- Python 3.12+ 支持
- 基于单连接会话模型与 Agent 通信
- 内置心跳、重连和基础类型注解

## 安装

```bash
python -m pip install -e ./sdks/python
```

## 快速开始

```python
from croupier import CroupierClient, ClientConfig

client = CroupierClient(
    ClientConfig(
        agent_addr="127.0.0.1:19090",
        service_id="python-demo",
        service_version="1.0.0",
    )
)

client.connect()
```

## 继续阅读

- [指南](/sdks/python/guide/)
- [API 参考](/sdks/python/api/)
- [仓库 README](https://github.com/cuihairu/croupier/tree/main/sdks/python)
