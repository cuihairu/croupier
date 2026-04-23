# API 参考

本文档提供 Croupier Python SDK 的完整 API 参考。

## 包结构

```python
from croupier import (
    # Provider 相关
    ClientConfig,
    FunctionDescriptor,
    FunctionHandler,
    CroupierClient,

    # Invoker 相关
    InvokerConfig,
    InvokeOptions,
    JobEventInfo,
    Invoker,
    SyncInvoker,
    create_invoker,
    create_sync_invoker,
)
```

## 核心类型

### FunctionHandler

函数处理器类型定义。

```python
FunctionHandler = Callable[[str, bytes], str]
```

**参数:**
- `context`: 调用上下文（JSON 字符串）
- `payload`: 函数参数（bytes）

**返回值:**
- `str`: 函数执行结果（JSON 字符串）

**使用示例:**

```python
def my_handler(context: str, payload: bytes) -> str:
    import json
    ctx = json.loads(context)
    data = json.loads(payload.decode('utf-8'))
    player_id = data.get('player_id')

    # 业务逻辑
    return json.dumps({"status": "success"})
```

---

### ClientConfig

客户端配置数据类。

```python
@dataclass
class ClientConfig:
    # 连接配置
    agent_addr: str = "127.0.0.1:19090"     # Agent gRPC 地址
    insecure: bool = True                    # 使用不安全的 gRPC 连接
    timeout_seconds: int = 30                # 连接超时（秒）

    # 服务标识
    service_id: str = "python-sdk-{uuid}"    # 服务标识符
    service_version: str = "1.0.0"           # 服务版本

    # TLS 配置（非 insecure 模式时使用）
    cert_file: Optional[str] = None          # 客户端证书文件路径
    key_file: Optional[str] = None           # 私钥文件路径
    ca_file: Optional[str] = None            # CA 证书文件路径

    # 心跳配置
    heartbeat_interval: int = 60             # 心跳间隔（秒）

    # 控制平面配置
    control_addr: Optional[str] = None       # 控制服务地址
    provider_lang: str = "python"            # Provider 语言
    provider_sdk: str = "croupier-python-sdk" # SDK 标识
```

**使用示例:**

```python
config = ClientConfig(
    agent_addr="localhost:19090",
    insecure=True,
    service_id="player-service",
    service_version="1.0.0",
    heartbeat_interval=30,
)
```

**环境变量覆盖:**

| 环境变量 | 配置字段 | 说明 |
|----------|----------|------|
| `CROUPIER_AGENT_ADDR` | agent_addr | Agent 地址 |
| `CROUPIER_SERVICE_ID` | service_id | 服务 ID |
| `CROUPIER_INSECURE` | insecure | 是否跳过 TLS |

---

### FunctionDescriptor

函数描述符数据类。

```python
@dataclass
class FunctionDescriptor:
    # 必填字段
    id: str                                  # 函数 ID，格式: [namespace.]entity.operation
    version: str = "1.0.0"                   # 语义化版本号

    # 推荐字段
    category: Optional[str] = None           # 业务分类
    risk: Optional[str] = None               # 风险等级: "low"|"medium"|"high"
    entity: Optional[str] = None             # 关联实体类型
    operation: Optional[str] = None          # 操作类型: "create"|"read"|"update"|"delete"
    enabled: bool = True                     # 是否启用
```

**使用示例:**

```python
desc = FunctionDescriptor(
    id="player.ban",
    version="1.0.0",
    category="player",
    risk="high",
    entity="player",
    operation="update",
    enabled=True,
)
```

---

## CroupierClient 类

主客户端类，管理与 Croupier Agent 的连接和函数注册。

### 构造函数

```python
def __init__(self, config: Optional[ClientConfig] = None) -> None
```

**参数:**
- `config`: 客户端配置，可选，默认使用 `ClientConfig()`

### 公共方法

#### register_function

注册单个函数。

```python
def register_function(
    self,
    descriptor: FunctionDescriptor,
    handler: FunctionHandler
) -> None
```

**参数:**
- `descriptor`: 函数描述符
- `handler`: 函数处理器

**异常:**
- `ValueError`: 描述符无效时抛出
- `RuntimeError`: 服务器已启动后注册时抛出

---

#### connect

连接到 Agent。

```python
def connect(self) -> None
```

**异常:**
- `RuntimeError`: 未注册任何函数时抛出
- `RuntimeError`: 无法绑定本地服务器时抛出

---

#### disconnect

断开连接并释放资源。

```python
def disconnect(self) -> None
```

---

## Invoker 类（异步）

异步调用端类，用于调用已注册的函数。

### InvokerConfig

调用端配置。

```python
@dataclass
class InvokerConfig:
    address: str = "127.0.0.1:8080"         # 服务器/Agent 地址
    timeout: int = 30000                     # 超时（毫秒）
    insecure: bool = True                    # 是否跳过 TLS

    # TLS 配置
    ca_file: str = ""                        # CA 证书路径
    cert_file: str = ""                      # 客户端证书路径
    key_file: str = ""                       # 私钥路径
    server_name: str = ""                    # TLS 服务器名称

    # 重连配置
    reconnect: ReconnectConfig = field(default_factory=ReconnectConfig)

    # 重试配置
    retry: RetryConfig = field(default_factory=RetryConfig)
```

