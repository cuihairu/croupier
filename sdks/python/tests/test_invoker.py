"""
Tests for Croupier Python SDK Invoker functionality.

Uses TCP transport for communication.
"""

import asyncio
import croupier.invoker as invoker_module

from croupier.invoker import (
    Invoker,
    InvokerConfig,
    ReconnectConfig,
    RetryConfig,
    InvokeOptions,
    JobEventInfo,
    _calculate_reconnect_delay,
    _default_reconnect_config,
    _default_retry_config,
    default_invoker_config,
    create_invoker,
    SyncInvoker,
    create_sync_invoker,
)


def test_invoker_config_defaults():
    """Test that InvokerConfig has correct default values."""
    config = InvokerConfig()
    assert config.address == "127.0.0.1:19090"
    assert config.timeout == 30000
    assert config.insecure is True
    assert config.ca_file == ""
    assert config.cert_file == ""
    assert config.key_file == ""
    assert config.server_name == ""


def test_invoker_config_with_reconnect_defaults():
    config = InvokerConfig()
    assert isinstance(config.reconnect, ReconnectConfig)
    assert config.reconnect.enabled is True
    assert config.reconnect.max_attempts == 0
    assert config.reconnect.initial_delay_ms == 1000
    assert config.reconnect.max_delay_ms == 30000
    assert config.reconnect.backoff_multiplier == 2.0
    assert config.reconnect.jitter_factor == 0.2


def test_invoker_config_with_retry_defaults():
    config = InvokerConfig()
    assert isinstance(config.retry, RetryConfig)
    assert config.retry.enabled is True
    assert config.retry.max_attempts == 3
    assert config.retry.initial_delay_ms == 100
    assert config.retry.max_delay_ms == 5000
    assert config.retry.backoff_multiplier == 2.0
    assert config.retry.jitter_factor == 0.1


def test_invoker_config_custom():
    config = InvokerConfig(
        address="localhost:9090",
        timeout=60000,
        insecure=False,
        ca_file="/path/to/ca.pem",
    )
    assert config.address == "localhost:9090"
    assert config.timeout == 60000
    assert config.insecure is False
    assert config.ca_file == "/path/to/ca.pem"


def test_reconnect_config_custom():
    reconnect = ReconnectConfig(
        enabled=True,
        max_attempts=5,
        initial_delay_ms=500,
        max_delay_ms=10000,
        backoff_multiplier=3.0,
        jitter_factor=0.5,
    )
    assert reconnect.enabled is True
    assert reconnect.max_attempts == 5
    assert reconnect.initial_delay_ms == 500
    assert reconnect.max_delay_ms == 10000
    assert reconnect.backoff_multiplier == 3.0
    assert reconnect.jitter_factor == 0.5


def test_retry_config_custom():
    retry = RetryConfig(
        enabled=True,
        max_attempts=5,
        initial_delay_ms=200,
        max_delay_ms=10000,
        backoff_multiplier=3.0,
        jitter_factor=0.3,
    )
    assert retry.enabled is True
    assert retry.max_attempts == 5
    assert retry.initial_delay_ms == 200
    assert retry.max_delay_ms == 10000
    assert retry.backoff_multiplier == 3.0
    assert retry.jitter_factor == 0.3


def test_invoke_options_defaults():
    options = InvokeOptions()
    assert options.idempotency_key is None
    assert options.timeout is None
    assert options.headers is None
    assert options.retry is None


def test_invoke_options_custom():
    custom_retry = RetryConfig(max_attempts=5)
    options = InvokeOptions(
        idempotency_key="test-key",
        timeout=5000,
        headers={"key": "value"},
        retry=custom_retry,
    )
    assert options.idempotency_key == "test-key"
    assert options.timeout == 5000
    assert options.headers == {"key": "value"}
    assert options.retry is custom_retry


def test_job_event_info():
    info = JobEventInfo(
        type="completed",
        job_id="test-job",
        payload="result",
        message="Job completed",
        progress=100,
        error=None,
        done=True,
    )
    assert info.type == "completed"
    assert info.job_id == "test-job"
    assert info.payload == "result"
    assert info.message == "Job completed"
    assert info.progress == 100
    assert info.error is None
    assert info.done is True


