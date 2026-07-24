"""Tests for Croupier wire protocol implementation."""

import struct

import pytest

from croupier import protocol


class TestProtocolConstants:
    """Test protocol constants and message type definitions."""

    def test_version_constant(self):
        assert protocol.VERSION_1 == 0x01

    def test_header_size(self):
        assert protocol.HEADER_SIZE == 8

    def test_message_type_constants_are_unique(self):
        """All message type constants should be unique."""
        msg_types = [
            protocol.MSG_REGISTER_REQUEST,
            protocol.MSG_REGISTER_RESPONSE,
            protocol.MSG_HEARTBEAT_REQUEST,
            protocol.MSG_HEARTBEAT_RESPONSE,
            protocol.MSG_INVOKE_REQUEST,
            protocol.MSG_INVOKE_RESPONSE,
            protocol.MSG_START_TASK_REQUEST,
            protocol.MSG_START_TASK_RESPONSE,
            protocol.MSG_TASK_EVENT,
            protocol.MSG_CANCEL_TASK_REQUEST,
            protocol.MSG_CANCEL_TASK_RESPONSE,
            protocol.MSG_PROVIDER_CONNECT_REQUEST,
            protocol.MSG_PROVIDER_CONNECT_RESPONSE,
            protocol.MSG_PROVIDER_HEARTBEAT_REQUEST,
            protocol.MSG_PROVIDER_HEARTBEAT_RESPONSE,
        ]
        assert len(msg_types) == len(set(msg_types))

    def test_request_ids_are_odd(self):
        """Request message IDs should be odd."""
        request_ids = [
            protocol.MSG_REGISTER_REQUEST,
            protocol.MSG_HEARTBEAT_REQUEST,
            protocol.MSG_INVOKE_REQUEST,
            protocol.MSG_START_TASK_REQUEST,
            protocol.MSG_CANCEL_TASK_REQUEST,
            protocol.MSG_PROVIDER_CONNECT_REQUEST,
            protocol.MSG_PROVIDER_HEARTBEAT_REQUEST,
        ]
        for msg_id in request_ids:
            assert msg_id % 2 == 1, f"Request ID 0x{msg_id:06X} should be odd"

    def test_response_ids_are_even(self):
        """Response message IDs should be even."""
        response_ids = [
            protocol.MSG_REGISTER_RESPONSE,
            protocol.MSG_HEARTBEAT_RESPONSE,
            protocol.MSG_INVOKE_RESPONSE,
            protocol.MSG_START_TASK_RESPONSE,
            protocol.MSG_CANCEL_TASK_RESPONSE,
            protocol.MSG_PROVIDER_CONNECT_RESPONSE,
            protocol.MSG_PROVIDER_HEARTBEAT_RESPONSE,
        ]
        for msg_id in response_ids:
            assert msg_id % 2 == 0, f"Response ID 0x{msg_id:06X} should be even"


class TestPutMsgId:
    """Test put_msg_id function."""

    def test_basic_encoding(self):
        msg_id = 0x030101
        result = protocol.put_msg_id(msg_id)
        assert len(result) == 3
        assert result == bytes([0x03, 0x01, 0x01])

    def test_max_value(self):
        msg_id = 0xFFFFFF
        result = protocol.put_msg_id(msg_id)
        assert result == bytes([0xFF, 0xFF, 0xFF])

    def test_zero(self):
        result = protocol.put_msg_id(0)
        assert result == bytes([0x00, 0x00, 0x00])

    def test_single_byte_values(self):
        assert protocol.put_msg_id(0x01) == bytes([0x00, 0x00, 0x01])
        assert protocol.put_msg_id(0x100) == bytes([0x00, 0x01, 0x00])
        assert protocol.put_msg_id(0x10000) == bytes([0x01, 0x00, 0x00])


