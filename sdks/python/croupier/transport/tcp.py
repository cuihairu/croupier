"""
TCP Transport Layer for Croupier SDK

Implements bidirectional multiplexed TCP transport for communication
with Croupier Agent using a single TCP connection.

Wire Protocol:
  Frame:   [4-byte length prefix (big-endian)][payload]
  Payload: [8-byte header][protobuf body]
  Header:  Version(1B) + MsgID(3B) + RequestID(4B)

Request messages have odd MsgID, Response messages have even MsgID.
Multiple concurrent request/response pairs multiplex on the same connection.
"""

from __future__ import annotations

import logging
import socket
import ssl
import struct
import threading
from typing import Callable, Optional, Tuple

from .. import protocol

LOG = logging.getLogger(__name__)

# Frame constants
_FRAME_HEADER_BYTES = 4  # 4-byte big-endian length prefix
_MAX_FRAME_BYTES = 32 * 1024 * 1024  # 32 MB


def _read_exact(sock: socket.socket, n: int) -> bytes:
    """Read exactly n bytes from socket."""
    data = bytearray()
    while len(data) < n:
        chunk = sock.recv(n - len(data))
        if not chunk:
            raise ConnectionError("connection closed")
        data.extend(chunk)
    return bytes(data)


def _write_frame(sock: socket.socket, payload: bytes) -> None:
    """Write a length-prefixed frame to socket."""
    if len(payload) > _MAX_FRAME_BYTES:
        raise ValueError(f"frame too large: {len(payload)} > {_MAX_FRAME_BYTES}")
    header = struct.pack(">I", len(payload))
    sock.sendall(header + payload)


def _read_frame(sock: socket.socket) -> bytes:
    """Read a length-prefixed frame from socket."""
    header = _read_exact(sock, _FRAME_HEADER_BYTES)
    size = struct.unpack(">I", header)[0]
    if size == 0:
        return b""
    if size > _MAX_FRAME_BYTES:
        raise ValueError(f"frame too large: {size} > {_MAX_FRAME_BYTES}")
    return _read_exact(sock, size)


class _PendingCall:
    """Tracks a pending request waiting for its response."""

    __slots__ = ("event", "resp_msg_id", "resp_body", "error")

    def __init__(self) -> None:
        self.event = threading.Event()
        self.resp_msg_id: int = 0
        self.resp_body: bytes = b""
        self.error: Optional[Exception] = None

    def wait(self, timeout: Optional[float] = None) -> bool:
        return self.event.wait(timeout=timeout)

    def deliver(self, msg_id: int, body: bytes) -> None:
        self.resp_msg_id = msg_id
        self.resp_body = body
        self.event.set()

    def fail(self, err: Exception) -> None:
        self.error = err
        self.event.set()