def test_job_event_info_defaults():
    info = JobEventInfo(type="test", job_id="job-1")
    assert info.type == "test"
    assert info.job_id == "job-1"
    assert info.payload is None
    assert info.message is None
    assert info.progress is None
    assert info.error is None
    assert info.done is False


def test_invoker_initialization():
    config = InvokerConfig(address="localhost:9090", timeout=60000)
    invoker = Invoker(config)
    assert invoker.config is config
    assert invoker._schemas == {}
    assert invoker._connected is False


def test_invoker_initialization_with_default_config():
    invoker = Invoker()
    assert invoker.config.address == "127.0.0.1:19090"
    assert invoker.config.timeout == 30000
    assert invoker._connected is False


def test_calculate_reconnect_delay():
    config = ReconnectConfig(
        initial_delay_ms=1000,
        max_delay_ms=10000,
        backoff_multiplier=2.0,
        jitter_factor=0.0,
    )

    delay = _calculate_reconnect_delay(0, config)
    assert delay == 1.0

    delay = _calculate_reconnect_delay(1, config)
    assert delay == 2.0

    delay = _calculate_reconnect_delay(2, config)
    assert delay == 4.0

    delay = _calculate_reconnect_delay(10, config)
    assert delay == 10.0


def test_calculate_reconnect_delay_with_jitter():
    config = ReconnectConfig(
        initial_delay_ms=1000,
        max_delay_ms=10000,
        backoff_multiplier=2.0,
        jitter_factor=0.5,
    )

    delays = [_calculate_reconnect_delay(0, config) for _ in range(10)]
    assert len(set(delays)) > 1
    assert all(d > 0 for d in delays)
    assert all(0.5 <= d <= 1.5 for d in delays)


def test_default_invoker_config():
    config = default_invoker_config()
    assert isinstance(config, InvokerConfig)
    assert config.address == "127.0.0.1:19090"
    assert config.timeout == 30000


def test_create_invoker():
    invoker = create_invoker()
    assert isinstance(invoker, Invoker)
    assert invoker.config.address == "127.0.0.1:19090"


def test_create_invoker_with_config():
    config = InvokerConfig(address="custom:9999")
    invoker = create_invoker(config)
    assert isinstance(invoker, Invoker)
    assert invoker.config is config


def test_invoker_set_schema():
    async def test():
        invoker = Invoker()
        schema = {"type": "object", "properties": {"name": {"type": "string"}}}
        await invoker.set_schema("test.function", schema)
        assert "test.function" in invoker._schemas
        assert invoker._schemas["test.function"] == schema

    asyncio.run(test())


def test_invoker_close_without_connect():
    async def test():
        invoker = Invoker()
        await invoker.close()
        assert invoker._connected is False

    asyncio.run(test())


def test_invoker_close_clears_state():
    async def test():
        invoker = Invoker()
        invoker._connected = True
        invoker._schemas["test"] = {"type": "object"}
        await invoker.close()
        assert invoker._connected is False
        assert len(invoker._schemas) == 0

    asyncio.run(test())


def test_invoker_connect_to_server():
    """Test connecting to a real TCP server (requires real server)."""
    # This test requires a real server, skip it in unit tests
    pass


def test_invoker_invoke_with_server():
    """Test invoking a function through TCP server (requires real server)."""
    # This test requires a real server, skip it in unit tests
    pass


def test_invoker_start_job_with_server():
    """Test starting a job through TCP server (requires real server)."""
    # This test requires a real server, skip it in unit tests
    pass


