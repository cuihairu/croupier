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
    tags: list[str] = field(default_factory=list)
    summary: Optional[str] = None
    description: Optional[str] = None
    operation_id: Optional[str] = None
    deprecated: bool = False
    input_schema: Optional[Dict[str, object] | str] = None
    output_schema: Optional[Dict[str, object] | str] = None
    resource: Optional[str] = None
    operation: Optional[str] = None
    capability: Optional[str] = None
    execution: Optional[str] = None
    approval_required: bool = False
    approval_policy_key: Optional[str] = None
    risk: Optional[str] = None
    enabled: bool = True
    permission: Optional[str] = None


def _normalize_hint_key(hint: str) -> Optional[str]:
    """校验并归一 hint 键：x_/X- 变体统一为 x- 形式。"""
    trimmed = (hint or "").strip()
    if not trimmed:
        return None
    lower = trimmed.lower()
    if lower.startswith("x_"):
        return "x-" + trimmed[2:]
    if lower.startswith("x-"):
        return trimmed
    return None


def set_field_hint(desc: FunctionDescriptor, field: str, hint: str, value: object) -> FunctionDescriptor:
    """F14：向 input_schema 的 properties[field] 合并单个呈现 hint（x-ui 契约）。"""
    if not (field or "").strip():
        raise ValueError("field key is required for set_field_hint")
    normalized = _normalize_hint_key(hint)
    if normalized is None:
        raise ValueError(f'hint "{hint}" must be an x- extension key (e.g. x-widget)')
    schema = desc.input_schema
    if isinstance(schema, str):
        schema = json.loads(schema) if schema.strip() else None
    schema = dict(schema) if isinstance(schema, dict) else {"type": "object"}
    schema.setdefault("type", "object")
    properties = dict(schema.get("properties") or {})
    property_entry = dict(properties.get(field) or {})
    property_entry[normalized] = value
    properties[field] = property_entry
    schema["properties"] = properties
    desc.input_schema = schema
    return desc


def set_field_widget(desc: FunctionDescriptor, field: str, widget: str) -> FunctionDescriptor:
    """等价于 set_field_hint(desc, field, "x-widget", widget)。"""
    if not (widget or "").strip():
        raise ValueError("widget is required for set_field_widget")
    return set_field_hint(desc, field, "x-widget", widget)


@dataclass
class ClientConfig:
    """Runtime configuration for the Python SDK client."""

    agent_addr: str = "127.0.0.1:19091"
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
    # F15：provider 侧入站校验（按函数声明的 input schema），默认关闭
    validate_input_payloads: bool = False
    auto_reconnect: bool = True
    reconnect_interval: float = 1.0
    reconnect_max_attempts: int = 0
    disable_logging: bool = False
    debug_logging: bool = False
    log_level: str = "INFO"
    enable_file_transfer: bool = False
    max_file_size: int = 10 * 1024 * 1024
    # F：下发文件仅落盘至此暂存目录（不自动应用）
    file_staging_dir: str = "./croupier-staging"
    # Optional RetryConfig (defined in croupier.invoker); aligns the provider
    # client config surface with the Go/Java SDKs. Typed as Any to avoid a
    # circular import at module definition time.
    retry: Any = None

    # TLS knobs (forward-compatible)
    tls_enabled: bool = False
    tls_insecure_skip_verify: bool = False


