/**
 * TCP Transport Layer for Croupier Java SDK.
 *
 * <p>Implements TCP-based transport for communication with Croupier Agent
 * using the Croupier wire protocol with multiplexed request/response.</p>
 *
 * <p>This implementation uses pure Java sockets, no native dependencies.</p>
 */
package io.github.cuihairu.croupier.sdk.transport;

import io.github.cuihairu.croupier.sdk.invoker.InvokerException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.net.Socket;
import java.net.SocketTimeoutException;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.RejectedExecutionException;
import java.util.concurrent.SynchronousQueue;
import java.util.concurrent.ThreadPoolExecutor;
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

    // Inbound request listener (Agent -> Provider calls). Read loop only
    // dispatches; handlers run on a bounded worker pool (default = CPU
    // cores, queue capacity = workers * 4, overflow fast-fails with an
    // empty response so the Agent-side failover takes over). Processing
    // inline would head-of-line block every request on one slow handler;
    // fire-and-forget threads would be unbounded.
    public interface InboundListener {
        byte[] onRequest(int msgId, int requestId, byte[] body) throws Exception;
    }

    private volatile InboundListener inboundListener;
    private volatile ExecutorService inboundPool;
    private final AtomicInteger inboundQueued = new AtomicInteger();

    /** Sets the inbound request listener; inbound dispatch is enabled only when set. */
    public void setInboundListener(InboundListener listener) {
        this.inboundListener = listener;
    }

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

        Socket nextSocket = new Socket();
        try {
            nextSocket.connect(new InetSocketAddress(host, port), timeoutMs);
            nextSocket.setSoTimeout(timeoutMs);
            outputStream = nextSocket.getOutputStream();
            inputStream = nextSocket.getInputStream();
            socket = nextSocket;

            // Start read loop
            closing = false;
            readLoopThread = new Thread(this::readLoop, "TCPTransport-ReadLoop");
            readLoopThread.setDaemon(true);
            readLoopThread.start();

            LOG.info("Connected to TCP server");
        } catch (IOException e) {
            try {
                nextSocket.close();
            } catch (IOException closeError) {
                e.addSuppressed(closeError);
            }
            socket = null;
            inputStream = null;
            outputStream = null;
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
    public byte[] request(int msgType, byte[] data) throws InvokerException {
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
     *
     * <p>Uses a short socket timeout (1s) so the loop can check the closing flag
     * periodically. SocketTimeoutException is caught and the loop continues;
     * other IOExceptions break the loop.</p>
     */
    private void readLoop() {
        byte[] headerBuf = new byte[FRAME_HEADER_BYTES];
        final int readLoopTimeoutMs = 1000; // 1 second — keeps the loop responsive to close()

        // Temporarily set a short read timeout for the read loop so that
        // recv() does not block for the full request timeout (30s).
        int originalTimeout = timeoutMs;
        try {
            socket.setSoTimeout(readLoopTimeoutMs);
        } catch (Exception e) {
            LOG.debug("Failed to set read-loop timeout", e);
        }

        try {
            while (!closing && socket != null && !socket.isClosed()) {
                // Read frame header
                int n;
                try {
                    n = readFully(inputStream, headerBuf);
                } catch (SocketTimeoutException e) {
                    // No data within 1s — check closing flag and retry
                    continue;
                }
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
                try {
                    n = readFully(inputStream, payload);
                } catch (SocketTimeoutException e) {
                    // Payload read timed out — connection is likely broken
                    LOG.warn("Timeout reading frame payload");
                    break;
                }
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

                // 响应优先按 pending reqId 匹配（保持既有语义：mock/旧对端
                // 可能用非标准响应 msgId 回帧）；未命中 pending 且是请求帧
                // 才走 inbound 分发（Agent -> Provider 调用）。
                ResponseLatch latch = pendingResponses.get(reqId);
                if (latch != null) {
                    latch.signal(body, msgId);
                } else if (Protocol.isRequest(msgId)) {
                    dispatchInbound(msgId, reqId, body);
                } else {
                    LOG.debug("No pending request for reqId: {}", reqId);
                }
            }
        } catch (IOException e) {
            if (!closing) {
                LOG.error("Read loop error", e);
            }
        } finally {
            // Restore original socket timeout
            try {
                if (socket != null && !socket.isClosed()) {
                    socket.setSoTimeout(originalTimeout);
                }
            } catch (Exception e) {
                LOG.debug("Failed to restore socket timeout", e);
            }
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
        ExecutorService pool = this.inboundPool;
        if (pool != null) {
            pool.shutdownNow();
            this.inboundPool = null;
        }

    }

    private void dispatchInbound(int msgId, int reqId, byte[] body) {
        InboundListener listener = inboundListener;
        if (listener == null) {
            LOG.debug("No inbound listener for msgId {}", Integer.toHexString(msgId));
            return;
        }
        ExecutorService pool = inboundPool();
        int workers = inboundWorkerCount();
        if (inboundQueued.get() >= workers * 4) {
            LOG.warn("Inbound queue full, fast-failing reqId={}", reqId);
            writeResponseSilently(Protocol.getResponseMsgID(msgId), reqId, new byte[0]);
            return;
        }
        inboundQueued.incrementAndGet();
        try {
            pool.execute(() -> {
                try {
                    byte[] resp;
                    try {
                        resp = listener.onRequest(msgId, reqId, body);
                    } catch (Exception e) {
                        LOG.error("Inbound handler failed: {}", e.getMessage(), e);
                        resp = new byte[0];
                    }
                    writeResponseSilently(Protocol.getResponseMsgID(msgId), reqId, resp);
                } finally {
                    inboundQueued.decrementAndGet();
                }
            });
        } catch (RejectedExecutionException rejected) {
            inboundQueued.decrementAndGet();
            writeResponseSilently(Protocol.getResponseMsgID(msgId), reqId, new byte[0]);
        }
    }

    private static int inboundWorkerCount() {
        return Math.max(2, Runtime.getRuntime().availableProcessors());
    }

    private ExecutorService inboundPool() {
        ExecutorService pool = this.inboundPool;
        if (pool == null) {
            synchronized (this) {
                pool = this.inboundPool;
                if (pool == null) {
                    int workers = inboundWorkerCount();
                    pool = new ThreadPoolExecutor(
                        workers, workers, 60L, TimeUnit.SECONDS,
                        new SynchronousQueue<>(),
                        r -> {
                            Thread t = new Thread(r, "croupier-inbound");
                            t.setDaemon(true);
                            return t;
                        },
                        new ThreadPoolExecutor.CallerRunsPolicy());
                    this.inboundPool = pool;
                }
            }
        }
        return pool;
    }

    private void writeResponseSilently(int respMsgId, int reqId, byte[] body) {
        try {
            byte[] frame = Protocol.newMessage(respMsgId, reqId, body);
            byte[] wrapped = ByteBuffer.allocate(4 + frame.length)
                .order(ByteOrder.BIG_ENDIAN)
                .putInt(frame.length)
                .put(frame)
                .array();
            synchronized (outputStream) {
                outputStream.write(wrapped);
                outputStream.flush();
            }
        } catch (IOException e) {
            LOG.debug("Failed to write inbound response: {}", e.getMessage());
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
