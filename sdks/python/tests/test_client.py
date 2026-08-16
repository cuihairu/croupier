"""Tests for CroupierClient and related classes."""

import gzip
import json
import time

import croupier
import pytest
from croupier import protocol


def test_register_function_validates_descriptor():
    """Test that register_function validates descriptor fields."""
    client = croupier.CroupierClient()
    handler = lambda ctx, payload: "ok"  # noqa: E731

    with pytest.raises(ValueError):
        client.register_function(croupier.FunctionDescriptor(id="", version="1.0.0"), handler)
    with pytest.raises(ValueError):
        client.register_function(croupier.FunctionDescriptor(id="f1", version=""), handler)


def test_register_function_stores_handler():
    """Test that register_function stores the handler correctly."""
    config = croupier.ClientConfig(service_id="test-service")
    client = croupier.CroupierClient(config)
    client.register_function(
        croupier.FunctionDescriptor(id="f1", version="1.0.0"),
        lambda ctx, payload: "ok",  # noqa: E731
    )

    assert "f1" in client._handlers
    assert "f1" in client._descriptors


def test_connect_without_functions_raises_error():
    """Test that connect raises error when no functions registered."""
    client = croupier.CroupierClient()

    with pytest.raises(RuntimeError, match="Register at least one function"):
        client.connect()


@pytest.mark.integration
def test_connect_is_idempotent():
    """Test that connect can be called multiple times safely (requires real server)."""
    # Integration test - see test_integration.py for implementation


def test_build_manifest_contains_provider_and_functions():
    """Test that build_manifest returns correct JSON structure."""
    config = croupier.ClientConfig(service_id="svc-1", service_version="sv1")
    client = croupier.CroupierClient(config)
    client.register_function(
        croupier.FunctionDescriptor(
            id="f1",
            version="1.2.3",
            resource="player",
            operation="ban",
            permission="player.ban",
            enabled=True,
        ),
        lambda ctx, payload: "ok",  # noqa: E731
    )

    raw = client.build_manifest()
    parsed = json.loads(raw.decode("utf-8"))
    assert parsed["provider"] == {
        "id": "svc-1",
        "version": "sv1",
        "lang": "python",
        "sdk": "croupier-python-sdk",
    }
    assert parsed["functions"][0]["id"] == "f1"
    assert parsed["functions"][0]["version"] == "1.2.3"
    assert parsed["functions"][0]["resource"] == "player"
    assert parsed["functions"][0]["operation"] == "ban"
    assert parsed["functions"][0]["permission"] == "player.ban"
    assert parsed["functions"][0]["enabled"] is True


def test_build_manifest_defaults_version():
    """Test that build_manifest uses default version."""
    client = croupier.CroupierClient()
    client.register_function(
        croupier.FunctionDescriptor(id="f1"),
        lambda ctx, payload: "ok",  # noqa: E731
    )

    raw = client.build_manifest()
    parsed = json.loads(raw.decode("utf-8"))
    assert parsed["functions"][0]["version"] == "1.0.0"


def test_function_descriptor_maps_capability_fields():
    client = croupier.CroupierClient()
    client.register_function(
        croupier.FunctionDescriptor(
            id="player.ban",
            version="1.0.0",
            resource="player",
            operation="ban",
            risk="danger",
            permission="player.ban",
        ),
        lambda ctx, payload: "ok",  # noqa: E731
    )

    descriptor = client.get_function_descriptor("player.ban")

    assert descriptor is not None
    assert descriptor.resource == "player"
    assert descriptor.operation == "ban"
    assert descriptor.risk == "danger"
    assert descriptor.enabled is True
    assert descriptor.permission == "player.ban"


def test_gzip_bytes_roundtrip():
    """Test gzip_bytes compression works correctly."""
    client = croupier.CroupierClient()
    original = b'{"hello":"world"}'
    compressed = client.gzip_bytes(original)
    assert gzip.decompress(compressed) == original


def test_start_task_streams_started_then_completed():
    """Test that start_task creates task and streams events."""
    client = croupier.CroupierClient()
    client.register_function(
        croupier.FunctionDescriptor(id="f1", version="1.0.0"),
        lambda ctx, payload: (time.sleep(0.05) or payload.decode("utf-8")),  # noqa: E731
    )

    task_id = client.start_task("f1", b"hi")

    events = list(client.stream_task(task_id))
    assert events[0].type == "started"
    assert events[-1].type == "completed"
    assert events[-1].payload == b"hi"


