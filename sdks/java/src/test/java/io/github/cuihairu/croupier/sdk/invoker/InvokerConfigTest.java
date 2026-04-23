package io.github.cuihairu.croupier.sdk.invoker;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.DisplayName;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Tests for InvokerConfig.
 */
class InvokerConfigTest {

    @Test
    @DisplayName("Builder should create config with all fields")
    void testBuilderAllFields() {
        RetryConfig retryConfig = RetryConfig.builder()
                .enabled(true)
                .maxAttempts(3)
                .initialDelayMs(100)
                .build();

        ReconnectConfig reconnectConfig = ReconnectConfig.builder()
                .enabled(true)
                .maxAttempts(5)
                .initialDelayMs(1000)
                .build();

        InvokerConfig config = InvokerConfig.builder()
                .address("localhost:19090")
                .insecure(false)
                .timeout(60000)
                .retry(retryConfig)
                .reconnect(reconnectConfig)
                .build();

        assertEquals("localhost:19090", config.getAddress());
        assertFalse(config.isInsecure());
        assertEquals(60000, config.getTimeout());
        assertNotNull(config.getRetry());
        assertNotNull(config.getReconnect());
    }

    @Test
    @DisplayName("Builder should have sensible defaults")
    void testBuilderDefaults() {
        InvokerConfig config = InvokerConfig.builder().build();

        assertEquals("127.0.0.1:19090", config.getAddress());
        assertTrue(config.isInsecure());
        assertEquals(30000, config.getTimeout());
    }

    @Test
    @DisplayName("createDefault should return valid config")
    void testCreateDefault() {
        InvokerConfig config = InvokerConfig.createDefault();

        assertNotNull(config);
        assertEquals("127.0.0.1:19090", config.getAddress());
        assertTrue(config.isInsecure());
    }

    @Test
    @DisplayName("Builder with partial values should use defaults for unset fields")
    void testBuilderPartialValues() {
        InvokerConfig config = InvokerConfig.builder()
                .address("192.168.1.1:8080")
                .build();

        assertEquals("192.168.1.1:8080", config.getAddress());
        assertTrue(config.isInsecure()); // default
        assertEquals(30000, config.getTimeout()); // default
    }

    @Test
    @DisplayName("Config should be equal for same values")
    void testEquals() {
        InvokerConfig config1 = InvokerConfig.builder()
                .address("localhost:19090")
                .insecure(true)
                .build();

        InvokerConfig config2 = InvokerConfig.builder()
                .address("localhost:19090")
                .insecure(true)
                .build();

        assertEquals(config1, config2);
    }

    @Test
    @DisplayName("Config hashCode should be consistent")
    void testHashCode() {
        InvokerConfig config1 = InvokerConfig.builder()
                .address("localhost:19090")
                .build();

        InvokerConfig config2 = InvokerConfig.builder()
                .address("localhost:19090")
                .build();

        assertEquals(config1.hashCode(), config2.hashCode());
    }

    @Test
    @DisplayName("toString should contain address")
    void testToString() {
        InvokerConfig config = InvokerConfig.builder()
                .address("localhost:19090")
                .build();

        String str = config.toString();
        assertTrue(str.contains("localhost:19090"));
    }
}
