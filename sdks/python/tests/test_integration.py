"""
Integration tests for Croupier Python SDK

These tests require a running croupier-agent on localhost:19090.
They test real TCP connection, function registration, and heartbeat.
"""

import os
import socket
import time

import croupier
import pytest

# Agent address configuration
AGENT_ADDR = os.getenv("CROUPIER_AGENT_ADDR", "127.0.0.1:19090")
INTEGRATION_TEST_TIMEOUT = 15  # seconds


def is_agent_available() -> bool:
    """Check if agent is available by attempting a TCP connection."""
    try:
        host, port_str = AGENT_ADDR.split(":")
        port = int(port_str)
        sock = socket.create_connection((host, port), timeout=2)
        sock.close()
        return True
    except (OSError, ValueError, socket.timeout):
        return False


@pytest.fixture(scope="module", autouse=True)
def check_agent() -> None:
    """Skip all integration tests if agent is not available."""
    if not is_agent_available():
        pytest.skip(f"croupier-agent not available at {AGENT_ADDR}")


class TestIntegrationConnect:
    """Tests for connecting to croupier-agent."""

    def test_connect_to_agent_and_register_function(self) -> None:
        """Test connecting to agent and registering a function."""
        config = croupier.ClientConfig(
            agent_addr=AGENT_ADDR,
            service_id="python-integration-test",
            service_version="1.0.0",
            heartbeat_interval=30,
        )
        client = croupier.CroupierClient(config)

        # Register a test function
        descriptor = croupier.FunctionDescriptor(
            id="test.ping",
            version="1.0.0",
        )

        def handler(ctx: str, payload: bytes) -> str:
            return f"pong: {payload.decode('utf-8')}"

        client.register_function(descriptor, handler)

        # Connect to agent
        client.connect()

        # Verify we are connected
        assert client._session_id != ""
        assert client._connected is True

        # Clean up
        client.disconnect()
        assert client._connected is False
        assert client._session_id == ""

    def test_connect_fails_with_invalid_agent_address(self) -> None:
        """Test that connect fails with invalid agent address."""
        config = croupier.ClientConfig(
            agent_addr="127.0.0.1:9999",  # Non-existent port
            service_id="python-integration-test",
            heartbeat_interval=30,
        )
        client = croupier.CroupierClient(config)

        descriptor = croupier.FunctionDescriptor(id="test.ping", version="1.0.0")
        client.register_function(descriptor, lambda ctx, payload: "ok")

        with pytest.raises((OSError, RuntimeError, socket.timeout)):
            client.connect()

    def test_connect_requires_at_least_one_function(self) -> None:
        """Test that connect requires at least one registered function."""
        config = croupier.ClientConfig(agent_addr=AGENT_ADDR)
        client = croupier.CroupierClient(config)

        # No functions registered - should fail
        with pytest.raises(RuntimeError, match="Register at least one function"):
            client.connect()

    def test_connect_is_idempotent(self) -> None:
        """Test that multiple connect calls are safe (idempotent)."""
        config = croupier.ClientConfig(
            agent_addr=AGENT_ADDR,
            service_id="python-integration-test-idempotent",
        )
        client = croupier.CroupierClient(config)

        client.register_function(
            croupier.FunctionDescriptor(id="test.idempotent", version="1.0.0"),
            lambda ctx, payload: "ok",
        )

        # First connect
        client.connect()
        first_session_id = client._session_id
        assert first_session_id != ""

        # Second connect should be safe (idempotent)
        client.connect()
        second_session_id = client._session_id
        assert second_session_id != ""

        # Session IDs should be the same (no reconnect)
        assert second_session_id == first_session_id

        # Clean up
        client.disconnect()

    def test_reconnect_after_disconnect(self) -> None:
        """Test reconnecting after disconnect."""
        config = croupier.ClientConfig(
            agent_addr=AGENT_ADDR,
            service_id="python-integration-test-reconnect",
        )
        client = croupier.CroupierClient(config)

        client.register_function(
            croupier.FunctionDescriptor(id="test.reconnect", version="1.0.0"),
            lambda ctx, payload: "ok",
        )

        # First connection
        client.connect()
        session_id_1 = client._session_id
        assert session_id_1 != ""

        # Disconnect
        client.disconnect()
        assert client._session_id == ""

        # Second connection
        client.connect()
        session_id_2 = client._session_id
        assert session_id_2 != ""

        # Session IDs should be different (new session)
        assert session_id_2 != session_id_1

        # Clean up
        client.disconnect()

    def test_disconnect_is_idempotent(self) -> None:
        """Test that multiple disconnect calls are safe (idempotent)."""
        config = croupier.ClientConfig(
            agent_addr=AGENT_ADDR,
            service_id="python-integration-test-disconnect",
        )
        client = croupier.CroupierClient(config)

        client.register_function(
            croupier.FunctionDescriptor(id="test.disconnect", version="1.0.0"),
            lambda ctx, payload: "ok",
        )

        client.connect()

        # Disconnect multiple times - should not throw
        client.disconnect()
        client.disconnect()
        client.disconnect()

        assert client._connected is False


class TestIntegrationHeartbeat:
    """Tests for heartbeat mechanism."""

    def test_heartbeat_is_sent_periodically(self) -> None:
        """Test that heartbeat is sent periodically."""
        config = croupier.ClientConfig(
            agent_addr=AGENT_ADDR,
            service_id="python-integration-test-heartbeat",
            heartbeat_interval=2,  # Short interval for testing
        )
        client = croupier.CroupierClient(config)

        client.register_function(
            croupier.FunctionDescriptor(id="test.hb", version="1.0.0"),
            lambda ctx, payload: "ok",
        )

        client.connect()
        assert client._session_id != ""

        # Wait for a few heartbeats
        time.sleep(5)

        # Should still be connected
        assert client._connected is True
        assert client._session_id != ""

        # Clean up
        client.disconnect()


class TestIntegrationMultipleFunctions:
    """Tests for registering multiple functions."""

    def test_register_multiple_functions(self) -> None:
        """Test registering multiple functions."""
        config = croupier.ClientConfig(
            agent_addr=AGENT_ADDR,
            service_id="python-integration-test-multi",
            heartbeat_interval=30,
        )
        client = croupier.CroupierClient(config)

        # Register multiple functions
        functions = [
            (
                croupier.FunctionDescriptor(id="test.ping", version="1.0.0"),
                lambda ctx, payload: f"pong: {payload.decode('utf-8')}",
            ),
            (
                croupier.FunctionDescriptor(id="test.echo", version="1.0.0"),
                lambda ctx, payload: payload.decode("utf-8"),
            ),
            (
                croupier.FunctionDescriptor(id="test.upper", version="1.0.0"),
                lambda ctx, payload: payload.decode("utf-8").upper(),
            ),
        ]

        for descriptor, handler in functions:
            client.register_function(descriptor, handler)

        client.connect()
        assert client._session_id != ""
        assert len(client._handlers) == 3

        # Clean up
        client.disconnect()
