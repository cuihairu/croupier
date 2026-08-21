"""
Shared test helpers: an in-process fake agent / echo peer speaking the
croupier wire protocol ([4-byte BE length][8-byte header][body]).

Used by TCP transport and client lifecycle tests. Not collected by pytest.
"""

from __future__ import annotations

import queue
import socket
import ssl
import struct
import threading

from croupier import protocol
from croupier import provider_pb2

MAX_SERVER_FRAME = 1024 * 1024  # guard for garbage input (e.g. TLS ClientHello)


def read_exact(sock: socket.socket, n: int, timeout: float = 5.0) -> bytes:
    sock.settimeout(timeout)
    data = bytearray()
    while len(data) < n:
        chunk = sock.recv(n - len(data))
        if not chunk:
            raise ConnectionError("connection closed")
        data.extend(chunk)
    return bytes(data)


def read_frame(sock: socket.socket, timeout: float = 5.0) -> bytes:
    header = read_exact(sock, 4, timeout)
    size = struct.unpack(">I", header)[0]
    if size == 0:
        return b""
    if size > MAX_SERVER_FRAME:
        raise ValueError(f"server refusing huge frame: {size}")
    return read_exact(sock, size, timeout)


def write_frame(sock: socket.socket, payload: bytes) -> None:
    sock.settimeout(5.0)
    sock.sendall(struct.pack(">I", len(payload)) + payload)


class FakePeer:
    """Minimal wire-protocol peer.

    - Accepts multiple sequential client connections (one serve thread each).
    - ``handle_message`` decides the reply for inbound requests; the default
      replies with ``msg_id + 1`` and an empty body (pure echo peer).
    - Non-request frames coming from the client (i.e. responses to frames we
      pushed) are placed on the ``responses`` queue for assertions.
    - All inbound requests are recorded on ``requests`` as
      ``(msg_id, req_id, body)`` tuples.
    """

    def __init__(self, port: int = 0, *, tls: bool = False, cert_file: str = "", key_file: str = ""):
        self.tls = tls
        self._cert_file = cert_file
        self._key_file = key_file
        # Response msg id offset applied to auto-replies (1 is the protocol
        # default; tests may override to simulate mismatched responses).
        self.reply_offset = 1
        self.requests: list[tuple[int, int, bytes]] = []
        self.responses: "queue.Queue[tuple[int, int, bytes]]" = queue.Queue()
        self._lock = threading.Lock()
        self._conns: list[socket.socket] = []
        self._stop = threading.Event()

        self._srv = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        self._srv.bind(("127.0.0.1", port))
        self._srv.listen(8)
        self._srv.settimeout(0.2)
        self.port = self._srv.getsockname()[1]
        self._accept_thread = threading.Thread(target=self._accept_loop, daemon=True)
        self._accept_thread.start()

    # ---- lifecycle ----

    def addr(self) -> str:
        return f"127.0.0.1:{self.port}"

    def stop(self) -> None:
        self._stop.set()
        self.close_connections()
        try:
            self._srv.close()
        except OSError:
            pass
        self._accept_thread.join(timeout=2)

    def close_connections(self) -> None:
        """Drop all accepted connections (simulates agent restart / net cut)."""
        with self._lock:
            conns, self._conns = list(self._conns), []
        for conn in conns:
            try:
                conn.close()
            except OSError:
                pass

    # ---- introspection ----

    def wait_requests(self, count: int, timeout: float = 5.0) -> list[tuple[int, int, bytes]]:
        """Wait until at least ``count`` inbound requests were recorded."""
        deadline = _monotonic() + timeout
        while _monotonic() < deadline:
            with self._lock:
                if len(self.requests) >= count:
                    return list(self.requests)
            _sleep(0.02)
        with self._lock:
            return list(self.requests)

    def wait_response(self, req_id: int, timeout: float = 5.0) -> tuple[int, bytes] | None:
        """Wait for the client's response frame matching ``req_id``."""
        deadline = _monotonic() + timeout
        while _monotonic() < deadline:
            try:
                msg_id, rid, body = self.responses.get(timeout=0.1)
            except queue.Empty:
                continue
            if rid == req_id:
                return msg_id, body
        return None

    # ---- pushing data to the client ----

    def _latest_conn(self, timeout: float = 2.0) -> socket.socket:
        """Wait for the first accepted client connection and return it."""
        deadline = _monotonic() + timeout
        while _monotonic() < deadline:
            with self._lock:
                if self._conns:
                    return self._conns[-1]
            _sleep(0.01)
        with self._lock:
            if self._conns:
                return self._conns[-1]
        raise ConnectionError("no client connection")

    def push(self, msg_id: int, req_id: int, body: bytes) -> None:
        write_frame(self._latest_conn(), protocol.new_message(msg_id, req_id, body))

    def push_raw(self, payload: bytes) -> None:
        write_frame(self._latest_conn(), payload)

    # ---- request handling (override point) ----

    def handle_message(self, msg_id: int, req_id: int, body: bytes) -> bytes | None:
        """Return reply body for a request, or None to stay silent."""
        if protocol.is_request(msg_id):
            return b""
        return None

    # ---- internals ----

    def _accept_loop(self) -> None:
        while not self._stop.is_set():
            try:
                conn, _ = self._srv.accept()
            except socket.timeout:
                continue
            except OSError:
                break
            try:
                if self.tls:
                    ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
                    ctx.load_cert_chain(self._cert_file, self._key_file)
                    conn = ctx.wrap_socket(conn, server_side=True)
            except (ssl.SSLError, OSError):
                try:
                    conn.close()
                except OSError:
                    pass
                continue
            with self._lock:
                self._conns.append(conn)
            threading.Thread(target=self._serve, args=(conn,), daemon=True).start()

    def _serve(self, conn: socket.socket) -> None:
        while not self._stop.is_set():
            try:
                frame = read_frame(conn, timeout=0.2)
            except (TimeoutError, socket.timeout):
                continue
            except (ConnectionError, OSError, ValueError):
                break
            if len(frame) < protocol.HEADER_SIZE:
                continue
            _, msg_id, req_id, body = protocol.parse_message(frame)

            if protocol.is_request(msg_id):
                with self._lock:
                    self.requests.append((msg_id, req_id, body))
                reply = self.handle_message(msg_id, req_id, body)
                if reply is not None:
                    try:
                        write_frame(conn, protocol.new_message(msg_id + self.reply_offset, req_id, reply))
                    except (ConnectionError, OSError):
                        break
            else:
                self.responses.put((msg_id, req_id, body))