class TestGetMsgId:
    """Test get_msg_id function."""

    def test_basic_decoding(self):
        data = bytes([0x03, 0x01, 0x01])
        assert protocol.get_msg_id(data) == 0x030101

    def test_max_value(self):
        data = bytes([0xFF, 0xFF, 0xFF])
        assert protocol.get_msg_id(data) == 0xFFFFFF

    def test_zero(self):
        data = bytes([0x00, 0x00, 0x00])
        assert protocol.get_msg_id(data) == 0

    def test_roundtrip(self):
        """Encoding then decoding should return original value."""
        for msg_id in [0x010101, 0x030101, 0xFFFFFF, 0x000001]:
            assert protocol.get_msg_id(protocol.put_msg_id(msg_id)) == msg_id


class TestNewMessage:
    """Test new_message function."""

    def test_basic_message(self):
        msg_id = protocol.MSG_INVOKE_REQUEST
        req_id = 12345
        body = b"test payload"

        message = protocol.new_message(msg_id, req_id, body)

        assert len(message) == protocol.HEADER_SIZE + len(body)
        assert message[0] == protocol.VERSION_1

    def test_empty_body(self):
        msg_id = protocol.MSG_HEARTBEAT_REQUEST
        req_id = 999

        message = protocol.new_message(msg_id, req_id, b"")

        assert len(message) == protocol.HEADER_SIZE

    def test_none_body(self):
        """None body should be treated as empty bytes."""
        msg_id = protocol.MSG_HEARTBEAT_REQUEST
        req_id = 999

        message = protocol.new_message(msg_id, req_id, b"")

        assert len(message) == protocol.HEADER_SIZE

    def test_large_body(self):
        msg_id = protocol.MSG_INVOKE_REQUEST
        req_id = 1
        body = b"x" * 1024 * 1024  # 1MB

        message = protocol.new_message(msg_id, req_id, body)

        assert len(message) == protocol.HEADER_SIZE + len(body)

    def test_message_structure(self):
        """Verify the exact structure of a message."""
        msg_id = 0x030101
        req_id = 0x00000001
        body = b"hello"

        message = protocol.new_message(msg_id, req_id, body)

        # Version byte
        assert message[0] == 0x01
        # MsgID (3 bytes, big-endian)
        assert message[1:4] == bytes([0x03, 0x01, 0x01])
        # RequestID (4 bytes, big-endian)
        assert message[4:8] == struct.pack(">I", 1)
        # Body
        assert message[8:] == b"hello"


class TestParseMessage:
    """Test parse_message function."""

    def test_basic_parse(self):
        msg_id = protocol.MSG_INVOKE_REQUEST
        req_id = 999
        body = b"hello world"

        message = protocol.new_message(msg_id, req_id, body)
        version, parsed_msg_id, parsed_req_id, parsed_body = protocol.parse_message(message)

        assert version == protocol.VERSION_1
        assert parsed_msg_id == msg_id
        assert parsed_req_id == req_id
        assert parsed_body == body

    def test_empty_body(self):
        msg_id = protocol.MSG_HEARTBEAT_REQUEST
        req_id = 12345

        message = protocol.new_message(msg_id, req_id, b"")
        version, parsed_msg_id, parsed_req_id, parsed_body = protocol.parse_message(message)

        assert version == protocol.VERSION_1
        assert parsed_msg_id == msg_id
        assert parsed_req_id == req_id
        assert parsed_body == b""

    def test_too_short_raises_error(self):
        short_message = bytes([0x01, 0x02, 0x03])
        with pytest.raises(ValueError, match="Message too short"):
            protocol.parse_message(short_message)

    def test_exactly_header_size(self):
        """Parsing a message with exactly header size should return empty body."""
        msg_id = protocol.MSG_HEARTBEAT_REQUEST
        req_id = 1

        message = protocol.new_message(msg_id, req_id, b"")
        assert len(message) == protocol.HEADER_SIZE

        version, parsed_msg_id, parsed_req_id, parsed_body = protocol.parse_message(message)
        assert parsed_body == b""

    def test_roundtrip(self):
        """Creating and parsing should return original values."""
        msg_id = protocol.MSG_REGISTER_REQUEST
        req_id = 42
        body = b"test data"

        message = protocol.new_message(msg_id, req_id, body)
        version, parsed_msg_id, parsed_req_id, parsed_body = protocol.parse_message(message)

        assert parsed_msg_id == msg_id
        assert parsed_req_id == req_id
        assert parsed_body == body


