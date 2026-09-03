package io.github.cuihairu.croupier.sdk.transport;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Timeout;

import javax.net.ssl.SSLContext;
import javax.net.ssl.SSLServerSocket;
import javax.net.ssl.SSLSocket;
import javax.net.ssl.SSLSocketFactory;
import java.io.InputStream;
import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;

import static org.junit.jupiter.api.Assertions.*;

/**
 * TlsSocketFactory / TLS TCPTransport 集成测试：
 * keytool 生成自签证书（SAN=localhost），SSLServerSocket 回显，
 * 验证 CA 校验握手成功、错误 serverName 被端点校验拒绝、mTLS 上下文可构建。
 */
@DisplayName("TLS transport handshake")
class TlsTransportTest {

    private static Path generateCert(Path dir, String alias, String caCert, String caP7) throws Exception {
        Path cert = dir.resolve(alias + ".pem");
        Path key = dir.resolve(alias + ".p8");
        Path ks = dir.resolve(alias + ".jks");
        // 自签证书：SAN=localhost + 127.0.0.1
        Process p = new ProcessBuilder(
            "keytool", "-genkeypair", "-alias", alias, "-keyalg", "RSA", "-keysize", "2048",
            "-dname", "CN=localhost", "-ext", "SAN=dns:localhost,ip:127.0.0.1",
            "-validity", "1", "-keystore", ks.toString(), "-storetype", "JKS",
            "-storepass", "changeit", "-keypass", "changeit", "-v").start();
        if (!p.waitFor(30, TimeUnit.SECONDS) || p.exitValue() != 0) {
            throw new IllegalStateException("keytool genkeypair failed");
        }
        // 导出证书 PEM
        Process export = new ProcessBuilder(
            "keytool", "-exportcert", "-alias", alias, "-keystore", ks.toString(),
            "-storetype", "JKS", "-storepass", "changeit", "-rfc", "-file", cert.toString()).start();
        if (!export.waitFor(30, TimeUnit.SECONDS) || export.exitValue() != 0) {
            throw new IllegalStateException("keytool exportcert failed");
        }
        // 导出 PKCS#8 私钥（mTLS 用例）
        if (caP7 != null) {
            Process pk8 = new ProcessBuilder(
                "keytool", "-importkeystore", "-srckeystore", ks.toString(), "-storetype", "JKS",
                "-srcstorepass", "changeit", "-srcalias", alias,
                "-destkeystore", dir.resolve(alias + ".p12").toString(), "-deststoretype", "PKCS12",
                "-deststorepass", "changeit", "-destkeypass", "changeit", "-noprompt").start();
            if (!pk8.waitFor(30, TimeUnit.SECONDS) || pk8.exitValue() != 0) {
                throw new IllegalStateException("keytool importkeystore failed");
            }
        }
        return cert;
    }