class FakeAgent(FakePeer):
    """FakePeer with protobuf-aware replies for the provider subprotocol."""

    def __init__(self, session_id: str = "sess-fake-42", port: int = 0):
        self.session_id = session_id
        self.connect_requests: list[provider_pb2.ProviderConnectRequest] = []
        self.heartbeats: list[provider_pb2.ProviderHeartbeatRequest] = []
        self.drain_completes: list[bytes] = []
        super().__init__(port=port)

    def handle_message(self, msg_id: int, req_id: int, body: bytes) -> bytes | None:
        if msg_id == protocol.MSG_PROVIDER_CONNECT_REQUEST:
            req = provider_pb2.ProviderConnectRequest()
            req.ParseFromString(body)
            with self._lock:
                self.connect_requests.append(req)
            resp = provider_pb2.ProviderConnectResponse(session_id=self.session_id)
            return resp.SerializeToString()
        if msg_id == protocol.MSG_PROVIDER_HEARTBEAT_REQUEST:
            req = provider_pb2.ProviderHeartbeatRequest()
            req.ParseFromString(body)
            with self._lock:
                self.heartbeats.append(req)
            return provider_pb2.ProviderHeartbeatResponse().SerializeToString()
        if msg_id == protocol.MSG_PROVIDER_DRAIN_COMPLETE_REQUEST:
            with self._lock:
                self.drain_completes.append(body)
            return b""
        return super().handle_message(msg_id, req_id, body)

    def wait_connects(self, count: int, timeout: float = 5.0) -> list[provider_pb2.ProviderConnectRequest]:
        deadline = _monotonic() + timeout
        while _monotonic() < deadline:
            with self._lock:
                if len(self.connect_requests) >= count:
                    return list(self.connect_requests)
            _sleep(0.02)
        with self._lock:
            return list(self.connect_requests)

    def wait_heartbeats(self, count: int, timeout: float = 5.0) -> list[provider_pb2.ProviderHeartbeatRequest]:
        deadline = _monotonic() + timeout
        while _monotonic() < deadline:
            with self._lock:
                if len(self.heartbeats) >= count:
                    return list(self.heartbeats)
            _sleep(0.02)
        with self._lock:
            return list(self.heartbeats)

    def wait_drain_completes(self, count: int, timeout: float = 5.0) -> int:
        deadline = _monotonic() + timeout
        while _monotonic() < deadline:
            with self._lock:
                if len(self.drain_completes) >= count:
                    return len(self.drain_completes)
            _sleep(0.02)
        with self._lock:
            return len(self.drain_completes)


def _monotonic() -> float:
    import time

    return time.monotonic()


def _sleep(seconds: float) -> None:
    import time

    time.sleep(seconds)


def free_port() -> int:
    """Reserve a port then release it (for connection-refused tests)."""
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.bind(("127.0.0.1", 0))
    port = sock.getsockname()[1]
    sock.close()
    return port


def generate_tls_certs(directory):
    """Generate a CA, a server cert for localhost, and a client cert.

    Returns dict with paths: ca_cert, server_cert, server_key, client_cert, client_key.
    Requires the ``openssl`` binary (present in CI/dev environments).
    """
    import subprocess

    d = directory

    def run(*args):
        subprocess.run(
            ["openssl", *args],
            check=True,
            capture_output=True,
            cwd=str(d),
        )

    run(
        "req", "-x509", "-newkey", "rsa:2048", "-keyout", "ca.key", "-out", "ca.crt",
        "-days", "2", "-nodes", "-subj", "/CN=croupier-test-ca",
    )
    run(
        "req", "-newkey", "rsa:2048", "-keyout", "server.key", "-out", "server.csr",
        "-nodes", "-subj", "/CN=localhost",
    )
    (d / "server.ext").write_text("subjectAltName=DNS:localhost\n")
    run(
        "x509", "-req", "-in", "server.csr", "-CA", "ca.crt", "-CAkey", "ca.key",
        "-CAcreateserial", "-out", "server.crt", "-days", "2",
        "-extfile", "server.ext",
    )
    run(
        "req", "-newkey", "rsa:2048", "-keyout", "client.key", "-out", "client.csr",
        "-nodes", "-subj", "/CN=croupier-test-client",
    )
    run(
        "x509", "-req", "-in", "client.csr", "-CA", "ca.crt", "-CAkey", "ca.key",
        "-CAcreateserial", "-out", "client.crt", "-days", "2",
    )
    return {
        "ca_cert": str(d / "ca.crt"),
        "server_cert": str(d / "server.crt"),
        "server_key": str(d / "server.key"),
        "client_cert": str(d / "client.crt"),
        "client_key": str(d / "client.key"),
    }
