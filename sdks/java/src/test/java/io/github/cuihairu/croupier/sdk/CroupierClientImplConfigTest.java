package io.github.cuihairu.croupier.sdk;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.util.concurrent.CompletableFuture;
import org.junit.jupiter.api.Test;

class CroupierClientImplConfigTest {

    @Test
    void clientWithProductionEnvironment() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setEnv("production");
        CroupierClientImpl client = new CroupierClientImpl(config);

        assertNotNull(client);
        assertEquals("production", config.getEnv());
    }

    @Test
    void clientWithStagingEnvironment() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setEnv("staging");
        CroupierClientImpl client = new CroupierClientImpl(config);

        assertNotNull(client);
        assertEquals("staging", config.getEnv());
    }

    @Test
    void clientWithCustomServiceVersion() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setServiceVersion("2.5.0");
        CroupierClientImpl client = new CroupierClientImpl(config);

        assertNotNull(client);
        assertEquals("2.5.0", config.getServiceVersion());
    }

    @Test
    void clientWithControlAddress() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setControlAddr("localhost:8080");
        CroupierClientImpl client = new CroupierClientImpl(config);

        assertNotNull(client);
        assertEquals("localhost:8080", config.getControlAddr());
    }

    @Test
    void clientWithTimeout() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setTimeoutSeconds(120);
        CroupierClientImpl client = new CroupierClientImpl(config);

        assertNotNull(client);
        assertEquals(120, config.getTimeoutSeconds());
    }

    @Test
    void clientWithTlsSettings() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setInsecure(false);
        config.setCaFile("/ca.pem");
        config.setCertFile("/cert.pem");
        config.setKeyFile("/key.pem");
        CroupierClientImpl client = new CroupierClientImpl(config);

        assertNotNull(client);
        assertFalse(config.isInsecure());
        assertEquals("/ca.pem", config.getCaFile());
        assertEquals("/cert.pem", config.getCertFile());
        assertEquals("/key.pem", config.getKeyFile());
    }

    @Test
    void clientWithAgentId() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setAgentId("agent-123");
        CroupierClientImpl client = new CroupierClientImpl(config);

        assertNotNull(client);
        assertEquals("agent-123", config.getAgentId());
    }

    @Test
    void clientWithProviderInfo() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setProviderLang("java");
        config.setProviderSdk("croupier-java-sdk");
        CroupierClientImpl client = new CroupierClientImpl(config);

        assertNotNull(client);
        assertEquals("java", config.getProviderLang());
        assertEquals("croupier-java-sdk", config.getProviderSdk());
    }
}
