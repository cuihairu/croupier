package io.github.cuihairu.croupier.sdk.transport;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Timeout;

import javax.net.ssl.SSLSocketFactory;
import java.lang.reflect.Field;
import java.net.ServerSocket;
import java.net.Socket;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * Final branch fillers: TCPTransport stale-socket guards and the blank-keyFile
 * mTLS combination. All driven through real sockets plus reflection on the
 * internal socket/closing fields (same approach as TCPTransportBranchTest).
 */
class BranchFillFinalTest {

    private static Object field(Object target, String name) throws Exception {
        Field f = target.getClass().getDeclaredField(name);
        f.setAccessible(true);
        return f.get(target);
    }

    private static void setField(Object target, String name, Object value) throws Exception {
        Field f = target.getClass().getDeclaredField(name);
        f.setAccessible(true);
        f.set(target, value);
    }

    @Test
    @Timeout(30)
    void freshTransportIsDisconnectedWithoutSocket() {
        TCPTransport transport = new TCPTransport("localhost", 1, 500);
        // socket == null short-circuit in isConnected()
        assertFalse(transport.isConnected());
        assertDoesNotThrow(transport::close);
    }

    @Test
    @Timeout(30)
    void staleUnconnectedSocketTriggersReconnectAndStateGuards() throws Exception {
        try (ServerSocket server = new ServerSocket(0)) {
            ExecutorService pool = Executors.newSingleThreadExecutor();
            pool.submit(() -> {
                // accept both the initial and the reconnecting connection
                for (int i = 0; i < 2; i++) {
                    try (Socket ignored = server.accept()) {
                        Thread.sleep(3000);
                    } catch (Exception ignored) {
                    }
                }
                return null;
            });

            TCPTransport transport = new TCPTransport("127.0.0.1", server.getLocalPort(), 3000);
            transport.connect();
            assertTrue(transport.isConnected());

            // Replace the live socket with a never-connected one: keeps the field
            // non-null while isConnected() reports false.
            setField(transport, "socket", new Socket());
            // isConnected(): socket != null && !socket.isConnected() -> false
            assertFalse(transport.isConnected());
            // request() guard: socket non-null but not connected
            assertThrows(IllegalStateException.class,
                () -> transport.request(0x010101, new byte[] {1}));
            // connect(): non-null yet unconnected socket must NOT early-return,
            // it has to dial a fresh connection instead.
            transport.connect();
            assertTrue(transport.isConnected());

            // closing flag alone (socket untouched) also reports disconnected
            setField(transport, "closing", true);
            assertFalse(transport.isConnected());

            assertDoesNotThrow(transport::close);
            pool.shutdownNow();
        }
    }

    @Test
    @Timeout(60)
    void blankKeyFileSkipsMutualTls() throws Exception {
        Path dir = Files.createTempDirectory("croupier-tls-blankkey");
        Path cert = exportCa(dir);
        // certFile present + keyFile non-null but blank -> CA-only context
        SSLSocketFactory factory = TlsSocketFactory.create(cert.toString(), cert.toString(), "   ");
        assertNotNull(factory);
    }

    private static Path exportCa(Path dir) throws Exception {
        Path ks = dir.resolve("ca.jks");
        Process p = new ProcessBuilder(
            "keytool", "-genkeypair", "-alias", "ca", "-keyalg", "RSA", "-keysize", "2048",
            "-dname", "CN=localhost", "-validity", "1",
            "-keystore", ks.toString(), "-storetype", "JKS",
            "-storepass", "changeit", "-keypass", "changeit").start();
        if (!p.waitFor(30, TimeUnit.SECONDS) || p.exitValue() != 0) {
            throw new IllegalStateException("keytool genkeypair failed");
        }
        Path cert = dir.resolve("ca.pem");
        Process export = new ProcessBuilder(
            "keytool", "-exportcert", "-alias", "ca", "-keystore", ks.toString(),
            "-storetype", "JKS", "-storepass", "changeit", "-rfc", "-file", cert.toString()).start();
        if (!export.waitFor(30, TimeUnit.SECONDS) || export.exitValue() != 0) {
            throw new IllegalStateException("keytool exportcert failed");
        }
        return cert;
    }
}
