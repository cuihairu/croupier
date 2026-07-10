"""
Croupier Python SDK - Invoker Implementation

Provides client functionality for invoking functions registered with the Croupier platform.
Supports synchronous calls, asynchronous tasks, and event streaming.

Uses TCP transport for communication with the Agent.
"""

from __future__ import annotations

import asyncio
import json
import logging
import random
from dataclasses import dataclass, field
from pathlib import Path
from types import ModuleType
from typing import AsyncIterator, Dict, Optional, Any

from . import protocol
from .transport.tcp import TCPTransport

# Reuse the proto module loader from the main package
_GENERATED_ROOT = Path(__file__).resolve().parent.parent / "generated"


def _ensure_parent_packages(module_name: str) -> None:
    parts = module_name.split(".")[:-1]
    prefix = ""
    for part in parts:
        prefix = f"{prefix}.{part}" if prefix else part
        if prefix not in __import__("sys").modules:
            module = ModuleType(prefix)
            module.__path__ = []  # type: ignore[attr-defined]
            __import__("sys").modules[prefix] = module


def _load_proto_module(module_name: str) -> ModuleType:
    import importlib.util
    import sys

    if module_name in sys.modules:
        return sys.modules[module_name]

    relative = Path(*module_name.split("."))  # type: ignore[arg-type]
    file_path = _GENERATED_ROOT / relative.with_suffix(".py")
    if not file_path.exists():
        raise ImportError(f"Generated module {module_name} not found at {file_path}")

    _ensure_parent_packages(module_name)
    spec = importlib.util.spec_from_file_location(module_name, file_path)
    if spec is None or spec.loader is None:
        raise ImportError(f"Unable to load module {module_name}")

    module = importlib.util.module_from_spec(spec)
    sys.modules[module_name] = module
    spec.loader.exec_module(module)
    return module


# Load protobuf modules (messages only, no gRPC)
invocation_pb2 = _load_proto_module("croupier.sdk.v1.invocation_pb2")

LOG = logging.getLogger(__name__)
_TASK_STREAM_POLL_INTERVAL_SECONDS = 0.5


@dataclass
class ReconnectConfig:
    """Configuration for automatic reconnection with exponential backoff."""

    enabled: bool = True
    max_attempts: int = 0  # 0 = infinite retries
    initial_delay_ms: int = 1000  # 1 second
    max_delay_ms: int = 30000  # 30 seconds
    backoff_multiplier: float = 2.0
    jitter_factor: float = 0.2  # Add randomness to delay (0-1)


def _default_reconnect_config() -> ReconnectConfig:
    """Create default reconnection configuration."""
    return ReconnectConfig()


@dataclass
class RetryConfig:
    """Configuration for retrying failed invocations."""

    enabled: bool = True
    max_attempts: int = 3  # Maximum retry attempts
    initial_delay_ms: int = 100  # Initial retry delay in milliseconds
    max_delay_ms: int = 5000  # Maximum retry delay in milliseconds
    backoff_multiplier: float = 2.0
    jitter_factor: float = 0.1


def _default_retry_config() -> RetryConfig:
    """Create default retry configuration."""
    return RetryConfig()


def _calculate_reconnect_delay(attempt: int, config: ReconnectConfig) -> float:
    """Calculate reconnection delay using exponential backoff with jitter."""
    delay = config.initial_delay_ms * (config.backoff_multiplier**attempt)
    delay = min(delay, config.max_delay_ms)
    jitter = delay * config.jitter_factor * (random.random() * 2 - 1)
    delay += jitter
    return max(delay, 0) / 1000.0


@dataclass
class InvokerConfig:
    """Configuration for the Invoker connection."""

    address: str = "127.0.0.1:19090"
    timeout: int = 30000  # milliseconds
    insecure: bool = True
    ca_file: str = ""
    cert_file: str = ""
    key_file: str = ""
    server_name: str = ""
    reconnect: ReconnectConfig = field(default_factory=_default_reconnect_config)
    retry: RetryConfig = field(default_factory=_default_retry_config)


@dataclass
class InvokeOptions:
    """Options for function invocation."""

    idempotency_key: Optional[str] = None
    timeout: Optional[int] = None
    headers: Optional[Dict[str, str]] = None
    retry: Optional[RetryConfig] = None