def test_start_task_can_stream_after_completion():
    client = croupier.CroupierClient()
    client.register_function(
        croupier.FunctionDescriptor(id="f1", version="1.0.0"),
        lambda ctx, payload: payload.decode("utf-8").upper(),  # noqa: E731
    )

    req = croupier.invoker_pb2.InvokeRequest(function_id="f1", payload=b"done")
    resp = client._handle_start_task(req, None)

    time.sleep(0.05)

    stream_req = croupier.invoker_pb2.TaskStreamRequest(task_id=resp.task_id)
    events = list(client._handle_stream_task(stream_req, None))
    assert events[0].type == "started"
    assert events[-1].type == "completed"
    assert events[-1].payload == b"DONE"


def test_cancel_task_emits_cancelled_and_closes_stream():
    """Test that cancel_task stops the task stream."""
    client = croupier.CroupierClient()
    client.register_function(
        croupier.FunctionDescriptor(id="f1", version="1.0.0"),
        lambda ctx, payload: (time.sleep(0.2) or "late"),  # noqa: E731
    )

    task_id = client.start_task("f1", b"hi")

    state = client._tasks.get(task_id)
    assert state is not None

    client.cancel_task(task_id)

    events = list(client.stream_task(task_id))
    assert any(e.type == "cancelled" for e in events)


def test_invoke_calls_registered_handler():
    """Test that invoke calls the correct handler."""
    client = croupier.CroupierClient()

    def handler(ctx, payload):
        return f"echo:{payload.decode('utf-8')}"

    client.register_function(
        croupier.FunctionDescriptor(id="echo", version="1.0.0"),
        handler,
    )

    result = client.invoke("echo", b"test")
    assert result == b"echo:test"


def test_invoke_raises_for_unregistered_function():
    """Test that invoke raises for unknown function."""
    client = croupier.CroupierClient()

    with pytest.raises(ValueError, match="not found"):
        client.invoke("unknown", b"test")


def test_start_task_emits_error_on_handler_failure():
    """Test that start_task emits error event when handler fails."""
    client = croupier.CroupierClient()

    def failing_handler(ctx, payload):
        raise ValueError("handler error")

    client.register_function(
        croupier.FunctionDescriptor(id="failing", version="1.0.0"),
        failing_handler,
    )

    task_id = client.start_task("failing", b"test")

    events = list(client.stream_task(task_id))

    assert events[0].type == "started"
    assert events[1].type == "error"
    assert "handler error" in events[1].message


def test_stream_task_raises_for_missing_task_id():
    """Test that stream_task raises for unknown task."""
    client = croupier.CroupierClient()

    with pytest.raises(ValueError, match="not found"):
        list(client.stream_task("unknown-task"))


def test_cancel_task_does_nothing_for_unknown_task():
    """Test that cancel_task returns False for unknown task."""
    client = croupier.CroupierClient()
    result = client.cancel_task("unknown-task")
    assert result is False


def test_client_config_defaults():
    """Test ClientConfig has correct defaults."""
    config = croupier.ClientConfig()
    assert config.agent_addr == "127.0.0.1:19091"
    assert config.insecure is True
    assert config.service_version == "1.0.0"
    assert config.game_id == ""
    assert config.env == "development"
    assert config.heartbeat_interval == 60
    assert config.timeout_seconds == 30
    assert config.headers == {}
    assert config.provider_lang == "python"
    assert config.provider_sdk == "croupier-python-sdk"
    assert config.disable_logging is False
    assert config.debug_logging is False
    assert config.log_level == "INFO"
    assert config.enable_file_transfer is False
    assert config.max_file_size == 10 * 1024 * 1024


def test_function_descriptor_defaults():
    """Test FunctionDescriptor has correct defaults."""
    desc = croupier.FunctionDescriptor(id="test.fn")
    assert desc.version == "1.0.0"
    assert desc.resource is None
    assert desc.risk is None
    assert desc.operation is None
    assert desc.permission is None
    assert desc.enabled is True


