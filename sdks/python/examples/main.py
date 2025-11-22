"""
Croupier Python SDK Example - File Transfer for Server Hot Reload
"""

import asyncio
import logging
import signal
import json
from typing import Dict, Any

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

    try:
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
    except ImportError:
        return {
            "status": "running",
            "uptime": asyncio.get_event_loop().time(),
            "process_id": __import__('os').getpid(),
            "timestamp": str(asyncio.get_event_loop().time())
        }


async def main():
    """主函数"""
    print("📡 Croupier Python SDK - File Transfer Example")
    print("==============================================")
    print("🔧 Ready for server-side hot reload support")

    # 基础客户端功能尚未实现
    # 此示例展示未来的API使用方式
    print("⚠️ Basic client is a placeholder - implementation in progress")

    print("\n📝 Function handlers defined:")
    print("  - player.ban: Player ban functionality")
    print("  - server.status: Server status monitoring")

    print("\n🎮 Example completed - use gRPC client directly for now")
    print("💡 File transfer capabilities will be added in future releases")


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        logger.info("📡 Received keyboard interrupt")
    except Exception as e:
        logger.error(f"❌ Unexpected error: {e}")
        raise