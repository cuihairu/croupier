package io.github.cuihairu.croupier.sdk;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import org.junit.jupiter.api.Test;

class ClientConfigTest {

    @Test
    void constructorWithGameIdAndServiceId() {
        ClientConfig config = new ClientConfig("game1", "svc1");

        assertEquals("game1", config.getGameId());
        assertEquals("svc1", config.getServiceId());
        assertEquals("development", config.getEnv());
        assertEquals("127.0.0.1:19090", config.getAgentAddr());
        assertTrue(config.isInsecure());
        assertEquals("127.0.0.1:0", config.getLocalListen());
    }

    @Test
    void settersAndGettersWork() {
        ClientConfig config = new ClientConfig("game1", "svc1");

        config.setAgentAddr("localhost:9999");
        assertEquals("localhost:9999", config.getAgentAddr());

        config.setLocalListen("0.0.0.0:8888");
        assertEquals("0.0.0.0:8888", config.getLocalListen());

        config.setEnv("production");
        assertEquals("production", config.getEnv());

        config.setServiceVersion("2.0.0");
        assertEquals("2.0.0", config.getServiceVersion());

        config.setProviderLang("java");
        assertEquals("java", config.getProviderLang());

        config.setProviderSdk("croupier-sdk");
        assertEquals("croupier-sdk", config.getProviderSdk());

        config.setControlAddr("localhost:8080");
        assertEquals("localhost:8080", config.getControlAddr());

        config.setTimeoutSeconds(30);
        assertEquals(30, config.getTimeoutSeconds());

        config.setInsecure(true);
        assertTrue(config.isInsecure());

        config.setCaFile("/path/to/ca.pem");
        assertEquals("/path/to/ca.pem", config.getCaFile());

        config.setCertFile("/path/to/cert.pem");
        assertEquals("/path/to/cert.pem", config.getCertFile());

        config.setKeyFile("/path/to/key.pem");
        assertEquals("/path/to/key.pem", config.getKeyFile());
    }

    @Test
    void toStringContainsGameAndServiceIds() {
        ClientConfig config = new ClientConfig("test-game", "test-svc");
        String str = config.toString();

        assertTrue(str.contains("test-game"));
        assertTrue(str.contains("test-svc"));
    }

    @Test
    void configWithProductionEnv() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setEnv("production");

