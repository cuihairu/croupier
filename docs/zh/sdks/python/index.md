---
title: Python SDK
---

# Python SDK

Python SDK 是 Croupier 官方的 Python 客户端，聚焦函数注册、调用处理与 Agent session 管理。

## 代码位置

- `sdks/python`

## 特性

- 支持 Python 3.12+
- 通过单连接 session 模型与 Agent 通信
- 内置心跳、重连与基础类型标注

```python
from croupier import set_field_widget, set_field_hint

desc = set_field_widget(FunctionDescriptor(id="player.ban"), "id", "Select")
desc = set_field_hint(desc, "id", "x-options-source", {
    "functionId": "player.list",
    "labelPath": "/items/*/name",
    "valuePath": "/items/*/id",
})
```

## 安装

```bash
python -m pip install -e ./sdks/python
```

## 快速开始

```python
from croupier import CroupierClient, ClientConfig

client = CroupierClient(
    ClientConfig(
        agent_addr="127.0.0.1:19091",
        service_id="python-demo",
        service_version="1.0.0",
    )
)

client.register_function(
    FunctionDescriptor(id="hello.world", version="1"),
    handler=lambda ctx, payload: {"message": "hello"},
)

client.connect()
```

## 延伸阅读

- [指南](/zh/sdks/python/guide/)
- [API 参考](/zh/sdks/python/api/)
- [仓库 README](https://github.com/cuihairu/croupier/tree/main/sdks/python)
