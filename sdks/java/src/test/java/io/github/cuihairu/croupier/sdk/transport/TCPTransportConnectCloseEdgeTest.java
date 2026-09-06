package io.github.cuihairu.croupier.sdk.transport;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Timeout;

import javax.net.ssl.SSLSocket;
import javax.net.ssl.SSLSocketFactory;
import java.io.IOException;
import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.net.InetAddress;
import java.net.ServerSocket;
import java.net.Socket;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.*;

/**
 * TCPTransport connect/close 边缘补测：
 * 连接失败清理、connect 失败时 close 异常的 suppressed/未检查分支、
 * close 时 socket 抛 IOException 被吞、override worker 分支。
 * （帧级 readLoop 场景由 TCPTransportEdgeTest 覆盖。）
 */
@DisplayName("TCPTransport connect/close edge cases")
class TCPTransportConnectCloseEdgeTest {

    private static int freePort() throws IOException {
        try (ServerSocket socket = new ServerSocket(0)) {
            return socket.getLocalPort();
        }
    }

    @Test
    @Timeout(20)
    @DisplayName("connect 到无人监听端口：清理并抛 RuntimeException")
    void connectFailureCleansUpAndThrows() throws Exception {
        TCPTransport transport = new TCPTransport("127.0.0.1", freePort(), 500);
        RuntimeException error = assertThrows(RuntimeException.class, transport::connect);
        assertTrue(error.getMessage().contains("Failed to connect"));
        assertFalse(transport.isConnected());
    }

    @Test
    @Timeout(15)
    @DisplayName("close 时 socket 抛 IOException 被吞掉")
    void closeSwallowsSocketCloseFailure() throws Exception {
        TCPTransport transport = new TCPTransport("127.0.0.1", 1, 100);
        Socket failingSocket = new Socket() {
            @Override public void close() throws IOException {
                throw new IOException("close refused");
            }
        };
        Field socketField = TCPTransport.class.getDeclaredField("socket");
        socketField.setAccessible(true);
        socketField.set(transport, failingSocket);
        assertDoesNotThrow(transport::close);
        assertFalse(transport.isConnected());
    }

    @Test
    @Timeout(30)
    @DisplayName("override worker 分支：setInboundWorkerCount(1) 串行分发")
    void dispatchWithWorkerOverride() throws Exception {
        try (ScriptedServerHolder server = new ScriptedServerHolder()) {
            TCPTransport transport = new TCPTransport("127.0.0.1", server.port(), 3000);
            transport.setInboundWorkerCount(1);
            AtomicInteger handled = new AtomicInteger();
            transport.setInboundListener((msgId, requestId, body) -> {
                handled.incrementAndGet();
                return "ok".getBytes();
            });
            transport.connect();
            Method dispatch = TCPTransport.class.getDeclaredMethod(
                "dispatchInbound", int.class, int.class, byte[].class);
            dispatch.setAccessible(true);
            dispatch.invoke(transport, Protocol.MSG_INVOKE_REQUEST, 7, new byte[]{1});
            long deadline = System.currentTimeMillis() + 5000;
            while (handled.get() == 0 && System.currentTimeMillis() < deadline) {
                Thread.sleep(10);
            }
            assertEquals(1, handled.get(), "override=1 dispatch must run handler serially");
            transport.close();
        }
    }