        assertEquals("production", config.getEnv());
    }

    @Test
    void configWithStagingEnv() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setEnv("staging");

        assertEquals("staging", config.getEnv());
    }

    @Test
    void configWithCustomTimeout() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setTimeoutSeconds(60);

        assertEquals(60, config.getTimeoutSeconds());
    }

    @Test
    void configWithTlsFiles() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setCaFile("/ca.pem");
        config.setCertFile("/cert.pem");
        config.setKeyFile("/key.pem");

        assertEquals("/ca.pem", config.getCaFile());
        assertEquals("/cert.pem", config.getCertFile());
        assertEquals("/key.pem", config.getKeyFile());
        assertTrue(config.isInsecure());
    }

    @Test
    void multipleConfigInstances() {
        ClientConfig config1 = new ClientConfig("game1", "svc1");
        ClientConfig config2 = new ClientConfig("game2", "svc2");

        assertEquals("game1", config1.getGameId());
        assertEquals("game2", config2.getGameId());
    }

    // ========== New Field Tests ==========

    @Test
    void agentIdCanBeSet() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setAgentId("agent-123");

        assertEquals("agent-123", config.getAgentId());
    }

    @Test
    void serverNameCanBeSet() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setServerName("example.com");

        assertEquals("example.com", config.getServerName());
    }

    @Test
    void authTokenCanBeSet() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setAuthToken("Bearer token123");

        assertEquals("Bearer token123", config.getAuthToken());
    }

    @Test
    void headersCanBeSet() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.getHeaders().put("X-Custom-Header", "value");
        config.getHeaders().put("X-Api-Key", "key123");

        assertEquals("value", config.getHeaders().get("X-Custom-Header"));
        assertEquals("key123", config.getHeaders().get("X-Api-Key"));
        assertEquals(2, config.getHeaders().size());
    }

    @Test
    void heartbeatIntervalCanBeSet() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setHeartbeatInterval(120);

        assertEquals(120, config.getHeartbeatInterval());
    }

    @Test
    void defaultHeartbeatIntervalIs60() {
        ClientConfig config = new ClientConfig("game1", "svc1");

        assertEquals(60, config.getHeartbeatInterval());
    }

    @Test
    void reconnectConfigCanBeSet() {
        ReconnectConfig reconnectConfig = ReconnectConfig.builder()
            .maxAttempts(5)
            .initialDelayMs(2000)
            .build();

        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setReconnect(reconnectConfig);

        assertEquals(reconnectConfig, config.getReconnect());
        assertEquals(5, config.getReconnect().getMaxAttempts());
        assertEquals(2000, config.getReconnect().getInitialDelayMs());
    }

    @Test
    void reconnectConfigDefaultsToNull() {
        ClientConfig config = new ClientConfig("game1", "svc1");

        // ReconnectConfig is nullable and defaults to null
        // Application code should handle null appropriately
        assertNull(config.getReconnect());
    }

    @Test
    void fileTransferCanBeEnabled() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setEnableFileTransfer(true);

        assertTrue(config.isEnableFileTransfer());
    }

    @Test
    void fileTransferIsDisabledByDefault() {
        ClientConfig config = new ClientConfig("game1", "svc1");

        assertFalse(config.isEnableFileTransfer());
    }

    @Test
    void maxFileSizeCanBeSet() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setMaxFileSize(52428800);  // 50MB

        assertEquals(52428800, config.getMaxFileSize());
    }

    @Test
    void defaultMaxFileSizeIs10MB() {
        ClientConfig config = new ClientConfig("game1", "svc1");

        assertEquals(10485760, config.getMaxFileSize());
    }

    @Test
    void loggingConfigCanBeSet() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setDisableLogging(true);
        config.setDebugLogging(true);
        config.setLogLevel("DEBUG");

        assertTrue(config.isDisableLogging());
        assertTrue(config.isDebugLogging());
        assertEquals("DEBUG", config.getLogLevel());
    }

    @Test
    void defaultLoggingConfig() {
        ClientConfig config = new ClientConfig("game1", "svc1");

        assertFalse(config.isDisableLogging());
        assertFalse(config.isDebugLogging());
        assertEquals("INFO", config.getLogLevel());
    }

    @Test
    void completeConfigurationExample() {
        ClientConfig config = new ClientConfig("game1", "svc1");
        config.setAgentAddr("localhost:9999");
        config.setAgentId("agent-123");
        config.setLocalListen("0.0.0.0:8888");
        config.setControlAddr("localhost:8080");
        config.setTimeoutSeconds(60);
        config.setInsecure(false);
        config.setHeartbeatInterval(120);
        config.setServerName("example.com");
        config.setAuthToken("Bearer token123");
        config.getHeaders().put("X-Custom", "value");
        config.setEnableFileTransfer(true);
        config.setMaxFileSize(52428800);
        config.setDisableLogging(false);
        config.setDebugLogging(true);
        config.setLogLevel("DEBUG");

        ReconnectConfig reconnectConfig = ReconnectConfig.builder()
            .maxAttempts(10)
            .build();
        config.setReconnect(reconnectConfig);

        assertEquals("localhost:9999", config.getAgentAddr());
        assertEquals("agent-123", config.getAgentId());
        assertEquals("0.0.0.0:8888", config.getLocalListen());
        assertEquals("localhost:8080", config.getControlAddr());
        assertEquals(60, config.getTimeoutSeconds());
        assertFalse(config.isInsecure());
        assertEquals(120, config.getHeartbeatInterval());
        assertEquals("example.com", config.getServerName());
        assertEquals("Bearer token123", config.getAuthToken());
        assertEquals("value", config.getHeaders().get("X-Custom"));
        assertTrue(config.isEnableFileTransfer());
        assertEquals(52428800, config.getMaxFileSize());
        assertTrue(config.isDebugLogging());
        assertEquals("DEBUG", config.getLogLevel());
        assertEquals(10, config.getReconnect().getMaxAttempts());
    }
}
