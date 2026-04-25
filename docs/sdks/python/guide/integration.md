# Croupier Python SDK 集成指南

## 安装

```bash
python -m pip install -e ./sdks/python
```

## 最小示例

```python
import json
from croupier import CroupierClient, ClientConfig, FunctionDescriptor

config = ClientConfig(agent_addr="127.0.0.1:19090", service_id="python-demo")
client = CroupierClient(config)

def player_ban(_context: str, payload: bytes) -> str:
    req = json.loads(payload.decode("utf-8"))
    return json.dumps({"status": "ok", "player_id": req["player_id"]})

client.register_function(FunctionDescriptor(id="player.ban", version="1.0.0"), player_ban)
client.connect()
```
