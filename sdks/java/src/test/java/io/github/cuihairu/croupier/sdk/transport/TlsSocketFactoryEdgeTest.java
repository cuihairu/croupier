package io.github.cuihairu.croupier.sdk.transport;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Timeout;

import java.io.IOException;
import java.io.InputStream;
import java.lang.reflect.Method;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;

import static org.junit.jupiter.api.Assertions.*;

/**
 * TlsSocketFactory / 端点名校验边缘补测：
 * SAN 不匹配时 CN 兜底命中、空白 serverName 回退 host、
 * readPrivateKey 的 RSA→EC 回退与无 PEM 报错。
 */
@DisplayName("TLS peer verification and key parsing edges")
class TlsSocketFactoryEdgeTest {

    @Test
    @Timeout(60)
    @DisplayName("TLS 证书 SAN 不匹配时 CN 兜底命中；空白 serverName 回退 host")
    void tlsCnFallbackAndBlankServerName() throws Exception {
        Path dir = Files.createTempDirectory("croupier-tls-cn");
        Path ks = dir.resolve("server.jks");
        Process gen = new ProcessBuilder(
            "keytool", "-genkeypair", "-alias", "server", "-keyalg", "RSA", "-keysize", "2048",
            "-dname", "CN=localhost", "-ext", "SAN=dns:mismatch.example",
            "-validity", "1", "-keystore", ks.toString(), "-storetype", "JKS",
            "-storepass", "changeit", "-keypass", "changeit").start();
        if (!gen.waitFor(30, TimeUnit.SECONDS) || gen.exitValue() != 0) {
            throw new IllegalStateException("keytool genkeypair failed");
        }

        char[] password = "changeit".toCharArray();
        java.security.KeyStore keyStore = java.security.KeyStore.getInstance("JKS");
        try (InputStream in = Files.newInputStream(ks)) {
            keyStore.load(in, password);
        }
        javax.net.ssl.KeyManagerFactory kmf =
            javax.net.ssl.KeyManagerFactory.getInstance(
                javax.net.ssl.KeyManagerFactory.getDefaultAlgorithm());
        kmf.init(keyStore, password);
        javax.net.ssl.SSLContext context = javax.net.ssl.SSLContext.getInstance("TLS");
        context.init(kmf.getKeyManagers(), null, null);
        javax.net.ssl.SSLServerSocket serverSocket = (javax.net.ssl.SSLServerSocket)
            context.getServerSocketFactory().createServerSocket(0);
        int port = serverSocket.getLocalPort();

        ExecutorService pool = Executors.newCachedThreadPool();
        for (int i = 0; i < 2; i++) {
            pool.submit(() -> {
                try (javax.net.ssl.SSLSocket socket =
                         (javax.net.ssl.SSLSocket) serverSocket.accept()) {
                    socket.startHandshake();
                    socket.setSoTimeout(1000);
                    try {
                        socket.getInputStream().read(new byte[16]);
                    } catch (IOException ignored) {
                        // peer may close without writing
                    }
                } catch (IOException ignored) {
                }
                return null;
            });
        }

        // SAN 不匹配 → CN=localhost 命中（verifyPeerName 的 CN 兜底分支）
        Path cert = dir.resolve("server.pem");
        Process export = new ProcessBuilder(
            "keytool", "-exportcert", "-alias", "server", "-keystore", ks.toString(),
            "-storetype", "JKS", "-storepass", "changeit", "-rfc", "-file", cert.toString()).start();
        if (!export.waitFor(30, TimeUnit.SECONDS) || export.exitValue() != 0) {
            throw new IllegalStateException("keytool exportcert failed");
        }

        javax.net.ssl.SSLSocketFactory factory =
            TlsSocketFactory.create(cert.toString(), null, null, null);

        TCPTransport transport = new TCPTransport("localhost", port, 5000, factory, "localhost");
        transport.connect();
        assertTrue(transport.isConnected());
        transport.close();

        // 空白 serverName → expectedName 回退 host
        TCPTransport blank = new TCPTransport("localhost", port, 5000, factory, "   ");
        blank.connect();
        assertTrue(blank.isConnected());
        blank.close();

        serverSocket.close();
        pool.shutdownNow();
    }