class _TaskState:
    def __init__(self) -> None:
        self.queue: "queue.Queue[Optional[invocation_pb2.TaskEvent]]" = queue.Queue()  # type: ignore[name-defined]
        self.done = threading.Event()
        self.cancelled = threading.Event()

    def push(self, event: invocation_pb2.TaskEvent, finished: bool = False) -> None:  # type: ignore[name-defined]
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
        self._tasks: Dict[str, _TaskState] = {}
        self._task_lock = threading.Lock()
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

        # F：控制面 manifest 上传（审查发现 #2）：fire-and-forget——
        # 控制面慢/不可达不得拖慢注册主路径（方法内部已 fail-open）
        threading.Thread(
            target=self._maybe_register_capabilities, daemon=True
        ).start()

    def _maybe_register_capabilities(self) -> None:
        """向控制面（control_addr）上传能力清单；失败仅告警不影响连接。"""
        control_addr = getattr(self._config, "control_addr", None)
        if not control_addr:
            return
        try:
            register_pb2 = _load_proto_module("croupier.agent.v1.register_pb2")
            request = register_pb2.RegisterCapabilitiesRequest(
                provider=register_pb2.ProviderMeta(
                    id=self._config.service_id,
                    version=self._config.service_version,
                    lang=self._config.provider_lang,
                    sdk=self._config.provider_sdk,
                ),
                manifest_json_gz=gzip.compress(self.build_manifest()),
            )
            transport = TCPTransport(
                address=control_addr,
                timeout_ms=max(self._config.timeout_seconds, 5) * 1000,
                tls_enabled=not self._config.insecure,
                tls_cert_file=self._config.cert_file or "",
                tls_key_file=self._config.key_file or "",
                tls_ca_file=self._config.ca_file or "",
                tls_server_name=self._config.server_name or "",
            )
            transport.connect()
            try:
                transport.call(protocol.MSG_REGISTER_CAPABILITIES_REQ, request.SerializeToString())
            finally:
                transport.close()
            LOG.info("Capabilities registered to control plane: %s", control_addr)
        except Exception as error:  # noqa: BLE001 — 上传失败不影响注册
            LOG.warning("Failed to register capabilities: %s", error)

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

    def get_function_descriptor(self, function_id: str) -> Optional[provider_pb2.ProviderFunctionDescriptor]:  # type: ignore[name-defined]
        """Get a protobuf function descriptor for the given function ID."""
        desc = self._descriptors.get(function_id)
        if desc is None:
            return None
        input_schema = (
            json.dumps(desc.input_schema)
            if isinstance(desc.input_schema, dict)
            else (desc.input_schema or "")
        )
        output_schema = (
            json.dumps(desc.output_schema)
            if isinstance(desc.output_schema, dict)
            else (desc.output_schema or "")
        )
        return provider_pb2.ProviderFunctionDescriptor(
            id=desc.id,
            version=desc.version,
            tags=desc.tags,
            summary=desc.summary or "",
            description=desc.description or "",
            operation_id=desc.operation_id or desc.id,
            deprecated=desc.deprecated,
            input_schema=input_schema,
            output_schema=output_schema,
            resource=desc.resource or "",
            operation=desc.operation or "",
            capability=desc.capability or "",
            execution=desc.execution or "",
            approval_required=desc.approval_required,
            approval_policy_key=desc.approval_policy_key or "",
            risk=desc.risk or "",
            enabled=desc.enabled,
            permission=desc.permission or "",
        )

    def get_provider_connect_request(self) -> provider_pb2.ProviderConnectRequest:  # type: ignore[name-defined]
        """Build a provider connect request for the agent."""
        return provider_pb2.ProviderConnectRequest(
            service_id=self._config.service_id,
            version=self._config.service_version,
            functions=[self.get_function_descriptor(fid) for fid in self._handlers.keys()],  # type: ignore[misc]
            sdk_language="python",
            sdk_version="1.0.0",
            sdk_name="croupier-python-sdk",
            protocol_version="1.0.0",
        )

    def _validate_inbound_payload(self, function_id: str, payload: bytes) -> None:
        """F15：按函数声明的 input schema 校验入站 payload。

        开关关闭 / 未注册 / schema 缺失时跳过（服务端仍是权威校验方）；
        校验失败抛 ValueError("payload validation failed: ...")。
        """
        if not getattr(self._config, "validate_input_payloads", False):
            return
        descriptor = self._descriptors.get(function_id)
        if descriptor is None:
            return
        schema = descriptor.input_schema
        if isinstance(schema, (bytes, bytearray)):
            schema = bytes(schema).decode("utf-8", "replace")
        if isinstance(schema, str):
            schema = json.loads(schema) if schema.strip() else None
        if not isinstance(schema, dict):
            return
        try:
            value = json.loads(payload.decode("utf-8")) if payload else {}
        except (json.JSONDecodeError, UnicodeDecodeError) as error:
            raise ValueError(f"payload must be valid JSON: {error}") from error
        import jsonschema

        try:
            jsonschema.validate(instance=value, schema=schema)
        except jsonschema.ValidationError as error:
            raise ValueError(f"payload validation failed: {error.message}") from error

    def invoke(
        self, function_id: str, payload: bytes, metadata: Optional[Dict[str, str]] = None
    ) -> bytes:
        """Invoke a registered function handler."""
        handler = self._handlers.get(function_id)
        if handler is None:
            raise ValueError(f"Function {function_id} not found")

        self._validate_inbound_payload(function_id, payload)

        metadata_json = json.dumps(metadata or {})
        result: bytes
        handler_result = handler(metadata_json, payload)
        if isinstance(handler_result, (bytes, bytearray)):
            result = bytes(handler_result)
        else:
            result = str(handler_result).encode("utf-8")
        return result

    def start_task(
        self, function_id: str, payload: bytes, metadata: Optional[Dict[str, str]] = None
    ) -> str:
        """Start an asynchronous task."""
        handler = self._handlers.get(function_id)
        if handler is None:
            raise ValueError(f"Function {function_id} not found")

        self._validate_inbound_payload(function_id, payload)

        task_id = f"{function_id}-{uuid.uuid4().hex}"
        state = _TaskState()
        with self._task_lock:
            self._tasks[task_id] = state

        state.push(
            invocation_pb2.TaskEvent(type="started", message="task started", progress=0, payload=b"")  # type: ignore[name-defined]
        )

        metadata_json = json.dumps(metadata or {})

        def _run_task() -> None:
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
                    invocation_pb2.TaskEvent(  # type: ignore[name-defined]
                        type="completed",
                        message="task completed",
                        progress=100,
                        payload=result,
                    ),
                    finished=True,
                )
            except Exception as exc:  # pylint: disable=broad-except
                if state.cancelled.is_set():
                    return
                LOG.exception("Task %s failed", task_id)
                state.push(
                    invocation_pb2.TaskEvent(  # type: ignore[name-defined]
                        type="error",
                        message=str(exc),
                        progress=0,
                        payload=b"",
                    ),
                    finished=True,
                )

        threading.Thread(target=_run_task, daemon=True).start()
        return task_id

    def stream_task(self, task_id: str):  # type: ignore[misc]
        """Stream task events."""
        with self._task_lock:
            state = self._tasks.get(task_id)
        if state is None:
            raise ValueError(f"Task {task_id} not found")

        while True:
            event = state.queue.get()
            if event is None:
                break
            yield event  # type: ignore[misc]
        with self._task_lock:
            self._tasks.pop(task_id, None)

    def cancel_task(self, task_id: str) -> bool:
        """Cancel a running task."""
        with self._task_lock:
            state = self._tasks.get(task_id)
        if state and not state.done.is_set():
            state.cancelled.set()
            state.push(
                invocation_pb2.TaskEvent(  # type: ignore[name-defined]
                    type="cancelled",
                    message="task cancelled",
                    progress=0,
                    payload=b"",
                ),
                finished=True,
            )
            return True
        return False

    def _handle_start_task(self, request, _context):  # type: ignore[no-untyped-def]
        """Compatibility shim for older direct handler tests/callers."""
        task_id = self.start_task(
            request.function_id,
            request.payload,
            dict(request.metadata),
        )
        return invocation_pb2.StartTaskResponse(task_id=task_id)  # type: ignore[name-defined]

    def _handle_stream_task(self, request, _context):  # type: ignore[no-untyped-def]
        """Compatibility shim for older direct handler tests/callers."""
        return self.stream_task(request.task_id)

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
            if descriptor.tags:
                entry["tags"] = descriptor.tags
            if descriptor.summary:
                entry["summary"] = descriptor.summary
            if descriptor.description:
                entry["description"] = descriptor.description
            if descriptor.operation_id:
                entry["operationId"] = descriptor.operation_id
            if descriptor.deprecated:
                entry["deprecated"] = True  # type: ignore[assignment]
            if descriptor.input_schema:
                entry["inputSchema"] = descriptor.input_schema
            if descriptor.output_schema:
                entry["outputSchema"] = descriptor.output_schema
            if descriptor.resource:
                entry["resource"] = descriptor.resource
            if descriptor.risk:
                entry["risk"] = descriptor.risk
            if descriptor.operation:
                entry["operation"] = descriptor.operation
            if descriptor.capability:
                entry["capability"] = descriptor.capability
            if descriptor.execution:
                entry["execution"] = descriptor.execution
            if descriptor.approval_required:
                entry["approvalRequired"] = True  # type: ignore[assignment]
            if descriptor.approval_policy_key:
                entry["approvalPolicyKey"] = descriptor.approval_policy_key
            if descriptor.permission:
                entry["permission"] = descriptor.permission
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

        request = self.get_provider_connect_request()
        _, response_data = transport.call(
            protocol.MSG_PROVIDER_CONNECT_REQUEST,
            request.SerializeToString(),
        )

        response = provider_pb2.ProviderConnectResponse()
        response.ParseFromString(response_data)
        if not response.session_id:
            transport.close()
            raise RuntimeError("Provider connect returned empty session_id")

        self._transport = transport
        self._session_id = response.session_id

    def _handle_inbound(self, msg_type: int, _req_id: int, body: bytes) -> bytes:
        """Handle inbound requests from the Agent (invoke, task, cancel, drain, stream)."""
        if msg_type == protocol.MSG_PROVIDER_DRAIN_REQUEST:
            return self._handle_drain_request(body)
        if msg_type == protocol.MSG_INVOKE_REQUEST:
            return self._handle_inbound_invoke(body)
        if msg_type == protocol.MSG_START_TASK_REQUEST:
            return self._handle_inbound_start_task(body)
        if msg_type == protocol.MSG_CANCEL_TASK_REQUEST:
            return self._handle_inbound_cancel_task(body)
        if msg_type == protocol.MSG_STREAM_TASK_REQUEST:
            return self._handle_inbound_stream_task(body)
        if msg_type == protocol.MSG_PROVIDER_FILE_PUSH_REQ:
            return self._handle_inbound_file_push(body)
        LOG.warning("Unsupported inbound MsgID: %s", protocol.msg_id_string(msg_type))
        return b""

    def _handle_inbound_file_push(self, body: bytes) -> bytes:
        """F：文件下发接收（hotpatch P1 传输层）。

        wire 与 Go/JS 一致（protobuf 兼容手写编解码）：
          FilePushRequest  { 1: transfer_id, 2: file_name, 3: content_sha256, 4: data }
          FilePushResponse { 1: transfer_id, 2: ok, 3: stored_path, 4: error }
        安全链全部强制：总开关 → 大小上限 → 仅 basename（拒穿越）→
        sha256 → 原子落盘暂存目录。**不自动应用**——应用由后续
        hotpatch runner 单独编排。任何失败回 error 响应。
        """

        def _fail(message: str) -> bytes:
            return self._encode_file_push_response("", False, "", message)

        if not self._config.enable_file_transfer:
            return _fail("file transfer is disabled on this provider")

        transfer_id, file_name, content_sha256, data = self._decode_file_push_request(body)

        if not transfer_id:
            return _fail("transferId is required")
        if not file_name or "/" in file_name or "\\" in file_name or ".." in file_name:
            return _fail(f'file name must be a bare basename: "{file_name}"')
        max_size = self._config.max_file_size or 10 * 1024 * 1024
        if not data:
            return _fail("file payload is empty")
        if len(data) > max_size:
            return _fail(f"file size {len(data)} exceeds max {max_size}")
        if not content_sha256:
            return _fail("contentSha256 is required")
        import hashlib

        actual = hashlib.sha256(data).hexdigest()
        if actual.lower() != content_sha256.lower():
            return _fail("checksum mismatch")

        staging_dir = self._config.file_staging_dir or "./croupier-staging"
        import os

        os.makedirs(staging_dir, exist_ok=True)
        target = os.path.join(staging_dir, os.path.basename(file_name))
        if not os.path.abspath(target).startswith(os.path.abspath(staging_dir)):
            return _fail('file name must be a bare basename: "' + file_name + '"')
        tmp_path = target + f".push-{transfer_id}"
        with open(tmp_path, "wb") as handle:
            handle.write(data)
        os.replace(tmp_path, target)
        return self._encode_file_push_response(transfer_id, True, target, "")

    @staticmethod
    def _decode_file_push_request(body: bytes):
        """手写 protobuf wire 解码：FilePushRequest 四字段（长度限定）。"""
        idx = 0
        fields: dict = {}
        while idx < len(body):
            tag = body[idx]
            idx += 1
            field_number = tag >> 3
            wire_type = tag & 0x7
            if wire_type != 2:
                raise ValueError(f"unsupported wire type {wire_type}")
            length = 0
            shift = 0
            while True:
                byte = body[idx]
                idx += 1
                length |= (byte & 0x7F) << shift
                if not byte & 0x80:
                    break
                shift += 7
            value = body[idx : idx + length]
            idx += length
            fields.setdefault(field_number, value)
        transfer_id = fields.get(1, b"").decode("utf-8")
        file_name = fields.get(2, b"").decode("utf-8")
        content_sha256 = fields.get(3, b"").decode("utf-8")
        return transfer_id, file_name, content_sha256, fields.get(4, b"")

    @staticmethod
    def _encode_file_push_response(transfer_id: str, ok: bool, stored_path: str, error: str) -> bytes:
        """手写 protobuf wire 编码 FilePushResponse。"""

        def _field(field_number: int, value: bytes) -> bytes:
            out = bytes([(field_number << 3) | 2])
            length = len(value)
            while length >= 0x80:
                out += bytes([(length & 0x7F) | 0x80])
                length >>= 7
            out += bytes([length])
            return out + value

        out = b""
        if transfer_id:
            out += _field(1, transfer_id.encode("utf-8"))
        if ok:
            out += bytes([0x10, 0x01])
        if stored_path:
            out += _field(3, stored_path.encode("utf-8"))
        if error:
            out += _field(4, error.encode("utf-8"))
        return out

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

    def _handle_inbound_start_task(self, body: bytes) -> bytes:
        if self._draining.is_set():
            resp = invocation_pb2.StartTaskResponse(task_id="")
            return resp.SerializeToString()  # type: ignore[no-any-return]
        req = invocation_pb2.InvokeRequest()
        req.ParseFromString(body)
        task_id = self.start_task(req.function_id, req.payload, dict(req.metadata))
        resp = invocation_pb2.StartTaskResponse(task_id=task_id)
        return resp.SerializeToString()  # type: ignore[no-any-return]

    def _handle_inbound_cancel_task(self, body: bytes) -> bytes:
        req = invocation_pb2.CancelTaskRequest()
        req.ParseFromString(body)
        self.cancel_task(req.task_id)
        resp = invocation_pb2.InvokeResponse()
        return resp.SerializeToString()  # type: ignore[no-any-return]

    def _handle_inbound_stream_task(self, body: bytes) -> bytes:
        req = invocation_pb2.TaskStreamRequest()
        req.ParseFromString(body)
        with self._task_lock:
            state = self._tasks.get(req.task_id)

        event = invocation_pb2.TaskEvent()
        if state is None:
            event.type = "error"
            event.message = f"task not found: {req.task_id}"
        elif state.cancelled.is_set():
            event.type = "error"
            event.message = "task was cancelled"
        elif state.done.is_set():
            # drain remaining events
            events = list(self.stream_task(req.task_id))
            if events:
                last = events[-1]
                event.type = last.type
                event.message = last.message
                event.progress = last.progress
                event.payload = last.payload
            else:
                event.type = "error"
                event.message = "task completed with no events"
        else:
            event.type = "progress"
            event.message = "task is running"

        return event.SerializeToString()  # type: ignore[no-any-return]

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
                # fmt: off
                LOG.warning(  # type: ignore[attr-defined]
                    "Drain timeout, %d calls still in-flight", self._active_calls._counter  # type: ignore[attr-defined]
                )
                # fmt: on
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

        # Network call outside the state lock.
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
        # Snapshot state under the lock, then perform the network call
        # WITHOUT holding _state_lock: a blocked socket write must never
        # stall every other state user (observed as a permanent futex wait
        # on the heartbeat thread after the agent restarts).
        with self._state_lock:
            if not self._transport or not self._session_id:
                raise RuntimeError("Client is not registered")
            transport = self._transport
            session_id = self._session_id
            service_id = self._config.service_id

        request = provider_pb2.ProviderHeartbeatRequest(
            service_id=service_id,
            session_id=session_id,
        )
        req_data = request.SerializeToString()

        transport.call(protocol.MSG_PROVIDER_HEARTBEAT_REQUEST, req_data)

    def _recover_connection(self) -> None:
        while not self._heartbeat_stop.is_set():
            try:
                # Do NOT hold _state_lock across the (blocking) dial/register
                # network calls; only take it to publish the new state.
                self._connect_and_register()
                with self._state_lock:
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
    "TaskEventInfo",
    "TaskStatus",
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
        TaskEventInfo,
        TaskStatus,
        ReconnectConfig,
        RetryConfig,
        SyncInvoker,
        create_invoker,
        create_sync_invoker,
    )
except ImportError:
    pass

# OpenAPI import helpers (mirrors the Go SDK's RegisterFromOpenAPI).
from .openapi import ImportOptions, RegisterFromOpenAPI, register_from_openapi  # noqa: E402
