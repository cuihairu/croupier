# Croupier Python SDK 集成指南

本指南提供完整的 Croupier Python SDK 集成步骤，帮助开发者快速接入游戏后端平台。

## 目录

- [快速开始](#快速开始)
- [安装](#安装)
- [核心概念](#核心概念)
- [完整接口参考](#完整接口参考)
- [配置说明](#配置说明)
- [生产部署](#生产部署)
- [故障排查](#故障排查)

---

## 快速开始

### 安装 SDK

```bash
# 从 PyPI 安装（稳定版）
pip install croupier-sdk

# 或从 GitHub 安装（最新版）
pip install git+https://github.com/cuihairu/croupier-sdk-python.git
```

### 最小集成示例

```python
from croupier import CroupierClient, FunctionDescriptor

# 定义函数处理器
def my_handler(context: str, payload: bytes) -> str:
    data = json.loads(payload.decode("utf-8"))
    # 处理业务逻辑
    return json.dumps({"status": "success"})

# 创建并启动客户端
config = {
    "agent_addr": "127.0.0.1:19090",
    "service_id": "my-service",
}

client = CroupierClient(config)

# 注册函数
descriptor = FunctionDescriptor(
    id="game.action",
    version="1.0.0",
    category="gameplay",
    risk="low"
)
client.register_function(descriptor, my_handler)

# 连接并启动服务
client.connect()
client.serve()  # 阻塞运行
```

---

## 安装

### 系统要求

- Python 3.9 或更高版本
- pip 包管理器

### 从源码安装

```bash
# 克隆仓库
git clone https://github.com/cuihairu/croupier-sdk-python.git
cd croupier-sdk-python

# 安装依赖
pip install -r requirements.txt

# 安装 SDK
pip install -e .
```

### 验证安装

```bash
python -c "from croupier import CroupierClient; print('安装成功！')"
```

---

## 核心概念

### 客户端 (Client)

客户端负责注册和管理游戏函数，接收来自 Agent 的调用请求。

```python
from croupier import CroupierClient

client = CroupierClient(config)
```

### 函数描述符 (FunctionDescriptor)

描述函数的元数据：

```python
from croupier import FunctionDescriptor

descriptor = FunctionDescriptor(
    id="player.ban",        # 函数唯一标识
    version="1.0.0",        # 版本号
    category="moderation",   # 业务分类
    risk="high",           # 风险等级: low, medium, high
    entity="player",       # 关联实体
    operation="update",    # 操作类型: create, read, update, delete
    enabled=True          # 是否启用
)
```

### 函数处理器 (Handler)

函数处理器是处理具体业务逻辑的函数：

```python
def handler(context: str, payload: bytes) -> str:
    """
    Args:
        context: 调用上下文，包含调用者信息
        payload: 请求负载，JSON 格式的 bytes

    Returns:
        str: JSON 格式的响应字符串
    """
    data = json.loads(payload.decode("utf-8"))
    # 处理业务逻辑
    return json.dumps({"status": "success"})
```

---

## 完整接口参考

### CroupierClient

#### 初始化

```python
from croupier import CroupierClient

config = {
    "agent_addr": "127.0.0.1:19090",
    "control_addr": "127.0.0.1:18080",
    "service_id": "my-service",
    "service_version": "1.0.0",
    "game_id": "my-game",
    "env": "production",
    "insecure": False,
}

client = CroupierClient(config)
```

#### 方法

| 方法 | 说明 | 返回值 |
|------|------|--------|
| `register_function(descriptor, handler)` | 注册函数 | `bool` |
| `unregister_function(function_id)` | 取消注册函数 | `bool` |
| `connect()` | 连接到 Agent | `bool` |
| `disconnect()` | 断开连接 | `None` |
| `serve()` | 启动服务循环（阻塞） | `None` |
| `is_connected()` | 检查连接状态 | `bool` |

### Invoker

用于主动调用远程函数（可选）：

```python
from croupier import CroupierInvoker, InvokerConfig

invoker_config = InvokerConfig(
    address="localhost:8080",
    insecure=True,
    timeout_seconds=30
)

invoker = CroupierInvoker(invoker_config)

# 连接
invoker.connect()

# 调用函数
result = invoker.invoke("player.get", '{"player_id":"123"}')

# 启动异步作业
job_id = invoker.start_job("item.create", '{"type":"sword"}')

# 流式获取作业事件
for event in invoker.stream_job(job_id):
    print(f"事件: {event.event_type}, 数据: {event.payload}")

# 取消作业
invoker.cancel_job(job_id)

# 关闭
invoker.close()
```

---

## 配置说明

### ClientConfig 完整参数

```python
from croupier import CroupierClient

config = {
    # === 连接配置 ===
    "agent_addr": "127.0.0.1:19090",      # Agent 地址
    "control_addr": "127.0.0.1:18080",    # Control 平台地址（可选）

    # === 身份配置 ===
    "service_id": "my-service",            # 服务标识（必填）
    "service_version": "1.0.0",           # 服务版本
    "game_id": "my-game",                 # 游戏标识
    "env": "production",                  # 环境: development, staging, production

    # === TLS 配置 ===
    "insecure": False,                    # 是否禁用 TLS
    "cert_file": "/path/to/cert.pem",    # 客户端证书
    "key_file": "/path/to/key.pem",      # 客户端私钥
    "ca_file": "/path/to/ca.pem",        # CA 证书

    # === 超时配置 ===
    "timeout_seconds": 30,

    # === 重连配置 ===
    "auto_reconnect": True,
    "reconnect_interval_seconds": 5,
    "reconnect_max_attempts": 0,         # 0 = 无限重试
}

client = CroupierClient(config)
```

### 环境变量

可通过环境变量覆盖配置：

```bash
export CROUPIER_AGENT_ADDR="127.0.0.1:19090"
export CROUPIER_SERVICE_ID="my-service"
export CROUPIER_INSECURE="false"
```

---

## 生产部署

### Docker 部署

创建 `Dockerfile`:

```dockerfile
FROM python:3.12-slim

WORKDIR /app

# 安装依赖
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# 复制代码
COPY . .

# 暴露健康检查端口
EXPOSE 8080

# 运行服务
CMD ["python", "-m", "src.main"]
```

创建 `docker-compose.yml`:

```yaml
version: '3.8'

services:
  game-service:
    build: .
    environment:
      - CROUPIER_AGENT_ADDR=agent:19090
      - CROUPIER_ENV=production
    ports:
      - "8080:8080"
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

### Kubernetes 部署

创建 `deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: croupier-game-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: game-service
  template:
    metadata:
      labels:
        app: game-service
    spec:
      containers:
      - name: game-service
        image: your-registry/croupier-game-service:latest
        env:
        - name: CROUPIER_AGENT_ADDR
          value: "croupier-agent:19090"
        - name: CROUPIER_ENV
          value: "production"
        ports:
        - containerPort: 8080
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: croupier-game-service
spec:
  selector:
    app: game-service
  ports:
  - port: 80
    targetPort: 8080
```

---

## 故障排查

### 连接失败

**问题**: 无法连接到 Agent

```python
# 检查配置
print(f"Agent 地址: {config['agent_addr']}")

# 检查网络连通性
import socket
host, port = config['agent_addr'].split(':')
sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
result = sock.connect_ex((host, int(port)))
print(f"连接测试: {result == 0 and '成功' or '失败'}")
sock.close()
```

**解决方案**:
1. 确认 Agent 服务正在运行
2. 检查防火墙规则
3. 验证地址格式

### 函数未注册

**问题**: 函数注册失败

```python
# 检查描述符
descriptor = FunctionDescriptor(
    id="player.ban",
    version="1.0.0"
)

# 验证必填字段
assert descriptor.id, "函数 ID 不能为空"
assert descriptor.version, "版本号不能为空"

# 注册前检查
if not client.is_connected():
    print("客户端未连接")
```

### 性能问题

**优化建议**:

1. 使用异步处理器处理耗时操作

```python
import asyncio
from concurrent.futures import ThreadPoolExecutor

executor = ThreadPoolExecutor(max_workers=10)

def async_handler(context: str, payload: bytes) -> str:
    # 提交到线程池异步处理
    future = executor.submit(process_long_running_task, payload)
    # 立即返回
    return json.dumps({"status": "submitted"})
```

2. 启用连接复用

```python
config["keepalive_enabled"] = True
config["keepalive_interval_seconds"] = 30
```

---

## 更多资源

- [约定规范](../conventions.md) - 命名约定和最佳实践
- [API 参考](../api/) - 详细的 API 文档
- [问题反馈](https://github.com/cuihairu/croupier-sdk-python/issues)