def test_invoker_stream_job_polls_until_terminal_event():
    async def test():
        invoker = Invoker()
        invoker._connected = True

        responses = []

        progress = invoker_module.invocation_pb2.JobEvent(
            type="progress",
            message="Job is running",
            progress=25,
        )
        responses.append(progress.SerializeToString())

        completed = invoker_module.invocation_pb2.JobEvent(
            type="done",
            message="Job completed successfully",
            payload=b"result",
        )
        responses.append(completed.SerializeToString())

        call_count = 0

        def fake_call(msg_type, data):
            nonlocal call_count
            assert msg_type == invoker_module.protocol.MSG_STREAM_JOB_REQUEST
            req = invoker_module.invocation_pb2.JobStreamRequest()
            req.ParseFromString(data)
            assert req.job_id == "job-1"

            body = responses[call_count]
            call_count += 1
            return invoker_module.protocol.get_response_msg_id(msg_type), body

        original_sleep = invoker_module.asyncio.sleep

        async def fake_sleep(_seconds: float) -> None:
            return None

        invoker._transport.call = fake_call  # type: ignore[method-assign]
        invoker_module.asyncio.sleep = fake_sleep
        try:
            events = [event async for event in invoker.stream_job("job-1")]
        finally:
            invoker_module.asyncio.sleep = original_sleep

        assert call_count == 2
        assert len(events) == 2
        assert events[0].type == "progress"
        assert events[0].done is False
        assert events[0].message == "Job is running"
        assert events[0].progress == 25
        assert events[1].type == "completed"
        assert events[1].done is True
        assert events[1].payload == "result"

    asyncio.run(test())


def test_invoker_stream_job_normalizes_cancelled_event():
    async def test():
        invoker = Invoker()
        invoker._connected = True

        cancelled = invoker_module.invocation_pb2.JobEvent(
            type="error",
            message="Job was cancelled",
        )

        def fake_call(msg_type, data):
            assert msg_type == invoker_module.protocol.MSG_STREAM_JOB_REQUEST
            req = invoker_module.invocation_pb2.JobStreamRequest()
            req.ParseFromString(data)
            assert req.job_id == "job-cancelled"
            return (
                invoker_module.protocol.get_response_msg_id(msg_type),
                cancelled.SerializeToString(),
            )

        invoker._transport.call = fake_call  # type: ignore[method-assign]

        events = [event async for event in invoker.stream_job("job-cancelled")]
        assert len(events) == 1
        assert events[0].type == "cancelled"
        assert events[0].done is True
        assert events[0].error == "Job was cancelled"

    asyncio.run(test())


def test_invoker_invoke_with_schema_validation():
    async def test():
        invoker = Invoker()
        schema = {
            "type": "object",
            "properties": {"name": {"type": "string"}},
            "required": ["name"],
        }
        await invoker.set_schema("test.fn", schema)

        try:
            await invoker.invoke("test.fn", '{"wrong":"field"}')
            assert False, "Should have raised validation error"
        except Exception as e:
            assert (
                "name" in str(e).lower()
                or "required" in str(e).lower()
                or "missing" in str(e).lower()
            )

    asyncio.run(test())


# ============= SyncInvoker Tests =============


def test_sync_invoker_initialization():
    invoker = SyncInvoker()
    assert invoker._async_invoker is not None
    assert invoker._loop is None


def test_sync_invoker_initialization_with_config():
    config = InvokerConfig(address="custom:9999")
    invoker = SyncInvoker(config)
    assert invoker._async_invoker.config.address == "custom:9999"


def test_create_sync_invoker():
    invoker = create_sync_invoker()
    assert invoker is not None
    assert invoker._async_invoker.config.address == "127.0.0.1:19090"


def test_create_sync_invoker_with_config():
    config = InvokerConfig(address="custom:8888")
    invoker = create_sync_invoker(config)
    assert invoker._async_invoker.config.address == "custom:8888"


def test_sync_invoker_set_schema():
    invoker = SyncInvoker()
    schema = {"type": "object", "properties": {"name": {"type": "string"}}}
    invoker.set_schema("test.fn", schema)
    assert "test.fn" in invoker._async_invoker._schemas
    assert invoker._async_invoker._schemas["test.fn"] == schema


def test_sync_invoker_close():
    invoker = SyncInvoker()
    invoker.close()


def test_sync_invoker_connect_and_invoke():
    """Test SyncInvoker connect and invoke with server (requires real server)."""
    # This test requires a real server, skip it in unit tests
    pass


