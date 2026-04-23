# Croupier Python SDK 约定规范

本文档详细说明使用 Croupier Python SDK 时需要遵守的约定和规范。

## 目录

- [命名约定](#命名约定)
- [函数注册约定](#函数注册约定)
- [虚拟对象设计约定](#虚拟对象设计约定)
- [错误处理约定](#错误处理约定)
- [版本管理约定](#版本管理约定)
- [安全约定](#安全约定)
- [避让规则](#避让规则)

---

## 命名约定

### 函数 ID 命名

函数 ID 采用 `entity.operation` 格式。

#### 格式规则

```
[namespace.]entity.operation
```

| 部分 | 说明 | 示例 |
|------|------|------|
| `namespace` (可选) | 命名空间 | `game`, `inventory`, `chat` |
| `entity` | 实体名称，小写 | `player`, `item`, `guild` |
| `operation` | 操作名称，小写动词 | `get`, `create`, `update`, `delete`, `ban` |

#### 命名示例

```python
# ✅ 正确的函数 ID
"player.get"              # 获取玩家信息
"player.ban"              # 封禁玩家
"item.create"             # 创建道具
"wallet.transfer"         # 钱包转账
"game.player.ban"         # 带命名空间

# ❌ 错误的函数 ID
"PlayerGet"               # 不要使用驼峰命名
"player-get"              # 不要使用连字符
"player_get"              # 不要使用下划线
"get_player"              # 实体应该在前
"player"                  # 缺少操作
```

### 实体命名

实体代表业务领域的对象，使用**小写单数名词**。

```python
# ✅ 推荐的实体名称
"player"      # 玩家
"item"        # 道具
"guild"       # 公会
"wallet"      # 钱包
"match"       # 比赛

# ❌ 避免使用
"players"     # 不要使用复数
"Player"      # 不要使用大写
"player_data" # 不要使用下划线
```

### 操作命名

操作使用**小写动词**。

#### CRUD 标准操作

| 操作 | 说明 | 示例 |
|------|------|------|
| `create` | 创建新实体 | `player.create` |
| `get` | 获取实体信息 | `player.get` |
| `update` | 更新实体信息 | `player.update` |
| `delete` | 删除实体 | `player.delete` |
| `list` | 列出集合 | `player.list` |

#### 业务操作

| 操作 | 说明 | 示例 |
|------|------|------|
| `ban` | 封禁 | `player.ban` |
| `unban` | 解封 | `player.unban` |
| `transfer` | 转移 | `wallet.transfer` |
| `add` | 添加 | `guild.member.add` |
| `remove` | 移除 | `guild.member.remove` |
| `join` | 加入 | `match.join` |
| `leave` | 离开 | `match.leave` |

---

## 函数注册约定

### 注册时机

所有函数必须在调用 `connect()` **之前**完成注册。

```python
client = CroupierClient(config)

# ✅ 正确：先注册，后连接
client.register_function(descriptor, handler)
client.connect()

# ❌ 错误：连接后不能再注册
client.connect()
client.register_function(descriptor, handler)  # 无效
```

### 函数唯一性

同一个服务实例内，函数 ID 必须唯一。

```python
# ❌ 错误：重复注册相同 ID
desc1 = FunctionDescriptor(id="player.get", version="1.0.0")
desc2 = FunctionDescriptor(id="player.get", version="1.0.0")

client.register_function(desc1, handler1)
client.register_function(desc2, handler2)  # 会覆盖或报错
```

### FunctionDescriptor 参数

```python
from croupier import FunctionDescriptor

descriptor = FunctionDescriptor(
    # ✅ 必填字段
    id="player.get",           # 函数 ID
    version="1.0.0",           # 版本号

    # ⭐ 推荐字段
    category="player",         # 业务分类
    risk="low",               # 风险等级: low, medium, high
    entity="player",           # 关联实体
    operation="read",          # 操作类型
    enabled=True               # 是否启用
)
```

### 风险等级 (Risk)

| 等级 | 说明 | 示例 |
|------|------|------|
| `low` | 只读操作 | `player.get`, `item.list` |
| `medium` | 有副作用但可逆 | `player.update`, `item.add` |
| `high` | 有重大影响 | `player.delete`, `player.ban`, `wallet.transfer` |

---

## 错误处理约定

### 函数处理器错误处理

```python
import json
from typing import Union

def safe_handler(context: str, payload: bytes) -> str:
    """
    安全的函数处理器，捕获所有异常并返回标准错误响应。
    """
    try:
        # 1. 解析输入
        data = json.loads(payload.decode("utf-8"))

        # 2. 验证参数
        if "player_id" not in data:
            return json.dumps({
                "status": "error",
                "code": "INVALID_PARAM",
                "message": "player_id is required"
            })

        # 3. 执行业务逻辑
        result = process_player(data["player_id"])

        # 4. 返回成功响应
        return json.dumps({
            "status": "success",
            "data": result
        })

    except json.JSONDecodeError as e:
        return json.dumps({
            "status": "error",
            "code": "INVALID_JSON",
            "message": str(e)
        })

    except Exception as e:
        return json.dumps({
            "status": "error",
            "code": "INTERNAL_ERROR",
            "message": str(e)
        })
```

### 标准错误响应格式

```json
{
  "status": "error",
  "code": "ERROR_CODE",
  "message": "Human readable error message",
  "details": {}
}
```

### 常见错误码

| 错误码 | 说明 | HTTP 等价 |
|--------|------|----------|
| `INVALID_PARAM` | 参数错误 | 400 |
| `INVALID_JSON` | JSON 格式错误 | 400 |
| `UNAUTHORIZED` | 未授权 | 401 |
| `FORBIDDEN` | 无权限 | 403 |
| `NOT_FOUND` | 资源不存在 | 404 |
| `INTERNAL_ERROR` | 内部错误 | 500 |

---

## 版本管理约定

### 语义化版本

遵循语义化版本规范：`MAJOR.MINOR.PATCH`

```python
# ✅ 版本号示例
"1.0.0"  # 初始版本
"1.0.1"  # bug 修复
"1.1.0"  # 新增功能（向后兼容）
"2.0.0"  # 重大变更（不兼容）
```

---

## 安全约定

### 输入验证

```python
def secure_handler(context: str, payload: bytes) -> str:
    # 1. 检查 payload
    if not payload:
        return json.dumps({"status": "error", "code": "EMPTY_PAYLOAD"})

    # 2. 验证 JSON
    try:
        data = json.loads(payload.decode("utf-8"))
    except json.JSONDecodeError:
        return json.dumps({"status": "error", "code": "INVALID_JSON"})

    # 3. 验证必需字段
    if "player_id" not in data:
        return json.dumps({"status": "error", "code": "MISSING_PLAYER_ID"})

    # 4. 类型检查
    if not isinstance(data["player_id"], str):
        return json.dumps({"status": "error", "code": "INVALID_PLAYER_ID_TYPE"})

    # 5. 值范围检查
    player_id = data["player_id"]
    if len(player_id) == 0 or len(player_id) > 64:
        return json.dumps({"status": "error", "code": "INVALID_PLAYER_ID_VALUE"})

    # 6. 业务逻辑
    return process_request(data)
```

### TLS 配置

生产环境必须启用 TLS：

```python
config = {
    "agent_addr": "agent.croupier.io:443",
    "insecure": False,  # 必须为 False
    "cert_file": "/etc/tls/client.crt",
    "key_file": "/etc/tls/client.key",
    "ca_file": "/etc/tls/ca.crt"
}
```

---

## 避让规则

### 版本优先级

高版本函数优先处理请求：

```python
# v2.0.0 会优先于 v1.0.0
desc_v2 = FunctionDescriptor(id="player.get", version="2.0.0")
desc_v1 = FunctionDescriptor(id="player.get", version="1.0.0")
```

### 风险等级路由

```python
# 高风险函数更谨慎地路由
desc_high_risk = FunctionDescriptor(
    id="player.ban",
    version="1.0.0",
    risk="high"  # 高风险，更谨慎
)
```

---

## 完整示例

### 符合约定的完整实现

```python
from croupier import CroupierClient, FunctionDescriptor
import json

class PlayerHandlers:
    """玩家管理处理器 - 符合所有约定"""

    @staticmethod
    def get(context: str, payload: bytes) -> str:
        """获取玩家信息 (low risk, read operation)"""
        try:
            data = json.loads(payload.decode("utf-8"))

            # 参数验证
            if "player_id" not in data:
                return PlayerHandlers._error("MISSING_PARAM", "player_id is required")

            # 业务逻辑
            result = {
                "status": "success",
                "data": {
                    "id": data["player_id"],
                    "name": "Player One",
                    "level": 50
                }
            }

            return json.dumps(result)

        except json.JSONDecodeError as e:
            return PlayerHandlers._error("INVALID_JSON", str(e))
        except Exception as e:
            return PlayerHandlers._error("INTERNAL_ERROR", str(e))

    @staticmethod
    def ban(context: str, payload: bytes) -> str:
        """封禁玩家 (high risk, sensitive operation)"""
        try:
            data = json.loads(payload.decode("utf-8"))

            # 高风险操作需要额外验证
            # 从 context 获取操作者信息并验证权限

            result = {
                "status": "success",
                "action": "ban",
                "player_id": data.get("player_id")
            }

            return json.dumps(result)

        except json.JSONDecodeError as e:
            return PlayerHandlers._error("INVALID_JSON", str(e))
        except Exception as e:
            return PlayerHandlers._error("INTERNAL_ERROR", str(e))

    @staticmethod
    def _error(code: str, message: str) -> str:
        """生成标准错误响应"""
        return json.dumps({
            "status": "error",
            "code": code,
            "message": message
        })


def main():
    # 配置
    config = {
        "agent_addr": "127.0.0.1:19090",
        "service_id": "player-service",
        "game_id": "my-game",
        "env": "production"
    }

    client = CroupierClient(config)

    # 注册函数 - 遵循命名约定
    client.register_function(
        FunctionDescriptor(
            id="player.get",
            version="1.0.0",
            category="player",
            risk="low",
            entity="player",
            operation="read"
        ),
        PlayerHandlers.get
    )

    client.register_function(
        FunctionDescriptor(
            id="player.ban",
            version="1.0.0",
            category="player",
            risk="high",
            entity="player",
            operation="update"
        ),
        PlayerHandlers.ban
    )

    # 连接并启动
    if client.connect():
        print("服务已启动")
        client.serve()


if __name__ == "__main__":
    main()
```
