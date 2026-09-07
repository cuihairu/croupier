package io.github.cuihairu.croupier.sdk.transport;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.util.List;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.*;

/**
 * TCPTransport 边缘路径补测：
 * verifyPeerName 的空证书/不可解析 SAN 分支、readLoop 恢复 soTimeout 失败的
 * 静默吞掉分支、inbound pool 拒绝执行时的快速失败回空响应分支。
 */
@DisplayName("TCPTransport TLS peer-name and read-loop dispatch edge paths")
class TCPTransportEdgeBoostTest {

    private static void setField(Object target, String name, Object value) throws Exception {
        Field field = TCPTransport.class.getDeclaredField(name);
        field.setAccessible(true);
        field.set(target, value);
    }

    private static void dispatchInbound(TCPTransport transport, int msgId, int reqId, byte[] body)
            throws Exception {
        Method method = TCPTransport.class.getDeclaredMethod(
            "dispatchInbound", int.class, int.class, byte[].class);
        method.setAccessible(true);
        method.invoke(transport, msgId, reqId, body);
    }

    /** verifyPeerName 是私有静态方法，反射驱动（stub SSLSocket + stub SSLSession）。 */
    private static void verifyPeerName(javax.net.ssl.SSLSocket socket, String serverName)
            throws Exception {
        Method method = TCPTransport.class.getDeclaredMethod(
            "verifyPeerName", javax.net.ssl.SSLSocket.class, String.class);
        method.setAccessible(true);
        try {
            method.invoke(null, socket, serverName);
        } catch (java.lang.reflect.InvocationTargetException e) {
            Throwable cause = e.getCause();
            if (cause instanceof Exception ex) {
                throw ex;
            }
            throw e;
        }
    }

    private static javax.net.ssl.SSLSession sessionWithPeerCerts(
            java.security.cert.Certificate[] peerCertificates) {
        return new javax.net.ssl.SSLSession() {
            @Override public int getApplicationBufferSize() { return 0; }
            @Override public String getCipherSuite() { return "TLS_STUB"; }
            @Override public long getCreationTime() { return 0L; }
            @Override public byte[] getId() { return new byte[0]; }
            @Override public long getLastAccessedTime() { return 0L; }
            @Override public java.security.cert.Certificate[] getLocalCertificates() { return null; }
            @Override public java.security.Principal getLocalPrincipal() { return null; }
            @Override public int getPacketBufferSize() { return 0; }
            @SuppressWarnings("deprecation")
            @Override public javax.security.cert.X509Certificate[] getPeerCertificateChain() {
                return new javax.security.cert.X509Certificate[0];
            }
            @Override public java.security.cert.Certificate[] getPeerCertificates() {
                return peerCertificates;
            }
            @Override public String getPeerHost() { return null; }
            @Override public int getPeerPort() { return 0; }
            @Override public java.security.Principal getPeerPrincipal() { return null; }
            @Override public String getProtocol() { return "stub"; }
            @Override public Object getValue(String name) { return null; }
            @Override public String[] getValueNames() { return new String[0]; }
            @Override public void invalidate() { }
            @Override public boolean isValid() { return false; }
            @Override public javax.net.ssl.SSLSessionContext getSessionContext() { return null; }
            @Override public void putValue(String name, Object value) { }
            @Override public void removeValue(String name) { }
        };
    }

    private static final class StubSslSocket extends javax.net.ssl.SSLSocket {
        private final javax.net.ssl.SSLSession session;

        StubSslSocket(javax.net.ssl.SSLSession session) {
            this.session = session;
        }

        @Override public javax.net.ssl.SSLSession getSession() { return session; }
        @Override public void startHandshake() throws IOException { }
        @Override public void addHandshakeCompletedListener(
            javax.net.ssl.HandshakeCompletedListener listener) { }
        @Override public boolean getEnableSessionCreation() { return false; }
        @Override public String[] getEnabledCipherSuites() { return new String[0]; }
        @Override public String[] getEnabledProtocols() { return new String[0]; }
        @Override public boolean getNeedClientAuth() { return false; }
        @Override public String[] getSupportedCipherSuites() { return new String[0]; }
        @Override public String[] getSupportedProtocols() { return new String[0]; }
        @Override public boolean getUseClientMode() { return false; }
        @Override public boolean getWantClientAuth() { return false; }
        @Override public void removeHandshakeCompletedListener(
            javax.net.ssl.HandshakeCompletedListener listener) { }
        @Override public void setEnableSessionCreation(boolean flag) { }
        @Override public void setEnabledCipherSuites(String[] suites) { }
        @Override public void setEnabledProtocols(String[] protocols) { }
        @Override public void setNeedClientAuth(boolean need) { }
        @Override public void setUseClientMode(boolean mode) { }
        @Override public void setWantClientAuth(boolean want) { }
    }

    /** SAN 可配置抛 CertificateParsingException 的 X509Certificate stub。 */
    private static final class StubCertificate extends java.security.cert.X509Certificate {
        private final boolean throwOnSubjectAlternativeNames;
        private final String subjectDn;

        StubCertificate(boolean throwOnSubjectAlternativeNames, String subjectDn) {
            this.throwOnSubjectAlternativeNames = throwOnSubjectAlternativeNames;
            this.subjectDn = subjectDn;
        }

