# 🔥 Croupier Python SDK 热重载示例

这个示例展示了如何在Python游戏服务器中集成Croupier SDK的热重载功能，支持异步操作和多种热更新工具。

## 🚀 快速开始

### 1. 安装依赖

```bash
cd examples/python-hotreload

# 创建虚拟环境（推荐）
python -m venv venv
source venv/bin/activate  # Linux/Mac
# 或
venv\Scripts\activate     # Windows

# 安装依赖
pip install -r requirements.txt
```

### 2. 启动Croupier Agent

```bash
# 在另一个终端启动Agent
cd ../../
make build
./bin/croupier-agent --config configs/agent.example.yaml
```

### 3. 选择运行方式

#### Uvicorn开发模式（推荐）
```bash
# 自动重载模式
uvicorn main:app --reload --host 0.0.0.0 --port 8000

# 或直接运行
python main.py
```

#### Watchdog文件监听
```bash
# 启用文件监听
ENABLE_FILE_WATCHING=true python main.py
```

#### 生产模式
```bash
# 使用Gunicorn部署
gunicorn main:app -w 4 -k uvicorn.workers.UvicornWorker
```

## 🔧 热重载特性

### 1. 异步自动重连

```python
# 自动重连配置
config = {
    'auto_reconnect': True,
    'reconnect_delay': 5.0,
    'max_retry_attempts': 10,
    'health_check_interval': 30.0
}
```

当连接断开时，SDK会：
- 检测连接状态
- 使用指数退避重连
- 自动重新注册函数
- 维护服务可用性

### 2. 模块热重载

```python
# importlib.reload支持
config['tools']['importlib_reload'] = True

# 当.py文件变化时：
# 1. 清除模块缓存
# 2. 重新导入模块
# 3. 重新注册函数
```

### 3. 文件监听

```python
# Watchdog文件监听配置
config['file_watching'] = {
    'enabled': True,
    'watch_dir': './functions',
    'patterns': ['*.py', '*.json', '*.yaml']
}
```

### 4. 异步函数重载

```python
# 单函数重载
await client.reload_function(
    function_id="player.ban",
    version="1.1.0",
    handler=handle_player_ban_v2
)

# 批量重载
functions = {
    'player.ban': {'version': '1.1.0', 'handler': handle_player_ban_v2},
    'server.status': {'version': '2.0.0', 'handler': handle_server_status_v2}
}
await client.reload_functions(functions)
```

## 📊 开发工具集成

### Uvicorn集成

```bash
# 开发模式自动重载
uvicorn main:app --reload --log-level debug

# 生产模式
uvicorn main:app --host 0.0.0.0 --port 8000 --workers 4
```

特性：
- 🔄 检测代码变更自动重启
- 📊 内置性能监控
- 🚀 ASGI标准支持
- 🔗 SDK自动重连

### Gunicorn集成

```bash
# 生产部署
gunicorn main:app \
  -w 4 \
  -k uvicorn.workers.UvicornWorker \
  --bind 0.0.0.0:8000 \
  --max-requests 1000 \
  --max-requests-jitter 100
```

特性：
- 🔄 热重载worker进程
- 📊 进程监控和重启
- 🚀 高性能并发处理
- 🛡️ 生产级稳定性

### Django/Flask集成

```python
# Django集成示例
# settings.py
if DEBUG:
    CROUPIER_HOTRELOAD = {
        'enabled': True,
        'auto_reconnect': True,
        'file_watching': {'enabled': True}
    }

# Flask集成示例
app = Flask(__name__)
if app.debug:
    client = create_hotreload_client({
        'enabled': True,
        'auto_reconnect': True
    })
```

## 🎯 功能演示

运行后会自动演示：

1. **基础连接**（启动时）
   - 异步连接到Agent
   - 注册游戏函数
   - 启动健康检查

2. **函数重载**（10秒后）
   - 升级`player.ban`到v1.1.0
   - 增加增强功能特性

3. **批量重载**（15秒后）
   - 更新`server.status`到v2.0.0
   - 增加详细系统信息

4. **状态监控**（每30秒）
   - 连接状态检查
   - 重载统计信息
   - 系统资源监控

## 🛠️ 开发工作流

### 修改函数逻辑

1. 编辑`main.py`中的函数实现
2. 如果启用了Uvicorn --reload，进程自动重启
3. 如果启用了文件监听，模块自动重载
4. SDK自动重连并注册新函数