    @Test
    @Timeout(30)
    @DisplayName("C-011：mTLS 缺 keystore 口令 fail-closed；提供口令可构造")
    void keyStorePasswordRequiredForMutualTls() throws Exception {
        Path dir = Files.createTempDirectory("croupier-tls-c11");
        Path ks = dir.resolve("ca.jks");
        Process gen = new ProcessBuilder(
            "keytool", "-genkeypair", "-alias", "ca", "-keyalg", "RSA", "-keysize", "2048",
            "-dname", "CN=localhost", "-validity", "1",
            "-keystore", ks.toString(), "-storetype", "JKS",
            "-storepass", "changeit", "-keypass", "changeit").start();
        if (!gen.waitFor(30, TimeUnit.SECONDS) || gen.exitValue() != 0) {
            throw new IllegalStateException("keytool genkeypair failed");
        }
        Path caCert = dir.resolve("ca.pem");
        Process export = new ProcessBuilder(
            "keytool", "-exportcert", "-alias", "ca", "-keystore", ks.toString(),
            "-storetype", "JKS", "-storepass", "changeit", "-rfc", "-file", caCert.toString()).start();
        if (!export.waitFor(30, TimeUnit.SECONDS) || export.exitValue() != 0) {
            throw new IllegalStateException("keytool exportcert failed");
        }

        // mTLS 已配置（cert+key 均非空）但口令缺失 → fail-closed，不静默用硬编码。
        // 口令检查先于读 cert/key 文件，故此处用不存在路径即可触发。
        IOException missing = assertThrows(IOException.class, () ->
            TlsSocketFactory.create(caCert.toString(), dir.resolve("cert.pem").toString(),
                dir.resolve("key.pem").toString(), null));
        assertTrue(missing.getMessage().contains("keyStorePassword"), missing.getMessage());

        // mTLS + 显式口令仍可构造（私钥条目被口令保护；证书/密钥不匹配不阻断
        // KeyStore 装载，仅握手才校验——本用例只验证工厂可构建）。
        java.security.KeyPairGenerator kpg = java.security.KeyPairGenerator.getInstance("EC");
        kpg.initialize(256);
        Path ecKey = dir.resolve("client.p8");
        Files.writeString(ecKey, "-----BEGIN PRIVATE KEY-----\n"
            + java.util.Base64.getMimeEncoder().encodeToString(kpg.generateKeyPair().getPrivate().getEncoded())
            + "\n-----END PRIVATE KEY-----\n");
        javax.net.ssl.SSLSocketFactory factory = TlsSocketFactory.create(
            caCert.toString(), caCert.toString(), ecKey.toString(), "sdk-pass".toCharArray());
        assertNotNull(factory);
    }

    @Test
    @Timeout(30)
    @DisplayName("readPrivateKey：RSA 失败回退 EC；无 PEM 抛 IOException")
    void readPrivateKeyFallbacks() throws Exception {
        Method readPrivateKey = TlsSocketFactory.class
            .getDeclaredMethod("readPrivateKey", String.class);
        readPrivateKey.setAccessible(true);

        // EC 私钥（PKCS#8）：RSA KeyFactory 解析失败 → EC 分支成功
        java.security.KeyPairGenerator kpg = java.security.KeyPairGenerator.getInstance("EC");
        kpg.initialize(256);
        java.security.KeyPair pair = kpg.generateKeyPair();
        Path ecKey = Files.createTempFile("ec", ".p8");
        Files.writeString(ecKey, "-----BEGIN PRIVATE KEY-----\n"
            + java.util.Base64.getMimeEncoder().encodeToString(pair.getPrivate().getEncoded())
            + "\n-----END PRIVATE KEY-----\n");
        Object recovered = readPrivateKey.invoke(null, ecKey.toString());
        assertTrue(recovered instanceof java.security.PrivateKey);
        assertEquals("EC", ((java.security.PrivateKey) recovered).getAlgorithm());

        // 非 PEM 内容
        Path garbage = Files.createTempFile("garbage", ".p8");
        Files.writeString(garbage, "definitely not a pem");
        try {
            Object failure = readPrivateKey.invoke(null, garbage.toString());
            fail("expected IOException but got " + failure);
        } catch (java.lang.reflect.InvocationTargetException e) {
            assertTrue(e.getCause() instanceof IOException);
            assertTrue(e.getCause().getMessage().contains("no PEM private key"));
        }
    }
}
