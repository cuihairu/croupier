"""
Croupier Python SDK with Hot Reload Support
"""

import asyncio
import importlib
import sys
import time
import logging
import threading
from typing import Dict, Any, Callable, Optional
from pathlib import Path
from dataclasses import dataclass, field
from contextlib import asynccontextmanager

# 第三方依赖
try:
    from watchdog.observers import Observer
    from watchdog.events import FileSystemEventHandler
    WATCHDOG_AVAILABLE = True
except ImportError:
    WATCHDOG_AVAILABLE = False
    logging.warning("watchdog not available, file watching disabled")


@dataclass
class HotReloadConfig:
    """热重载配置"""
    enabled: bool = True
    auto_reconnect: bool = True
    reconnect_delay: float = 5.0
    max_retry_attempts: int = 10
    health_check_interval: float = 30.0
    graceful_shutdown_timeout: float = 30.0

    # 文件监听配置
    file_watching: Dict[str, Any] = field(default_factory=lambda: {
        'enabled': False,
        'watch_dir': './functions',
        'patterns': ['*.py', '*.json', '*.yaml']
    })

    # 工具集成配置
    tools: Dict[str, bool] = field(default_factory=lambda: {
        'uvicorn': True,
        'watchdog': True,
        'importlib_reload': True
    })


@dataclass
class HotReloadMetrics:
    """热重载指标"""
    reconnect_count: int = 0
    last_reconnect_time: Optional[float] = None
    function_reloads: int = 0
    config_reloads: int = 0
    failed_reloads: int = 0
    connection_status: str = "disconnected"
    uptime: float = 0.0


class ReloadHandler(FileSystemEventHandler):
    """文件变更处理器"""

    def __init__(self, client):
        self.client = client
        self.logger = logging.getLogger(__name__)

    def on_modified(self, event):
        if event.is_directory:
            return

        file_path = event.src_path
        self.logger.info(f"📁 File modified: {file_path}")

        # 根据文件类型触发不同的重载行为
        if file_path.endswith('.py'):
            self.handle_python_file_change(file_path)
        elif file_path.endswith(('.json', '.yaml', '.yml')):
            self.handle_config_file_change(file_path)

    def handle_python_file_change(self, file_path):
        """处理Python文件变更"""
        if self.client.config.tools['importlib_reload']:
            try:
                module_name = self.path_to_module(file_path)
                if module_name and module_name in sys.modules:
                    importlib.reload(sys.modules[module_name])
                    self.logger.info(f"🔄 Reloaded module: {module_name}")

                    # 触发重新注册
                    asyncio.create_task(self.client.reregister_all_functions())
            except Exception as e:
                self.logger.error(f"❌ Failed to reload module: {e}")

    def handle_config_file_change(self, file_path):
        """处理配置文件变更"""
        self.logger.info(f"🔄 Configuration file changed: {file_path}")
        # 这里可以实现配置重载逻辑

    def path_to_module(self, file_path):
        """将文件路径转换为模块名"""
        try:
            path = Path(file_path)
            if path.suffix != '.py':
                return None

            # 简单的路径到模块名转换
            relative_path = path.relative_to(Path.cwd())
            module_parts = list(relative_path.parts[:-1]) + [relative_path.stem]
            return '.'.join(module_parts)
        except Exception:
            return None


