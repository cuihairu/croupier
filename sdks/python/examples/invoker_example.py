"""
Croupier Python SDK - Invoker Example

Demonstrates how to use the Invoker to call functions registered with the Croupier platform.
"""

from __future__ import annotations

import asyncio
import json
import sys
import time
from typing import Optional

from croupier import (
    InvokerConfig,
    InvokeOptions,
    TaskEventInfo,
    create_sync_invoker,
)


async def sync_invoke_example() -> None:
    """Demonstrate synchronous function invocation."""
    print("=" * 60)
    print("同步调用示例 (Synchronous Invocation)")
    print("=" * 60)

    # Import Invoker (async version)
    from croupier import create_invoker

    # L3 调用方仅访问 Server HTTP API；Provider TCP 注册链路不用于此处。
    config = InvokerConfig(
        address="http://127.0.0.1:18780/api/v1",
        timeout=30000,
        insecure=True,
    )

    # Create Invoker instance
    invoker = create_invoker(config)

    try:
        # Connect to server
        await invoker.connect()
        print("✅ 已连接到服务器\n")

        # Prepare invocation payload
        function_id = "player.ban"
        payload = json.dumps({
            "player_id": "12345",
            "reason": "作弊行为",
            "duration": 86400  # 24 hours
        })

        # Set invocation options with idempotency key
        options = InvokeOptions(
            idempotency_key=f"sync-{int(time.time())}",
            headers={
                "X-Game-ID": "my-game",
                "X-Env": "development",
            }
        )

        # Invoke function synchronously
        result = await invoker.invoke(function_id, payload, options)
        print(f"📨 调用结果: {result}")

    except Exception as e:
        print(f"❌ 调用失败: {e}")
    finally:
        await invoker.close()


async def async_task_example() -> None:
    """Demonstrate asynchronous task with event streaming."""
    print("\n" + "=" * 60)
    print("异步任务示例 (Asynchronous Task)")
    print("=" * 60)

    from croupier import create_invoker

    # 创建独立 Server HTTP Invoker。
    invoker = create_invoker(InvokerConfig(address="http://127.0.0.1:18780/api/v1"))

    try:
        await invoker.connect()
        print("✅ 已连接到服务器\n")

        # Start an asynchronous task
        function_id = "player.ban"
        payload = json.dumps({
            "player_id": "67890",
            "reason": "严重违规",
            "duration": 604800  # 7 days
        })

        task_id = await invoker.start_task(function_id, payload)
        print(f"🚀 任务已启动，Task ID: {task_id}\n")

        status = await invoker.get_task_status(task_id)
        print(f"📊 当前任务状态: {status.status} ({status.progress or 0}%)\n")

        # Stream task events
        print("📡 接收任务事件...")
        async for event in invoker.stream_task(task_id):
            print(f"📬 事件 [{event.type}]: {event.message}")

            if event.payload:
                try:
                    payload_data = json.loads(event.payload)
                    print(f"   载荷: {json.dumps(payload_data, indent=2, ensure_ascii=False)}")
                except json.JSONDecodeError:
                    print(f"   载荷: {event.payload}")

            if event.progress is not None:
                print(f"   进度: {event.progress}%")

            if event.error:
                print(f"   错误: {event.error}")

            if event.done:
                break

        print("✅ 任务完成")

    except Exception as e:
        print(f"❌ 任务失败: {e}")
    finally:
        await invoker.close()


async def task_cancel_example() -> None:
    """Demonstrate task cancellation."""
    print("\n" + "=" * 60)
    print("取消任务示例 (Task Cancellation)")
    print("=" * 60)

    from croupier import create_invoker

    # 创建独立 Server HTTP Invoker。
    invoker = create_invoker(InvokerConfig(address="http://127.0.0.1:18780/api/v1"))

    try:
        await invoker.connect()
        print("✅ 已连接到服务器\n")

        # Start a long-running task
        function_id = "player.ban"
        payload = json.dumps({
            "player_id": "11111",
            "reason": "测试取消",
            "duration": 9999999  # Very long duration
        })

        task_id = await invoker.start_task(function_id, payload)
        print(f"🚀 任务已启动，Task ID: {task_id}\n")

        # Wait a bit then cancel
        await asyncio.sleep(1)

        # Cancel the task
        await invoker.cancel_task(task_id)
        print(f"🛑 任务已取消: {task_id}\n")

        # Verify the task was actually cancelled
        print("📡 验证任务状态...")
        async for event in invoker.stream_task(task_id):
            print(f"📬 事件 [{event.type}]: {event.message}")
            if event.done:
                break

    except Exception as e:
        print(f"❌ 操作失败: {e}")
    finally:
        await invoker.close()


