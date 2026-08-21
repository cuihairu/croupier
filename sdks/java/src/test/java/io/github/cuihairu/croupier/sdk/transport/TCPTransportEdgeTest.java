package io.github.cuihairu.croupier.sdk.transport;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Timeout;

import java.io.DataInputStream;
import java.io.DataOutputStream;
import java.io.IOException;
import java.net.InetAddress;
import java.net.ServerSocket;
import java.net.Socket;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.util.concurrent.atomic.AtomicReference;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Frame-level edge case tests for TCPTransport against a scripted ServerSocket.
 */
@DisplayName("TCPTransport frame edge cases")
class TCPTransportEdgeTest {

    private interface ServerScript {
        void run(Socket client, DataInputStream in, DataOutputStream out) throws Exception;
    }

    private static final class ScriptServer implements AutoCloseable {
        private final ServerSocket serverSocket;
        private final AtomicReference<Throwable> failure = new AtomicReference<>();

        ScriptServer(ServerScript script) throws IOException {
            serverSocket = new ServerSocket(0, 1, InetAddress.getLoopbackAddress());
            Thread acceptor = new Thread(() -> {
                try (Socket client = serverSocket.accept()) {
                    client.setSoTimeout(5000);
                    script.run(client, new DataInputStream(client.getInputStream()),
                        new DataOutputStream(client.getOutputStream()));
                } catch (Exception e) {
                    failure.set(e);
                }
            }, "script-server");
            acceptor.setDaemon(true);
            acceptor.start();
        }

        int port() {
            return serverSocket.getLocalPort();
        }

        @Override
        public void close() throws IOException {
            serverSocket.close();
        }
    }

    /** Reads one request frame and returns its reqId. */
    private static int readRequest(DataInputStream in) throws IOException {
        int frameSize = in.readInt();
        byte[] payload = new byte[frameSize];
        in.readFully(payload);
        return ByteBuffer.wrap(payload, 4, 4).order(ByteOrder.BIG_ENDIAN).getInt();
    }

    private static void writeFrame(DataOutputStream out, int version, int msgId, int reqId, byte[] body) throws IOException {
        out.writeInt(8 + body.length);
        byte[] header = new byte[8];
        header[0] = (byte) version;
        header[1] = (byte) (msgId >> 16);
        header[2] = (byte) (msgId >> 8);
        header[3] = (byte) msgId;
        ByteBuffer.wrap(header, 4, 4).order(ByteOrder.BIG_ENDIAN).putInt(reqId);
        out.write(header);
        out.write(body);
        out.flush();
    }

    @Test
    @DisplayName("malformed frames are skipped and the following valid response resolves the request")
    @Timeout(10)
    void skipsBadFramesThenResolves() throws Exception {
        try (ScriptServer server = new ScriptServer((client, in, out) -> {
            int reqId = readRequest(in);
            // Unknown protocol version → frame ignored.
            writeFrame(out, 0x02, 0x030106, reqId, "junk".getBytes());
            // Payload shorter than the protocol header → frame ignored.
            out.writeInt(4);
            out.write(new byte[]{9, 9, 9, 9});
            out.flush();
            // Response for an unknown reqId → dropped by the read loop.
            writeFrame(out, 0x01, 0x030106, 424242, "orphan".getBytes());
            // Real response.
            writeFrame(out, 0x01, 0x030106, reqId, "ok".getBytes());
        })) {
            TCPTransport transport = new TCPTransport("127.0.0.1", server.port(), 5000);
            transport.connect();
            byte[] response = transport.request(0x030101, "ping".getBytes());
            assertEquals("ok", new String(response));
            transport.close();
        }
    }

    @Test
    @DisplayName("zero-length frames terminate the read loop and the connection")
    @Timeout(10)
    void zeroFrameBreaksReadLoop() throws Exception {
        try (ScriptServer server = new ScriptServer((client, in, out) -> {
            // Note: protobuf-java treats a zero tag as invalid, so a 0-length frame
            // is the only way to hit the transport's zero-size guard here.
            out.writeInt(0);
            out.flush();
            Thread.sleep(2000);
        })) {
            TCPTransport transport = new TCPTransport("127.0.0.1", server.port(), 800);
            transport.connect();
            long deadline = System.currentTimeMillis() + 3000;
            while (transport.isConnected() && System.currentTimeMillis() < deadline) {
                Thread.sleep(20);
            }
            assertFalse(transport.isConnected(), "transport should close after zero-size frame");
            assertThrows(RuntimeException.class, () -> transport.request(0x030101, "x".getBytes()));
        }
    }

