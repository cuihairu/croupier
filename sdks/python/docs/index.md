---
home: true
title: Croupier Python SDK
titleTemplate: false
heroImage: /logo.png
heroText: Croupier Python SDK
tagline: Python SDK for Croupier Game Backend Platform
actions:
  - text: 快速开始
    link: /guide/quick-start.html
    type: primary
features:
  - title: 简单易用
    details: Pythonic API 设计
  - title: 异步支持
    details: 基于 asyncio 的异步处理
  - title: 类型提示
    details: 完整的类型注解

footer: Apache License 2.0 | Copyright © 2024 Croupier
---

## 简介

Croupier Python SDK 是 [Croupier](https://github.com/cuihairu/croupier) 的 Python 客户端实现。

## 安装

```bash
pip install croupier-sdk
```

## 快速开始

```python
from croupier_sdk import CroupierClient, ClientConfig

config = ClientConfig(
    agent_addr="localhost:19090",
    game_id="my-game",
    env="development",
    insecure=True,
)

client = CroupierClient(config)

def hello_handler(ctx, payload):
    return {"message": "Hello from Python!"}

client.register_function({
    "id": "hello.world",
    "version": "0.1.0",
}, hello_handler)

client.serve()
```
