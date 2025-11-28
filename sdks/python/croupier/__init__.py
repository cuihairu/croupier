"""
Croupier Python SDK

Provides a local gRPC server for function handlers and registers them with the
nearest agent so the platform can invoke them.
"""

from __future__ import annotations

import gzip
import importlib.util
import json
import logging
import queue
import sys
import threading
import time
import uuid
from concurrent import futures
from dataclasses import dataclass, field
from io import BytesIO
from pathlib import Path
from types import ModuleType
from typing import Callable, Dict, Optional

import grpc

__version__ = "1.0.0"
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


local_pb2 = _load_proto_module("croupier.agent.local.v1.local_pb2")
local_pb2_grpc = _load_proto_module("croupier.agent.local.v1.local_pb2_grpc")
function_pb2 = _load_proto_module("croupier.function.v1.function_pb2")
function_pb2_grpc = _load_proto_module("croupier.function.v1.function_pb2_grpc")
control_pb2 = _load_proto_module("croupier.control.v1.control_pb2")
control_pb2_grpc = _load_proto_module("croupier.control.v1.control_pb2_grpc")

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
    local_listen: str = "0.0.0.0:0"
    heartbeat_interval: int = 60
    timeout_seconds: int = 30
    cert_file: Optional[str] = None
    key_file: Optional[str] = None
    ca_file: Optional[str] = None
    control_addr: Optional[str] = None
    provider_lang: str = "python"
    provider_sdk: str = "croupier-python-sdk"


class _JobState:
    def __init__(self) -> None:
        self.queue: "queue.Queue[Optional[function_pb2.JobEvent]]" = queue.Queue()
        self.done = threading.Event()

    def push(self, event: function_pb2.JobEvent, finished: bool = False) -> None:
        self.queue.put(event)
        if finished:
            self.queue.put(None)
            self.done.set()


class _FunctionService(function_pb2_grpc.FunctionServiceServicer):
    def __init__(self, client: "CroupierClient") -> None:
        self._client = client

    def Invoke(self, request, context):  # type: ignore[override]
        return self._client._handle_invoke(request, context)

    def StartJob(self, request, context):  # type: ignore[override]
        return self._client._handle_start_job(request, context)

    def StreamJob(self, request, context):  # type: ignore[override]
        return self._client._handle_stream_job(request, context)

    def CancelJob(self, request, context):  # type: ignore[override]
        return self._client._handle_cancel_job(request, context)