    @Test
    @DisplayName("oversized frames terminate the read loop")
    @Timeout(10)
    void oversizedFrameBreaksReadLoop() throws Exception {
        try (ScriptServer server = new ScriptServer((client, in, out) -> {
            out.writeInt(33 * 1024 * 1024);
            out.flush();
            Thread.sleep(2000);
        })) {
            TCPTransport transport = new TCPTransport("127.0.0.1", server.port(), 800);
            transport.connect();
            long deadline = System.currentTimeMillis() + 3000;
            while (transport.isConnected() && System.currentTimeMillis() < deadline) {
                Thread.sleep(20);
            }
            assertFalse(transport.isConnected(), "transport should close after oversized frame");
        }
    }

    @Test
    @DisplayName("a stalled frame payload breaks the read loop")
    @Timeout(10)
    void stalledPayloadBreaksReadLoop() throws Exception {
        try (ScriptServer server = new ScriptServer((client, in, out) -> {
            out.writeInt(100);
            out.write(new byte[10]);
            out.flush();
            Thread.sleep(3000);
        })) {
            TCPTransport transport = new TCPTransport("127.0.0.1", server.port(), 800);
            transport.connect();
            long deadline = System.currentTimeMillis() + 3000;
            while (transport.isConnected() && System.currentTimeMillis() < deadline) {
                Thread.sleep(20);
            }
            assertFalse(transport.isConnected(), "transport should close after payload read timeout");
        }
    }

    @Test
    @DisplayName("a truncated frame payload breaks the read loop")
    @Timeout(10)
    void truncatedPayloadBreaksReadLoop() throws Exception {
        try (ScriptServer server = new ScriptServer((client, in, out) -> {
            out.writeInt(100);
            out.write(new byte[10]);
            out.flush();
            client.close();
        })) {
            TCPTransport transport = new TCPTransport("127.0.0.1", server.port(), 800);
            transport.connect();
            long deadline = System.currentTimeMillis() + 3000;
            while (transport.isConnected() && System.currentTimeMillis() < deadline) {
                Thread.sleep(20);
            }
            assertFalse(transport.isConnected(), "transport should close after short payload");
        }
    }

    @Test
    @DisplayName("an idle connection does not kill the read loop")
    @Timeout(10)
    void survivesIdleReadTimeout() throws Exception {
        try (ScriptServer server = new ScriptServer((client, in, out) -> {
            int reqId = readRequest(in);
            // Longer than the read loop's 1s socket timeout; the loop must retry.
            Thread.sleep(1500);
            writeFrame(out, 0x01, 0x030106, reqId, "late".getBytes());
        })) {
            TCPTransport transport = new TCPTransport("127.0.0.1", server.port(), 5000);
            transport.connect();
            byte[] response = transport.request(0x030101, "slow".getBytes());
            assertEquals("late", new String(response));
            transport.close();
        }
    }

    @Test
    @DisplayName("interrupting a waiting request reports the interruption")
    @Timeout(10)
    void interruptedRequest() throws Exception {
        try (ScriptServer server = new ScriptServer((client, in, out) -> {
            readRequest(in);
            Thread.sleep(3000);
        })) {
            TCPTransport transport = new TCPTransport("127.0.0.1", server.port(), 8000);
            transport.connect();

            AtomicReference<Throwable> captured = new AtomicReference<>();
            Thread requester = new Thread(() -> {
                try {
                    transport.request(0x030101, "x".getBytes());
                } catch (Throwable error) {
                    captured.set(error);
                }
            });
            requester.start();
            Thread.sleep(200);
            requester.interrupt();
            requester.join(3000);

            Throwable error = captured.get();
            assertInstanceOf(RuntimeException.class, error);
            assertEquals("Request interrupted", error.getMessage());
            assertTrue(requester.isInterrupted(), "interrupt flag should be restored");
            transport.close();
        }
    }

    @Test
    @DisplayName("connection reset propagates to the read loop and closes the transport")
    @Timeout(10)
    void connectionResetClosesTransport() throws Exception {
        try (ScriptServer server = new ScriptServer((client, in, out) -> {
            readRequest(in);
            client.setSoLinger(true, 0);
            client.close();
        })) {
            TCPTransport transport = new TCPTransport("127.0.0.1", server.port(), 500);
            transport.connect();
            assertThrows(RuntimeException.class, () -> transport.request(0x030101, "x".getBytes()));
            long deadline = System.currentTimeMillis() + 3000;
            while (transport.isConnected() && System.currentTimeMillis() < deadline) {
                Thread.sleep(20);
            }
            assertFalse(transport.isConnected());
        }
    }

    @Test
    @DisplayName("close preserves the caller's interrupt flag when joining the read loop")
    @Timeout(10)
    void closeWithInterruptedCaller() throws Exception {
        try (ScriptServer server = new ScriptServer((client, in, out) -> {
            readRequest(in);
            Thread.sleep(5000);
        })) {
            TCPTransport transport = new TCPTransport("127.0.0.1", server.port(), 5000);
            transport.connect();
            Thread.currentThread().interrupt();
            transport.close();
            assertTrue(Thread.interrupted(), "interrupt flag should be preserved by close()");
        } finally {
            Thread.interrupted();
        }
    }
}