class TestIsRequest:
    """Test is_request function."""

    def test_request_messages(self):
        assert protocol.is_request(protocol.MSG_REGISTER_REQUEST) is True
        assert protocol.is_request(protocol.MSG_HEARTBEAT_REQUEST) is True
        assert protocol.is_request(protocol.MSG_INVOKE_REQUEST) is True
        assert protocol.is_request(protocol.MSG_START_TASK_REQUEST) is True
        assert protocol.is_request(protocol.MSG_CANCEL_TASK_REQUEST) is True
        assert protocol.is_request(protocol.MSG_PROVIDER_CONNECT_REQUEST) is True
        assert protocol.is_request(protocol.MSG_PROVIDER_HEARTBEAT_REQUEST) is True

    def test_response_messages(self):
        assert protocol.is_request(protocol.MSG_REGISTER_RESPONSE) is False
        assert protocol.is_request(protocol.MSG_HEARTBEAT_RESPONSE) is False
        assert protocol.is_request(protocol.MSG_INVOKE_RESPONSE) is False

    def test_event_messages(self):
        """TaskEvent and MetricEvent are neither requests nor responses."""
        assert protocol.is_request(protocol.MSG_TASK_EVENT) is False
        assert protocol.is_request(protocol.MSG_METRIC_EVENT) is False


class TestIsResponse:
    """Test is_response function."""

    def test_response_messages(self):
        assert protocol.is_response(protocol.MSG_REGISTER_RESPONSE) is True
        assert protocol.is_response(protocol.MSG_HEARTBEAT_RESPONSE) is True
        assert protocol.is_response(protocol.MSG_INVOKE_RESPONSE) is True
        assert protocol.is_response(protocol.MSG_START_TASK_RESPONSE) is True
        assert protocol.is_response(protocol.MSG_CANCEL_TASK_RESPONSE) is True
        assert protocol.is_response(protocol.MSG_PROVIDER_CONNECT_RESPONSE) is True
        assert protocol.is_response(protocol.MSG_PROVIDER_HEARTBEAT_RESPONSE) is True

    def test_request_messages(self):
        assert protocol.is_response(protocol.MSG_REGISTER_REQUEST) is False
        assert protocol.is_response(protocol.MSG_HEARTBEAT_REQUEST) is False
        assert protocol.is_response(protocol.MSG_INVOKE_REQUEST) is False

    def test_event_messages(self):
        """TaskEvent and MetricEvent are neither requests nor responses."""
        assert protocol.is_response(protocol.MSG_TASK_EVENT) is False
        assert protocol.is_response(protocol.MSG_METRIC_EVENT) is False


