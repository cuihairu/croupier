---
title: Python SDK
---

# Python SDK

The Python SDK is Croupier's official Python client, focused on function registration, invocation handling, and Agent session management.

## Code Location

- `sdks/python`

## Features

- Python 3.12+ support
- Communicates with the Agent over a single-connection session model
- Built-in heartbeats, reconnection, and basic type annotations

```python
from croupier import set_field_widget, set_field_hint

desc = set_field_widget(FunctionDescriptor(id="player.ban"), "id", "Select")
desc = set_field_hint(desc, "id", "x-options-source", {
    "functionId": "player.list",
    "labelPath": "/items/*/name",
    "valuePath": "/items/*/id",
})
```

## Installation

```bash
python -m pip install -e ./sdks/python
```

## Quick Start

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

## Keep Reading

- [Guide](/sdks/python/guide/)
- [API Reference](/sdks/python/api/)
- [Repository README](https://github.com/cuihairu/croupier/tree/main/sdks/python)
