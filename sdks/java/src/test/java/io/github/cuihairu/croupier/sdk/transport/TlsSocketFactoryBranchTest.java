package io.github.cuihairu.croupier.sdk.transport;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Timeout;

import javax.net.ssl.SSLSocketFactory;
import java.io.OutputStream;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.concurrent.TimeUnit;

import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;

/** Branch fillers for TlsSocketFactory mTLS guard combinations. */
class TlsSocketFactoryBranchTest {

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

    @Test
    @Timeout(60)
    void certWithoutKeyAndKeyWithoutCertSkipMutualTls() throws Exception {
        Path dir = Files.createTempDirectory("croupier-tls-guards");
        Path ca = exportCa(dir);

        // certFile present + keyFile blank -> CA-only context
        SSLSocketFactory certOnly = TlsSocketFactory.create(ca.toString(), ca.toString(), null);
        assertNotNull(certOnly);

        // certFile blank + keyFile present -> CA-only context (cert guard short-circuits)
        SSLSocketFactory keyOnly = TlsSocketFactory.create(ca.toString(), null, "/tmp/whatever.key");
        assertNotNull(keyOnly);

        // blank certFile value also short-circuits
        SSLSocketFactory blankCert = TlsSocketFactory.create(ca.toString(), "  ", "/tmp/whatever.key");
        assertNotNull(blankCert);

        // missing CA still fails fast
        assertThrows(Exception.class, () -> TlsSocketFactory.create(null, null, null));
    }
}
