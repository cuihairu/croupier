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
        assertEquals("127.0.0.1:19091", config.getAgentAddr());
        assertTrue(config.isInsecure());
    }

    @Test
    void settersAndGettersWork() {
        ClientConfig config = new ClientConfig("game1", "svc1");

        config.setAgentAddr("localhost:9999");
        assertEquals("localhost:9999", config.getAgentAddr());

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

        config.setTimeoutSeconds(60);
        assertEquals(60, config.getTimeoutSeconds());

        config.setInsecure(false);
        assertFalse(config.isInsecure());

        config.setCaFile("/path/to/ca.pem");
        assertEquals("/path/to/ca.pem", config.getCaFile());

        config.setCertFile("/path/to/cert.pem");
        assertEquals("/path/to/cert.pem", config.getCertFile());

        config.setKeyFile("/path/to/key.pem");
        assertEquals("/path/to/key.pem", config.getKeyFile());
    }

    @Test
    void constructorWithAllParameters() {
        ClientConfig config = new ClientConfig("game2", "svc2");

        config.setAgentAddr("custom.agent.com:8888");
        config.setEnv("staging");
        config.setServiceVersion("1.5.0");
        config.setTimeoutSeconds(45);
        config.setInsecure(true);

        assertEquals("game2", config.getGameId());
        assertEquals("svc2", config.getServiceId());
        assertEquals("custom.agent.com:8888", config.getAgentAddr());
        assertEquals("staging", config.getEnv());
        assertEquals("1.5.0", config.getServiceVersion());
        assertEquals(45, config.getTimeoutSeconds());
        assertTrue(config.isInsecure());
    }

    @Test
    void testAgentIdSetter() {
        ClientConfig config = new ClientConfig("game1", "svc1");

        assertNull(config.getAgentId());

        config.setAgentId("agent-123");
        assertEquals("agent-123", config.getAgentId());
    }

    @Test
    void defaultValues() {
        ClientConfig config = new ClientConfig("game3", "svc3");

        assertEquals("development", config.getEnv());
        assertEquals("127.0.0.1:19091", config.getAgentAddr());
        assertEquals(30, config.getTimeoutSeconds());
        assertTrue(config.isInsecure());
        assertEquals("1.0.0", config.getServiceVersion());
        assertNull(config.getAgentId());
        assertNull(config.getControlAddr());
        assertNull(config.getCaFile());
        assertNull(config.getCertFile());
        assertNull(config.getKeyFile());
    }

    @Test
    void testControlAddr() {
        ClientConfig config = new ClientConfig("game1", "svc1");

        config.setControlAddr("localhost:8080");
        assertEquals("localhost:8080", config.getControlAddr());
    }
}