@dataclass
class TaskEventInfo:
    """Information about a task event."""

    type: str  # "started" | "progress" | "completed" | "error" | "cancelled"
    task_id: str
    payload: Optional[str] = None
    message: Optional[str] = None
    progress: Optional[int] = None
    error: Optional[str] = None
    done: bool = False


class Invoker:
    """
    Client for invoking functions registered with the Croupier platform.

    Supports:
    - Synchronous function invocation
    - Asynchronous task execution with event streaming
    - Task cancellation
    - Payload validation with schemas
    - Automatic reconnection with exponential backoff

    Uses TCP transport for communication with the Agent.
    """

    def __init__(self, config: Optional[InvokerConfig] = None):
        """Initialize the invoker with configuration."""
        self.config = config or InvokerConfig()
        self._transport = TCPTransport(
            address=self.config.address,
            timeout_ms=self.config.timeout,
        )
        self._schemas: Dict[str, Dict[str, Any]] = {}
        self._connected = False
        self._lock: Optional[asyncio.Lock] = None

        # Reconnection state
        self._reconnect_task: Optional[asyncio.Task] = None
        self._reconnect_attempts = 0
        self._is_reconnecting = False
        self._stop_reconnect = asyncio.Event()

    async def connect(self) -> None:
        """Connect to the server/agent."""
        if self._lock is None:
            self._lock = asyncio.Lock()

        async with self._lock:
            if self._connected:
                return

            LOG.info(f"Connecting to server/agent at: {self.config.address}")

            # Use TCP transport
            loop = asyncio.get_event_loop()
            await loop.run_in_executor(None, self._transport.connect)

            self._connected = True
            self._reconnect_attempts = 0
            LOG.info(f"Connected to: {self.config.address}")

    async def invoke(
        self, function_id: str, payload: str, options: Optional[InvokeOptions] = None
    ) -> str:
        """Synchronously invoke a function."""
        options = options or InvokeOptions()

        # Client-side validation
        if function_id in self._schemas:
            schema = self._schemas[function_id]
            self._validate_payload(payload, schema)

        if not self._connected:
            await self.connect()

        # Build InvokeRequest
        req = invocation_pb2.InvokeRequest(
            function_id=function_id,
            payload=payload.encode("utf-8"),
            metadata=options.headers or {},
        )
        if options.idempotency_key:
            req.idempotency_key = options.idempotency_key

        # Send request via TCP
        loop = asyncio.get_event_loop()
        req_data = req.SerializeToString()
        _, resp_data = await loop.run_in_executor(
            None,
            self._transport.call,
            protocol.MSG_INVOKE_REQUEST,
            req_data,
        )

        # Parse response
        resp = invocation_pb2.InvokeResponse()
        resp.ParseFromString(resp_data)

        return str(resp.payload.decode("utf-8"))

    async def start_task(
        self, function_id: str, payload: str, options: Optional[InvokeOptions] = None
    ) -> str:
        """Start an asynchronous task."""
        options = options or InvokeOptions()

        if not self._connected:
            await self.connect()

        # Client-side validation
        if function_id in self._schemas:
            schema = self._schemas[function_id]
            self._validate_payload(payload, schema)

        # Build StartTaskRequest (using InvokeRequest for now)
        req = invocation_pb2.InvokeRequest(
            function_id=function_id,
            payload=payload.encode("utf-8"),
            metadata=options.headers or {},
        )
        if options.idempotency_key:
            req.idempotency_key = options.idempotency_key

        # Send request via TCP
        loop = asyncio.get_event_loop()
        req_data = req.SerializeToString()
        _, resp_data = await loop.run_in_executor(
            None,
            self._transport.call,
            protocol.MSG_START_TASK_REQUEST,
            req_data,
        )

        # Parse response
        resp = invocation_pb2.StartTaskResponse()
        resp.ParseFromString(resp_data)

        return str(resp.task_id)

    async def stream_task(self, task_id: str) -> AsyncIterator[TaskEventInfo]:
        """Stream events from a running task."""
        if not self._connected:
            await self.connect()

        # Build StreamTaskRequest
        req = invocation_pb2.TaskStreamRequest(task_id=task_id)
        req_data = req.SerializeToString()

        loop = asyncio.get_event_loop()
        while True:
            _, resp_data = await loop.run_in_executor(
                None,
                self._transport.call,
                protocol.MSG_STREAM_TASK_REQUEST,
                req_data,
            )

            event = invocation_pb2.TaskEvent()
            event.ParseFromString(resp_data)
            task_event = self._normalize_task_event(task_id, event)

            yield task_event

            if task_event.done:
                break

            await asyncio.sleep(_TASK_STREAM_POLL_INTERVAL_SECONDS)

    async def cancel_task(self, task_id: str) -> None:
        """Cancel a running task."""
        if not self._connected:
            await self.connect()

        # Build CancelTaskRequest
        req = invocation_pb2.CancelTaskRequest(task_id=task_id)
        req_data = req.SerializeToString()

        # Send request via TCP
        loop = asyncio.get_event_loop()
        await loop.run_in_executor(
            None,
            self._transport.call,
            protocol.MSG_CANCEL_TASK_REQUEST,
            req_data,
        )

    async def set_schema(self, function_id: str, schema: Dict[str, Any]) -> None:
        """Set validation schema for a function."""
        self._schemas[function_id] = schema
        LOG.debug(f"Set schema for function: {function_id}")

    async def close(self) -> None:
        """Close the invoker."""
        if self._lock is None:
            self._lock = asyncio.Lock()

        self._stop_reconnect.set()
        if self._reconnect_task and not self._reconnect_task.done():
            self._reconnect_task.cancel()
            try:
                await self._reconnect_task
            except asyncio.CancelledError:
                pass

        async with self._lock:
            self._schemas.clear()

            # Close TCP transport
            loop = asyncio.get_event_loop()
            await loop.run_in_executor(None, self._transport.close)

            self._connected = False
            LOG.info("Invoker closed")

    def _normalize_task_event(self, task_id: str, event: Any) -> TaskEventInfo:
        event_type = str(getattr(event, "type", ""))
        message = str(getattr(event, "message", "")) or None
        payload_bytes = bytes(getattr(event, "payload", b""))

        if event_type == "done":
            event_type = "completed"
        elif event_type == "error" and message and "cancel" in message.lower():
            event_type = "cancelled"

        error = message if event_type in ("error", "cancelled") else None
        done = event_type in ("completed", "error", "cancelled")

        return TaskEventInfo(
            type=event_type,
            task_id=task_id,
            payload=payload_bytes.decode("utf-8") if payload_bytes else None,
            message=message,
            progress=int(getattr(event, "progress", 0)),
            error=error,
            done=done,
        )

    def _validate_payload(self, payload: str, schema: Dict[str, Any]) -> None:
        """Validate payload against JSON Schema."""
        if not schema:
            if not payload:
                raise Exception("Payload cannot be empty")
            return

        try:
            payload_obj = json.loads(payload)
        except json.JSONDecodeError as e:
            raise Exception(f"Invalid JSON payload: {e}") from e

        # Required field validation
        required = schema.get("required", [])
        if isinstance(required, list):
            for fieldname in required:
                if fieldname not in payload_obj:
                    raise Exception(
                        f"Payload validation failed: missing required field '{fieldname}'"
                    )

        LOG.debug(f"Payload validation for {len(payload)} characters completed")

    def _schedule_reconnect(self) -> None:
        """Schedule a reconnection attempt with exponential backoff."""
        if self._is_reconnecting:
            return

        if (
            self.config.reconnect.max_attempts > 0
            and self._reconnect_attempts >= self.config.reconnect.max_attempts
        ):
            LOG.error("Max reconnection attempts reached. Giving up.")
            return

        self._is_reconnecting = True

        if self._reconnect_task and not self._reconnect_task.done():
            self._reconnect_task.cancel()

        self._stop_reconnect.clear()
        self._reconnect_task = asyncio.create_task(self._reconnect_loop())

    async def _reconnect_loop(self) -> None:
        """Background task that handles reconnection with exponential backoff."""
        try:
            while not self._stop_reconnect.is_set():
                if (
                    self.config.reconnect.max_attempts > 0
                    and self._reconnect_attempts >= self.config.reconnect.max_attempts
                ):
                    LOG.error("Max reconnection attempts reached. Giving up.")
                    break

                delay = _calculate_reconnect_delay(self._reconnect_attempts, self.config.reconnect)
                self._reconnect_attempts += 1

                LOG.info(
                    f"Scheduling reconnection attempt {self._reconnect_attempts} "
                    f"in {delay:.1f}s"
                )

                try:
                    await asyncio.wait_for(self._stop_reconnect.wait(), timeout=delay)
                    break
                except asyncio.TimeoutError:
                    pass

                try:
                    await self.connect()
                    LOG.info("Reconnection successful")
                    break
                except Exception as e:
                    LOG.warning(f"Reconnection attempt failed: {e}")

        except asyncio.CancelledError:
            LOG.debug("Reconnection task cancelled")
        finally:
            self._is_reconnecting = False


