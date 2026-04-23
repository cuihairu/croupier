package io.github.cuihairu.croupier.sdk.invoker;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.DisplayName;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Tests for ReconnectConfig.
 */
class ReconnectConfigTest {

    @Test
    @DisplayName("Builder should create config with all fields")
    void testBuilderAllFields() {
        ReconnectConfig config = ReconnectConfig.builder()
                .enabled(true)
                .maxAttempts(10)
                .initialDelayMs(1000)
                .maxDelayMs(60000)
                .backoffMultiplier(1.5)
                .jitterFactor(0.2)
                .build();

        assertTrue(config.isEnabled());
        assertEquals(10, config.getMaxAttempts());
        assertEquals(1000, config.getInitialDelayMs());
        assertEquals(60000, config.getMaxDelayMs());
        assertEquals(1.5, config.getBackoffMultiplier(), 0.001);
        assertEquals(0.2, config.getJitterFactor(), 0.001);
    }

    @Test
    @DisplayName("Builder should have sensible defaults")
    void testBuilderDefaults() {
        ReconnectConfig config = ReconnectConfig.builder().build();

        assertTrue(config.isEnabled());
        assertEquals(0, config.getMaxAttempts()); // 0 means infinite
        assertEquals(1000, config.getInitialDelayMs());
        assertEquals(30000, config.getMaxDelayMs());
        assertEquals(2.0, config.getBackoffMultiplier(), 0.001);
        assertEquals(0.2, config.getJitterFactor(), 0.001);
    }

    @Test
    @DisplayName("createDefault should return valid config")
    void testCreateDefault() {
        ReconnectConfig config = ReconnectConfig.createDefault();

        assertNotNull(config);
        assertTrue(config.isEnabled());
    }

    @Test
    @DisplayName("Config with same values should be equal")
    void testEquals() {
        ReconnectConfig config1 = ReconnectConfig.builder()
                .maxAttempts(5)
                .build();

        ReconnectConfig config2 = ReconnectConfig.builder()
                .maxAttempts(5)
                .build();

        assertEquals(config1, config2);
    }

    @Test
    @DisplayName("toString should contain field values")
    void testToString() {
        ReconnectConfig config = ReconnectConfig.builder()
                .initialDelayMs(500)
                .build();

        String str = config.toString();
        assertTrue(str.contains("500"));
    }

    @Test
    @DisplayName("Disabled config should have enabled=false")
    void testDisabledConfig() {
        ReconnectConfig config = ReconnectConfig.builder()
                .enabled(false)
                .build();

        assertFalse(config.isEnabled());
    }

    @Test
    @DisplayName("Infinite reconnect should have maxAttempts=0")
    void testInfiniteReconnect() {
        ReconnectConfig config = ReconnectConfig.builder()
                .maxAttempts(0)
                .build();

        assertEquals(0, config.getMaxAttempts());
    }
}
