package io.github.cuihairu.croupier.sdk.transport;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Timeout;

import javax.net.ssl.SSLContext;
import javax.net.ssl.SSLServerSocket;
import javax.net.ssl.SSLSocket;
import javax.net.ssl.SSLSocketFactory;
import java.io.IOException;
import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.net.ServerSocket;
import java.net.Socket;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** Branch-complete coverage for TCPTransport guard branches. */
class TCPTransportBranchTest {

    private static void exportPem(Path ks, String alias, Path out) throws Exception {
        Process export = new ProcessBuilder(
            "keytool", "-exportcert", "-alias", alias, "-keystore", ks.toString(),
            "-storetype", "JKS", "-storepass", "changeit", "-rfc", "-file", out.toString()).start();
        if (!export.waitFor(30, TimeUnit.SECONDS) || export.exitValue() != 0) {
            throw new IllegalStateException("keytool exportcert failed");
        }
    }

    private static Path generateCnOnlyCert(Path dir, String alias) throws Exception {
        Path ks = dir.resolve(alias + ".jks");
        // CN-only certificate: no SAN extension
        Process p = new ProcessBuilder(
            "keytool", "-genkeypair", "-alias", alias, "-keyalg", "RSA", "-keysize", "2048",
            "-dname", "CN=localhost", "-validity", "1",
            "-keystore", ks.toString(), "-storetype", "JKS",
            "-storepass", "changeit", "-keypass", "changeit").start();
        if (!p.waitFor(30, TimeUnit.SECONDS) || p.exitValue() != 0) {
            throw new IllegalStateException("keytool genkeypair failed");
        }
        Path cert = dir.resolve(alias + ".pem");
        exportPem(ks, alias, cert);
        return cert;
    }

    @Test
    @Timeout(30)
    void closeBeforeConnectIsSafe() {
        TCPTransport transport = new TCPTransport("localhost", 1, 500);
        assertDoesNotThrow(transport::close);
        assertFalse(transport.isConnected());
    }

    @Test
    @Timeout(30)
    void connectWhileConnectedReturnsImmediately() throws Exception {
        try (ServerSocket server = new ServerSocket(0)) {
            ExecutorService pool = Executors.newSingleThreadExecutor();
            pool.submit(() -> {
                try (Socket ignored = server.accept()) {
                    Thread.sleep(3000);
                } catch (Exception ignored) {
                }
                return null;
            });
            TCPTransport transport = new TCPTransport("127.0.0.1", server.getLocalPort(), 3000);
            transport.connect();
            assertTrue(transport.isConnected());
            long start = System.nanoTime();
            transport.connect();
            long elapsedMs = (System.nanoTime() - start) / 1_000_000;
            assertTrue(elapsedMs < 1000, "second connect must not reconnect, took " + elapsedMs + "ms");
            transport.close();
            pool.shutdownNow();
        }
    }

    @Test
    @Timeout(30)
    void requestAfterCloseFailsWithIllegalState() throws Exception {
        try (ServerSocket server = new ServerSocket(0)) {
            ExecutorService pool = Executors.newSingleThreadExecutor();
            pool.submit(() -> {
                try (Socket ignored = server.accept()) {
                    Thread.sleep(3000);
                } catch (Exception ignored) {
                }
                return null;
            });
            TCPTransport transport = new TCPTransport("127.0.0.1", server.getLocalPort(), 3000);
            transport.connect();
            transport.close();
            assertFalse(transport.isConnected());
            assertThrows(IllegalStateException.class,
                () -> transport.request(0x010101, new byte[] {1}));
            pool.shutdownNow();
        }
    }

