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

- 呈现 hints 便捷层（x-ui-* 契约，见 [呈现 Hints 契约](/architecture/presentation-hints)）：

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
    function_id="hello.world",
    handler=lambda payload: {"message": "hello"},
)

client.connect()
```

## 继续阅读

- [指南](/sdks/python/guide/)
- [API 参考](/sdks/python/api/)
- [仓库 README](https://github.com/cuihairu/croupier/tree/main/sdks/python)