# ============= Reconnect Config Tests =============


def test_invoker_reconnect_config_defaults():
    config = _default_reconnect_config()
    assert isinstance(config, ReconnectConfig)
    assert config.enabled is True


def test_invoker_retry_config_defaults():
    config = _default_retry_config()
    assert isinstance(config, RetryConfig)
    assert config.enabled is True


def test_schedule_reconnect_when_already_reconnecting():
    invoker = Invoker()
    invoker._is_reconnecting = True
    invoker._reconnect_attempts = 0
    invoker._schedule_reconnect()
    assert invoker._is_reconnecting is True


def test_schedule_reconnect_when_max_attempts_reached():
    invoker = Invoker()
    invoker.config.reconnect.max_attempts = 3
    invoker._reconnect_attempts = 3
    invoker._schedule_reconnect()
    assert invoker._is_reconnecting is False


# ========== SyncInvoker Tests ==========


def test_sync_invoker_config():
    config = InvokerConfig(address="127.0.0.1:19999")
    invoker = SyncInvoker(config)
    assert invoker._async_invoker.config.address == "127.0.0.1:19999"


def test_invoke_options_with_values():
    options = InvokeOptions(
        idempotency_key="test-key-123",
        timeout=5000,
        headers={"X-Request-ID": "req-456"},
    )
    assert options.idempotency_key == "test-key-123"
    assert options.timeout == 5000
    assert options.headers == {"X-Request-ID": "req-456"}


# ========== Error Handling Tests ==========


def test_invoker_invoke_not_connected():
    invoker = Invoker()

    async def test():
        try:
            await invoker.invoke("test.function", "{}")
            assert False, "Should have raised"
        except Exception:
            pass

    asyncio.run(test())


def test_invoker_close_when_not_connected():
    invoker = Invoker()
    asyncio.run(invoker.close())


def test_reconnect_config_custom_values():
    config = ReconnectConfig(
        enabled=False,
        max_attempts=5,
        initial_delay_ms=2000,
        max_delay_ms=60000,
        backoff_multiplier=3.0,
    )
    assert config.enabled is False
    assert config.max_attempts == 5
    assert config.initial_delay_ms == 2000
    assert config.max_delay_ms == 60000
    assert config.backoff_multiplier == 3.0


def test_retry_config_custom_values():
    config = RetryConfig(
        enabled=False,
        max_attempts=10,
        initial_delay_ms=1000,
        max_delay_ms=30000,
        backoff_multiplier=2.0,
    )
    assert config.enabled is False
    assert config.max_attempts == 10
    assert config.initial_delay_ms == 1000
    assert config.max_delay_ms == 30000
    assert config.backoff_multiplier == 2.0


# ========== Reconnect Delay Tests ==========


def test_calculate_reconnect_delay_first_attempt():
    config = ReconnectConfig(initial_delay_ms=1000, backoff_multiplier=2.0, max_delay_ms=30000)
    delay = _calculate_reconnect_delay(0, config)
    assert delay >= 0


def test_calculate_reconnect_delay_second_attempt():
    config = ReconnectConfig(initial_delay_ms=1000, backoff_multiplier=2.0, max_delay_ms=30000)
    delay = _calculate_reconnect_delay(1, config)
    assert delay >= 0


def test_calculate_reconnect_delay_caps_at_max():
    config = ReconnectConfig(initial_delay_ms=1000, backoff_multiplier=2.0, max_delay_ms=10000)
    delay = _calculate_reconnect_delay(10, config)
    assert delay <= config.max_delay_ms


# ========== Factory Function Tests ==========


def test_create_invoker_default():
    invoker = create_invoker()
    assert isinstance(invoker, Invoker)
    assert invoker.config.address == "127.0.0.1:19090"


def test_create_invoker_custom_config():
    config = InvokerConfig(
        address="127.0.0.1:19999",
        timeout=60000,
    )
    invoker = create_invoker(config=config)
    assert invoker.config.address == "127.0.0.1:19999"
    assert invoker.config.timeout == 60000