# Convenience functions
def default_invoker_config() -> InvokerConfig:
    """Create a default Invoker configuration."""
    return InvokerConfig()


def create_invoker(config: Optional[InvokerConfig] = None) -> Invoker:
    """Create a new Invoker instance."""
    return Invoker(config)


# Synchronous wrapper
class SyncInvoker:
    """
    Synchronous wrapper around the async Invoker.

    Provides a blocking interface for applications that don't use asyncio.
    """

    def __init__(self, config: Optional[InvokerConfig] = None):
        self._async_invoker = Invoker(config)
        self._loop: Optional[asyncio.AbstractEventLoop] = None

    def _get_loop(self) -> asyncio.AbstractEventLoop:
        """Get or create an event loop."""
        try:
            loop = asyncio.get_event_loop()
            if loop.is_running():
                raise RuntimeError("Running event loop detected")
            return loop
        except RuntimeError:
            loop = asyncio.new_event_loop()
            asyncio.set_event_loop(loop)
            return loop

    def connect(self) -> None:
        """Connect to the server."""
        loop = self._get_loop()
        loop.run_until_complete(self._async_invoker.connect())

    def invoke(
        self, function_id: str, payload: str, options: Optional[InvokeOptions] = None
    ) -> str:
        """Synchronously invoke a function."""
        loop = self._get_loop()
        return loop.run_until_complete(self._async_invoker.invoke(function_id, payload, options))

    def start_task(
        self, function_id: str, payload: str, options: Optional[InvokeOptions] = None
    ) -> str:
        """Start an asynchronous task."""
        loop = self._get_loop()
        return loop.run_until_complete(self._async_invoker.start_task(function_id, payload, options))

    def stream_task(self, task_id: str):
        """Stream events from a running task."""
        loop = self._get_loop()
        async_gen = self._async_invoker.stream_task(task_id)

        class SyncIterator:
            def __init__(self, async_gen, loop):
                self._async_gen = async_gen
                self._loop = loop

            def __iter__(self):
                return self

            def __next__(self):
                try:
                    return self._loop.run_until_complete(self._async_gen.__anext__())
                except StopAsyncIteration:
                    raise StopIteration

        return SyncIterator(async_gen, loop)

    def cancel_task(self, task_id: str) -> None:
        """Cancel a running task."""
        loop = self._get_loop()
        loop.run_until_complete(self._async_invoker.cancel_task(task_id))

    def set_schema(self, function_id: str, schema: Dict[str, Any]) -> None:
        """Set validation schema for a function."""
        self._async_invoker._schemas[function_id] = schema
        LOG.debug(f"Set schema for function: {function_id}")

    def close(self) -> None:
        """Close the invoker."""
        loop = self._get_loop()
        loop.run_until_complete(self._async_invoker.close())


def create_sync_invoker(config: Optional[InvokerConfig] = None) -> SyncInvoker:
    """Create a new synchronous Invoker instance."""
    return SyncInvoker(config)


__all__ = [
    "ReconnectConfig",
    "InvokerConfig",
    "InvokeOptions",
    "TaskEventInfo",
    "Invoker",
    "SyncInvoker",
    "default_invoker_config",
    "create_invoker",
    "create_sync_invoker",
]
