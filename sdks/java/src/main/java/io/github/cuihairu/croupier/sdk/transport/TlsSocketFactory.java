package io.github.cuihairu.croupier.sdk.transport;

import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.GeneralSecurityException;
import java.security.KeyStore;
import java.security.PrivateKey;
import java.security.SecureRandom;
import java.security.cert.Certificate;
import java.security.cert.CertificateFactory;
import java.util.ArrayList;
import java.util.Base64;
import java.util.List;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

import javax.net.ssl.KeyManager;
import javax.net.ssl.KeyManagerFactory;
import javax.net.ssl.SSLContext;
import javax.net.ssl.SSLSocketFactory;
import javax.net.ssl.TrustManagerFactory;

/**
 * Builds an {@link SSLSocketFactory} from PEM files for secure
 * Agent/Server connections (ClientConfig TLS settings, insecure=false).
 *
 * <p>Supported inputs:</p>
 * <ul>
 *   <li>{@code caFile} — PEM CA certificate chain used to verify the peer
 *       (required for non-insecure sessions).</li>
 *   <li>{@code certFile} + {@code keyFile} — optional PEM client certificate
 *       and PKCS#8 private key for mutual TLS.</li>
 * </ul>
 */
public final class TlsSocketFactory {

    private static final Pattern PEM_BODY =
        Pattern.compile("-----BEGIN [A-Z ]+-----(.*?)-----END [A-Z ]+-----", Pattern.DOTALL);

    private TlsSocketFactory() {
    }

    /**
     * Creates a socket factory from PEM files. At least {@code caFile} must
     * be provided and readable; mutual TLS is enabled only when both
     * {@code certFile} and {@code keyFile} are present.
     */
    public static SSLSocketFactory create(String caFile, String certFile, String keyFile)
            throws IOException, GeneralSecurityException {
        if (caFile == null || caFile.isBlank()) {
            throw new IOException("TLS enabled but caFile is not configured");
        }

        CertificateFactory certFactory = CertificateFactory.getInstance("X.509");

        // Trust store：CA 链
        KeyStore trustStore = KeyStore.getInstance(KeyStore.getDefaultType());
        trustStore.load(null, null);
        List<Certificate> cas = readCertificates(caFile, certFactory);
        if (cas.isEmpty()) {
            throw new IOException("no certificates found in caFile: " + caFile);
        }
        char[] password = "croupier".toCharArray();
        int alias = 0;
        for (Certificate ca : cas) {
            trustStore.setCertificateEntry("ca-" + (alias++), ca);
        }
        TrustManagerFactory tmf = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm());
        tmf.init(trustStore);

        // Key store：mTLS 客户端证书（可选）
        KeyManager[] keyManagers = null;
        if (certFile != null && !certFile.isBlank() && keyFile != null && !keyFile.isBlank()) {
            List<Certificate> clientChain = readCertificates(certFile, certFactory);
            PrivateKey privateKey = readPrivateKey(keyFile);
            KeyStore keyStore = KeyStore.getInstance(KeyStore.getDefaultType());
            keyStore.load(null, null);
            keyStore.setKeyEntry("client", privateKey, password, clientChain.toArray(new Certificate[0]));
            KeyManagerFactory kmf = KeyManagerFactory.getInstance(KeyManagerFactory.getDefaultAlgorithm());
            kmf.init(keyStore, password);
            keyManagers = kmf.getKeyManagers();
        }

        SSLContext context = SSLContext.getInstance("TLS");
        context.init(keyManagers, tmf.getTrustManagers(), new SecureRandom());
        return context.getSocketFactory();
    }

    private static List<Certificate> readCertificates(String file, CertificateFactory factory)
            throws IOException, GeneralSecurityException {
        String pem = Files.readString(Path.of(file), StandardCharsets.UTF_8);
        List<Certificate> out = new ArrayList<>();
        Matcher matcher = PEM_BODY.matcher(pem);
        while (matcher.find()) {
            byte[] der = Base64.getMimeDecoder().decode(matcher.group(1).replaceAll("\\s", ""));
            out.add(factory.generateCertificate(new ByteArrayInputStream(der)));
        }
        return out;
    }

    private static PrivateKey readPrivateKey(String file) throws IOException, GeneralSecurityException {
        String pem = Files.readString(Path.of(file), StandardCharsets.UTF_8);
        Matcher matcher = PEM_BODY.matcher(pem);
        if (!matcher.find()) {
            throw new IOException("no PEM private key found in " + file);
        }
        byte[] der = Base64.getMimeDecoder().decode(matcher.group(1).replaceAll("\\s", ""));
        java.security.KeyFactory keyFactory;
        try {
            keyFactory = java.security.KeyFactory.getInstance("RSA");
            return keyFactory.generatePrivate(new java.security.spec.PKCS8EncodedKeySpec(der));
        } catch (GeneralSecurityException rsaFailure) {
            keyFactory = java.security.KeyFactory.getInstance("EC");
            return keyFactory.generatePrivate(new java.security.spec.PKCS8EncodedKeySpec(der));
        }
    }
}
