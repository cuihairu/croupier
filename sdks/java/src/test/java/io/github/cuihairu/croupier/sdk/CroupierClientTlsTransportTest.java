package io.github.cuihairu.croupier.sdk;

import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.transport.TCPTransport;
import io.github.cuihairu.croupier.sdk.transport.TlsSocketFactory;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Timeout;
import org.junit.jupiter.api.io.TempDir;

import javax.net.ssl.KeyManagerFactory;
import javax.net.ssl.SSLContext;
import javax.net.ssl.SSLServerSocket;
import javax.net.ssl.SSLSocketFactory;
import java.io.InputStream;
import java.lang.reflect.Field;
import java.lang.reflect.Method;
import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.KeyStore;
import java.security.PrivateKey;
import java.security.cert.CertificateFactory;
import java.util.ArrayList;
import java.util.Base64;
import java.util.List;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.function.BiFunction;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * AUD-107: the default transport factory must honor ClientConfig security
 * settings — insecure=false builds the PEM TLS factory with peer-name
 * verification for the first connect and every reconnect, while the dev
 * default (insecure=true) stays plaintext. Wrong CA or wrong server name
 * must be rejected.
 */
@DisplayName("Default transport honors ClientConfig TLS settings (AUD-107)")
class CroupierClientTlsTransportTest {

    private static Path caPem;
    private static Path otherCaPem;
    private static Path serverJks;
    private static Path clientCaPem;
    private static Path clientChainPem;
    private static Path clientKeyPem;

    @BeforeAll
    static void generateCerts(@TempDir Path dir) throws Exception {
        char[] pass = "changeit".toCharArray();

        // CA #1 (trusted) and CA #2 (used as an untrusted anchor)
        Path caJks = dir.resolve("ca.jks");
        keytool("-genkeypair", "-alias", "ca", "-keyalg", "RSA", "-keysize", "2048",
            "-dname", "CN=aud107-ca", "-ext", "bc:c", "-validity", "1",
            "-keystore", caJks.toString(), "-storetype", "JKS",
            "-storepass", "changeit", "-keypass", "changeit");
        caPem = dir.resolve("ca.pem");
        exportCert(caJks, "ca", caPem);

        Path ca2Jks = dir.resolve("ca2.jks");
        keytool("-genkeypair", "-alias", "ca2", "-keyalg", "RSA", "-keysize", "2048",
            "-dname", "CN=aud107-other-ca", "-ext", "bc:c", "-validity", "1",
            "-keystore", ca2Jks.toString(), "-storetype", "JKS",
            "-storepass", "changeit", "-keypass", "changeit");
        otherCaPem = dir.resolve("ca2.pem");
        exportCert(ca2Jks, "ca2", otherCaPem);

        // Server cert signed by CA #1, SAN covering localhost
        serverJks = dir.resolve("server.jks");
        keytool("-genkeypair", "-alias", "server", "-keyalg", "RSA", "-keysize", "2048",
            "-dname", "CN=localhost", "-ext", "san=dns:localhost,ip:127.0.0.1", "-validity", "1",
            "-keystore", serverJks.toString(), "-storetype", "JKS",
            "-storepass", "changeit", "-keypass", "changeit");
        Path serverPem = issueAndChain(dir, "server", serverJks, caJks, caPem, "san=dns:localhost,ip:127.0.0.1");
        importChain(serverJks, "server", serverPem);

        // mTLS client cert signed by CA #1; key exported as PKCS#8 PEM in Java
        Path clientJks = dir.resolve("client.jks");
        keytool("-genkeypair", "-alias", "client", "-keyalg", "RSA", "-keysize", "2048",
            "-dname", "CN=aud107-client", "-validity", "1",
            "-keystore", clientJks.toString(), "-storetype", "JKS",
            "-storepass", "changeit", "-keypass", "changeit");
        clientChainPem = issueAndChain(dir, "client", clientJks, caJks, caPem, null);
        clientCaPem = caPem;
        KeyStore ks = KeyStore.getInstance("JKS");
        try (InputStream in = Files.newInputStream(clientJks)) {
            ks.load(in, pass);
        }
        PrivateKey key = (PrivateKey) ks.getKey("client", pass);
        clientKeyPem = dir.resolve("client-key.pem");
        Files.writeString(clientKeyPem, pem("PRIVATE KEY", key.getEncoded()));
    }