    @Test
    @Timeout(15)
    @DisplayName("connect 失败时 nextSocket.close 的异常进入 suppressed/未检查分支")
    void connectFailureCloseSuppressed() {
        SSLSocketFactory factory = new SSLSocketFactory() {
            @Override public Socket createSocket() {
                return new StubSslSocket() {
                    @Override public void connect(java.net.SocketAddress endpoint, int timeout) throws IOException {
                        throw new IOException("handshake refused");
                    }
                    @Override public synchronized void close() throws IOException {
                        throw new IOException("close failed too");
                    }
                };
            }
            @Override public Socket createSocket(String host, int port) { return createSocket(); }
            @Override public Socket createSocket(String host, int port, InetAddress localHost, int localPort) { return createSocket(); }
            @Override public Socket createSocket(InetAddress host, int port) { return createSocket(); }
            @Override public Socket createSocket(InetAddress address, int port, InetAddress localAddress, int localPort) { return createSocket(); }
            @Override public Socket createSocket(Socket s, String host, int port, boolean autoClose) { return createSocket(); }
            @Override public String[] getDefaultCipherSuites() { return new String[0]; }
            @Override public String[] getSupportedCipherSuites() { return new String[0]; }
        };
        TCPTransport transport = new TCPTransport("127.0.0.1", 1, 500, factory, "localhost");
        RuntimeException error = assertThrows(RuntimeException.class, transport::connect);
        assertTrue(error.getMessage().contains("Failed to connect"));
        assertEquals(1, error.getCause().getSuppressed().length);

        // close 抛 RuntimeException 的未检查分支
        SSLSocketFactory runtimeClosing = new SSLSocketFactory() {
            @Override public Socket createSocket() {
                return new StubSslSocket() {
                    @Override public void connect(java.net.SocketAddress endpoint, int timeout) throws IOException {
                        throw new IOException("connect refused");
                    }
                    @Override public synchronized void close() {
                        throw new IllegalStateException("close exploded");
                    }
                };
            }
            @Override public Socket createSocket(String host, int port) { return createSocket(); }
            @Override public Socket createSocket(String host, int port, InetAddress localHost, int localPort) { return createSocket(); }
            @Override public Socket createSocket(InetAddress host, int port) { return createSocket(); }
            @Override public Socket createSocket(InetAddress address, int port, InetAddress localAddress, int localPort) { return createSocket(); }
            @Override public Socket createSocket(Socket s, String host, int port, boolean autoClose) { return createSocket(); }
            @Override public String[] getDefaultCipherSuites() { return new String[0]; }
            @Override public String[] getSupportedCipherSuites() { return new String[0]; }
        };
        TCPTransport unchecked = new TCPTransport("127.0.0.1", 1, 500, runtimeClosing, "localhost");
        RuntimeException second = assertThrows(RuntimeException.class, unchecked::connect);
        assertTrue(second.getMessage().contains("Failed to connect"));
    }

    /** SSLSocket 桩：仅保留 connect/close 可覆写，其余握手 API 空实现。 */
    private abstract static class StubSslSocket extends SSLSocket {
        @Override public String[] getSupportedCipherSuites() { return new String[0]; }
        @Override public String[] getEnabledCipherSuites() { return new String[0]; }
        @Override public void setEnabledCipherSuites(String[] suites) { }
        @Override public String[] getSupportedProtocols() { return new String[0]; }
        @Override public String[] getEnabledProtocols() { return new String[0]; }
        @Override public void setEnabledProtocols(String[] protocols) { }
        @Override public javax.net.ssl.SSLSession getSession() { return null; }
        @Override public void addHandshakeCompletedListener(javax.net.ssl.HandshakeCompletedListener listener) { }
        @Override public void removeHandshakeCompletedListener(javax.net.ssl.HandshakeCompletedListener listener) { }
        @Override public void startHandshake() { }
        @Override public void setUseClientMode(boolean mode) { }
        @Override public boolean getUseClientMode() { return true; }
        @Override public void setNeedClientAuth(boolean need) { }
        @Override public boolean getNeedClientAuth() { return false; }
        @Override public void setWantClientAuth(boolean want) { }
        @Override public boolean getWantClientAuth() { return false; }
        @Override public void setEnableSessionCreation(boolean flag) { }
        @Override public boolean getEnableSessionCreation() { return true; }
    }

    /** 保持连接打开的哑服务端（供 dispatch 场景使用）。 */
    private static final class ScriptedServerHolder implements AutoCloseable {
        final ServerSocket serverSocket;
        final Thread acceptor;

        ScriptedServerHolder() throws IOException {
            serverSocket = new ServerSocket(0, 1, InetAddress.getLoopbackAddress());
            acceptor = new Thread(() -> {
                try (Socket client = serverSocket.accept()) {
                    client.setSoTimeout(5000);
                    // 读取并丢弃客户端写出的响应帧，防止缓冲区打满
                    while (client.getInputStream().read(new byte[64]) >= 0) {
                        // drain
                    }
                } catch (IOException ignored) {
                    // closed
                }
            }, "dispatch-drain-server");
            acceptor.setDaemon(true);
            acceptor.start();
        }

        int port() {
            return serverSocket.getLocalPort();
        }

        @Override public void close() {
            try {
                serverSocket.close();
            } catch (IOException ignored) {
            }
        }
    }
}