### ReconnectConfig

重连配置。

```python
@dataclass
class ReconnectConfig:
    enabled: bool = True                     # 是否启用自动重连
    max_attempts: int = 0                    # 最大重连次数（0 = 无限）
    initial_delay_ms: int = 1000             # 初始延迟（毫秒）
    max_delay_ms: int = 30000                # 最大延迟（毫秒）
    backoff_multiplier: float = 2.0          # 退避乘数
    jitter_factor: float = 0.2               # 抖动因子（0-1）
```

### RetryConfig

重试配置。

```python
@dataclass
class RetryConfig:
    enabled: bool = True                     # 是否启用重试
    max_attempts: int = 3                    # 最大重试次数
    initial_delay_ms: int = 100              # 初始延迟（毫秒）
    max_delay_ms: int = 5000                 # 最大延迟（毫秒）
    backoff_multiplier: float = 2.0          # 退避乘数
    jitter_factor: float = 0.1               # 抖动因子
    retryable_status_codes: tuple = (        # 可重试的状态码
        grpc.StatusCode.UNAVAILABLE,
        grpc.StatusCode.INTERNAL,
        grpc.StatusCode.UNKNOWN,
        grpc.StatusCode.ABORTED,
        grpc.StatusCode.DEADLINE_EXCEEDED,
    )
```

### InvokeOptions

调用选项。

```python
@dataclass
class InvokeOptions:
    idempotency_key: Optional[str] = None    # 幂等键
    timeout: Optional[int] = None            # 超时（毫秒）
    headers: Optional[Dict[str, str]] = None # 请求头
    retry: Optional[RetryConfig] = None      # 重试配置覆盖
```

### JobEventInfo

任务事件信息。

```python
@dataclass
class JobEventInfo:
    type: str                                # 事件类型: "started"|"progress"|"completed"|"error"|"cancelled"
    job_id: str                              # 任务 ID
    payload: Optional[str] = None            # 结果负载
    message: Optional[str] = None            # 事件消息
    progress: Optional[int] = None           # 进度 0-100
    error: Optional[str] = None              # 错误信息
    done: bool = False                       # 是否完成
```

### 构造函数

```python
def __init__(self, config: Optional[InvokerConfig] = None)
```

### 异步方法

#### connect

连接到服务器。

```python
async def connect(self) -> None
```

---

#### invoke

同步调用函数。

```python
async def invoke(
    self,
    function_id: str,
    payload: str,
    options: Optional[InvokeOptions] = None
) -> str
```

**参数:**
- `function_id`: 函数 ID
- `payload`: 请求负载（JSON 字符串）
- `options`: 调用选项

**返回值:**
- `str`: 响应负载（JSON 字符串）

**异常:**
- `Exception`: 调用失败时抛出

---

#### start_job

启动异步任务。

```python
async def start_job(
    self,
    function_id: str,
    payload: str,
    options: Optional[InvokeOptions] = None
) -> str
```

**返回值:**
- `str`: 任务 ID

---

#### stream_job

流式获取任务事件。

```python
async def stream_job(self, job_id: str) -> AsyncIterator[JobEventInfo]
```

**返回值:**
- `AsyncIterator[JobEventInfo]`: 任务事件异步迭代器

---

#### cancel_job

取消任务。

```python
async def cancel_job(self, job_id: str) -> None
```

---

#### set_schema

设置函数验证 Schema。

```python
async def set_schema(self, function_id: str, schema: Dict[str, Any]) -> None
```

---

#### close

关闭连接。

```python
async def close(self) -> None
```

---

## SyncInvoker 类（同步）

同步调用端类，为不使用 asyncio 的应用提供阻塞接口。

### 构造函数

```python
def __init__(self, config: Optional[InvokerConfig] = None)
```

### 同步方法

```python
def connect(self) -> None
def invoke(self, function_id: str, payload: str, options: Optional[InvokeOptions] = None) -> str
def start_job(self, function_id: str, payload: str, options: Optional[InvokeOptions] = None) -> str
def stream_job(self, job_id: str) -> Iterator[JobEventInfo]
def cancel_job(self, job_id: str) -> None
def set_schema(self, function_id: str, schema: Dict[str, Any]) -> None
def close(self) -> None
```

---

## 便捷函数

### create_invoker

创建异步 Invoker 实例。

```python
def create_invoker(config: Optional[InvokerConfig] = None) -> Invoker
```

### create_sync_invoker

创建同步 Invoker 实例。

```python
def create_sync_invoker(config: Optional[InvokerConfig] = None) -> SyncInvoker
```

---

## 完整示例

### Provider 示例