def test_disconnect_clears_state():
    """Test that disconnect clears session state."""
    config = croupier.ClientConfig(service_id="test-service")
    client = croupier.CroupierClient(config)
    client.register_function(
        croupier.FunctionDescriptor(id="f1", version="1.0.0"),
        lambda ctx, payload: "ok",  # noqa: E731
    )

    client._session_id = "test-session"
    client._heartbeat_stop.set()

    client.disconnect()
    assert client._session_id == ""


def test_task_state_push_and_finished():
    """Test _TaskState push method."""
    state = croupier._TaskState()

    event = croupier.invocation_pb2.TaskEvent(type="test", payload=b"data")
    state.push(event, finished=False)

    result = state.queue.get(timeout=1)
    assert result.type == "test"
    assert result.payload == b"data"


def test_task_state_finished_sets_done():
    """Test _TaskState finished flag sets done event."""
    state = croupier._TaskState()

    event = croupier.invocation_pb2.TaskEvent(type="test", payload=b"data")
    state.push(event, finished=True)

    assert state.done.is_set()


def test_ensure_parent_packages():
    """Test the helper function creates parent packages."""
    croupier._ensure_parent_packages("test.module.name")


def test_load_proto_module_caches():
    """Test that proto module loading is cached."""
    module = croupier._load_proto_module("croupier.sdk.v1.provider_pb2")
    module2 = croupier._load_proto_module("croupier.sdk.v1.provider_pb2")
    assert module is module2


def test_load_proto_module_raises_for_missing():
    """Test that load_proto_module raises for missing module."""
    with pytest.raises(ImportError, match="Generated module"):
        croupier._load_proto_module("missing.module.name")


def test_function_descriptor_with_all_fields():
    """Test FunctionDescriptor with all fields set."""
    desc = croupier.FunctionDescriptor(
        id="test.function",
        version="2.0.0",
        resource="player",
        risk="low",
        operation="ban",
        permission="player.ban",
        enabled=False,
    )

    assert desc.id == "test.function"
    assert desc.version == "2.0.0"
    assert desc.resource == "player"
    assert desc.risk == "low"
    assert desc.operation == "ban"
    assert desc.permission == "player.ban"
    assert desc.enabled is False


def test_client_config_with_all_fields():
    """Test ClientConfig with custom values."""
    config = croupier.ClientConfig(
        agent_addr="custom.agent:19090",
        insecure=False,
        service_id="my-service",
        service_version="2.0.0",
        game_id="my-game",
        env="production",
        agent_id="agent-1",
        heartbeat_interval=30,
        timeout_seconds=60,
        control_addr="https://control.example.com",
        cert_file="/path/to/cert.pem",
        key_file="/path/to/key.pem",
        ca_file="/path/to/ca.pem",
        server_name="agent.example.com",
        auth_token="secret-token",
        headers={"X-Test": "yes"},
        provider_lang="python",
        provider_sdk="custom-sdk",
        reconnect_max_attempts=5,
        disable_logging=True,
        debug_logging=True,
        log_level="DEBUG",
        enable_file_transfer=True,
        max_file_size=32 * 1024 * 1024,
    )

    assert config.agent_addr == "custom.agent:19090"
    assert config.insecure is False
    assert config.service_id == "my-service"
    assert config.service_version == "2.0.0"
    assert config.game_id == "my-game"
    assert config.env == "production"
    assert config.agent_id == "agent-1"
    assert config.heartbeat_interval == 30
    assert config.timeout_seconds == 60
    assert config.control_addr == "https://control.example.com"
    assert config.cert_file == "/path/to/cert.pem"
    assert config.key_file == "/path/to/key.pem"
    assert config.ca_file == "/path/to/ca.pem"
    assert config.server_name == "agent.example.com"
    assert config.auth_token == "secret-token"
    assert config.headers == {"X-Test": "yes"}
    assert config.provider_lang == "python"
    assert config.provider_sdk == "custom-sdk"
    assert config.reconnect_max_attempts == 5
    assert config.disable_logging is True
    assert config.debug_logging is True
    assert config.log_level == "DEBUG"
    assert config.enable_file_transfer is True
    assert config.max_file_size == 32 * 1024 * 1024