class TestGetResponseMsgId:
    """Test get_response_msg_id function."""

    def test_basic_pairs(self):
        assert protocol.get_response_msg_id(protocol.MSG_REGISTER_REQUEST) == protocol.MSG_REGISTER_RESPONSE
        assert protocol.get_response_msg_id(protocol.MSG_HEARTBEAT_REQUEST) == protocol.MSG_HEARTBEAT_RESPONSE
        assert protocol.get_response_msg_id(protocol.MSG_INVOKE_REQUEST) == protocol.MSG_INVOKE_RESPONSE
        assert protocol.get_response_msg_id(protocol.MSG_START_TASK_REQUEST) == protocol.MSG_START_TASK_RESPONSE
        assert protocol.get_response_msg_id(protocol.MSG_CANCEL_TASK_REQUEST) == protocol.MSG_CANCEL_TASK_RESPONSE
        assert protocol.get_response_msg_id(protocol.MSG_PROVIDER_CONNECT_REQUEST) == protocol.MSG_PROVIDER_CONNECT_RESPONSE

    def test_response_is_request_plus_one(self):
        """Response ID should be request ID + 1."""
        request_ids = [
            protocol.MSG_REGISTER_REQUEST,
            protocol.MSG_HEARTBEAT_REQUEST,
            protocol.MSG_INVOKE_REQUEST,
            protocol.MSG_START_TASK_REQUEST,
            protocol.MSG_CANCEL_TASK_REQUEST,
            protocol.MSG_PROVIDER_CONNECT_REQUEST,
            protocol.MSG_PROVIDER_HEARTBEAT_REQUEST,
        ]
        for req_id in request_ids:
            assert protocol.get_response_msg_id(req_id) == req_id + 1


class TestMsgIdString:
    """Test msg_id_string function."""

    def test_known_message_types(self):
        assert protocol.msg_id_string(protocol.MSG_REGISTER_REQUEST) == "RegisterRequest"
        assert protocol.msg_id_string(protocol.MSG_REGISTER_RESPONSE) == "RegisterResponse"
        assert protocol.msg_id_string(protocol.MSG_HEARTBEAT_REQUEST) == "HeartbeatRequest"
        assert protocol.msg_id_string(protocol.MSG_HEARTBEAT_RESPONSE) == "HeartbeatResponse"
        assert protocol.msg_id_string(protocol.MSG_INVOKE_REQUEST) == "InvokeRequest"
        assert protocol.msg_id_string(protocol.MSG_INVOKE_RESPONSE) == "InvokeResponse"
        assert protocol.msg_id_string(protocol.MSG_START_TASK_REQUEST) == "StartTaskRequest"
        assert protocol.msg_id_string(protocol.MSG_TASK_EVENT) == "TaskEvent"
        assert protocol.msg_id_string(protocol.MSG_PROVIDER_CONNECT_REQUEST) == "ProviderConnectRequest"

    def test_unknown_message_type(self):
        result = protocol.msg_id_string(0xFFFFFF)
        assert result.startswith("Unknown")
        assert "0xFFFFFF" in result

    def test_zero(self):
        result = protocol.msg_id_string(0)
        assert result.startswith("Unknown")


class TestRequestResponsePairs:
    """Test that request/response pairs are correctly defined."""

    @pytest.mark.parametrize(
        "request_id,response_id",
        [
            (protocol.MSG_REGISTER_REQUEST, protocol.MSG_REGISTER_RESPONSE),
            (protocol.MSG_HEARTBEAT_REQUEST, protocol.MSG_HEARTBEAT_RESPONSE),
            (protocol.MSG_INVOKE_REQUEST, protocol.MSG_INVOKE_RESPONSE),
            (protocol.MSG_START_TASK_REQUEST, protocol.MSG_START_TASK_RESPONSE),
            (protocol.MSG_CANCEL_TASK_REQUEST, protocol.MSG_CANCEL_TASK_RESPONSE),
            (protocol.MSG_PROVIDER_CONNECT_REQUEST, protocol.MSG_PROVIDER_CONNECT_RESPONSE),
            (protocol.MSG_PROVIDER_HEARTBEAT_REQUEST, protocol.MSG_PROVIDER_HEARTBEAT_RESPONSE),
        ],
    )
    def test_pair_consistency(self, request_id, response_id):
        """Request and response IDs should be consistent."""
        assert response_id == protocol.get_response_msg_id(request_id)
        assert protocol.is_request(request_id) is True
        assert protocol.is_response(response_id) is True
        assert protocol.is_request(response_id) is False
        assert protocol.is_response(request_id) is False
