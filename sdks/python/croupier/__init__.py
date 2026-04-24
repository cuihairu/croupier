"""
Croupier Python SDK

Provides function registration and invocation for the Croupier platform.
The SDK connects to the Agent via a single bidirectional TCP session.
"""

from __future__ import annotations

import gzip
import importlib.util
import json
import logging
import queue
import sys
import threading
import uuid
from dataclasses import dataclass, field
from io import BytesIO
from pathlib import Path
from types import ModuleType
from typing import Callable, Dict, Optional

from importlib.metadata import version

try:
    __version__ = version("croupier-sdk")
except Exception:
    __version__ = "unknown"

__author__ = "Croupier Team"
__email__ = "dev@croupier.io"

LOG = logging.getLogger(__name__)

_GENERATED_ROOT = Path(__file__).resolve().parent.parent / "generated"


def _ensure_parent_packages(module_name: str) -> None:
    parts = module_name.split(".")[:-1]
    prefix = ""
    for part in parts:
        prefix = f"{prefix}.{part}" if prefix else part
        if prefix not in sys.modules:
            package = ModuleType(prefix)
            package.__path__ = []  # type: ignore[attr-defined]
            sys.modules[prefix] = package


def _load_proto_module(module_name: str) -> ModuleType:
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


# Load protobuf modules (no gRPC, only messages)
provider_pb2 = _load_proto_module("croupier.sdk.v1.provider_pb2")
invocation_pb2 = _load_proto_module("croupier.sdk.v1.invocation_pb2")
# Backward-compatible alias kept for older call sites and tests.
invoker_pb2 = invocation_pb2

from . import protocol  # noqa: E402
from .transport.tcp import TCPTransport  # noqa: E402

FunctionHandler = Callable[[str, bytes], str]


@dataclass
class FunctionDescriptor:
    """Describe a function exposed to the platform."""

    id: str
    version: str = "1.0.0"
    category: Optional[str] = None
    risk: Optional[str] = None
    entity: Optional[str] = None
    operation: Optional[str] = None
    enabled: bool = True


@dataclass
class ClientConfig:
    """Runtime configuration for the Python SDK client."""

    agent_addr: str = "127.0.0.1:19090"
    insecure: bool = True
    service_id: str = field(default_factory=lambda: f"python-sdk-{uuid.uuid4().hex[:8]}")
    service_version: str = "1.0.0"
    game_id: str = ""
    env: str = "development"
    agent_id: Optional[str] = None
    heartbeat_interval: int = 60
    timeout_seconds: int = 30
    control_addr: Optional[str] = None
    cert_file: Optional[str] = None
    key_file: Optional[str] = None
    ca_file: Optional[str] = None
    server_name: Optional[str] = None
    auth_token: Optional[str] = None
    headers: Dict[str, str] = field(default_factory=dict)
    provider_lang: str = "python"
    provider_sdk: str = "croupier-python-sdk"
    auto_reconnect: bool = True
    reconnect_interval: float = 1.0
    reconnect_max_attempts: int = 0
    disable_logging: bool = False
    debug_logging: bool = False
    log_level: str = "INFO"
    enable_file_transfer: bool = False
    max_file_size: int = 10 * 1024 * 1024

    # TLS knobs (forward-compatible)
    tls_enabled: bool = False
    tls_insecure_skip_verify: bool = False


class _JobState:
    def __init__(self) -> None:
        self.queue: "queue.Queue[Optional[invocation_pb2.JobEvent]]" = queue.Queue()  # type: ignore[name-defined]
        self.done = threading.Event()
        self.cancelled = threading.Event()

    def push(self, event: invocation_pb2.JobEvent, finished: bool = False) -> None:  # type: ignore[name-defined]
        if self.done.is_set():
            return
        self.queue.put(event)
        if finished:
            self.queue.put(None)
            self.done.set()