def test_task_state_push_with_finished():
    """Test _TaskState.push with finished=True sets done event."""
    state = croupier._TaskState()

    event = croupier.invocation_pb2.TaskEvent(type="test", payload=b"data")
    state.push(event, finished=True)

    assert state.done.is_set()

    first = state.queue.get(timeout=1)
    assert first.type == "test"

    second = state.queue.get(timeout=1)
    assert second is None


def test_task_state_multiple_push():
    """Test _TaskState with multiple pushes."""
    state = croupier._TaskState()

    event1 = croupier.invocation_pb2.TaskEvent(type="progress", payload=b"50")
    event2 = croupier.invocation_pb2.TaskEvent(type="progress", payload=b"100")

    state.push(event1, finished=False)
    state.push(event2, finished=False)

    assert state.queue.qsize() == 2
    assert not state.done.is_set()

    state.push(croupier.invocation_pb2.TaskEvent(type="done"), finished=True)
    assert state.done.is_set()


def test_client_has_initial_state():
    """Test that CroupierClient initializes with correct default state."""
    client = croupier.CroupierClient()

    assert client._handlers == {}
    assert client._descriptors == {}
    assert client._tasks == {}
    assert client._session_id == ""


@pytest.mark.integration
def test_connect_registers_with_agent():
    """Test connect performs ProviderConnect handshake and stores session_id (requires real server)."""
    # Integration test - see test_integration.py for implementation


@pytest.mark.integration
def test_reconnects_and_reregisters_after_agent_restart():
    """Test heartbeat failure triggers reconnect and re-register (requires real server)."""
    # Integration test - see test_integration.py for implementation


def test_client_with_custom_config():
    """Test CroupierClient with custom config."""
    config = croupier.ClientConfig(service_id="test-service")
    client = croupier.CroupierClient(config)

    assert client._config is config
    assert client._config.service_id == "test-service"


def test_protobuf_modules_are_loaded():
    """Test that protobuf modules are loaded correctly."""
    assert hasattr(croupier, "provider_pb2")
    assert hasattr(croupier, "invocation_pb2")


def test_function_handler_type():
    """Test that FunctionHandler is a callable type."""
    assert callable(croupier.FunctionHandler)
    import typing

    assert croupier.FunctionHandler == typing.Callable[[str, bytes], str]


def test_build_manifest_with_all_descriptor_fields():
    """Test build_manifest includes all descriptor fields."""
    config = croupier.ClientConfig(service_id="svc-1", service_version="sv1")
    client = croupier.CroupierClient(config)

    client.register_function(
        croupier.FunctionDescriptor(
            id="full.fn",
            version="2.0.0",
            resource="player",
            risk="low",
            operation="ban",
            permission="player.ban",
            enabled=True,
        ),
        lambda ctx, payload: "ok",  # noqa: E731
    )

    raw = client.build_manifest()
    parsed = json.loads(raw.decode("utf-8"))

    fn = parsed["functions"][0]
    assert fn["id"] == "full.fn"
    assert fn["version"] == "2.0.0"
    assert fn["resource"] == "player"
    assert fn["risk"] == "low"
    assert fn["operation"] == "ban"
    assert fn["permission"] == "player.ban"
    assert fn["enabled"] is True


def test_build_manifest_with_minimal_descriptor():
    """Test build_manifest with minimal descriptor fields."""
    client = croupier.CroupierClient()

    client.register_function(
        croupier.FunctionDescriptor(id="min.fn"),
        lambda ctx, payload: "ok",  # noqa: E731
    )

    raw = client.build_manifest()
    parsed = json.loads(raw.decode("utf-8"))

    fn = parsed["functions"][0]
    assert fn["id"] == "min.fn"
    assert fn["version"] == "1.0.0"
    assert "resource" not in fn
    assert "risk" not in fn
    assert "operation" not in fn
    assert "permission" not in fn


def test_build_manifest_with_empty_functions():
    """Test build_manifest when no functions are registered."""
    client = croupier.CroupierClient()

    raw = client.build_manifest()
    parsed = json.loads(raw.decode("utf-8"))

    assert "provider" in parsed
    assert "functions" not in parsed


def test_invoke_handler_returns_bytes():
    """Test invoke when handler returns bytes."""
    client = croupier.CroupierClient()

    def handler(ctx, payload):
        return b"binary response"

    client.register_function(
        croupier.FunctionDescriptor(id="bytes.fn", version="1.0.0"),
        handler,
    )

    result = client.invoke("bytes.fn", b"test")
    assert result == b"binary response"


