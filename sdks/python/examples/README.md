# 📡 Croupier Python SDK 文件传输示例

这个示例展示了如何使用Croupier Python SDK进行文件传输，为服务器端热重载提供基础支持。

## 🚀 快速开始

### 1. 安装依赖

```bash
cd examples/python-file-transfer

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

### 3. 运行示例

```bash
# 运行基础示例
python main.py
```

## 📡 文件传输功能

### 基础文件上传

```python
# 计划中的文件上传 API
await client.upload_file({
    "file_path": "./functions/player_ban.py",
    "content": file_content,
    "metadata": {
        "version": "1.0.0",
        "author": "game-team",
        "description": "Player ban functionality"
    }
})
```

### 批量文件传输

```python
# 计划中的批量上传
files = [
    {
        "file_path": "functions/player_ban.py",
        "content": ban_code,
        "metadata": {"version": "1.0.0"}
    },
    {
        "file_path": "functions/wallet_transfer.py",
        "content": transfer_code,
        "metadata": {"version": "1.0.0"}
    }
]

for file_info in files:
    await client.upload_file(file_info)
```

## 🛠️ 开发状态

当前SDK文件传输功能正在开发中：

- ✅ 接口定义完成
- ✅ 类型提示支持
- 🚧 文件传输实现（开发中）
- 🚧 批量操作支持（规划中）
- 🚧 传输进度监控（规划中）

## 🎯 功能演示

当前示例展示：

1. **基础架构**
   - 异步客户端配置
   - 接口定义展示
   - 错误处理示例

2. **文件处理**
   - 文件读取示例
   - 元数据处理
   - 基础文件操作

## 🔧 配置选项

### 客户端配置

```python
config = {
    "agent_addr": "127.0.0.1:19090",
    "timeout": 30000,
    "retry_attempts": 3,
    "chunk_size": 1024 * 1024,  # 1MB chunks
    "max_file_size": 100 * 1024 * 1024  # 100MB max
}
```

### 文件传输配置

```python
transfer_config = {
    "compression": True,
    "checksum_verification": True,
    "retry_failed_uploads": True,
    "parallel_uploads": 4
}
```

## 📊 示例函数处理器

### 玩家封禁处理器

```python
async def handle_player_ban(payload: Dict[str, Any]) -> Dict[str, Any]:
    """处理玩家封禁请求"""
    logger.info(f"🚫 Processing player ban: {payload}")

    await asyncio.sleep(0.1)  # 模拟处理延迟

    return {
        "result": "success",
        "message": "Player banned",
        "player_id": payload.get("player_id"),
        "reason": payload.get("reason"),
        "timestamp": str(asyncio.get_event_loop().time())
    }
```

### 服务器状态处理器

```python
async def handle_server_status(payload: Dict[str, Any]) -> Dict[str, Any]:
    """处理服务器状态请求"""
    logger.info(f"📊 Processing server status: {payload}")

    return {
        "status": "running",
        "uptime": asyncio.get_event_loop().time(),
        "process_id": os.getpid(),
        "timestamp": str(asyncio.get_event_loop().time())
    }
```

## 🚨 故障排除

### 常见问题

1. **连接问题**
   ```
   Connection refused: [Errno 111] Connection refused
   ```
   - 确保Croupier Agent正在运行
   - 检查网络连接和端口配置

2. **文件权限问题**
   ```
   Permission denied: 'functions/test.py'
   ```
   - 检查文件路径权限
   - 确保有写入权限

3. **依赖问题**
   ```
   ModuleNotFoundError: No module named 'psutil'
   ```
   - 安装可选依赖：`pip install psutil`

### 最佳实践

1. **文件组织**
   - 将功能文件放在专门的目录
   - 使用版本控制管理代码
   - 保持文件结构清晰

2. **错误处理**
   - 实现重试机制
   - 添加日志记录
   - 优雅处理网络错误

3. **性能优化**
   - 使用适当的文件块大小
   - 实现并发上传
   - 监控传输进度

## 📚 依赖说明

### 核心依赖
```bash
# 基础异步支持
asyncio          # Python 3.7+ 内置
```

### 可选依赖
```bash
# 系统监控
psutil           # 系统资源监控

# 文件处理
aiofiles         # 异步文件操作
```

## 📚 相关文档

- [Croupier 主文档](https://docs.croupier.io)
- [gRPC API 参考](https://docs.croupier.io/api/grpc)
- [Python 异步编程](https://docs.python.org/3/library/asyncio.html)

---

*📡 为服务器热重载提供强大的文件传输支持！*