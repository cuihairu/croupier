"""
Tests for Croupier Python SDK TCP Transport Layer.
"""

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

    def test_transport_address_parsing(self):
        """Test various address formats."""
        # Standard address
        transport = TCPTransport(address="127.0.0.1:19090")
        assert transport.address == "127.0.0.1:19090"

        # With scheme
        transport = TCPTransport(address="tcp://127.0.0.1:19090")
        assert transport.address == "tcp://127.0.0.1:19090"

    def test_transport_timeout_config(self):
        """Test timeout configuration."""
        transport = TCPTransport(timeout_ms=60000)
        assert transport.timeout_ms == 60000

    def test_transport_tls_config(self):
        """Test TLS configuration options."""
        transport = TCPTransport(
            tls_enabled=True,
            tls_cert_file="/path/to/cert.pem",
            tls_key_file="/path/to/key.pem",
            tls_ca_file="/path/to/ca.pem",
            tls_server_name="example.com",
            tls_insecure_skip_verify=True,
        )
        assert transport._tls_enabled is True
        assert transport._tls_cert_file == "/path/to/cert.pem"
        assert transport._tls_key_file == "/path/to/key.pem"
        assert transport._tls_ca_file == "/path/to/ca.pem"
        assert transport._tls_server_name == "example.com"
        assert transport._tls_insecure_skip_verify is True

    def test_transport_strip_scheme(self):
        """Test _strip_scheme static method."""
        assert TCPTransport._strip_scheme("tcp://127.0.0.1:19090") == "127.0.0.1:19090"
        assert TCPTransport._strip_scheme("tls://127.0.0.1:19090") == "127.0.0.1:19090"
        assert TCPTransport._strip_scheme("127.0.0.1:19090") == "127.0.0.1:19090"

    def test_transport_multiple_close(self):
        """Test that calling close multiple times is safe."""
        transport = TCPTransport()
        transport.close()
        transport.close()  # Should not raise
        assert not transport.is_connected()


class TestPendingCall:
    """Tests for _PendingCall helper."""

    def test_pending_call_initial_state(self):
        from croupier.transport.tcp import _PendingCall

        pending = _PendingCall()
        assert pending.resp_msg_id == 0
        assert pending.resp_body == b""
        assert pending.error is None

    def test_pending_call_deliver(self):
        from croupier.transport.tcp import _PendingCall

        pending = _PendingCall()
        pending.deliver(0x030102, b"response")

        assert pending.resp_msg_id == 0x030102
        assert pending.resp_body == b"response"
        assert pending.event.is_set()

    def test_pending_call_fail(self):
        from croupier.transport.tcp import _PendingCall

        pending = _PendingCall()
        err = ConnectionError("test error")
        pending.fail(err)

        assert pending.error is err
        assert pending.event.is_set()

    def test_pending_call_wait_timeout(self):
        from croupier.transport.tcp import _PendingCall

        pending = _PendingCall()
        result = pending.wait(timeout=0.01)
        assert result is False  # timeout, not set


class TestFrameHelpers:
    """Tests for frame read/write helpers."""

    def test_write_frame_basic(self):
        from unittest.mock import MagicMock
        from croupier.transport.tcp import _write_frame
        import struct

        mock_sock = MagicMock()
        payload = b"test payload"

        _write_frame(mock_sock, payload)

        expected_header = struct.pack(">I", len(payload))
        mock_sock.sendall.assert_called_once_with(expected_header + payload)

    def test_write_frame_too_large(self):
        from unittest.mock import MagicMock
        from croupier.transport.tcp import _write_frame

        mock_sock = MagicMock()
        payload = b"x" * (32 * 1024 * 1024 + 1)  # 32MB + 1

        with pytest.raises(ValueError, match="frame too large"):
            _write_frame(mock_sock, payload)

    def test_read_frame_basic(self):
        from unittest.mock import MagicMock
        from croupier.transport.tcp import _read_frame
        import struct

        mock_sock = MagicMock()
        payload = b"test payload"
        header = struct.pack(">I", len(payload))

        mock_sock.recv.side_effect = [header, payload]

        result = _read_frame(mock_sock)
        assert result == payload

    def test_read_frame_empty(self):
        from unittest.mock import MagicMock
        from croupier.transport.tcp import _read_frame
        import struct

        mock_sock = MagicMock()
        header = struct.pack(">I", 0)

        mock_sock.recv.return_value = header

        result = _read_frame(mock_sock)
        assert result == b""

    def test_read_exact_basic(self):
        from unittest.mock import MagicMock
        from croupier.transport.tcp import _read_exact

        mock_sock = MagicMock()
        mock_sock.recv.side_effect = [b"he", b"ll", b"o"]

        result = _read_exact(mock_sock, 5)
        assert result == b"hello"

    def test_read_exact_connection_closed(self):
        from unittest.mock import MagicMock
        from croupier.transport.tcp import _read_exact

        mock_sock = MagicMock()
        mock_sock.recv.return_value = b""

        with pytest.raises(ConnectionError, match="connection closed"):
            _read_exact(mock_sock, 5)