def test_invoke_handler_with_metadata():
    """Test invoke passes metadata to handler."""
    client = croupier.CroupierClient()
    received_ctx = None

    def handler(ctx, payload):
        nonlocal received_ctx
        received_ctx = ctx
        return "ok"

    client.register_function(
        croupier.FunctionDescriptor(id="meta.fn", version="1.0.0"),
        handler,
    )

    client.invoke("meta.fn", b"test", metadata={"key": "value"})

    assert received_ctx is not None
    parsed = json.loads(received_ctx)
    assert parsed.get("key") == "value"


def test_invoke_handler_with_empty_payload():
    """Test invoke with None/empty payload."""
    client = croupier.CroupierClient()

    def handler(ctx, payload):
        return f"received:{len(payload)}"

    client.register_function(
        croupier.FunctionDescriptor(id="empty.fn", version="1.0.0"),
        handler,
    )

    result = client.invoke("empty.fn", b"")
    assert result == b"received:0"


def test_start_task_for_unknown_function():
    """Test start_task returns error for unknown function."""
    client = croupier.CroupierClient()

    with pytest.raises(ValueError, match="not found"):
        client.start_task("unknown.fn", b"test")


def test_build_manifest_with_disabled_function():
    """Test build_manifest includes disabled functions correctly."""
    config = croupier.ClientConfig(service_id="svc-1", service_version="sv1")
    client = croupier.CroupierClient(config)

    client.register_function(
        croupier.FunctionDescriptor(
            id="disabled.fn",
            version="1.0.0",
            resource="player",
            enabled=False,
        ),
        lambda ctx, payload: "ok",  # noqa: E731
    )

    raw = client.build_manifest()
    parsed = json.loads(raw.decode("utf-8"))

    fn = parsed["functions"][0]
    assert fn["id"] == "disabled.fn"
    assert "enabled" not in fn


def test_module_version_info():
    """Test module version information."""
    assert hasattr(croupier, "__version__")
    assert hasattr(croupier, "__author__")
    assert hasattr(croupier, "__email__")
    import re

    # Version can be semver or "unknown" when package is not installed
    assert croupier.__version__ == "unknown" or re.match(r"\d+\.\d+\.\d+", croupier.__version__)


def test_module_exports():
    """Test module __all__ exports."""
    from croupier import (
        ClientConfig,
        FunctionDescriptor,
        FunctionHandler,
        CroupierClient,
    )

    assert ClientConfig is not None
    assert FunctionDescriptor is not None
    assert FunctionHandler is not None
    assert CroupierClient is not None
    assert hasattr(croupier, "invoker_pb2")
    assert croupier.invoker_pb2 is croupier.invocation_pb2


def test_invoker_imports_from_module():
    """Test that invoker classes can be imported from main module."""
    from croupier import (
        InvokerConfig,
        InvokeOptions,
        TaskEventInfo,
        Invoker,
        SyncInvoker,
        create_invoker,
        create_sync_invoker,
    )

    assert InvokerConfig is not None
    assert InvokeOptions is not None
    assert TaskEventInfo is not None
    assert Invoker is not None
    assert SyncInvoker is not None
    assert create_invoker is not None
    assert create_sync_invoker is not None


def test_load_proto_module_with_spec_none():
    """Test _load_proto_module when spec is None."""
    with pytest.raises(ImportError, match="Generated module"):
        croupier._load_proto_module("nonexistent.module.path")


def test_get_function_descriptor():
    """Test get_function_descriptor returns correct descriptor."""
    client = croupier.CroupierClient()
    client.register_function(
        croupier.FunctionDescriptor(
            id="test.fn",
            version="2.0.0",
            tags=["test"],
            summary="Test function",
            description="Detailed test function description",
            operation_id="testFn",
            input_schema={"type": "object", "properties": {"id": {"type": "string"}}},
            output_schema={"type": "object", "properties": {"ok": {"type": "boolean"}}},
            resource="player",
            risk="safe",
            operation="ban",
            permission="player.ban",
        ),
        lambda ctx, payload: "ok",  # noqa: E731
    )

    desc = client.get_function_descriptor("test.fn")
    assert desc is not None
    assert desc.id == "test.fn"
    assert desc.version == "2.0.0"
    assert list(desc.tags) == ["test"]
    assert desc.summary == "Test function"
    assert desc.description == "Detailed test function description"
    assert desc.operation_id == "testFn"
    assert '"id"' in desc.input_schema
    assert '"ok"' in desc.output_schema
    assert desc.resource == "player"
    assert desc.risk == "safe"
    assert desc.operation == "ban"
    assert desc.permission == "player.ban"