    private static Path issueAndChain(Path dir, String alias, Path jks, Path caJks,
                                      Path caCert, String ext) throws Exception {
        Path csr = dir.resolve(alias + ".csr");
        keytool("-certreq", "-alias", alias, "-keystore", jks.toString(),
            "-storetype", "JKS", "-storepass", "changeit", "-file", csr.toString());
        List<String> gencert = new ArrayList<>(List.of("-gencert", "-alias", "ca",
            "-keystore", caJks.toString(), "-storetype", "JKS", "-storepass", "changeit",
            "-rfc", "-infile", csr.toString(), "-outfile", dir.resolve(alias + "-cert.pem").toString()));
        if (ext != null) {
            gencert.add("-ext");
            gencert.add(ext);
        }
        keytool(gencert.toArray(new String[0]));
        // chain file = leaf cert + CA cert (PEM concat)
        Path chain = dir.resolve(alias + "-chain.pem");
        Files.writeString(chain,
            Files.readString(dir.resolve(alias + "-cert.pem")) + Files.readString(caCert));
        return chain;
    }

    private static void importChain(Path jks, String alias, Path chain) throws Exception {
        keytool("-importcert", "-alias", alias, "-file", chain.toString(),
            "-keystore", jks.toString(), "-storetype", "JKS",
            "-storepass", "changeit", "-noprompt");
    }

    private static void exportCert(Path jks, String alias, Path out) throws Exception {
        keytool("-exportcert", "-alias", alias, "-keystore", jks.toString(),
            "-storetype", "JKS", "-storepass", "changeit", "-rfc", "-file", out.toString());
    }

    private static void keytool(String... args) throws Exception {
        List<String> cmd = new ArrayList<>();
        cmd.add("keytool");
        for (String arg : args) {
            cmd.add(arg);
        }
        Process p = new ProcessBuilder(cmd).redirectErrorStream(true).start();
        String output = new String(p.getInputStream().readAllBytes(), StandardCharsets.UTF_8);
        if (!p.waitFor(30, TimeUnit.SECONDS) || p.exitValue() != 0) {
            throw new IllegalStateException("keytool failed: " + String.join(" ", args) + "\n" + output);
        }
    }

    private static String pem(String label, byte[] der) {
        return "-----BEGIN " + label + "-----\n"
            + Base64.getMimeEncoder().encodeToString(der)
            + "\n-----END " + label + "-----\n";
    }

    /** TLS fake agent: answers one ProviderConnectRequest over the accepted socket. */
    private static final class FakeTlsAgent {
        private final SSLServerSocket serverSocket;
        private final ExecutorService pool = Executors.newSingleThreadExecutor();

        FakeTlsAgent(SSLServerSocket serverSocket) {
            this.serverSocket = serverSocket;
        }

        int port() {
            return serverSocket.getLocalPort();
        }

        void start() {
            pool.submit(() -> {
                try (var socket = serverSocket.accept()) {
                    socket.setSoTimeout(10000);
                    ((javax.net.ssl.SSLSocket) socket).startHandshake();
                    InputStream in = socket.getInputStream();
                    byte[] header = in.readNBytes(4);
                    int frameSize = ByteBuffer.wrap(header).order(ByteOrder.BIG_ENDIAN).getInt();
                    byte[] payload = in.readNBytes(frameSize);
                    int reqId = ByteBuffer.wrap(payload, 4, 4).order(ByteOrder.BIG_ENDIAN).getInt();
                    byte[] body = SdkWireMessages.encodeProviderConnectResponse(
                        new SdkWireMessages.ProviderConnectResponse("tls-session-1"));
                    byte[] frame = new byte[12 + body.length];
                    ByteBuffer.wrap(frame, 0, 4).order(ByteOrder.BIG_ENDIAN).putInt(8 + body.length);
                    frame[4] = 0x01;
                    int msgType = Protocol.MSG_PROVIDER_CONNECT_RESPONSE;
                    frame[5] = (byte) (msgType >> 16);
                    frame[6] = (byte) (msgType >> 8);
                    frame[7] = (byte) msgType;
                    ByteBuffer.wrap(frame, 8, 4).order(ByteOrder.BIG_ENDIAN).putInt(reqId);
                    System.arraycopy(body, 0, frame, 12, body.length);
                    socket.getOutputStream().write(frame);
                    socket.getOutputStream().flush();
                    Thread.sleep(300);
                } catch (Exception ignored) {
                    // handshake/refusal failures are asserted client-side
                }
            });
        }

