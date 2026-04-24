"""
Tests for Croupier Python SDK TCP Transport Layer.
"""

import time

import pytest

from croupier.protocol import (
    MSG_INVOKE_REQUEST,
    MSG_INVOKE_RESPONSE,
    get_response_msg_id,
    is_request,
    is_response,
    msg_id_string,
    new_message,
    parse_message,
)
from croupier.transport.tcp import TCPTransport


class TestProtocol:
    """Tests for protocol encoding/decoding."""

    def test_new_message(self):
        msg_type = MSG_INVOKE_REQUEST
        req_id = 12345
        body = b"test payload"

        message = new_message(msg_type, req_id, body)

        assert len(message) == 8 + len(body)
        assert message[0] == 0x01  # Version

    def test_parse_message(self):
        msg_type = MSG_INVOKE_REQUEST
        req_id = 999
        body = b"hello world"

        message = new_message(msg_type, req_id, body)
        version, parsed_type, parsed_id, parsed_body = parse_message(message)

        assert version == 0x01
        assert parsed_type == msg_type
        assert parsed_id == req_id
        assert parsed_body == body

    def test_get_response_msg_id(self):
        assert get_response_msg_id(MSG_INVOKE_REQUEST) == MSG_INVOKE_RESPONSE

    def test_is_request(self):
        assert is_request(MSG_INVOKE_REQUEST) is True
        assert is_request(MSG_INVOKE_RESPONSE) is False

    def test_is_response(self):
        assert is_response(MSG_INVOKE_RESPONSE) is True
        assert is_response(MSG_INVOKE_REQUEST) is False

    def test_msg_id_string(self):
        assert msg_id_string(MSG_INVOKE_REQUEST) == "InvokeRequest"
        assert msg_id_string(MSG_INVOKE_RESPONSE) == "InvokeResponse"
        assert msg_id_string(0xFFFFFF) == "Unknown(0xFFFFFF)"


class TestTCPTransport:
    """Tests for TCP transport layer."""

    def test_transport_not_connected_error(self):
        """Test that call raises error when not connected."""
        import random

        port = 19000 + random.randint(0, 1000)
        transport = TCPTransport(address=f"127.0.0.1:{port}")
        with pytest.raises(RuntimeError, match="Not connected"):
            transport.call(MSG_INVOKE_REQUEST, b"data")

    def test_transport_context_manager_not_connected(self):
        """Test context manager without server (expect connection error)."""
        import random

        port = 19000 + random.randint(0, 1000)
        # Use a non-existent address - connection will fail
        with pytest.raises(Exception):
            with TCPTransport(address=f"127.0.0.1:{port}") as transport:
                transport.is_connected()

    def test_transport_close_without_connect(self):
        """Test closing without connection is safe."""
        transport = TCPTransport()
        transport.close()  # Should not raise
        assert not transport.is_connected()

    def test_transport_set_handler(self):
        """Test setting handler before connection."""

        def handler(msg_type, req_id, body):
            return body

        transport = TCPTransport()
        transport.set_handler(handler)
        # Handler is stored internally, can't directly verify but shouldn't raise
        transport.close()
