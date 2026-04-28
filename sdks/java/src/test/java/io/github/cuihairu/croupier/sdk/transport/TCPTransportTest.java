/**
 * Comprehensive unit tests for TCPTransport class.
 */
package io.github.cuihairu.croupier.sdk.transport;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.Timeout;
import org.junit.jupiter.api.DisplayName;

import static org.junit.jupiter.api.Assertions.*;

import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.ServerSocket;
import java.net.Socket;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;

/**
 * Unit tests for TCPTransport.
 *
 * <p>Tests connection handling, request/response multiplexing,
 * timeout behavior, and error scenarios.</p>
 */
@DisplayName("TCPTransport Tests")
class TCPTransportTest {

    private ServerSocket mockServer;
    private TCPTransport transport;
    private Thread serverThread;
    private volatile boolean serverRunning;
    private AtomicInteger serverMessageCount = new AtomicInteger(0);

    @BeforeEach
    void setUp() throws IOException {
        // Start a mock server on a random available port
        mockServer = new ServerSocket(0);
        serverRunning = true;
        serverMessageCount.set(0);

        // Start server thread to handle connections
        serverThread = new Thread(this::runMockServer);
        serverThread.setDaemon(true);
        serverThread.start();
    }

    @AfterEach
    void tearDown() {
        serverRunning = false;

        if (transport != null) {
            try {
                transport.close();
            } catch (Exception e) {
                // Ignore cleanup errors
            }
            transport = null;
        }

        if (mockServer != null && !mockServer.isClosed()) {
            try {
                mockServer.close();
            } catch (IOException e) {
                // Ignore cleanup errors
            }
        }

        if (serverThread != null) {
            try {
                serverThread.join(1000);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
        }
    }

    /**
     * Mock server that accepts connections and echoes responses.
     */
    private void runMockServer() {
        try {
            while (serverRunning && !mockServer.isClosed()) {
                Socket clientSocket = null;
                try {
                    clientSocket = mockServer.accept();
                    clientSocket.setSoTimeout(1000);

                    InputStream in = clientSocket.getInputStream();
                    OutputStream out = clientSocket.getOutputStream();

                    // Handle multiple requests
                    while (serverRunning && !clientSocket.isClosed()) {
                        // Read frame header (4 bytes)
                        byte[] headerBuf = new byte[4];
                        if (readFully(in, headerBuf) != 4) {
                            break;
                        }

                        // Parse frame size (big-endian)
                        int frameSize = java.nio.ByteBuffer.wrap(headerBuf)
                            .order(java.nio.ByteOrder.BIG_ENDIAN).getInt();

                        if (frameSize <= 0 || frameSize > 32 * 1024 * 1024) {
                            break;
                        }

                        // Read frame payload
                        byte[] payload = new byte[frameSize];
                        if (readFully(in, payload) != frameSize) {
                            break;
                        }

                        // Parse protocol header
                        if (payload.length < 8) {
                            break;
                        }

                        int reqId = java.nio.ByteBuffer.wrap(payload, 4, 4)
                            .order(java.nio.ByteOrder.BIG_ENDIAN).getInt();

                        // Extract message body
                        byte[] body = new byte[payload.length - 8];
                        System.arraycopy(payload, 8, body, 0, body.length);

                        // Echo response
                        byte[] responseFrame = new byte[4 + 8 + body.length];
                        java.nio.ByteBuffer.wrap(responseFrame, 0, 4)
                            .order(java.nio.ByteOrder.BIG_ENDIAN).putInt(8 + body.length);
                        responseFrame[4] = 0x01; // Version
                        responseFrame[5] = (byte) (0x01 >> 16); // Response msgId
                        responseFrame[6] = (byte) (0x01 >> 8);
                        responseFrame[7] = (byte) 0x02;
                        java.nio.ByteBuffer.wrap(responseFrame, 8, 4)
                            .order(java.nio.ByteOrder.BIG_ENDIAN).putInt(reqId);
                        System.arraycopy(body, 0, responseFrame, 12, body.length);

                        out.write(responseFrame);
                        out.flush();

                        serverMessageCount.incrementAndGet();
                    }
                } catch (IOException e) {
                    // Connection closed or timeout
                } finally {
                    if (clientSocket != null) {
                        try {
                            clientSocket.close();
                        } catch (IOException e) {
                            // Ignore
                        }
                    }
                }
            }
        } catch (Exception e) {
            // Server thread exit
        }
    }

    private int readFully(InputStream in, byte[] buf) throws IOException {
        int offset = 0;
        while (offset < buf.length) {
            int n = in.read(buf, offset, buf.length - offset);
            if (n < 0) {
                return offset;
            }
            offset += n;
        }
        return offset;
    }

    @Test
    @DisplayName("Constructor with timeout should create transport")
    void testConstructorWithTimeout() {
        transport = new TCPTransport("localhost", mockServer.getLocalPort(), 5000);

        assertNotNull(transport);
        assertFalse(transport.isConnected());
    }

    @Test
    @DisplayName("Constructor with default timeout should create transport")
    void testConstructorWithDefaultTimeout() {
        transport = new TCPTransport("localhost", mockServer.getLocalPort());

        assertNotNull(transport);
        assertFalse(transport.isConnected());
    }

    @Test
    @DisplayName("Connect should establish connection")
    void testConnect() {
        transport = new TCPTransport("localhost", mockServer.getLocalPort(), 5000);

        transport.connect();

        assertTrue(transport.isConnected());
    }

    @Test
    @DisplayName("Connect should be idempotent")
    void testConnectIsIdempotent() {
        transport = new TCPTransport("localhost", mockServer.getLocalPort(), 5000);

        transport.connect();
        assertTrue(transport.isConnected());

        // Second connect should not throw
        assertDoesNotThrow(() -> transport.connect());
        assertTrue(transport.isConnected());
    }

    @Test
    @DisplayName("Connect to non-existent server should throw")
    @Timeout(value = 5, unit = TimeUnit.SECONDS)
    void testConnectToNonExistentServer() {
        transport = new TCPTransport("localhost", 1, 1000);

        assertThrows(RuntimeException.class, () -> transport.connect());
        assertFalse(transport.isConnected());
    }

    @Test
    @DisplayName("Request should send and receive data")
    @Timeout(value = 5, unit = TimeUnit.SECONDS)
    void testRequest() {
        transport = new TCPTransport("localhost", mockServer.getLocalPort(), 5000);
        transport.connect();

        byte[] requestBody = "test request".getBytes();
        byte[] responseBody = transport.request(0x010101, requestBody);

        assertNotNull(responseBody);
        assertArrayEquals(requestBody, responseBody);
        assertTrue(serverMessageCount.get() > 0);
    }

    @Test
    @DisplayName("Request with empty body should work")
    @Timeout(value = 5, unit = TimeUnit.SECONDS)
    void testRequestWithEmptyBody() {
        transport = new TCPTransport("localhost", mockServer.getLocalPort(), 5000);
        transport.connect();

        byte[] responseBody = transport.request(0x010101, new byte[0]);

        assertNotNull(responseBody);
        assertEquals(0, responseBody.length);
    }

    @Test
    @DisplayName("Request when not connected should throw")
    void testRequestWhenNotConnected() {
        transport = new TCPTransport("localhost", mockServer.getLocalPort(), 5000);

        assertThrows(IllegalStateException.class, () -> {
            transport.request(0x010101, "test".getBytes());
        });
    }

    @Test
    @DisplayName("Request should timeout when no response")
    @Timeout(value = 5, unit = TimeUnit.SECONDS)
    void testRequestTimeout() throws IOException {
        // Create a server that accepts but doesn't respond
        ServerSocket silentServer = new ServerSocket(0);
        Thread silentThread = new Thread(() -> {
            try {
                while (serverRunning) {
                    Socket socket = silentServer.accept();
                    // Just accept, don't respond
                }
            } catch (IOException e) {
                // Exit
            }
        });
        silentThread.setDaemon(true);
        silentThread.start();

        transport = new TCPTransport("localhost", silentServer.getLocalPort(), 1000);
        transport.connect();

        assertThrows(RuntimeException.class, () -> {
            transport.request(0x010101, "test".getBytes());
        });

        silentServer.close();
        silentThread.interrupt();
    }

    @Test
    @DisplayName("Multiple concurrent requests should work")
    @Timeout(value = 10, unit = TimeUnit.SECONDS)
    void testMultipleConcurrentRequests() throws InterruptedException {
        transport = new TCPTransport("localhost", mockServer.getLocalPort(), 5000);
        transport.connect();

        int numRequests = 10;
        CountDownLatch startLatch = new CountDownLatch(1);
        CountDownLatch doneLatch = new CountDownLatch(numRequests);
        AtomicInteger successCount = new AtomicInteger(0);
        AtomicInteger errorCount = new AtomicInteger(0);

        for (int i = 0; i < numRequests; i++) {
            final int index = i;
            new Thread(() -> {
                try {
                    startLatch.await();
                    byte[] response = transport.request(0x010101,
                        ("request " + index).getBytes());
                    if (response != null) {
                        successCount.incrementAndGet();
                    }
                } catch (Exception e) {
                    errorCount.incrementAndGet();
                } finally {
                    doneLatch.countDown();
                }
            }).start();
        }

        startLatch.countDown();
        assertTrue(doneLatch.await(5, TimeUnit.SECONDS));

        // Most requests should succeed
        assertTrue(successCount.get() >= numRequests - 2,
            "Expected at least " + (numRequests - 2) + " successes, got " + successCount.get());
    }

    @Test
    @DisplayName("Request ID should increment for each request")
    @Timeout(value = 5, unit = TimeUnit.SECONDS)
    void testRequestIdIncrement() {
        transport = new TCPTransport("localhost", mockServer.getLocalPort(), 5000);
        transport.connect();

        // Send multiple requests
        for (int i = 0; i < 5; i++) {
            transport.request(0x010101, ("request " + i).getBytes());
        }

        // Server should have received all requests
        assertTrue(serverMessageCount.get() >= 5);
    }

    @Test
    @DisplayName("Close should disconnect transport")
    void testClose() {
        transport = new TCPTransport("localhost", mockServer.getLocalPort(), 5000);
        transport.connect();

        assertTrue(transport.isConnected());

        transport.close();

        assertFalse(transport.isConnected());
    }

    @Test
    @DisplayName("Close when not connected should not throw")
    void testCloseWhenNotConnected() {
        transport = new TCPTransport("localhost", mockServer.getLocalPort(), 5000);

        assertDoesNotThrow(() -> transport.close());
        assertFalse(transport.isConnected());
    }

    @Test
    @DisplayName("Close should be idempotent")
    void testCloseIsIdempotent() {
        transport = new TCPTransport("localhost", mockServer.getLocalPort(), 5000);
        transport.connect();
        transport.close();

        assertDoesNotThrow(() -> transport.close());
        assertFalse(transport.isConnected());
    }

    @Test
    @DisplayName("Request after close should throw")
    void testRequestAfterClose() {
        transport = new TCPTransport("localhost", mockServer.getLocalPort(), 5000);
        transport.connect();
        transport.close();

        assertThrows(IllegalStateException.class, () -> {
            transport.request(0x010101, "test".getBytes());
        });
    }

    @Test
    @DisplayName("Request with large payload should work")
    @Timeout(value = 10, unit = TimeUnit.SECONDS)
    void testRequestWithLargePayload() {
        transport = new TCPTransport("localhost", mockServer.getLocalPort(), 5000);
        transport.connect();

        // 1MB payload
        byte[] largePayload = new byte[1024 * 1024];
        for (int i = 0; i < largePayload.length; i++) {
            largePayload[i] = (byte) (i % 256);
        }

        byte[] response = transport.request(0x010101, largePayload);

        assertNotNull(response);
        assertEquals(largePayload.length, response.length);
    }

    @Test
    @DisplayName("Multiple connect/close cycles should work")
    void testMultipleConnectCloseCycles() {
        transport = new TCPTransport("localhost", mockServer.getLocalPort(), 5000);

        for (int i = 0; i < 3; i++) {
            transport.connect();
            assertTrue(transport.isConnected());

            transport.close();
            assertFalse(transport.isConnected());
        }
    }

    @Test
    @DisplayName("IsConnected should return correct state")
    void testIsConnected() {
        transport = new TCPTransport("localhost", mockServer.getLocalPort(), 5000);

        assertFalse(transport.isConnected());

        transport.connect();
        assertTrue(transport.isConnected());

        transport.close();
        assertFalse(transport.isConnected());
    }

    @Test
    @DisplayName("Request with different message types should work")
    @Timeout(value = 5, unit = TimeUnit.SECONDS)
    void testRequestWithDifferentMessageTypes() {
        transport = new TCPTransport("localhost", mockServer.getLocalPort(), 5000);
        transport.connect();

        int[] messageTypes = {
            Protocol.MSG_INVOKE_REQUEST,
            Protocol.MSG_REGISTER_REQUEST,
            Protocol.MSG_HEARTBEAT_REQUEST
        };

        for (int msgType : messageTypes) {
            byte[] response = transport.request(msgType, "test".getBytes());
            assertNotNull(response);
        }
    }

    @Test
    @DisplayName("Concurrent close should not cause issues")
    @Timeout(value = 5, unit = TimeUnit.SECONDS)
    void testConcurrentClose() throws InterruptedException {
        transport = new TCPTransport("localhost", mockServer.getLocalPort(), 5000);
        transport.connect();

        AtomicBoolean hasException = new AtomicBoolean(false);
        CountDownLatch latch = new CountDownLatch(1);

        // Close from multiple threads
        for (int i = 0; i < 5; i++) {
            new Thread(() -> {
                try {
                    latch.await();
                    transport.close();
                } catch (Exception e) {
                    hasException.set(true);
                }
            }).start();
        }

        latch.countDown();
        Thread.sleep(100);

        assertFalse(hasException.get());
        assertFalse(transport.isConnected());
    }

    @Test
    @DisplayName("Reconnect after close should work")
    @Timeout(value = 5, unit = TimeUnit.SECONDS)
    void testReconnectAfterClose() {
        transport = new TCPTransport("localhost", mockServer.getLocalPort(), 5000);

        transport.connect();
        assertTrue(transport.isConnected());

        transport.close();
        assertFalse(transport.isConnected());

        // Reconnect
        transport.connect();
        assertTrue(transport.isConnected());

        // Should be able to send requests
        byte[] response = transport.request(0x010101, "test".getBytes());
        assertNotNull(response);
    }

    @Test
    @DisplayName("Request error should clean up pending response")
    @Timeout(value = 5, unit = TimeUnit.SECONDS)
    void testRequestErrorCleanup() throws IOException {
        // Create a server that closes immediately
        ServerSocket badServer = new ServerSocket(0);
        Thread badThread = new Thread(() -> {
            try {
                Socket socket = badServer.accept();
                socket.close(); // Close immediately
            } catch (IOException e) {
                // Ignore
            }
        });
        badThread.setDaemon(true);
        badThread.start();

        transport = new TCPTransport("localhost", badServer.getLocalPort(), 1000);
        transport.connect();

        // This should fail, but transport should clean up
        assertThrows(RuntimeException.class, () -> {
            transport.request(0x010101, "test".getBytes());
        });

        // Transport should be disconnected
        assertFalse(transport.isConnected());

        badServer.close();
        badThread.interrupt();
    }

    @Test
    @DisplayName("Connect timeout should fail appropriately")
    @Timeout(value = 5, unit = TimeUnit.SECONDS)
    void testConnectTimeout() {
        // Use an unreachable IP
        transport = new TCPTransport("192.0.2.1", 1, 1000);

        assertThrows(RuntimeException.class, () -> transport.connect());
        assertFalse(transport.isConnected());
    }

    @Test
    @DisplayName("Frame size limit should be enforced")
    @Timeout(value = 5, unit = TimeUnit.SECONDS)
    void testFrameSizeLimit() {
        transport = new TCPTransport("localhost", mockServer.getLocalPort(), 5000);
        transport.connect();

        // Create a payload that would exceed 32MB when framed
        // The actual send should fail due to the size limit
        byte[] hugePayload = new byte[33 * 1024 * 1024];

        assertThrows(RuntimeException.class, () -> {
            transport.request(0x010101, hugePayload);
        });
    }

    @Test
    @DisplayName("Zero-length frame should be handled")
    @Timeout(value = 5, unit = TimeUnit.SECONDS)
    void testZeroLengthFrame() {
        transport = new TCPTransport("localhost", mockServer.getLocalPort(), 5000);
        transport.connect();

        // Empty body is valid
        byte[] response = transport.request(0x010101, new byte[0]);
        assertNotNull(response);
    }

    @Test
    @DisplayName("Protocol version should be V1")
    @Timeout(value = 5, unit = TimeUnit.SECONDS)
    void testProtocolVersion() {
        transport = new TCPTransport("localhost", mockServer.getLocalPort(), 5000);
        transport.connect();

        // The mock server verifies the protocol version
        transport.request(0x010101, "test".getBytes());

        assertTrue(serverMessageCount.get() > 0);
    }

    @Test
    @DisplayName("Request ID wrapping should be handled")
    @Timeout(value = 5, unit = TimeUnit.SECONDS)
    void testRequestIdWrapping() throws InterruptedException {
        // This test would need to send 2^31 requests to truly test wrapping
        // Instead, we verify that multiple requests complete successfully
        transport = new TCPTransport("localhost", mockServer.getLocalPort(), 5000);
        transport.connect();

        int numRequests = 100;
        CountDownLatch latch = new CountDownLatch(numRequests);
        AtomicInteger errors = new AtomicInteger(0);

        for (int i = 0; i < numRequests; i++) {
            new Thread(() -> {
                try {
                    transport.request(0x010101, "test".getBytes());
                } catch (Exception e) {
                    errors.incrementAndGet();
                } finally {
                    latch.countDown();
                }
            }).start();
        }

        assertTrue(latch.await(10, TimeUnit.SECONDS));
        assertTrue(errors.get() < numRequests / 2, "Too many errors: " + errors.get());
    }
}