        @Override public List<List<?>> getSubjectAlternativeNames()
                throws java.security.cert.CertificateParsingException {
            if (throwOnSubjectAlternativeNames) {
                throw new java.security.cert.CertificateParsingException("malformed SAN extension");
            }
            return null;
        }

        @Override public javax.security.auth.x500.X500Principal getSubjectX500Principal() {
            return new javax.security.auth.x500.X500Principal(subjectDn);
        }

        @Override public java.security.Principal getSubjectDN() {
            return new javax.security.auth.x500.X500Principal(subjectDn);
        }

        @Override public void checkValidity() { }
        @Override public void checkValidity(java.util.Date date) { }
        @Override public int getBasicConstraints() { return -1; }
        @Override public boolean[] getKeyUsage() { return null; }
        @Override public boolean[] getIssuerUniqueID() { return null; }
        @Override public boolean[] getSubjectUniqueID() { return null; }
        @Override public byte[] getExtensionValue(String oid) { return null; }
        @Override public java.util.Set<String> getNonCriticalExtensionOIDs() { return null; }
        @Override public java.util.Set<String> getCriticalExtensionOIDs() { return null; }
        @Override public boolean hasUnsupportedCriticalExtension() { return false; }
        @Override public int getVersion() { return 3; }
        @Override public java.math.BigInteger getSerialNumber() {
            return java.math.BigInteger.ZERO;
        }
        @Override public java.security.Principal getIssuerDN() { return null; }
        @Override public java.util.Date getNotBefore() { return null; }
        @Override public java.util.Date getNotAfter() { return null; }
        @Override public byte[] getTBSCertificate() { return new byte[0]; }
        @Override public byte[] getSignature() { return new byte[0]; }
        @Override public String getSigAlgName() { return "stub"; }
        @Override public String getSigAlgOID() { return "1.2.3"; }
        @Override public byte[] getSigAlgParams() { return new byte[0]; }
        @Override public byte[] getEncoded() { return new byte[0]; }
        @Override public java.security.PublicKey getPublicKey() { return null; }
        @Override public void verify(java.security.PublicKey key) { }
        @Override public void verify(java.security.PublicKey key, String sigProvider) { }
        @Override public String toString() { return "StubCertificate"; }
    }

    @Test
    @DisplayName("verifyPeerName：会话无对端证书 → 抛 IOException")
    void verifyPeerNameRejectsMissingPeerCertificate() throws Exception {
        javax.net.ssl.SSLSession session =
            sessionWithPeerCerts(new java.security.cert.Certificate[0]);
        IOException error = assertThrows(java.io.IOException.class,
            () -> verifyPeerName(new StubSslSocket(session), "agent.example.com"));
        assertTrue(error.getMessage().contains("no peer certificate"));
    }

    @Test
    @DisplayName("verifyPeerName：SAN 解析失败回退 CN 匹配")
    void verifyPeerNameFallsBackToCnWhenSanUnparsable() throws Exception {
        StubCertificate cert = new StubCertificate(true, "CN=agent.example.com");
        javax.net.ssl.SSLSession session =
            sessionWithPeerCerts(new java.security.cert.Certificate[]{cert});
        assertDoesNotThrow(() -> verifyPeerName(new StubSslSocket(session), "agent.example.com"));
    }

    @Test
    @DisplayName("readLoop finally 恢复 soTimeout 抛错被吞掉且连接关闭")
    void readLoopRestoreTimeoutFailureIsSwallowed() throws Exception {
        TCPTransport transport = new TCPTransport("127.0.0.1", 1, 3000);
        java.net.Socket socket = new java.net.Socket() {
            @Override public synchronized void setSoTimeout(int timeout)
                    throws java.net.SocketException {
                throw new java.net.SocketException("setSoTimeout disabled for test");
            }
        };
        setField(transport, "socket", socket);
        // 只有 2 字节可读：readFully 不足帧头 → 循环退出 → finally 恢复超时抛错 → 静默
        setField(transport, "inputStream", new ByteArrayInputStream(new byte[2]));

        Method readLoop = TCPTransport.class.getDeclaredMethod("readLoop");
        readLoop.setAccessible(true);
        assertDoesNotThrow(() -> readLoop.invoke(transport));
        assertFalse(transport.isConnected(), "transport must self-close after read loop exit");
    }

    @Test
    @DisplayName("inbound pool 拒绝执行：快速失败回空响应，handler 不执行")
    void dispatchInboundFailsFastWhenPoolRejects() throws Exception {
        TCPTransport transport = new TCPTransport("127.0.0.1", 1, 3000);
        ByteArrayOutputStream sink = new ByteArrayOutputStream();
        setField(transport, "outputStream", sink);
        ExecutorService deadPool = Executors.newSingleThreadExecutor();
        deadPool.shutdownNow();
        setField(transport, "inboundPool", deadPool);
        AtomicInteger handled = new AtomicInteger();
        transport.setInboundListener((msgId, requestId, body) -> {
            handled.incrementAndGet();
            return "ok".getBytes();
        });

        assertDoesNotThrow(() ->
            dispatchInbound(transport, Protocol.MSG_INVOKE_REQUEST, 7, "req".getBytes()));
        assertEquals(0, handled.get(), "handler must not run when pool rejects");
        // 快速失败回空响应帧：4 字节长度 + 8 字节协议头 + 空 body
        assertEquals(12, sink.size(), "empty fail-fast response frame should be written");
    }
}