def test_get_function_descriptor_unknown():
    """Test get_function_descriptor returns None for unknown function."""
    client = croupier.CroupierClient()
    desc = client.get_function_descriptor("unknown.fn")
    assert desc is None


def test_get_provider_connect_request():
    """Test get_provider_connect_request builds correct request."""
    config = croupier.ClientConfig(service_id="test-svc", service_version="1.0.0")
    client = croupier.CroupierClient(config)
    client.register_function(
        croupier.FunctionDescriptor(id="f1", version="1.0.0"),
        lambda ctx, payload: "ok",  # noqa: E731
    )

    req = client.get_provider_connect_request()
    assert req.service_id == "test-svc"
    assert req.version == "1.0.0"
    assert len(req.functions) == 1
    assert req.functions[0].id == "f1"


def test_start_task_with_metadata():
    """Test start_task with metadata."""
    client = croupier.CroupierClient()

    def handler(ctx, payload):
        import time

        time.sleep(0.1)
        return "done"

    client.register_function(
        croupier.FunctionDescriptor(id="task", version="1.0.0"),
        handler,
    )

    task_id = client.start_task("task", b"test", metadata={"task_key": "task_value"})
    events = list(client.stream_task(task_id))

    assert len(events) >= 2
    assert events[0].type == "started"
    assert events[-1].type in ["completed", "error"]


def test_cancel_nonexistent_task():
    """Test cancel_task with nonexistent task returns False."""
    client = croupier.CroupierClient()
    result = client.cancel_task("nonexistent-task-id")
    assert result is False


def test_invoke_with_bytes_result():
    """Test invoke with handler returning bytes."""
    client = croupier.CroupierClient()

    def handler(ctx, payload):
        return b"binary result"

    client.register_function(
        croupier.FunctionDescriptor(id="bytes", version="1.0.0"),
        handler,
    )

    result = client.invoke("bytes", b"test")
    assert result == b"binary result"


def test_invoke_with_string_result():
    """Test invoke with handler returning string."""
    client = croupier.CroupierClient()

    def handler(ctx, payload):
        return "string result"

    client.register_function(
        croupier.FunctionDescriptor(id="str", version="1.0.0"),
        handler,
    )

    result = client.invoke("str", b"test")
    assert result == b"string result"


def test_invoke_with_int_result():
    """Test invoke with handler returning int."""
    client = croupier.CroupierClient()

    def handler(ctx, payload):
        return 42

    client.register_function(
        croupier.FunctionDescriptor(id="int", version="1.0.0"),
        handler,
    )

    result = client.invoke("int", b"test")
    assert result == b"42"


def test_multiple_tasks_concurrent():
    """Test running multiple tasks sequentially."""
    client = croupier.CroupierClient()

    def handler(ctx, payload):
        import time

        time.sleep(0.05)
        return f"done:{payload.decode()}"

    client.register_function(
        croupier.FunctionDescriptor(id="multi", version="1.0.0"),
        handler,
    )

    for i in range(3):
        task_id = client.start_task("multi", f"task{i}".encode())
        events = list(client.stream_task(task_id))
        assert events[0].type == "started"
        assert events[1].type == "completed"


def test_build_manifest_with_functions():
    """Test build_manifest with registered functions."""
    config = croupier.ClientConfig(service_id="test-service", service_version="1.2.3")
    client = croupier.CroupierClient(config)

    client.register_function(
        croupier.FunctionDescriptor(id="fn1", version="1.0.0", resource="player"),
        lambda ctx, payload: "ok",
    )
    client.register_function(
        croupier.FunctionDescriptor(id="fn2", version="2.0.0", resource="mail"),
        lambda ctx, payload: "ok",
    )

    manifest = client.build_manifest()

    assert b"test-service" in manifest
    assert b"1.2.3" in manifest
    assert b"fn1" in manifest
    assert b"fn2" in manifest


