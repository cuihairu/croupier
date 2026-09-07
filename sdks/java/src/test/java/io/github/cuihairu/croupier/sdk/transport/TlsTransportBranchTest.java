package io.github.cuihairu.croupier.sdk.transport;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Timeout;

import javax.net.ssl.SSLContext;
import javax.net.ssl.SSLServerSocket;
import javax.net.ssl.SSLSocket;
import javax.net.ssl.SSLSocketFactory;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** Third-wave fillers: TLS serverName null branch and subject CN iteration. */
class TlsTransportBranchTest {

    private static Path generateCert(Path dir, String alias, String dname) throws Exception {
        Path ks = dir.resolve(alias + ".jks");
        Process p = new ProcessBuilder(
            "keytool", "-genkeypair", "-alias", alias, "-keyalg", "RSA", "-keysize", "2048",
            "-dname", dname, "-validity", "1",
            "-keystore", ks.toString(), "-storetype", "JKS",
            "-storepass", "changeit", "-keypass", "changeit").start();
        if (!p.waitFor(30, TimeUnit.SECONDS) || p.exitValue() != 0) {
            throw new IllegalStateException("keytool genkeypair failed");
        }
        Path cert = dir.resolve(alias + ".pem");
        Process export = new ProcessBuilder(
            "keytool", "-exportcert", "-alias", alias, "-keystore", ks.toString(),
            "-storetype", "JKS", "-storepass", "changeit", "-rfc", "-file", cert.toString()).start();
        if (!export.waitFor(30, TimeUnit.SECONDS) || export.exitValue() != 0) {
            throw new IllegalStateException("keytool exportcert failed");
        }
        return cert;
    }

    private interface TlsServerRunner {
        void run(SSLServerSocket serverSocket) throws Exception;
    }

    private static void withTlsServer(Path cert, TlsServerRunner body) throws Exception {
        char[] password = "changeit".toCharArray();
        java.security.KeyStore ks = java.security.KeyStore.getInstance("JKS");
        try (java.io.InputStream in = Files.newInputStream(
            cert.getParent().resolve(cert.getFileName().toString().replace(".pem", ".jks")))) {
            ks.load(in, password);
        }
        javax.net.ssl.KeyManagerFactory kmf = javax.net.ssl.KeyManagerFactory.getInstance(
            javax.net.ssl.KeyManagerFactory.getDefaultAlgorithm());
        kmf.init(ks, password);
        SSLContext serverContext = SSLContext.getInstance("TLS");
        serverContext.init(kmf.getKeyManagers(), null, null);
        try (SSLServerSocket serverSocket = (SSLServerSocket) serverContext.getServerSocketFactory()
            .createServerSocket(0)) {
            body.run(serverSocket);
        }
    }

    @Test
    @Timeout(60)
    void nullServerNameSkipsEndpointCheckEntirely() throws Exception {
        Path dir = Files.createTempDirectory("croupier-tls-nullsan");
        Path cert = generateCert(dir, "nullsan", "CN=localhost");
        ExecutorService pool = Executors.newSingleThreadExecutor();
        withTlsServer(cert, serverSocket -> {
            pool.submit(() -> {
                try (SSLSocket sock = (SSLSocket) serverSocket.accept()) {
                    sock.startHandshake();
                    Thread.sleep(1000);
                } catch (Exception ignored) {
                }
                return null;
            });
            // null serverName: the `serverName != null` guard short-circuits
            SSLSocketFactory factory = TlsSocketFactory.create(cert.toString(), null, null);
            TCPTransport transport = new TCPTransport("localhost", serverSocket.getLocalPort(),
                5000, factory, null);
            transport.connect();
            assertTrue(transport.isConnected());
            transport.close();
        });
        pool.shutdownNow();
    }

    @Test
    @Timeout(60)
    void nonCnSubjectPartsAreSkippedDuringVerification() throws Exception {
        // O= placed before CN exercises the regionMatches false edge before the
        // matching CN part is reached
        Path dir = Files.createTempDirectory("croupier-tls-ou");
        Path cert = generateCert(dir, "withou", "O=Games Inc, OU=Platform, CN=localhost");
        ExecutorService pool = Executors.newSingleThreadExecutor();
        withTlsServer(cert, serverSocket -> {
            pool.submit(() -> {
                try (SSLSocket sock = (SSLSocket) serverSocket.accept()) {
                    sock.startHandshake();
                    Thread.sleep(2000);
                } catch (Exception ignored) {
                }
                return null;
            });
            SSLSocketFactory factory = TlsSocketFactory.create(cert.toString(), null, null);
            TCPTransport transport = new TCPTransport("localhost", serverSocket.getLocalPort(),
                5000, factory, "localhost");
            assertDoesNotThrow(transport::connect);
            assertTrue(transport.isConnected());
            transport.close();
        });
        pool.shutdownNow();
    }
}
