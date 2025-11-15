"""
Croupier Python SDK Hot Reload Example
"""

import asyncio
import logging
import signal
import json
from typing import Dict, Any

from croupier.hotreload_client import create_hotreload_client, HotReloadConfig

# 配置日志
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


# 游戏函数：玩家封禁
async def handle_player_ban(payload: Dict[str, Any]) -> Dict[str, Any]:
    """处理玩家封禁请求"""
    logger.info(f"🚫 Processing player ban: {payload}")

    # 模拟处理延迟
    await asyncio.sleep(0.1)

    return {
        "result": "success",
        "message": "Player banned",
        "player_id": payload.get("player_id"),
        "reason": payload.get("reason"),
        "timestamp": str(asyncio.get_event_loop().time())
    }


# 游戏函数：服务器状态
async def handle_server_status(payload: Dict[str, Any]) -> Dict[str, Any]:
    """处理服务器状态请求"""
    logger.info(f"📊 Processing server status request: {payload}")

    import psutil
    import os

    return {
        "status": "running",
        "uptime": asyncio.get_event_loop().time(),
        "process_id": os.getpid(),
        "memory": {
            "used_mb": psutil.virtual_memory().used / 1024 / 1024,
            "available_mb": psutil.virtual_memory().available / 1024 / 1024,
            "percent": psutil.virtual_memory().percent
        },
        "cpu_percent": psutil.cpu_percent(),
        "connections": 42,
        "timestamp": str(asyncio.get_event_loop().time())
    }


# 升级版函数：增强玩家封禁 V2
async def handle_player_ban_v2(payload: Dict[str, Any]) -> Dict[str, Any]:
    """处理增强版玩家封禁请求"""
    logger.info(f"🚫 [V2] Processing enhanced player ban: {payload}")

    await asyncio.sleep(0.15)

    return {
        "result": "success",
        "message": "Player banned with enhanced features",
        "version": "2.0",
        "player_id": payload.get("player_id"),
        "reason": payload.get("reason"),
        "features": ["account_ban", "ip_ban", "device_ban"],
        "ban_duration": payload.get("duration", 24) * 3600,  # 转换为秒
        "timestamp": str(asyncio.get_event_loop().time())
    }


# 升级版函数：增强服务器状态 V2
async def handle_server_status_v2(payload: Dict[str, Any]) -> Dict[str, Any]:
    """处理增强版服务器状态请求"""
    logger.info(f"📊 [V2] Processing enhanced server status: {payload}")

    import psutil
    import os

    return {
        "status": "running",
        "version": "2.0",
        "uptime": asyncio.get_event_loop().time(),
        "process_id": os.getpid(),
        "system": {
            "cpu": {
                "usage_percent": psutil.cpu_percent(interval=1),
                "count": psutil.cpu_count(),
                "freq": psutil.cpu_freq().current if psutil.cpu_freq() else None
            },
            "memory": {
                "total_gb": psutil.virtual_memory().total / 1024 / 1024 / 1024,
                "used_gb": psutil.virtual_memory().used / 1024 / 1024 / 1024,
                "available_gb": psutil.virtual_memory().available / 1024 / 1024 / 1024,
                "percent": psutil.virtual_memory().percent
            },
            "disk": {
                "total_gb": psutil.disk_usage('/').total / 1024 / 1024 / 1024,
                "used_gb": psutil.disk_usage('/').used / 1024 / 1024 / 1024,
                "free_gb": psutil.disk_usage('/').free / 1024 / 1024 / 1024,
                "percent": psutil.disk_usage('/').percent
            }
        },
        "network": {
            "connections": len(psutil.net_connections()),
            "io": psutil.net_io_counters()._asdict()
        },
        "performance": {
            "requests_per_second": 1250,
            "avg_response_time_ms": 23
        },
        "timestamp": str(asyncio.get_event_loop().time())
    }