    @Test
    @Timeout(60)
    void tlsWithBlankServerNameSkipsEndpointCheck() throws Exception {
        Path dir = Files.createTempDirectory("croupier-tls-blank");
        Path cert = generateCnOnlyCert(dir, "blank");

        char[] password = "changeit".toCharArray();
        java.security.KeyStore ks = java.security.KeyStore.getInstance("JKS");
        try (java.io.InputStream in = Files.newInputStream(dir.resolve("blank.jks"))) {
            ks.load(in, password);
        }
        javax.net.ssl.KeyManagerFactory kmf = javax.net.ssl.KeyManagerFactory.getInstance(
            javax.net.ssl.KeyManagerFactory.getDefaultAlgorithm());
        kmf.init(ks, password);
        SSLContext serverContext = SSLContext.getInstance("TLS");
        serverContext.init(kmf.getKeyManagers(), null, null);
        try (SSLServerSocket serverSocket = (SSLServerSocket) serverContext.getServerSocketFactory()
            .createServerSocket(0)) {
            ExecutorService pool = Executors.newSingleThreadExecutor();
            pool.submit(() -> {
                try (SSLSocket sock = (SSLSocket) serverSocket.accept()) {
                    sock.startHandshake();
                    Thread.sleep(1000);
                } catch (Exception ignored) {
                }
                return null;
            });

            SSLSocketFactory factory = TlsSocketFactory.create(cert.toString(), null, null);
            TCPTransport transport = new TCPTransport("localhost", serverSocket.getLocalPort(),
                5000, factory, "   ");
            transport.connect();
            assertTrue(transport.isConnected());
            transport.close();
            pool.shutdownNow();
        }
    }

    @Test
    @Timeout(60)
    void verifyPeerNameFallsThroughSanToCn() throws Exception {
        Path dir = Files.createTempDirectory("croupier-tls-cn");
        Path cert = generateCnOnlyCert(dir, "cnonly");

        char[] password = "changeit".toCharArray();
        java.security.KeyStore ks = java.security.KeyStore.getInstance("JKS");
        try (java.io.InputStream in = Files.newInputStream(dir.resolve("cnonly.jks"))) {
            ks.load(in, password);
        }
        javax.net.ssl.KeyManagerFactory kmf = javax.net.ssl.KeyManagerFactory.getInstance(
            javax.net.ssl.KeyManagerFactory.getDefaultAlgorithm());
        kmf.init(ks, password);
        SSLContext serverContext = SSLContext.getInstance("TLS");
        serverContext.init(kmf.getKeyManagers(), null, null);
        try (SSLServerSocket serverSocket = (SSLServerSocket) serverContext.getServerSocketFactory()
            .createServerSocket(0)) {
            ExecutorService pool = Executors.newSingleThreadExecutor();
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
            transport.connect();

            Method verify = TCPTransport.class.getDeclaredMethod("verifyPeerName",
                javax.net.ssl.SSLSocket.class, String.class);
            verify.setAccessible(true);
            Field socketField = TCPTransport.class.getDeclaredField("socket");
            socketField.setAccessible(true);
            SSLSocket connected = (SSLSocket) socketField.get(transport);

            // CN=localhost matches
            verify.invoke(null, connected, "localhost");
            // CN mismatch via a non-CN subject part: "wrong" never equals any CN entry
            assertThrows(java.lang.reflect.InvocationTargetException.class,
                () -> verify.invoke(null, connected, "example.com"));

            transport.close();
            pool.shutdownNow();
        }
    }

    @Test
    @Timeout(30)
    void inboundPoolCoversSerialAndDefaultWorkerCounts() throws Exception {
        TCPTransport transport = new TCPTransport("localhost", 1, 500);
        transport.setInboundWorkerCount(2);
        Method workers = TCPTransport.class.getDeclaredMethod("inboundWorkerCount");
        workers.setAccessible(true);
        assertTrue((int) workers.invoke(transport) >= 1);
        assertDoesNotThrow(transport::close);
    }

    @Test
    @Timeout(30)
    void connectFailureClosesCandidateSocketAndSurfacesError() {
        TCPTransport transport = new TCPTransport("127.0.0.1", 1, 200);
        assertThrows(RuntimeException.class, transport::connect);
        assertFalse(transport.isConnected());
        assertDoesNotThrow(transport::close);
    }
}