        void close() {
            pool.shutdownNow();
            try {
                serverSocket.close();
            } catch (Exception ignored) {
            }
        }
    }

    private static SSLServerSocket openServer(Path trustCa, boolean needClientAuth) throws Exception {
        KeyStore serverKeyStore = KeyStore.getInstance("JKS");
        try (InputStream in = Files.newInputStream(serverJks)) {
            serverKeyStore.load(in, "changeit".toCharArray());
        }
        KeyManagerFactory kmf = KeyManagerFactory.getInstance(KeyManagerFactory.getDefaultAlgorithm());
        kmf.init(serverKeyStore, "changeit".toCharArray());

        javax.net.ssl.TrustManagerFactory tmf = null;
        if (needClientAuth) {
            KeyStore trust = KeyStore.getInstance(KeyStore.getDefaultType());
            trust.load(null, null);
            var cf = CertificateFactory.getInstance("X.509");
            trust.setCertificateEntry("ca", cf.generateCertificate(Files.newInputStream(trustCa)));
            tmf = javax.net.ssl.TrustManagerFactory.getInstance(
                javax.net.ssl.TrustManagerFactory.getDefaultAlgorithm());
            tmf.init(trust);
        }

        SSLContext context = SSLContext.getInstance("TLS");
        context.init(kmf.getKeyManagers(), tmf == null ? null : tmf.getTrustManagers(), null);
        SSLServerSocket socket = (SSLServerSocket)
            context.getServerSocketFactory().createServerSocket(0);
        socket.setNeedClientAuth(needClientAuth);
        return socket;
    }

    private static ClientConfig secureConfig(Path caFile, String serverName, int port) {
        ClientConfig config = new ClientConfig("game-1", "svc-1");
        config.setEnv("development");
        config.setInsecure(false);
        config.setCaFile(caFile.toString());
        config.setServerName(serverName);
        config.setAgentAddr("localhost:" + port);
        config.setTimeoutSeconds(10);
        return config;
    }

    private static ClientConfig devConfig() {
        ClientConfig config = new ClientConfig("game-1", "svc-1");
        config.setEnv("development");
        return config;
    }

    @SuppressWarnings("unchecked")
    private static BiFunction<String, Integer, io.github.cuihairu.croupier.sdk.transport.TransportClient>
    defaultFactory(ClientConfig config) throws Exception {
        Method factory = CroupierClientImpl.class.getDeclaredMethod(
            "createTransportFactory", ClientConfig.class);
        factory.setAccessible(true);
        return (BiFunction<String, Integer, io.github.cuihairu.croupier.sdk.transport.TransportClient>)
            factory.invoke(null, config);
    }

    private static Object transportField(Object transport, String name) throws Exception {
        Field f = TCPTransport.class.getDeclaredField(name);
        f.setAccessible(true);
        return f.get(transport);
    }

    private static List<String> causeMessages(Throwable failure) {
        List<String> messages = new ArrayList<>();
        for (Throwable t = failure; t != null; t = t.getCause()) {
            messages.add(String.valueOf(t.getMessage()));
            if (t.getCause() == t) {
                break;
            }
        }
        return messages;
    }

    @Test
    @Timeout(60)
    @DisplayName("insecure=false: correct CA and server name register over TLS")
    void secureModeRegistersOverTlsWithTrustedCa() throws Exception {
        SSLServerSocket server = openServer(null, false);
        FakeTlsAgent agent = new FakeTlsAgent(server);
        try {
            agent.start();
            ClientConfig config = secureConfig(caPem, "localhost", agent.port());
            CroupierClientImpl client = new CroupierClientImpl(config);
            client.registerFunction(new FunctionDescriptor("demo.echo", "1.0.0"),
                (context, payload) -> payload);
            client.connect().get(20, TimeUnit.SECONDS);
            assertTrue(client.isConnected(), "TLS session with trusted CA must register");
            client.close();
        } finally {
            agent.close();
        }
    }