class CroupierClient:
    """
    Registers local handlers and manages function execution over a single
    bidirectional TCP session to the Agent.

    Lifecycle::

        client = CroupierClient(config)
        client.register_function(descriptor, handler)
        client.connect()   # TCP dial + handshake + heartbeat start
        ...
        client.disconnect()
    """

    def __init__(self, config: Optional[ClientConfig] = None) -> None:
        self._config = config or ClientConfig()
        self._handlers: Dict[str, FunctionHandler] = {}
        self._descriptors: Dict[str, FunctionDescriptor] = {}
        self._jobs: Dict[str, _JobState] = {}
        self._job_lock = threading.Lock()
        self._session_id = ""
        self._connected = False

        self._heartbeat_stop = threading.Event()
        self._heartbeat_thread: Optional[threading.Thread] = None
        self._transport: Optional[TCPTransport] = None
        self._state_lock = threading.RLock()

        # Drain state: when Agent sends ProviderDrainRequest, we stop
        # accepting new inbound requests, finish in-flight work, send
        # DrainComplete, and trigger a reconnect.
        self._draining = threading.Event()
        self._active_calls = threading.Semaphore(0)  # tracks in-flight count
        self._active_calls._counter = 0  # type: ignore[attr-defined]

    # ---- public API ----

    def register_function(self, descriptor: FunctionDescriptor, handler: FunctionHandler) -> None:
        if not descriptor.id or not descriptor.version:
            raise ValueError("Function descriptor must include id and version.")
        self._descriptors[descriptor.id] = descriptor
        self._handlers[descriptor.id] = handler

    def connect(self) -> None:
        with self._state_lock:
            if self._connected:
                return
            if not self._handlers:
                raise RuntimeError("Register at least one function before connecting.")

            self._heartbeat_stop.clear()
            self._connect_and_register()
            self._start_heartbeat_loop()
            self._connected = True
            LOG.info("Client connected with %d functions", len(self._handlers))

    def disconnect(self) -> None:
        self._heartbeat_stop.set()
        if self._heartbeat_thread:
            self._heartbeat_thread.join(timeout=2)

        with self._state_lock:
            if self._transport:
                self._transport.close()
                self._transport = None
            self._heartbeat_thread = None
            self._session_id = ""
            self._connected = False

    def get_function_descriptor(self, function_id: str) -> Optional[provider_pb2.LocalFunctionDescriptor]:  # type: ignore[name-defined]
        """Get a protobuf function descriptor for the given function ID."""
        desc = self._descriptors.get(function_id)
        if desc is None:
            return None
        return provider_pb2.LocalFunctionDescriptor(
            id=desc.id,
            version=desc.version,
            category=desc.category or "",
            risk=desc.risk or "",
            entity=desc.entity or "",
            operation=desc.operation or "",
        )

    def get_register_request(self) -> provider_pb2.RegisterLocalRequest:  # type: ignore[name-defined]
        """Build a registration request for the agent."""
        return provider_pb2.RegisterLocalRequest(
            service_id=self._config.service_id,
            version=self._config.service_version,
            rpc_addr="",
            functions=[self.get_function_descriptor(fid) for fid in self._handlers.keys()],  # type: ignore[misc]
        )

    def invoke(
        self, function_id: str, payload: bytes, metadata: Optional[Dict[str, str]] = None
    ) -> bytes:
        """Invoke a registered function handler."""
        handler = self._handlers.get(function_id)
        if handler is None:
            raise ValueError(f"Function {function_id} not found")

        metadata_json = json.dumps(metadata or {})
        result: bytes
        handler_result = handler(metadata_json, payload)
        if isinstance(handler_result, (bytes, bytearray)):
            result = bytes(handler_result)
        else:
            result = str(handler_result).encode("utf-8")
        return result

    def start_job(
        self, function_id: str, payload: bytes, metadata: Optional[Dict[str, str]] = None
    ) -> str:
        """Start an asynchronous job."""
        handler = self._handlers.get(function_id)
        if handler is None:
            raise ValueError(f"Function {function_id} not found")

        job_id = f"{function_id}-{uuid.uuid4().hex}"
        state = _JobState()
        with self._job_lock:
            self._jobs[job_id] = state

        state.push(
            invocation_pb2.JobEvent(type="started", message="job started", progress=0, payload=b"")  # type: ignore[name-defined]
        )

        metadata_json = json.dumps(metadata or {})

        def _run_job() -> None:
            try:
                handler_result = handler(metadata_json, payload)
                if state.cancelled.is_set():
                    return
                result: bytes
                if isinstance(handler_result, (bytes, bytearray)):
                    result = bytes(handler_result)
                else:
                    result = str(handler_result).encode("utf-8")
                state.push(
                    invocation_pb2.JobEvent(  # type: ignore[name-defined]
                        type="completed",
                        message="job completed",
                        progress=100,
                        payload=result,
                    ),
                    finished=True,
                )
            except Exception as exc:  # pylint: disable=broad-except
                if state.cancelled.is_set():
                    return
                LOG.exception("Job %s failed", job_id)
                state.push(
                    invocation_pb2.JobEvent(  # type: ignore[name-defined]
                        type="error",
                        message=str(exc),
                        progress=0,
                        payload=b"",
                    ),
                    finished=True,
                )

        threading.Thread(target=_run_job, daemon=True).start()
        return job_id

    def stream_job(self, job_id: str):  # type: ignore[misc]
        """Stream job events."""
        with self._job_lock:
            state = self._jobs.get(job_id)
        if state is None:
            raise ValueError(f"Job {job_id} not found")

        while True:
            event = state.queue.get()
            if event is None:
                break
            yield event  # type: ignore[misc]
        with self._job_lock:
            self._jobs.pop(job_id, None)

    def cancel_job(self, job_id: str) -> bool:
        """Cancel a running job."""
        with self._job_lock:
            state = self._jobs.get(job_id)
        if state and not state.done.is_set():
            state.cancelled.set()
            state.push(
                invocation_pb2.JobEvent(  # type: ignore[name-defined]
                    type="cancelled",
                    message="job cancelled",
                    progress=0,
                    payload=b"",
                ),
                finished=True,
            )
            return True
        return False

    def _handle_start_job(self, request, _context):  # type: ignore[no-untyped-def]
        """Compatibility shim for older direct handler tests/callers."""
        job_id = self.start_job(
            request.function_id,
            request.payload,
            dict(request.metadata),
        )
        return invocation_pb2.StartJobResponse(job_id=job_id)  # type: ignore[name-defined]

    def _handle_stream_job(self, request, _context):  # type: ignore[no-untyped-def]
        """Compatibility shim for older direct handler tests/callers."""
        return self.stream_job(request.job_id)

    def build_manifest(self) -> bytes:
        """Build a provider manifest JSON."""
        provider = {
            "id": self._config.service_id,
            "version": self._config.service_version,
            "lang": self._config.provider_lang,
            "sdk": self._config.provider_sdk,
        }
        functions = []
        for descriptor in self._descriptors.values():
            entry = {
                "id": descriptor.id,
                "version": descriptor.version or "1.0.0",
            }
            if descriptor.category:
                entry["category"] = descriptor.category
            if descriptor.risk:
                entry["risk"] = descriptor.risk
            if descriptor.entity:
                entry["entity"] = descriptor.entity
            if descriptor.operation:
                entry["operation"] = descriptor.operation
            if descriptor.enabled:
                entry["enabled"] = True  # type: ignore[assignment]
            functions.append(entry)

        manifest: Dict[str, object] = {"provider": provider}
        if functions:
            manifest["functions"] = functions
        return json.dumps(manifest, separators=(",", ":")).encode("utf-8")

    def gzip_bytes(self, payload: bytes) -> bytes:
        """Gzip compress bytes."""
        buffer = BytesIO()
        with gzip.GzipFile(fileobj=buffer, mode="wb") as handle:
            handle.write(payload)
        return buffer.getvalue()

    # ---- TCP session internals ----

    def _connect_and_register(self) -> None:
        """Dial the Agent over TCP, send ProviderConnectRequest, store session."""
        address = self._normalize_agent_addr(self._config.agent_addr)

        if self._transport:
            self._transport.close()

        tls_enabled = self._config.tls_enabled or not self._config.insecure

        transport = TCPTransport(
            address=address,
            timeout_ms=self._config.timeout_seconds * 1000,
            tls_enabled=tls_enabled,
            tls_cert_file=self._config.cert_file or "",
            tls_key_file=self._config.key_file or "",
            tls_ca_file=self._config.ca_file or "",
            tls_server_name=self._config.server_name or "",
            tls_insecure_skip_verify=self._config.tls_insecure_skip_verify,
        )
        # Set inbound handler BEFORE connecting so reader thread can dispatch
        transport.set_handler(self._handle_inbound)
        transport.connect()

        request = self.get_register_request()
        _, response_data = transport.call(
            protocol.MSG_REGISTER_LOCAL_REQUEST,
            request.SerializeToString(),
        )

        response = provider_pb2.RegisterLocalResponse()
        response.ParseFromString(response_data)
        if not response.session_id:
            transport.close()
            raise RuntimeError("RegisterLocal returned empty session_id")

        self._transport = transport
        self._session_id = response.session_id

    def _handle_inbound(self, msg_type: int, _req_id: int, body: bytes) -> bytes:
        """Handle inbound requests from the Agent (invoke, job, cancel, drain, stream)."""
        if msg_type == protocol.MSG_PROVIDER_DRAIN_REQUEST:
            return self._handle_drain_request(body)
        if msg_type == protocol.MSG_INVOKE_REQUEST:
            return self._handle_inbound_invoke(body)
        if msg_type == protocol.MSG_START_JOB_REQUEST:
            return self._handle_inbound_start_job(body)
        if msg_type == protocol.MSG_CANCEL_JOB_REQUEST:
            return self._handle_inbound_cancel_job(body)
        if msg_type == protocol.MSG_STREAM_JOB_REQUEST:
            return self._handle_inbound_stream_job(body)
        LOG.warning("Unsupported inbound MsgID: %s", protocol.msg_id_string(msg_type))
        return b""

    def _handle_inbound_invoke(self, body: bytes) -> bytes:
        if self._draining.is_set():
            resp = invocation_pb2.InvokeResponse(payload=b"")
            return resp.SerializeToString()  # type: ignore[no-any-return]
        req = invocation_pb2.InvokeRequest()
        req.ParseFromString(body)
        with self._active_call_tracker():
            result = self.invoke(req.function_id, req.payload, dict(req.metadata))
        resp = invocation_pb2.InvokeResponse(payload=result)
        return resp.SerializeToString()  # type: ignore[no-any-return]

    def _handle_inbound_start_job(self, body: bytes) -> bytes:
        if self._draining.is_set():
            resp = invocation_pb2.StartJobResponse(job_id="")
            return resp.SerializeToString()  # type: ignore[no-any-return]
        req = invocation_pb2.InvokeRequest()
        req.ParseFromString(body)
        job_id = self.start_job(req.function_id, req.payload, dict(req.metadata))
        resp = invocation_pb2.StartJobResponse(job_id=job_id)
        return resp.SerializeToString()  # type: ignore[no-any-return]

    def _handle_inbound_cancel_job(self, body: bytes) -> bytes:
        req = invocation_pb2.CancelJobRequest()
        req.ParseFromString(body)
        self.cancel_job(req.job_id)
        resp = invocation_pb2.InvokeResponse()
        return resp.SerializeToString()  # type: ignore[no-any-return]

    def _handle_inbound_stream_job(self, body: bytes) -> bytes:
        req = invocation_pb2.JobStreamRequest()
        req.ParseFromString(body)
        with self._job_lock:
            state = self._jobs.get(req.job_id)

        event = invocation_pb2.JobEvent()
        if state is None:
            event.type = "error"
            event.message = f"job not found: {req.job_id}"
        elif state.cancelled.is_set():
            event.type = "error"
            event.message = "job was cancelled"
        elif state.done.is_set():
            # drain remaining events
            events = list(self.stream_job(req.job_id))
            if events:
                last = events[-1]
                event.type = last.type
                event.message = last.message
                event.progress = last.progress
                event.payload = last.payload
            else:
                event.type = "error"
                event.message = "job completed with no events"
        else:
            event.type = "progress"
            event.message = "job is running"

        return event.SerializeToString()

    # ---- drain internals ----

    def is_draining(self) -> bool:
        """Return True when the Agent has requested a drain."""
        return self._draining.is_set()

    def _active_call_tracker(self):
        """Context manager that tracks in-flight calls for drain coordination."""

        class _Tracker:
            def __init__(self, client: "CroupierClient") -> None:
                self._client = client

            def __enter__(self) -> "_Tracker":
                self._client._active_calls._counter += 1  # type: ignore[attr-defined]
                self._client._active_calls.release()
                return self

            def __exit__(self, *_args) -> None:
                try:
                    self._client._active_calls.acquire(timeout=0)
                except Exception:
                    pass
                self._client._active_calls._counter -= 1  # type: ignore[attr-defined]

        return _Tracker(self)

    def _handle_drain_request(self, body: bytes) -> bytes:
        """Handle ProviderDrainRequest from Agent.

        Sets draining state, replies immediately, then waits for in-flight
        calls to finish, sends DrainComplete, and triggers reconnect.
        """
        if self._draining.is_set():
            # Already draining — return empty response.
            return b""

        self._draining.set()
        LOG.info("Drain requested by Agent, waiting for in-flight calls to finish")

        # Reply with empty ProviderDrainResponse immediately.
        # Start a background thread to wait for completion and send DrainComplete.
        threading.Thread(target=self._drain_and_reconnect, daemon=True).start()

        return b""

    def _drain_and_reconnect(self) -> None:
        """Wait for active calls to complete, send DrainComplete, then reconnect."""
        # Wait for in-flight calls to drain (up to 30 seconds).
        deadline = __import__("time").monotonic() + 30
        while self._active_calls._counter > 0:  # type: ignore[attr-defined]
            if __import__("time").monotonic() > deadline:
                LOG.warning(
                    "Drain timeout, %d calls still in-flight", self._active_calls._counter
                )  # type: ignore[attr-defined]
                break
            __import__("time").sleep(0.1)

        # Send DrainComplete notification to Agent.
        try:
            self._send_drain_complete()
        except Exception as exc:  # pylint: disable=broad-except
            LOG.warning("Failed to send DrainComplete: %s", exc)

        # Clear draining state and reconnect.
        self._draining.clear()

        if self._config.auto_reconnect:
            LOG.info("Triggering reconnect after drain")
            try:
                self._recover_connection()
            except Exception as exc:  # pylint: disable=broad-except
                LOG.warning("Reconnect after drain failed: %s", exc)

    def _send_drain_complete(self) -> None:
        """Send ProviderDrainCompleteRequest to Agent."""
        with self._state_lock:
            if not self._transport or not self._session_id:
                return
            transport = self._transport

        # ProviderDrainCompleteRequest body is empty (no proto definition needed).
        transport.call(protocol.MSG_PROVIDER_DRAIN_COMPLETE_REQUEST, b"")
        LOG.info("DrainComplete sent to Agent")

    def _start_heartbeat_loop(self) -> None:
        if self._heartbeat_thread and self._heartbeat_thread.is_alive():
            return

        self._heartbeat_thread = threading.Thread(
            target=self._heartbeat_loop,
            name="croupier-heartbeat",
            daemon=True,
        )
        self._heartbeat_thread.start()

    def _heartbeat_loop(self) -> None:
        interval = max(self._config.heartbeat_interval, 1)
        while not self._heartbeat_stop.wait(interval):
            try:
                self._send_heartbeat()
            except Exception as exc:  # pylint: disable=broad-except
                LOG.warning("Heartbeat failed, attempting reconnect: %s", exc)
                if not self._config.auto_reconnect:
                    continue
                self._recover_connection()

    def _send_heartbeat(self) -> None:
        with self._state_lock:
            if not self._transport or not self._session_id:
                raise RuntimeError("Client is not registered")
            transport = self._transport
            request = provider_pb2.HeartbeatRequest(
                service_id=self._config.service_id,
                session_id=self._session_id,
            )
            req_data = request.SerializeToString()

        transport.call(protocol.MSG_HEARTBEAT_LOCAL_REQUEST, req_data)

    def _recover_connection(self) -> None:
        while not self._heartbeat_stop.is_set():
            try:
                with self._state_lock:
                    self._connect_and_register()
                    self._connected = True
                LOG.info("Reconnected and re-registered service %s", self._config.service_id)
                return
            except Exception as exc:  # pylint: disable=broad-except
                LOG.warning("Reconnect attempt failed: %s", exc)
                if self._heartbeat_stop.wait(max(self._config.reconnect_interval, 0.1)):
                    return

    @staticmethod
    def _normalize_agent_addr(address: str) -> str:
        # Strip protocol prefix if present (e.g. "tcp://")
        if "://" in address:
            return address.split("://", 1)[1]
        return address


__all__ = [
    "__version__",
    "ClientConfig",
    "FunctionDescriptor",
    "FunctionHandler",
    "CroupierClient",
    # Proto message types
    "provider_pb2",
    "invocation_pb2",
    "invoker_pb2",
    # Invoker related exports
    "InvokerConfig",
    "InvokeOptions",
    "JobEventInfo",
    "ReconnectConfig",
    "RetryConfig",
    "Invoker",
    "SyncInvoker",
    "create_invoker",
    "create_sync_invoker",
]

# Import Invoker classes when available
try:
    from .invoker import (
        Invoker,
        InvokerConfig,
        InvokeOptions,
        JobEventInfo,
        ReconnectConfig,
        RetryConfig,
        SyncInvoker,
        create_invoker,
        create_sync_invoker,
    )
except ImportError:
    pass