async def schema_validation_example() -> None:
    """Demonstrate schema validation."""
    print("\n" + "=" * 60)
    print("Schema 验证示例 (Schema Validation)")
    print("=" * 60)

    from croupier import create_invoker

    # 创建独立 Server HTTP Invoker。
    invoker = create_invoker(InvokerConfig(address="http://127.0.0.1:18780/api/v1"))

    try:
        await invoker.connect()
        print("✅ 已连接到服务器\n")

        # Set function schema
        schema = {
            "type": "object",
            "properties": {
                "player_id": {"type": "string"},
                "reason": {"type": "string"},
                "duration": {"type": "number", "minimum": 0}
            },
            "required": ["player_id", "reason"]
        }

        await invoker.set_schema("player.ban", schema)
        print("✅ Schema 已设置\n")

        # Test valid payload
        valid_payload = json.dumps({
            "player_id": "22222",
            "reason": "测试验证",
            "duration": 3600
        })

        print("测试有效载荷...")
        try:
            result = await invoker.invoke("player.ban", valid_payload)
            print(f"✅ 有效载荷验证通过: {result}\n")
        except Exception as e:
            print(f"❌ 有效载荷验证失败: {e}\n")

        # Test invalid payload (missing required field)
        invalid_payload = json.dumps({
            "player_id": "22222"
            # Missing 'reason' field
        })

        print("测试无效载荷（缺少必需字段）...")
        try:
            await invoker.invoke("player.ban", invalid_payload)
            print("❌ 无效载荷应该被拒绝\n")
        except Exception as e:
            print(f"✅ 无效载荷被正确拒绝: {e}\n")

    except Exception as e:
        print(f"❌ 操作失败: {e}")
    finally:
        await invoker.close()


def sync_wrapper_example():
    """Demonstrate using the synchronous wrapper for non-async applications."""
    print("\n" + "=" * 60)
    print("同步封装示例 (Synchronous Wrapper)")
    print("=" * 60)

    # Use sync invoker for applications without asyncio
    invoker = create_sync_invoker(InvokerConfig(
        address="http://127.0.0.1:18780/api/v1",
        timeout=30000,
        insecure=True,
    ))

    try:
        # Connect
        invoker.connect()
        print("✅ 已连接到服务器\n")

        # Prepare payload
        payload = json.dumps({
            "player_id": "99999",
            "reason": "同步调用测试",
            "duration": 3600
        })

        # Invoke synchronously (blocking)
        options = InvokeOptions(
            idempotency_key=f"sync-wrapper-{int(time.time())}"
        )

        result = invoker.invoke("player.ban", payload, options)
        print(f"📨 调用结果: {result}")

    except Exception as e:
        print(f"❌ 调用失败: {e}")
    finally:
        invoker.close()


async def main_async():
    """Main async function."""
    print("🎮 Croupier Python SDK Invoker 示例")
    print("=" * 60)
    print()

    # Run async examples
    await sync_invoke_example()
    await async_task_example()
    await task_cancel_example()
    await schema_validation_example()

    print("\n✅ 所有异步示例完成")
    print("\n提示: 使用 'python invoker_example.py sync' 运行同步封装示例")


def main():
    """Main entry point."""
    if len(sys.argv) > 1 and sys.argv[1] == "sync":
        # Run sync wrapper example
        print("🎮 Croupier Python SDK - 同步封装示例")
        print("=" * 60)
        print()
        sync_wrapper_example()
    else:
        # Run async examples
        try:
            asyncio.run(main_async())
        except KeyboardInterrupt:
            print("\n\n⚠️  程序被用户中断")


if __name__ == "__main__":
    main()
