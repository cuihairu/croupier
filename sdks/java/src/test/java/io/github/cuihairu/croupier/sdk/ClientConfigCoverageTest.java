package io.github.cuihairu.croupier.sdk;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.HashMap;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Tests for ClientConfig getters/setters and toString to improve coverage.
 */
class ClientConfigCoverageTest {

    @Test
    @DisplayName("Default constructor should have default values")
    void defaultConstructor() {
        ClientConfig config = new ClientConfig();

        assertEquals("127.0.0.1:19090", config.getAgentAddr());
        assertNull(config.getAgentId());
        assertNull(config.getGameId());
        assertEquals("development", config.getEnv());
        assertNull(config.getServiceId());
        assertEquals("1.0.0", config.getServiceVersion());
        assertNull(config.getControlAddr());
        assertEquals(30, config.getTimeoutSeconds());
        assertTrue(config.isInsecure());
        assertEquals(60, config.getHeartbeatInterval());
        assertNull(config.getCaFile());
        assertNull(config.getCertFile());
        assertNull(config.getKeyFile());
        assertNull(config.getServerName());
        assertNull(config.getAuthToken());
        assertNotNull(config.getHeaders());
        assertTrue(config.getHeaders().isEmpty());
        assertEquals("java", config.getProviderLang());
        assertEquals("croupier-java-sdk", config.getProviderSdk());
        assertNull(config.getReconnect());
        assertFalse(config.isEnableFileTransfer());
        assertEquals(10485760, config.getMaxFileSize());
        assertFalse(config.isDisableLogging());
        assertFalse(config.isDebugLogging());
        assertEquals("INFO", config.getLogLevel());
    }

    @Test
    @DisplayName("Parameterized constructor should set gameId and serviceId")
    void parameterizedConstructor() {
        ClientConfig config = new ClientConfig("game-1", "svc-1");

        assertEquals("game-1", config.getGameId());
        assertEquals("svc-1", config.getServiceId());
    }

    @Test
    @DisplayName("All setters and getters should work")
    void allSettersAndGetters() {
        ClientConfig config = new ClientConfig();

        config.setAgentAddr("10.0.0.1:19090");
        assertEquals("10.0.0.1:19090", config.getAgentAddr());

        config.setAgentId("agent-1");
        assertEquals("agent-1", config.getAgentId());

        config.setGameId("game-2");
        assertEquals("game-2", config.getGameId());

        config.setEnv("production");
        assertEquals("production", config.getEnv());

        config.setServiceId("svc-2");
        assertEquals("svc-2", config.getServiceId());

        config.setServiceVersion("2.0.0");
        assertEquals("2.0.0", config.getServiceVersion());

        config.setControlAddr("control:8080");
        assertEquals("control:8080", config.getControlAddr());

        config.setTimeoutSeconds(60);
        assertEquals(60, config.getTimeoutSeconds());

        config.setInsecure(false);
        assertFalse(config.isInsecure());

        config.setHeartbeatInterval(30);
        assertEquals(30, config.getHeartbeatInterval());

        config.setCaFile("/path/to/ca.pem");
        assertEquals("/path/to/ca.pem", config.getCaFile());

        config.setCertFile("/path/to/cert.pem");
        assertEquals("/path/to/cert.pem", config.getCertFile());

        config.setKeyFile("/path/to/key.pem");
        assertEquals("/path/to/key.pem", config.getKeyFile());

        config.setServerName("my-server");
        assertEquals("my-server", config.getServerName());

        config.setAuthToken("token-123");
        assertEquals("token-123", config.getAuthToken());

        Map<String, String> headers = new HashMap<>();
        headers.put("X-Custom", "value");
        config.setHeaders(headers);
        assertEquals("value", config.getHeaders().get("X-Custom"));

        config.setProviderLang("go");
        assertEquals("go", config.getProviderLang());

        config.setProviderSdk("croupier-go-sdk");
        assertEquals("croupier-go-sdk", config.getProviderSdk());

        ReconnectConfig reconnect = ReconnectConfig.builder().enabled(true).build();
        config.setReconnect(reconnect);
        assertNotNull(config.getReconnect());

        config.setEnableFileTransfer(true);
        assertTrue(config.isEnableFileTransfer());

        config.setMaxFileSize(5242880);
        assertEquals(5242880, config.getMaxFileSize());

        config.setDisableLogging(true);
        assertTrue(config.isDisableLogging());

        config.setDebugLogging(true);
        assertTrue(config.isDebugLogging());

        config.setLogLevel("DEBUG");
        assertEquals("DEBUG", config.getLogLevel());
    }

    @Test
    @DisplayName("toString should include all key fields")
    void toStringIncludesFields() {
        ClientConfig config = new ClientConfig("game-1", "svc-1");
        config.setEnv("staging");

        String str = config.toString();

        assertTrue(str.contains("ClientConfig{"));
        assertTrue(str.contains("gameId='game-1'"));
        assertTrue(str.contains("serviceId='svc-1'"));
        assertTrue(str.contains("env='staging'"));
    }

    @Test
    @DisplayName("toString with reconnect config")
    void toStringWithReconnect() {
        ClientConfig config = new ClientConfig("g", "s");
        config.setReconnect(ReconnectConfig.builder().enabled(true).build());

        String str = config.toString();
        assertTrue(str.contains("reconnect="));
    }
}