    @Test
    @Timeout(60)
    @DisplayName("insecure=false: wrong server name is rejected")
    void wrongServerNameIsRejected() throws Exception {
        SSLServerSocket server = openServer(null, false);
        FakeTlsAgent agent = new FakeTlsAgent(server);
        try {
            agent.start();
            ClientConfig config = secureConfig(caPem, "wrong.example", agent.port());
            CroupierClientImpl client = new CroupierClientImpl(config);
            client.registerFunction(new FunctionDescriptor("demo.echo", "1.0.0"),
                (context, payload) -> payload);
            ExecutionException failure = assertThrows(ExecutionException.class,
                () -> client.connect().get(20, TimeUnit.SECONDS));
            assertTrue(causeMessages(failure).stream()
                    .anyMatch(m -> m.contains("does not match serverName")),
                "peer-name mismatch must surface: " + causeMessages(failure));
            assertFalse(client.isConnected());
            client.close();
        } finally {
            agent.close();
        }
    }

    @Test
    @Timeout(60)
    @DisplayName("insecure=false: untrusted CA is rejected")
    void untrustedCaIsRejected() throws Exception {
        SSLServerSocket server = openServer(null, false);
        FakeTlsAgent agent = new FakeTlsAgent(server);
        try {
            agent.start();
            ClientConfig config = secureConfig(otherCaPem, "localhost", agent.port());
            CroupierClientImpl client = new CroupierClientImpl(config);
            client.registerFunction(new FunctionDescriptor("demo.echo", "1.0.0"),
                (context, payload) -> payload);
            ExecutionException failure = assertThrows(ExecutionException.class,
                () -> client.connect().get(20, TimeUnit.SECONDS));
            assertTrue(causeMessages(failure).stream()
                    .anyMatch(m -> m.contains("unable to find valid certification path") || m.contains("PKIX")),
                "missing trust anchor must surface: " + causeMessages(failure));
            assertFalse(client.isConnected());
            client.close();
        } finally {
            agent.close();
        }
    }

    @Test
    @Timeout(60)
    @DisplayName("insecure=false: mTLS presents client certificate when configured")
    void mutualTlsPresentsClientCertificate() throws Exception {
        SSLServerSocket server = openServer(caPem, true);
        FakeTlsAgent agent = new FakeTlsAgent(server);
        try {
            agent.start();
            SSLSocketFactory factory = TlsSocketFactory.create(
                clientCaPem.toString(), clientChainPem.toString(), clientKeyPem.toString(),
                "changeit".toCharArray());
            TCPTransport transport = new TCPTransport("localhost", agent.port(), 10000,
                factory, "localhost");
            transport.connect();
            assertTrue(transport.isConnected(), "server must accept the client certificate");
            transport.close();
        } finally {
            agent.close();
        }
    }

    @Test
    @Timeout(30)
    @DisplayName("dev default (insecure=true) keeps plaintext transport")
    void insecureModeKeepsPlaintextTransport() throws Exception {
        var transport = defaultFactory(devConfig()).apply("localhost:19091", 1500);
        assertTrue(transport instanceof TCPTransport);
        assertNullField(transport, "socketFactory", "plaintext dev mode must not carry a TLS factory");
    }

    @Test
    @Timeout(30)
    @DisplayName("insecure=false factory builds TLS transport with server name")
    void secureModeFactoryBuildsTlsTransport() throws Exception {
        ClientConfig config = devConfig();
        config.setInsecure(false);
        config.setCaFile(caPem.toString());
        config.setServerName("agent.internal");
        var transport = defaultFactory(config).apply("localhost:19091", 1500);
        assertTrue(transport instanceof TCPTransport);
        assertNotNull(transportField(transport, "socketFactory"),
            "secure mode must install the TLS factory");
        assertEquals("agent.internal", transportField(transport, "serverName"),
            "serverName must reach the peer-name verification");
    }

    @Test
    @Timeout(30)
    @DisplayName("insecure=false without caFile fails fast per dial")
    void secureModeWithoutCaFailsFast() throws Exception {
        ClientConfig config = devConfig();
        config.setInsecure(false);
        IllegalStateException failure = assertThrows(IllegalStateException.class,
            () -> defaultFactory(config).apply("localhost:19091", 1500));
        assertTrue(failure.getMessage().contains("caFile"), failure.getMessage());
    }

    private static void assertNullField(Object transport, String name, String message) throws Exception {
        assertTrue(transportField(transport, name) == null, message);
    }
}