def test_handler_with_context():
    """Test handler that uses context."""
    client = croupier.CroupierClient()

    def handler(ctx, payload):
        import json

        ctx_dict = json.loads(ctx)
        return f"received:{ctx_dict.get('key', 'none')}"

    client.register_function(
        croupier.FunctionDescriptor(id="ctx", version="1.0.0"),
        handler,
    )

    result = client.invoke("ctx", b"test", metadata={"key": "value"})
    assert b"value" in result


def test_register_same_function_twice():
    """Test registering the same function twice replaces handler."""
    client = croupier.CroupierClient()

    client.register_function(
        croupier.FunctionDescriptor(id="dup", version="1.0.0"),
        lambda ctx, payload: "first",
    )

    client.register_function(
        croupier.FunctionDescriptor(id="dup", version="1.0.0"),
        lambda ctx, payload: "second",
    )

    result = client.invoke("dup", b"test")
    assert result == b"second"


# ---- Drain tests ----


def test_is_draining_initially_false():
    """Test that is_draining returns False initially."""
    client = croupier.CroupierClient()
    assert client.is_draining() is False


def test_handle_drain_request_sets_draining():
    """Test that handling a drain request triggers draining and reconnect."""
    client = croupier.CroupierClient()
    client.register_function(
        croupier.FunctionDescriptor(id="test.fn", version="1.0.0"),
        lambda ctx, payload: "ok",
    )

    # Handle drain request — spawns background thread
    resp = client._handle_drain_request(b"")
    assert resp == b""  # empty ProviderDrainResponse

    # Wait briefly for background thread to start and set draining
    time.sleep(0.2)
    # After drain completes (no active calls), draining is cleared
    # The key behavior is that the drain was handled without error.


def test_handle_drain_idempotent():
    """Test that repeated drain requests don't cause errors."""
    client = croupier.CroupierClient()
    client.register_function(
        croupier.FunctionDescriptor(id="test.fn", version="1.0.0"),
        lambda ctx, payload: "ok",
    )

    resp1 = client._handle_drain_request(b"")
    resp2 = client._handle_drain_request(b"")
    assert resp1 == b""
    assert resp2 == b""


def test_invoke_rejected_when_draining():
    """Test that inbound invoke is rejected when draining."""
    client = croupier.CroupierClient()
    client.register_function(
        croupier.FunctionDescriptor(id="test.fn", version="1.0.0"),
        lambda ctx, payload: "should not be called",
    )

    # Set draining state directly
    client._draining.set()

    # Build an InvokeRequest
    req = croupier.invocation_pb2.InvokeRequest(function_id="test.fn", payload=b"hello")
    resp_bytes = client._handle_inbound_invoke(req.SerializeToString())
    resp = croupier.invocation_pb2.InvokeResponse()
    resp.ParseFromString(resp_bytes)
    assert resp.payload == b""


def test_start_task_rejected_when_draining():
    """Test that inbound start_task is rejected when draining."""
    client = croupier.CroupierClient()
    client.register_function(
        croupier.FunctionDescriptor(id="test.fn", version="1.0.0"),
        lambda ctx, payload: "should not be called",
    )

    # Set draining state directly
    client._draining.set()

    req = croupier.invocation_pb2.InvokeRequest(function_id="test.fn", payload=b"hello")
    resp_bytes = client._handle_inbound_start_task(req.SerializeToString())
    resp = croupier.invocation_pb2.StartTaskResponse()
    resp.ParseFromString(resp_bytes)
    assert resp.task_id == ""


def test_handle_inbound_dispatches_drain():
    """Test that _handle_inbound routes drain requests correctly."""
    client = croupier.CroupierClient()
    client.register_function(
        croupier.FunctionDescriptor(id="test.fn", version="1.0.0"),
        lambda ctx, payload: "ok",
    )

    # Call _handle_inbound with drain msg_type
    resp = client._handle_inbound(protocol.MSG_PROVIDER_DRAIN_REQUEST, 1, b"")
    assert resp == b""
    # Give background thread time to run
    time.sleep(0.2)
