package io.github.cuihairu.croupier.sdk.transport;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Timeout;

import java.io.IOException;
import java.io.InputStream;
import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.*;

/**
 * TCPTransport 入站分发边缘补测：
 * 入站队列满快速失败、默认 worker 分支、连接关闭后写响应失败的静默路径。
 */
@DisplayName("TCPTransport inbound dispatch edge paths")
class TCPTransportInboundDispatchTest {

    /** 建一个接收单连接并持续排空入站数据的服务端。 */
    private static final class OneShotServer implements AutoCloseable {
        final java.net.ServerSocket serverSocket;
        final ExecutorService pool = Executors.newSingleThreadExecutor();
        final AtomicInteger framesRead = new AtomicInteger();

        OneShotServer() throws IOException {
            serverSocket = new java.net.ServerSocket(0, 1, java.net.InetAddress.getLoopbackAddress());
            pool.submit(() -> {
                try (java.net.Socket socket = serverSocket.accept()) {
                    java.io.DataInputStream in =
                        new java.io.DataInputStream(socket.getInputStream());
                    while (true) {
                        int length = in.readInt();
                        if (length <= 0 || length > 1024 * 1024) {
                            break;
                        }
                        byte[] frame = in.readNBytes(length);
                        if (frame.length < length) {
                            break;
                        }
                        framesRead.incrementAndGet();
                    }
                } catch (IOException ignored) {
                    // client closed
                }
                return null;
            });
        }

        int port() {
            return serverSocket.getLocalPort();
        }

        void awaitFrame(int expected, long timeoutMs) throws InterruptedException {
            long deadline = System.currentTimeMillis() + timeoutMs;
            while (framesRead.get() < expected && System.currentTimeMillis() < deadline) {
                Thread.sleep(10);
            }
        }

        @Override public void close() {
            try {
                serverSocket.close();
            } catch (IOException ignored) {
            }
            pool.shutdownNow();
        }
    }

    private static void dispatchInbound(TCPTransport transport, int msgId, int reqId, byte[] body)
            throws Exception {
        Method method = TCPTransport.class.getDeclaredMethod(
            "dispatchInbound", int.class, int.class, byte[].class);
        method.setAccessible(true);
        method.invoke(transport, msgId, reqId, body);
    }

    @Test
    @Timeout(30)
    @DisplayName("入站队列满快速失败 + 默认 worker 分支 + 写失败静默")
    void inboundDispatchEdgePaths() throws Exception {
        try (OneShotServer server = new OneShotServer()) {
            TCPTransport transport = new TCPTransport("127.0.0.1", server.port(), 5000);
            AtomicInteger handled = new AtomicInteger();
            transport.setInboundListener((msgId, requestId, body) -> {
                handled.incrementAndGet();
                return "ok".getBytes();
            });
            transport.connect();
            assertTrue(transport.isConnected());

            // 默认 worker 分支（未设置 override）+ 正常分发 → 响应帧写出
            dispatchInbound(transport, Protocol.MSG_INVOKE_REQUEST, 101,
                "req".getBytes());
            server.awaitFrame(1, 5000);
            assertEquals(1, handled.get());

            // 队列满：直接回空响应，handler 不执行
            setQueued(transport, 10_000);
            dispatchInbound(transport, Protocol.MSG_INVOKE_REQUEST, 102,
                "req".getBytes());
            server.awaitFrame(2, 5000);
            assertEquals(1, handled.get(), "queued-full must fast-fail without running handler");

            // 写响应失败（socket 已关）：静默吞掉 IOException
            transport.close();
            assertFalse(transport.isConnected());
            setQueued(transport, 0);
            assertDoesNotThrow(() -> dispatchInbound(transport, Protocol.MSG_INVOKE_REQUEST, 104,
                "req".getBytes()));
        }
    }

    private static void setQueued(TCPTransport transport, int value) throws Exception {
        Field field = TCPTransport.class.getDeclaredField("inboundQueued");
        field.setAccessible(true);
        ((AtomicInteger) field.get(transport)).set(value);
    }
}