class CroupierClient:
    """Registers local handlers and keeps the agent connection alive."""

    def __init__(self, config: Optional[ClientConfig] = None) -> None:
        self._config = config or ClientConfig()
        self._handlers: Dict[str, FunctionHandler] = {}
        self._descriptors: Dict[str, FunctionDescriptor] = {}
        self._jobs: Dict[str, _JobState] = {}
        self._job_lock = threading.Lock()

        self._server: Optional[grpc.Server] = None
        self._channel: Optional[grpc.Channel] = None
        self._agent_stub: Optional[local_pb2_grpc.LocalControlServiceStub] = None
        self._local_address = ""
        self._session_id = ""

        self._heartbeat_stop = threading.Event()
        self._heartbeat_thread: Optional[threading.Thread] = None

    def register_function(self, descriptor: FunctionDescriptor, handler: FunctionHandler) -> None:
        if not descriptor.id or not descriptor.version:
            raise ValueError("Function descriptor must include id and version.")
        if self._server is not None:
            raise RuntimeError("Cannot register new functions after the server has started.")
        self._descriptors[descriptor.id] = descriptor
        self._handlers[descriptor.id] = handler

    def connect(self) -> None:
        if self._server is not None:
            return
        if not self._handlers:
            raise RuntimeError("Register at least one function before connecting.")

        self._start_local_server()
        self._connect_agent()
        self._register_with_agent()
        self._start_heartbeat()

    def disconnect(self) -> None:
        self._heartbeat_stop.set()
        if self._heartbeat_thread:
            self._heartbeat_thread.join(timeout=2)
        self._heartbeat_thread = None
        self._session_id = ""

        if self._server:
            self._server.stop(0)
            self._server = None
        if self._channel:
            self._channel.close()
            self._channel = None

    def _start_local_server(self) -> None:
        host, port = self._config.local_listen.split(":")
        port_value = int(port) if port else 0

        self._server = grpc.server(futures.ThreadPoolExecutor(max_workers=32))
        function_pb2_grpc.add_FunctionServiceServicer_to_server(_FunctionService(self), self._server)

        bound_port = self._server.add_insecure_port(f"{host}:{port_value}")
        if bound_port == 0:
            raise RuntimeError(f"Unable to bind local server on {self._config.local_listen}")
        self._server.start()

        advertised_host = host if host not in ("0.0.0.0", "::") else "127.0.0.1"
        self._local_address = f"{advertised_host}:{bound_port}"
        LOG.info("Local function server listening at %s", self._local_address)

    def _connect_agent(self) -> None:
        if self._config.insecure:
            self._channel = grpc.insecure_channel(self._config.agent_addr)
        else:
            creds = grpc.ssl_channel_credentials()
            self._channel = grpc.secure_channel(self._config.agent_addr, creds)
        self._agent_stub = local_pb2_grpc.LocalControlServiceStub(self._channel)

    def _register_with_agent(self) -> None:
        if not self._agent_stub:
            raise RuntimeError("Agent stub not initialized")

        request = local_pb2.RegisterLocalRequest(
            service_id=self._config.service_id,
            version=self._config.service_version,
            rpc_addr=self._local_address,
            functions=[
                local_pb2.LocalFunctionDescriptor(id=desc.id, version=desc.version)
                for desc in self._descriptors.values()
            ],
        )
        response = self._agent_stub.RegisterLocal(
            request, timeout=self._config.timeout_seconds
        )
        self._session_id = response.session_id
        LOG.info("Registered %d functions with agent", len(request.functions))
        self._register_capabilities()

    def _register_capabilities(self) -> None:
        if not self._config.control_addr:
            return

        channel: Optional[grpc.Channel] = None
        try:
            manifest = self._build_manifest()
            compressed = self._gzip_bytes(manifest)
            channel = self._create_control_channel()
            stub = control_pb2_grpc.ControlServiceStub(channel)
            request = control_pb2.RegisterCapabilitiesRequest(
                provider=control_pb2.ProviderMeta(
                    id=self._config.service_id,
                    version=self._config.service_version,
                    lang=self._config.provider_lang,
                    sdk=self._config.provider_sdk,
                ),
                manifest_json_gz=compressed,
            )
            stub.RegisterCapabilities(request, timeout=self._config.timeout_seconds)
            LOG.info(
                "Uploaded provider capabilities manifest with %d functions",
                len(self._descriptors),
            )
        except grpc.RpcError as exc:  # pragma: no cover - exercised in integration tests
            LOG.warning("ControlService.RegisterCapabilities failed: %s", exc, exc_info=True)
        except Exception:  # pragma: no cover
            LOG.exception("Failed to upload provider manifest")
        finally:
            if channel is not None:
                channel.close()

    def _start_heartbeat(self) -> None:
        self._heartbeat_stop.clear()

        def _loop() -> None:
            while not self._heartbeat_stop.is_set():
                if not self._agent_stub or not self._session_id:
                    break
                try:
                    self._agent_stub.Heartbeat(
                        local_pb2.HeartbeatRequest(
                            service_id=self._config.service_id,
                            session_id=self._session_id,
                        ),
                        timeout=self._config.timeout_seconds,
                    )
                except grpc.RpcError as exc:
                    LOG.warning("Heartbeat failed: %s", exc)
                time.sleep(max(1, self._config.heartbeat_interval))

        self._heartbeat_thread = threading.Thread(target=_loop, daemon=True)
        self._heartbeat_thread.start()

    # === Local function handling ==================================================

    def _handle_invoke(self, request, context):  # type: ignore[override]
        handler = self._handlers.get(request.function_id)
        if handler is None:
            context.abort(grpc.StatusCode.NOT_FOUND, f"Function {request.function_id} not found")

        metadata_json = json.dumps(dict(request.metadata))
        payload = bytes(request.payload or b"")
        try:
            result = handler(metadata_json, payload)
            if not isinstance(result, (bytes, bytearray)):
                result = str(result).encode("utf-8")
            return function_pb2.InvokeResponse(payload=result)
        except Exception as exc:  # pylint: disable=broad-except
            LOG.exception("Handler %s failed", request.function_id)
            context.abort(grpc.StatusCode.INTERNAL, str(exc))

    def _handle_start_job(self, request, context):  # type: ignore[override]
        handler = self._handlers.get(request.function_id)
        if handler is None:
            context.abort(grpc.StatusCode.NOT_FOUND, f"Function {request.function_id} not found")

        job_id = f"{request.function_id}-{uuid.uuid4().hex}"
        state = _JobState()
        with self._job_lock:
            self._jobs[job_id] = state

        state.push(
            function_pb2.JobEvent(type="started", message="job started", progress=0, payload=b"")
        )

        metadata_json = json.dumps(dict(request.metadata))
        payload = bytes(request.payload or b"")

        def _run_job() -> None:
            try:
                result = handler(metadata_json, payload)
                if not isinstance(result, (bytes, bytearray)):
                    result = str(result).encode("utf-8")
                state.push(
                    function_pb2.JobEvent(
                        type="completed",
                        message="job completed",
                        progress=100,
                        payload=result,
                    ),
                    finished=True,
                )
            except Exception as exc:  # pylint: disable=broad-except
                LOG.exception("Job %s failed", job_id)
                state.push(
                    function_pb2.JobEvent(
                        type="error",
                        message=str(exc),
                        progress=0,
                        payload=b"",
                    ),
                    finished=True,
                )
            finally:
                with self._job_lock:
                    self._jobs.pop(job_id, None)

        threading.Thread(target=_run_job, daemon=True).start()
        return function_pb2.StartJobResponse(job_id=job_id)

    def _handle_stream_job(self, request, context):  # type: ignore[override]
        with self._job_lock:
            state = self._jobs.get(request.job_id)
        if state is None:
            context.abort(grpc.StatusCode.NOT_FOUND, f"Job {request.job_id} not found")

        while True:
            event = state.queue.get()
            if event is None:
                break
            yield event  # type: ignore[misc]

    def _handle_cancel_job(self, request, _context):  # type: ignore[override]
        with self._job_lock:
            state = self._jobs.pop(request.job_id, None)
        if state:
            state.push(
                function_pb2.JobEvent(
                    type="cancelled",
                    message="job cancelled",
                    progress=0,
                    payload=b"",
                ),
                finished=True,
            )
        return function_pb2.StartJobResponse(job_id=request.job_id)

    def _create_control_channel(self) -> grpc.Channel:
        if self._config.insecure:
            return grpc.insecure_channel(self._config.control_addr)
        creds = grpc.ssl_channel_credentials()
        return grpc.secure_channel(self._config.control_addr, creds)

    def _build_manifest(self) -> bytes:
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
                entry["enabled"] = True
            functions.append(entry)

        manifest: Dict[str, object] = {"provider": provider}
        if functions:
            manifest["functions"] = functions
        return json.dumps(manifest, separators=(",", ":")).encode("utf-8")

    def _gzip_bytes(self, payload: bytes) -> bytes:
        buffer = BytesIO()
        with gzip.GzipFile(fileobj=buffer, mode="wb") as handle:
            handle.write(payload)
        return buffer.getvalue()


__all__ = [
    "ClientConfig",
    "FunctionDescriptor",
    "FunctionHandler",
    "CroupierClient",
]