    @Test
    @Timeout(60)
    @DisplayName("CA 校验握手成功并完成回显；错误 serverName 被拒绝")
    void handshakeAndEndpointVerification() throws Exception {
        Path dir = Files.createTempDirectory("croupier-tls");
        Path cert = generateCert(dir, "server", null, null);

        // TLS 回显服务端（信任自身证书 = 客户端视角的 CA 语义）
        char[] password = "changeit".toCharArray();
        java.security.KeyStore ks = java.security.KeyStore.getInstance("JKS");
        try (InputStream in = Files.newInputStream(dir.resolve("server.jks"))) {
            ks.load(in, password);
        }
        javax.net.ssl.KeyManagerFactory kmf = javax.net.ssl.KeyManagerFactory.getInstance(
            javax.net.ssl.KeyManagerFactory.getDefaultAlgorithm());
        kmf.init(ks, password);
        SSLContext serverContext = SSLContext.getInstance("TLS");
        serverContext.init(kmf.getKeyManagers(), null, null);
        SSLServerSocket serverSocket = (SSLServerSocket) serverContext.getServerSocketFactory()
            .createServerSocket(0);
        int port = serverSocket.getLocalPort();

        ExecutorService pool = Executors.newSingleThreadExecutor();
        pool.submit(() -> {
            try (SSLSocket sock = (SSLSocket) serverSocket.accept()) {
                // accept() 的 TLS 握手是惰性的（首次读写才发生）：
                // 必须显式 startHandshake 参与协商，否则客户端会读到 EOF
                sock.startHandshake();
                Thread.sleep(1000);
            }
            return null;
        });

        // 客户端：caFile = 服务器自签证书（作为信任锚），serverName=localhost
        // TLS 握手成功即证明 CA 校验 + 端点名校验通过
        SSLSocketFactory factory = TlsSocketFactory.create(cert.toString(), null, null);
        TCPTransport transport = new TCPTransport("localhost", port, 5000, factory, "localhost");
        transport.connect();
        assertTrue(transport.isConnected());

        // 端点名校验语义（确定性）：反射取好连接上的 SSL socket，
        // verifyPeerName 对 localhost 通过、对 example.com 抛 IOException
        Method verify = TCPTransport.class.getDeclaredMethod("verifyPeerName",
            javax.net.ssl.SSLSocket.class, String.class);
        verify.setAccessible(true);
        Field socketField = TCPTransport.class.getDeclaredField("socket");
        socketField.setAccessible(true);
        javax.net.ssl.SSLSocket connected =
            (javax.net.ssl.SSLSocket) socketField.get(transport);
        verify.invoke(null, connected, "localhost");
        // 反射调用把目标异常包成 InvocationTargetException，解包断言
        java.lang.reflect.InvocationTargetException mismatch = assertThrows(
            java.lang.reflect.InvocationTargetException.class,
            () -> verify.invoke(null, connected, "example.com"));
        assertEquals(java.io.IOException.class, mismatch.getCause().getClass());

        transport.close();

        serverSocket.close();
        pool.shutdownNow();
    }
    @Test
    @Timeout(60)
    @DisplayName("TlsSocketFactory 守卫分支：缺 caFile / 坏 PEM / mTLS 上下文构建")
    void factoryGuardsAndMutualTls() throws Exception {
        Path dir = Files.createTempDirectory("croupier-tls-mtls");
        Path cert = generateCert(dir, "srv", null, null);

        // 缺 caFile → IOException
        assertThrows(java.io.IOException.class,
            () -> TlsSocketFactory.create(null, null, null));
        assertThrows(java.io.IOException.class,
            () -> TlsSocketFactory.create("  ", null, null));

        // 坏 CA 内容（非 PEM）→ 无证书可解析
        Path badCa = dir.resolve("bad-ca.pem");
        Files.writeString(badCa, "not a pem document");
        assertThrows(Exception.class, () -> TlsSocketFactory.create(badCa.toString(), null, null));

        // mTLS：程序化生成客户端 RSA 私钥（PKCS#8 PEM）+ 复用服务端证书作链
        java.security.KeyPairGenerator kpg = java.security.KeyPairGenerator.getInstance("RSA");
        kpg.initialize(2048);
        java.security.KeyPair kp = kpg.generateKeyPair();
        String pkcs8Pem = "-----BEGIN PRIVATE KEY-----\n"
            + java.util.Base64.getMimeEncoder().encodeToString(kp.getPrivate().getEncoded())
            + "\n-----END PRIVATE KEY-----\n";
        Path clientKey = dir.resolve("client.key");
        Files.writeString(clientKey, pkcs8Pem);

        // certFile 存在但 keyFile 为空 → 不启用 mTLS，仅 CA 校验
        assertNotNull(TlsSocketFactory.create(cert.toString(), cert.toString(), null));

        // keyFile 内容坏 → 私钥解析失败
        Path badKey = dir.resolve("bad.key");
        Files.writeString(badKey, "-----BEGIN PRIVATE KEY-----\nbm90IGEga2V5\n-----END PRIVATE KEY-----\n");
        assertThrows(Exception.class,
            () -> TlsSocketFactory.create(cert.toString(), cert.toString(), badKey.toString()));

        // 完整 mTLS 上下文构建成功
        SSLSocketFactory mtlsFactory = TlsSocketFactory.create(cert.toString(), cert.toString(), clientKey.toString());
        assertNotNull(mtlsFactory);
    }
    @Test
    @Timeout(30)
    @DisplayName("inboundWorkers=1 串行模式：pool 大小为 1（单线程游戏服兼容）")
    void inboundWorkersSerialMode() throws Exception {
        TCPTransport transport = new TCPTransport("localhost", 1, 1000);
        transport.setInboundWorkerCount(1);
        Field poolSizeField = TCPTransport.class.getDeclaredField("inboundWorkerOverride");
        poolSizeField.setAccessible(true);
        assertEquals(1, poolSizeField.getInt(transport));
    }
}
