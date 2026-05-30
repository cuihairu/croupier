/**
 * TCP Transport Layer for Croupier Java SDK.
 *
 * <p>Implements TCP-based transport for communication with Croupier Agent
 * using the Croupier wire protocol with multiplexed request/response.</p>
 *
 * <p>This implementation uses pure Java sockets, no native dependencies.</p>
 */
package io.github.cuihairu.croupier.sdk.transport;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.net.Socket;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;

/**
 * TCP-based transport client using Croupier wire protocol.
 * Supports multiplexed request/response over a single connection.
 */
public class TCPTransport implements TransportClient {
    private static final Logger LOG = LoggerFactory.getLogger(TCPTransport.class);
    private static final int FRAME_HEADER_BYTES = 4;
    private static final int PROTOCOL_HEADER_SIZE = 8;
    private static final int MAX_FRAME_BYTES = 32 * 1024 * 1024; // 32 MB
    private static final int VERSION_1 = 0x01;

    private final String host;
    private final int port;
    private final int timeoutMs;
    private Socket socket;
    private InputStream inputStream;
    private OutputStream outputStream;
    private final AtomicInteger nextReqId = new AtomicInteger(1);
    private final Map<Integer, ResponseLatch> pendingResponses = new ConcurrentHashMap<>();
    private volatile boolean closing = false;
    private Thread readLoopThread;

    private static class ResponseLatch {
        final CountDownLatch latch = new CountDownLatch(1);
        byte[] body;
        int msgId;

        void await(long timeoutMs) throws InterruptedException {
            latch.await(timeoutMs, TimeUnit.MILLISECONDS);
        }

        void signal(byte[] body, int msgId) {
            this.body = body;
            this.msgId = msgId;
            latch.countDown();
        }
    }

    /**
     * Initialize TCP transport.
     *
     * @param host      Agent host
     * @param port      Agent port
     * @param timeoutMs Request timeout in milliseconds
     */
    public TCPTransport(String host, int port, int timeoutMs) {
        this.host = host;
        this.port = port;
        this.timeoutMs = timeoutMs;
    }

    /**
     * Initialize TCP transport with default timeout (30s).
     */
    public TCPTransport(String host, int port) {
        this(host, port, 30000);
    }

    /**
     * Connect to the TCP server (Agent).
     */
    @Override
    public synchronized void connect() {
        if (socket != null && socket.isConnected()) {
            return;
        }

        LOG.info("Connecting to TCP server at {}:{}", host, port);

        try {
            socket = new Socket();
            socket.connect(new InetSocketAddress(host, port), timeoutMs);
            socket.setSoTimeout(timeoutMs);
            outputStream = socket.getOutputStream();
            inputStream = socket.getInputStream();

            // Start read loop
            closing = false;
            readLoopThread = new Thread(this::readLoop, "TCPTransport-ReadLoop");
            readLoopThread.setDaemon(true);
            readLoopThread.start();

            LOG.info("Connected to TCP server");
        } catch (IOException e) {
            throw new RuntimeException("Failed to connect to " + host + ":" + port, e);
        }
    }

    /**
     * Sends a request and returns the response.
     *
     * @param msgType Protocol message type
     * @param data    Protobuf request body
     * @return Protobuf response body
     */
    @Override
    public byte[] request(int msgType, byte[] data) {
        if (socket == null || !socket.isConnected()) {
            throw new IllegalStateException("Not connected");
        }

        int reqId = nextReqId.getAndIncrement();
        ResponseLatch latch = new ResponseLatch();
        pendingResponses.put(reqId, latch);

        try {
            // Create frame: [4-byte length][8-byte protocol header][body]
            byte[] frame = new byte[FRAME_HEADER_BYTES + PROTOCOL_HEADER_SIZE + data.length];

            // Frame length (big-endian)
            ByteBuffer.wrap(frame, 0, 4).order(ByteOrder.BIG_ENDIAN)
                .putInt(PROTOCOL_HEADER_SIZE + data.length);

            // Protocol header
            frame[4] = VERSION_1;
            putMsgId(frame, 5, msgType);
            ByteBuffer.wrap(frame, 8, 4).order(ByteOrder.BIG_ENDIAN).putInt(reqId);

            // Request body
            System.arraycopy(data, 0, frame, 12, data.length);

            // Send frame
            synchronized (outputStream) {
                outputStream.write(frame);
                outputStream.flush();
            }

            // Wait for response
            latch.await(timeoutMs);
            if (latch.body == null) {
                throw new RuntimeException("Timeout waiting for response");
            }

            return latch.body;
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            throw new RuntimeException("Request interrupted", e);
        } catch (IOException e) {
            throw new RuntimeException("Request failed", e);
        } finally {
            pendingResponses.remove(reqId);
        }
    }