```python
import json
import logging
from croupier import (
    ClientConfig,
    FunctionDescriptor,
    CroupierClient,
)

logging.basicConfig(level=logging.INFO)

def main():
    # 配置
    config = ClientConfig(
        agent_addr="localhost:19090",
        insecure=True,
        service_id="player-service",
        service_version="1.0.0",
    )

    # 创建客户端
    client = CroupierClient(config)

    # 定义处理器
    def ban_player(context: str, payload: bytes) -> str:
        data = json.loads(payload.decode('utf-8'))
        player_id = data.get('player_id')
        reason = data.get('reason', '未指定')

        # 业务逻辑
        print(f"封禁玩家: {player_id}, 原因: {reason}")

        return json.dumps({"status": "success"})

    # 注册函数
    desc = FunctionDescriptor(
        id="player.ban",
        version="1.0.0",
        category="player",
        risk="high",
        entity="player",
        operation="update",
    )
    client.register_function(desc, ban_player)

    # 连接并启动
    try:
        client.connect()
        print("服务已启动，按 Ctrl+C 退出")

        # 保持运行
        import time
        while True:
            time.sleep(1)

    except KeyboardInterrupt:
        print("正在关闭...")
    finally:
        client.disconnect()

if __name__ == "__main__":
    main()
```

### Invoker 异步示例

```python
import asyncio
from croupier import (
    InvokerConfig,
    InvokeOptions,
    create_invoker,
)

async def main():
    # 配置
    config = InvokerConfig(
        address="localhost:8080",
        timeout=30000,
        insecure=True,
    )

    # 创建 Invoker
    invoker = create_invoker(config)

    try:
        # 连接
        await invoker.connect()

        # 同步调用
        options = InvokeOptions(
            idempotency_key="ban-12345-20260117",
            timeout=10000,
        )

        result = await invoker.invoke(
            "player.ban",
            '{"player_id": "12345", "reason": "违规"}',
            options,
        )
        print(f"调用结果: {result}")

        # 启动异步任务
        job_id = await invoker.start_job(
            "player.export",
            '{"format": "csv"}',
        )
        print(f"任务已启动: {job_id}")

        # 流式获取任务事件
        async for event in invoker.stream_job(job_id):
            if event.type == "progress":
                print(f"进度: {event.progress}%")
            elif event.type == "completed":
                print(f"完成: {event.payload}")
            elif event.type == "error":
                print(f"错误: {event.error}")

            if event.done:
                break

    finally:
        await invoker.close()

if __name__ == "__main__":
    asyncio.run(main())
```

### Invoker 同步示例

```python
from croupier import (
    InvokerConfig,
    InvokeOptions,
    create_sync_invoker,
)

def main():
    # 配置
    config = InvokerConfig(
        address="localhost:8080",
        timeout=30000,
        insecure=True,
    )

    # 创建同步 Invoker
    invoker = create_sync_invoker(config)

    try:
        # 连接
        invoker.connect()

        # 同步调用
        result = invoker.invoke(
            "player.get",
            '{"player_id": "12345"}',
        )
        print(f"调用结果: {result}")

        # 启动异步任务
        job_id = invoker.start_job(
            "player.export",
            '{"format": "csv"}',
        )

        # 流式获取任务事件
        for event in invoker.stream_job(job_id):
            print(f"事件: {event.type} - {event.message}")
            if event.done:
                break

    finally:
        invoker.close()

if __name__ == "__main__":
    main()
```

### Schema 验证示例

```python
import asyncio
from croupier import create_invoker, InvokerConfig

async def main():
    invoker = create_invoker(InvokerConfig(address="localhost:8080"))

    # 设置 JSON Schema 验证
    await invoker.set_schema("player.ban", {
        "type": "object",
        "required": ["player_id"],
        "properties": {
            "player_id": {"type": "string"},
            "reason": {"type": "string"},
            "duration": {"type": "integer"},
        }
    })

    await invoker.connect()

    try:
        # 有效的调用
        result = await invoker.invoke(
            "player.ban",
            '{"player_id": "12345", "reason": "违规"}',
        )
        print(f"成功: {result}")

        # 无效的调用（缺少 player_id）- 将抛出异常
        await invoker.invoke(
            "player.ban",
            '{"reason": "违规"}',
        )
    except Exception as e:
        print(f"验证失败: {e}")
    finally:
        await invoker.close()

if __name__ == "__main__":
    asyncio.run(main())
```

---

## 错误处理

### 异常类型

Python SDK 使用标准异常类型：

| 异常 | 场景 |
|------|------|
| `ValueError` | 参数无效 |
| `RuntimeError` | 运行时错误（如未连接、未注册函数等） |
| `Exception` | gRPC 调用失败 |

### 错误处理示例

```python
import asyncio
from croupier import create_invoker, InvokerConfig

async def main():
    invoker = create_invoker(InvokerConfig())

    try:
        await invoker.connect()
        result = await invoker.invoke("player.get", '{"player_id": "123"}')
        print(result)

    except ConnectionError as e:
        print(f"连接失败: {e}")

    except TimeoutError as e:
        print(f"超时: {e}")

    except Exception as e:
        print(f"调用失败: {e}")

    finally:
        await invoker.close()

if __name__ == "__main__":
    asyncio.run(main())
```