class TCPTransport:
    """
    Bidirectional multiplexed TCP transport.

    A single TCP connection supports:
    - **Outbound requests** via :meth:`call` (send request, wait for response)
    - **Inbound requests** via a handler callback (agent pushes invoke / job)

    Usage::

        transport = TCPTransport(address="127.0.0.1:19091", timeout_ms=30000)
        transport.set_handler(my_handler)
        transport.connect()

        # Send a request-response
        resp_type, resp_data = transport.call(MSG_INVOKE_REQUEST, proto_bytes)

        transport.close()
    """

    def __init__(
        self,
        address: str = "127.0.0.1:19091",
        timeout_ms: int = 30000,
        *,
        tls_enabled: bool = False,
        tls_cert_file: str = "",
        tls_key_file: str = "",
        tls_ca_file: str = "",
        tls_server_name: str = "",
        tls_insecure_skip_verify: bool = False,
    ):
        self.address = address
        self.timeout_ms = timeout_ms
        self._tls_enabled = tls_enabled
        self._tls_cert_file = tls_cert_file
        self._tls_key_file = tls_key_file
        self._tls_ca_file = tls_ca_file
        self._tls_server_name = tls_server_name
        self._tls_insecure_skip_verify = tls_insecure_skip_verify

        self._sock: Optional[socket.socket] = None
        self._connected = False

        self._request_id = 0
        self._write_lock = threading.Lock()

        # pending request_id -> _PendingCall
        self._pending: dict[int, _PendingCall] = {}
        self._pending_lock = threading.Lock()

        self._reader_thread: Optional[threading.Thread] = None
        self._running = False

        # inbound request handler: (msg_id, req_id, body) -> response_body
        self._handler: Optional[Callable[[int, int, bytes], bytes]] = None

    # ---- public API ----

    def set_handler(self, handler: Callable[[int, int, bytes], bytes]) -> None:
        """Set handler for inbound requests from the remote peer."""
        self._handler = handler

    def connect(self) -> None:
        """Dial the remote endpoint and start the background reader."""
        if self._connected:
            return

        addr = self._strip_scheme(self.address)
        host, port_str = addr.rsplit(":", 1)
        port = int(port_str)

        raw = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        raw.settimeout(self.timeout_ms / 1000.0)

        try:
            raw.connect((host, port))
        except Exception:
            raw.close()
            raise

        if self._tls_enabled:
            raw = self._wrap_tls(raw)

        self._sock = raw
        self._connected = True
        self._running = True

        self._reader_thread = threading.Thread(
            target=self._reader_loop,
            name="croupier-tcp-reader",
            daemon=True,
        )
        self._reader_thread.start()
        LOG.info("TCP transport connected to %s", self.address)

    def close(self) -> None:
        """Close the connection and release resources."""
        if not self._connected and not self._running:
            return

        self._running = False
        self._connected = False

        if self._sock:
            try:
                self._sock.close()
            except Exception:
                pass
            self._sock = None

        # wake up any pending calls
        with self._pending_lock:
            for p in self._pending.values():
                p.fail(ConnectionError("connection closed"))
            self._pending.clear()

        if self._reader_thread:
            self._reader_thread.join(timeout=2)
            self._reader_thread = None

        LOG.info("TCP transport closed")

    def is_connected(self) -> bool:
        return self._connected

    def call(self, msg_type: int, data: bytes) -> Tuple[int, bytes]:
        """
        Send a request and block until the matching response arrives.

        Returns:
            (response_msg_type, response_body)

        Raises:
            RuntimeError: not connected
            TimeoutError: response not received within timeout
            ConnectionError: connection lost while waiting
        """
        if not self._connected or self._sock is None:
            raise RuntimeError("Not connected")

        self._request_id = (self._request_id + 1) & 0xFFFFFFFF
        req_id = self._request_id

        pending = _PendingCall()
        with self._pending_lock:
            self._pending[req_id] = pending

        try:
            message = protocol.new_message(msg_type, req_id, data)
            with self._write_lock:
                _write_frame(self._sock, message)

            LOG.debug("Sent msg_type=0x%06X req_id=%d (%d bytes)", msg_type, req_id, len(data))

            timeout = self.timeout_ms / 1000.0
            if not pending.wait(timeout=timeout):
                raise TimeoutError(f"request timed out after {self.timeout_ms}ms")

            if pending.error is not None:
                raise pending.error

            expected = protocol.get_response_msg_id(msg_type)
            if pending.resp_msg_id != expected:
                raise RuntimeError(
                    f"unexpected response: expected 0x{expected:06X}, "
                    f"got 0x{pending.resp_msg_id:06X}"
                )

            return pending.resp_msg_id, pending.resp_body
        finally:
            with self._pending_lock:
                self._pending.pop(req_id, None)

    def send_response(self, msg_type: int, req_id: int, data: bytes) -> None:
        """Send a response frame (used by inbound-request handler)."""
        if not self._connected or self._sock is None:
            return
        message = protocol.new_message(msg_type, req_id, data)
        with self._write_lock:
            _write_frame(self._sock, message)

    # ---- context manager ----

    def __enter__(self) -> "TCPTransport":
        self.connect()
        return self

    def __exit__(self, exc_type, exc_val, exc_tb) -> None:
        self.close()

    # ---- internals ----

    def _reader_loop(self) -> None:
        """Background thread: read frames, dispatch responses and inbound requests."""
        while self._running:
            try:
                if self._sock is None:
                    break

                self._sock.settimeout(1.0)
                try:
                    frame = _read_frame(self._sock)
                except (TimeoutError, socket.timeout):
                    continue

                if len(frame) < protocol.HEADER_SIZE:
                    continue

                _, msg_id, req_id, body = protocol.parse_message(frame)

                if protocol.is_response(msg_id):
                    with self._pending_lock:
                        p = self._pending.get(req_id)
                    if p is not None:
                        p.deliver(msg_id, body)
                    else:
                        LOG.debug("Discarding response for unknown req_id=%d", req_id)
                else:
                    self._handle_inbound(msg_id, req_id, body)

            except (ConnectionError, OSError):
                if self._running:
                    LOG.warning("Connection lost")
                    self._connected = False
                    # fail all pending so callers don't hang
                    with self._pending_lock:
                        for p in self._pending.values():
                            p.fail(ConnectionError("connection lost"))
                        self._pending.clear()
                break
            except Exception as exc:
                if self._running:
                    LOG.error("Reader error: %s", exc)
                break

    def _handle_inbound(self, msg_id: int, req_id: int, body: bytes) -> None:
        if self._handler is None:
            LOG.warning("No handler for inbound %s", protocol.msg_id_string(msg_id))
            return
        try:
            resp_body = self._handler(msg_id, req_id, body)
            resp_msg_id = protocol.get_response_msg_id(msg_id)
            self.send_response(resp_msg_id, req_id, resp_body)
        except Exception as exc:
            LOG.error("Handler error for %s: %s", protocol.msg_id_string(msg_id), exc)

    @staticmethod
    def _strip_scheme(address: str) -> str:
        if "://" in address:
            return address.split("://", 1)[1]
        return address

    def _wrap_tls(self, raw: socket.socket) -> ssl.SSLSocket:
        ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
        if self._tls_insecure_skip_verify:
            ctx.check_hostname = False
            ctx.verify_mode = ssl.CERT_NONE
        else:
            ctx.check_hostname = bool(self._tls_server_name)
            if self._tls_ca_file:
                ctx.load_verify_locations(self._tls_ca_file)
            else:
                ctx.load_default_certs()
        if self._tls_cert_file and self._tls_key_file:
            ctx.load_cert_chain(self._tls_cert_file, self._tls_key_file)
        server_name = self._tls_server_name or None
        return ctx.wrap_socket(raw, server_hostname=server_name)