class HotReloadableClient:
    """支持热重载的Croupier Python客户端"""

    def __init__(self, config: HotReloadConfig):
        self.config = config
        self.logger = logging.getLogger(__name__)

        # 状态管理
        self.is_connected = False
        self.is_reloading = False
        self.functions: Dict[str, Any] = {}
        self.start_time = time.time()

        # 指标
        self.metrics = HotReloadMetrics()

        # 文件监听
        self.observer = None
        self.handler = None

        # 异步任务管理
        self.reconnect_task = None
        self.health_check_task = None
        self.shutdown_event = asyncio.Event()

        if self.config.enabled:
            self.setup_hot_reload_support()

    def register_function(self, function_id: str, version: str, handler: Callable):
        """注册函数"""
        if self.is_reloading:
            raise RuntimeError("Cannot register functions during reload operation")

        self.functions[function_id] = {
            'id': function_id,
            'version': version,
            'handler': handler,
            'registered_at': time.time()
        }

        self.logger.info(f"📝 Registered function: {function_id} (version: {version})")
        return self

    async def connect(self):
        """连接到Agent"""
        self.logger.info(f"🔌 Connecting to Croupier Agent")

        try:
            # 这里实现实际的gRPC连接逻辑
            await self._establish_connection()

            # 注册所有函数
            await self.register_all_functions()

            self.is_connected = True
            self.metrics.connection_status = "connected"
            self.logger.info("✅ Successfully connected to Agent")

            return self
        except Exception as e:
            self.logger.error(f"❌ Connection failed: {e}")
            self.metrics.connection_status = "error"
            raise

    async def reload_function(self, function_id: str, version: str, handler: Callable):
        """重新加载单个函数"""
        if self.is_reloading:
            raise RuntimeError("Reload operation already in progress")

        self.is_reloading = True
        self.metrics.connection_status = "reloading"
        self.logger.info(f"🔄 Reloading function: {function_id}")

        try:
            # 保存旧函数用于回滚
            old_function = self.functions.get(function_id)

            # 更新函数
            self.functions[function_id] = {
                'id': function_id,
                'version': version,
                'handler': handler,
                'reloaded_at': time.time()
            }

            # 重新注册到Agent
            await self._register_single_function(function_id, version, handler)

            self.metrics.function_reloads += 1
            self.logger.info(f"✅ Function {function_id} reloaded successfully")

            return self
        except Exception as e:
            self.metrics.failed_reloads += 1
            self.logger.error(f"❌ Failed to reload function {function_id}: {e}")

            # 回滚
            if old_function:
                self.functions[function_id] = old_function

            raise
        finally:
            self.is_reloading = False
            self.metrics.connection_status = "connected"

    async def reload_functions(self, functions: Dict[str, Dict[str, Any]]):
        """批量重载函数"""
        if self.is_reloading:
            raise RuntimeError("Reload operation already in progress")

        self.is_reloading = True
        self.logger.info(f"🔄 Batch reloading {len(functions)} functions")

        results = []
        errors = []

        try:
            for function_id, func_data in functions.items():
                try:
                    await self.reload_function(
                        function_id,
                        func_data['version'],
                        func_data['handler']
                    )
                    results.append(function_id)
                except Exception as e:
                    errors.append({'function_id': function_id, 'error': str(e)})

            if errors:
                error_msg = f"Failed to reload {len(errors)} out of {len(functions)} functions"
                self.logger.error(error_msg)
                raise RuntimeError(error_msg)

            self.logger.info(f"✅ Successfully reloaded all {len(results)} functions")
            return self
        finally:
            self.is_reloading = False

    async def reload_config(self, new_config: HotReloadConfig):
        """重载配置"""
        self.logger.info("🔄 Reloading client configuration")

        # 合并配置
        self.config = new_config

        self.metrics.config_reloads += 1
        self.logger.info("✅ Configuration reloaded successfully")

        return self

    def get_reload_status(self) -> HotReloadMetrics:
        """获取重载状态"""
        self.metrics.uptime = time.time() - self.start_time
        return self.metrics

    async def reconnect(self):
        """重新连接"""
        self.logger.info("🔄 Attempting to reconnect...")

        try:
            # 断开当前连接
            await self.disconnect()

            # 重新连接
            await self.connect()

            self.metrics.reconnect_count += 1
            self.metrics.last_reconnect_time = time.time()

            self.logger.info("✅ Reconnection successful")
            return self
        except Exception as e:
            self.metrics.failed_reloads += 1
            self.logger.error(f"❌ Reconnection failed: {e}")
            raise

    async def graceful_shutdown(self, timeout: float = None):
        """优雅关闭"""
        if timeout is None:
            timeout = self.config.graceful_shutdown_timeout

        self.logger.info(f"🛑 Starting graceful shutdown (timeout: {timeout}s)")

        # 设置关闭事件
        self.shutdown_event.set()

        # 停止文件监听
        self.stop_file_watching()

        # 取消异步任务
        if self.reconnect_task and not self.reconnect_task.done():
            self.reconnect_task.cancel()
        if self.health_check_task and not self.health_check_task.done():
            self.health_check_task.cancel()

        # 断开连接
        try:
            await asyncio.wait_for(self.disconnect(), timeout=timeout)
        except asyncio.TimeoutError:
            self.logger.warning("⚠️ Graceful shutdown timeout, forcing close")

        self.logger.info("✅ Graceful shutdown completed")

    def setup_hot_reload_support(self):
        """设置热重载支持"""
        # 启动自动重连
        if self.config.auto_reconnect:
            self.reconnect_task = asyncio.create_task(self.auto_reconnect_loop())

        # 启动健康检查
        self.health_check_task = asyncio.create_task(self.health_check_loop())

        # 启动文件监听
        if self.config.file_watching['enabled'] and WATCHDOG_AVAILABLE:
            self.start_file_watching()

        self.logger.info("🔥 Hot reload support enabled")

    async def auto_reconnect_loop(self):
        """自动重连循环"""
        while not self.shutdown_event.is_set():
            try:
                await asyncio.sleep(self.config.health_check_interval)

                if not self.is_connected and not self.is_reloading:
                    await self.attempt_reconnect()
            except asyncio.CancelledError:
                break
            except Exception as e:
                self.logger.error(f"❌ Auto reconnect error: {e}")

    async def health_check_loop(self):
        """健康检查循环"""
        while not self.shutdown_event.is_set():
            try:
                await asyncio.sleep(self.config.health_check_interval)

                if self.is_connected:
                    # 执行健康检查
                    await self._perform_health_check()
            except asyncio.CancelledError:
                break
            except Exception as e:
                self.logger.error(f"❌ Health check error: {e}")
                self.is_connected = False

    async def attempt_reconnect(self):
        """尝试重连"""
        delay = self.config.reconnect_delay

        for attempt in range(1, self.config.max_retry_attempts + 1):
            self.logger.info(f"🔄 Reconnection attempt {attempt}/{self.config.max_retry_attempts}")

            try:
                await self.reconnect()
                return  # 成功重连
            except Exception as e:
                self.logger.error(f"❌ Reconnection attempt {attempt} failed: {e}")

                if attempt < self.config.max_retry_attempts:
                    await asyncio.sleep(delay)
                    # 指数退避
                    delay = min(delay * 1.5, 60.0)

        self.logger.error("❌ All reconnection attempts failed")

    def start_file_watching(self):
        """启动文件监听"""
        if not WATCHDOG_AVAILABLE:
            self.logger.warning("⚠️ Watchdog not available, file watching disabled")
            return

        watch_dir = self.config.file_watching.get('watch_dir', './functions')
        if not Path(watch_dir).exists():
            self.logger.warning(f"⚠️ Watch directory does not exist: {watch_dir}")
            return

        self.logger.info(f"👀 Watching directory: {watch_dir}")

        self.handler = ReloadHandler(self)
        self.observer = Observer()
        self.observer.schedule(self.handler, watch_dir, recursive=True)
        self.observer.start()

    def stop_file_watching(self):
        """停止文件监听"""
        if self.observer:
            self.observer.stop()
            self.observer.join()
            self.observer = None
            self.logger.info("👀 File watching stopped")

    async def register_all_functions(self):
        """注册所有函数到Agent"""
        self.logger.info(f"📋 Registering {len(self.functions)} functions with Agent")

        for function_id, func_data in self.functions.items():
            await self._register_single_function(
                function_id,
                func_data['version'],
                func_data['handler']
            )

    async def reregister_all_functions(self):
        """重新注册所有函数"""
        if self.is_connected:
            await self.register_all_functions()

    async def _establish_connection(self):
        """建立连接（实际实现需要添加gRPC逻辑）"""
        # 模拟连接延迟
        await asyncio.sleep(0.1)

    async def _register_single_function(self, function_id: str, version: str, handler: Callable):
        """注册单个函数到Agent（实际实现需要添加gRPC逻辑）"""
        # 模拟注册延迟
        await asyncio.sleep(0.05)

    async def _perform_health_check(self):
        """执行健康检查（实际实现需要添加gRPC逻辑）"""
        # 模拟健康检查
        await asyncio.sleep(0.01)

    async def disconnect(self):
        """断开连接"""
        if self.is_connected:
            self.is_connected = False
            self.metrics.connection_status = "disconnected"
            self.logger.info("🔌 Disconnected from Agent")


# 工厂函数
def create_hotreload_client(config_dict: Dict[str, Any] = None) -> HotReloadableClient:
    """创建热重载客户端"""
    config = HotReloadConfig()

    if config_dict:
        for key, value in config_dict.items():
            if hasattr(config, key):
                setattr(config, key, value)

    return HotReloadableClient(config)


# 上下文管理器
@asynccontextmanager
async def hotreload_client(config_dict: Dict[str, Any] = None):
    """热重载客户端上下文管理器"""
    client = create_hotreload_client(config_dict)

    try:
        await client.connect()
        yield client
    finally:
        await client.graceful_shutdown()