async def print_reload_status(client):
    """打印热重载状态"""
    print("\n🔥 热重载状态:")
    print("================")

    status = client.get_reload_status()
    print(f"连接状态: {status.connection_status}")
    print(f"重连次数: {status.reconnect_count}")
    print(f"函数重载: {status.function_reloads}")
    print(f"配置重载: {status.config_reloads}")
    print(f"失败次数: {status.failed_reloads}")
    print(f"运行时间: {status.uptime:.1f}s")
    if status.last_reconnect_time:
        print(f"最后重连: {status.last_reconnect_time:.1f}")
    print("================\n")


async def demonstrate_hot_reload(client):
    """演示热重载功能"""
    await asyncio.sleep(10)

    logger.info("🔄 Demonstrating hot reload features...")

    # 1. 测试函数重载
    logger.info("\n1. Testing function reload...")
    await asyncio.sleep(5)

    try:
        await client.reload_function("player.ban", "1.1.0", handle_player_ban_v2)
        logger.info("✅ Function reload successful")
    except Exception as e:
        logger.error(f"❌ Function reload failed: {e}")

    # 2. 测试批量重载
    logger.info("\n2. Testing batch reload...")
    await asyncio.sleep(3)

    functions = {
        "server.status": {
            "version": "2.0.0",
            "handler": handle_server_status_v2
        }
    }

    try:
        await client.reload_functions(functions)
        logger.info("✅ Batch reload successful")
    except Exception as e:
        logger.error(f"❌ Batch reload failed: {e}")

    # 3. 定期打印状态
    while True:
        await asyncio.sleep(30)
        logger.info("\n📊 Current hot reload status:")
        await print_reload_status(client)


async def main():
    """主函数"""
    print("🔥 Croupier Python SDK with Hot Reload Example")

    # 热重载配置
    config = {
        'enabled': True,
        'auto_reconnect': True,
        'reconnect_delay': 5.0,
        'max_retry_attempts': 5,
        'health_check_interval': 30.0,
        'graceful_shutdown_timeout': 30.0,
        'file_watching': {
            'enabled': True,
            'watch_dir': './functions',
            'patterns': ['*.py', '*.json']
        },
        'tools': {
            'uvicorn': True,
            'watchdog': True,
            'importlib_reload': True
        }
    }

    # 创建热重载客户端
    client = create_hotreload_client(config)

    # 设置信号处理
    def signal_handler():
        logger.info("📡 Received shutdown signal")
        return asyncio.create_task(client.graceful_shutdown())

    # 注册信号处理器
    loop = asyncio.get_event_loop()
    for sig in [signal.SIGINT, signal.SIGTERM]:
        loop.add_signal_handler(
            sig, lambda: asyncio.create_task(client.graceful_shutdown())
        )

    try:
        # 注册函数
        client.register_function("player.ban", "1.0.0", handle_player_ban)
        client.register_function("server.status", "1.0.0", handle_server_status)

        # 连接到Agent
        await client.connect()

        # 打印初始状态
        await print_reload_status(client)

        logger.info("✅ Server is running!")
        logger.info("💡 Modify .py files to trigger hot reload")
        logger.info("💡 Use Ctrl+C for graceful shutdown")

        # 启动演示任务
        demo_task = asyncio.create_task(demonstrate_hot_reload(client))

        # 等待关闭
        await client.shutdown_event.wait()

        # 取消演示任务
        demo_task.cancel()

        logger.info("🛑 Service shutdown complete")

    except KeyboardInterrupt:
        logger.info("📡 Received keyboard interrupt")
        await client.graceful_shutdown()
    except Exception as e:
        logger.error(f"❌ Unexpected error: {e}")
        await client.graceful_shutdown()
        raise


if __name__ == "__main__":
    # 安装依赖检查
    try:
        import psutil
    except ImportError:
        print("❌ Missing psutil dependency. Install with: pip install psutil")
        exit(1)

    try:
        import watchdog
    except ImportError:
        print("⚠️ Missing watchdog dependency. File watching will be disabled.")
        print("   Install with: pip install watchdog")

    # 运行主程序
    asyncio.run(main())