### 测试API调用

```bash
# 测试玩家封禁
curl -X POST http://localhost:8080/api/invoke \
  -H "Content-Type: application/json" \
  -d '{
    "function_id": "player.ban",
    "payload": "{\"player_id\":\"123\",\"reason\":\"cheating\",\"duration\":24}"
  }'

# 测试服务器状态
curl -X POST http://localhost:8080/api/invoke \
  -H "Content-Type: application/json" \
  -d '{
    "function_id": "server.status",
    "payload": "{}"
  }'
```

### 监控重载状态

```python
# 获取详细状态
status = client.get_reload_status()
print(f"连接状态: {status.connection_status}")
print(f"重连次数: {status.reconnect_count}")
print(f"函数重载: {status.function_reloads}")
print(f"运行时间: {status.uptime:.1f}s")
```

## 🎮 不同部署模式对比

| 模式 | 重载方式 | 停机时间 | 适用场景 | 命令 |
|------|---------|----------|----------|------|
| **Uvicorn --reload** | 进程重启 | ~1-2秒 | 开发环境 | `uvicorn main:app --reload` |
| **Watchdog监听** | 模块重载 | ~100ms | 开发调试 | `ENABLE_FILE_WATCHING=true python main.py` |
| **Gunicorn** | Worker重载 | ~500ms | 生产环境 | `gunicorn main:app -w 4` |
| **Django开发服务器** | 进程重启 | ~1秒 | Django项目 | `python manage.py runserver` |

## 🔍 调试和监控

### 异步日志
```python
import logging

# 配置异步日志
logging.basicConfig(
    level=logging.DEBUG,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
    handlers=[
        logging.StreamHandler(),
        logging.FileHandler('hotreload.log')
    ]
)
```

### 性能监控
```python
import psutil
import asyncio

# 监控系统资源
async def monitor_resources():
    while True:
        cpu = psutil.cpu_percent()
        memory = psutil.virtual_memory().percent
        print(f"CPU: {cpu}%, Memory: {memory}%")
        await asyncio.sleep(30)
```

### 错误处理
```python
try:
    await client.reload_function("test", "1.0.0", handler)
except RuntimeError as e:
    logger.error(f"Reload failed: {e}")
    # 实现回滚逻辑
except asyncio.TimeoutError:
    logger.error("Reload timeout")
    # 实现超时处理
```

## 🚨 故障排除

### 常见问题

1. **异步上下文错误**
   ```
   RuntimeError: no running event loop
   ```
   - 确保在async函数中调用
   - 使用`asyncio.run()`或`asyncio.create_task()`

2. **模块重载失败**
   ```
   ImportError: cannot import name
   ```
   - 检查模块依赖关系
   - 清理`__pycache__`目录
   - 重启Python进程

3. **文件监听不工作**
   ```
   watchdog.events.FileSystemEventHandler not triggered
   ```
   - 安装watchdog: `pip install watchdog`
   - 检查监听目录权限
   - 确认文件扩展名匹配

### 最佳实践

1. **开发环境**
   - 使用虚拟环境隔离依赖
   - 启用详细日志和调试模式
   - 使用Uvicorn --reload快速迭代

2. **测试环境**
   - 使用Gunicorn模拟生产配置
   - 测试重载和恢复功能
   - 验证异步操作正确性

3. **生产环境**
   - 使用进程管理器（systemd/supervisor）
   - 配置日志轮转和监控
   - 禁用开发特性，启用优雅关闭

## 📚 依赖说明

### 核心依赖
```bash
# 基础异步支持
asyncio          # Python 3.7+ 内置
aiofiles         # 异步文件操作

# 热重载支持
watchdog         # 文件监听
importlib        # Python内置模块重载
```

### 可选依赖
```bash
# Web框架集成
uvicorn          # ASGI服务器
gunicorn         # WSGI服务器
fastapi          # 现代Web框架
django           # Django框架

# 监控和调试
psutil           # 系统资源监控
prometheus_client # 指标收集
```

## 📚 相关文档

- [SDK热重载支持文档](../../docs/SDK_HOTRELOAD_SUPPORT.md)
- [热更新最佳实践](../../docs/HOTRELOAD_BEST_PRACTICES.md)
- [Python异步编程指南](https://docs.python.org/3/library/asyncio.html)

---

*🔥 享受高效的Python异步热重载开发体验！*