    /**
     * Read loop for incoming frames.
     */
    private void readLoop() {
        byte[] headerBuf = new byte[FRAME_HEADER_BYTES];

        try {
            while (!closing && socket != null && !socket.isClosed()) {
                // Read frame header
                int n = readFully(inputStream, headerBuf);
                if (n < FRAME_HEADER_BYTES) {
                    break;
                }

                // Parse frame size
                int frameSize = ByteBuffer.wrap(headerBuf).order(ByteOrder.BIG_ENDIAN).getInt();
                if (frameSize == 0 || frameSize > MAX_FRAME_BYTES) {
                    LOG.warn("Invalid frame size: {}", frameSize);
                    break;
                }

                // Read frame payload
                byte[] payload = new byte[frameSize];
                n = readFully(inputStream, payload);
                if (n < frameSize) {
                    break;
                }

                // Parse protocol header
                if (payload.length < PROTOCOL_HEADER_SIZE) {
                    continue;
                }

                int version = payload[0] & 0xFF;
                if (version != VERSION_1) {
                    LOG.warn("Unknown protocol version: {}", version);
                    continue;
                }

                int msgId = getMsgId(payload, 1);
                int reqId = ByteBuffer.wrap(payload, 4, 4).order(ByteOrder.BIG_ENDIAN).getInt();
                byte[] body = new byte[payload.length - PROTOCOL_HEADER_SIZE];
                System.arraycopy(payload, PROTOCOL_HEADER_SIZE, body, 0, body.length);

                // Route to pending request
                ResponseLatch latch = pendingResponses.get(reqId);
                if (latch != null) {
                    latch.signal(body, msgId);
                } else {
                    LOG.debug("No pending request for reqId: {}", reqId);
                }
            }
        } catch (IOException e) {
            if (!closing) {
                LOG.error("Read loop error", e);
            }
        } finally {
            if (!closing) {
                close();
            }
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

    /**
     * Puts a 24-bit message ID into the buffer at the given offset.
     */
    private static void putMsgId(byte[] buf, int offset, int msgId) {
        buf[offset] = (byte) (msgId >> 16);
        buf[offset + 1] = (byte) (msgId >> 8);
        buf[offset + 2] = (byte) msgId;
    }

    /**
     * Gets a 24-bit message ID from the buffer at the given offset.
     */
    private static int getMsgId(byte[] buf, int offset) {
        return ((buf[offset] & 0xFF) << 16) | ((buf[offset + 1] & 0xFF) << 8) | (buf[offset + 2] & 0xFF);
    }

    /**
     * Returns true when connected.
     */
    @Override
    public boolean isConnected() {
        return socket != null && socket.isConnected() && !closing;
    }

    /**
     * Closes the transport.
     */
    @Override
    public void close() {
        closing = true;
        if (socket != null) {
            try {
                socket.close();
            } catch (IOException e) {
                LOG.debug("Error closing socket", e);
            }
            socket = null;
        }
        if (readLoopThread != null) {
            try {
                readLoopThread.join(1000);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            }
            readLoopThread = null;
        }
        pendingResponses.clear();
    }
}
