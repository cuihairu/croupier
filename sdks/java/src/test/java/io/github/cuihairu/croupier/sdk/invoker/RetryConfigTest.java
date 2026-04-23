package io.github.cuihairu.croupier.sdk.invoker;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.DisplayName;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Tests for RetryConfig.
 */
class RetryConfigTest {

    @Test
    @DisplayName("Builder should create config with all fields")
    void testBuilderAllFields() {
        RetryConfig config = RetryConfig.builder()
                .enabled(true)
                .maxAttempts(5)
                .initialDelayMs(100)
                .maxDelayMs(10000)
                .backoffMultiplier(2.0)
                .jitterFactor(0.1)
                .build();

        assertTrue(config.isEnabled());
        assertEquals(5, config.getMaxAttempts());
        assertEquals(100, config.getInitialDelayMs());
        assertEquals(10000, config.getMaxDelayMs());
        assertEquals(2.0, config.getBackoffMultiplier(), 0.001);
        assertEquals(0.1, config.getJitterFactor(), 0.001);
    }

    @Test
    @DisplayName("Builder should have sensible defaults")
    void testBuilderDefaults() {
        RetryConfig config = RetryConfig.builder().build();

        assertTrue(config.isEnabled());
        assertEquals(3, config.getMaxAttempts());
        assertEquals(100, config.getInitialDelayMs());
        assertEquals(5000, config.getMaxDelayMs());
        assertEquals(2.0, config.getBackoffMultiplier(), 0.001);
        assertEquals(0.1, config.getJitterFactor(), 0.001);
    }

    @Test
    @DisplayName("createDefault should return valid config")
    void testCreateDefault() {
        RetryConfig config = RetryConfig.createDefault();

        assertNotNull(config);
        assertTrue(config.isEnabled());
    }

    @Test
    @DisplayName("Config with same values should be equal")
    void testEquals() {
        RetryConfig config1 = RetryConfig.builder()
                .maxAttempts(3)
                .build();

        RetryConfig config2 = RetryConfig.builder()
                .maxAttempts(3)
                .build();

        assertEquals(config1, config2);
    }

    @Test
    @DisplayName("toString should contain field values")
    void testToString() {
        RetryConfig config = RetryConfig.builder()
                .maxAttempts(5)
                .build();

        String str = config.toString();
        assertTrue(str.contains("5"));
    }

    @Test
    @DisplayName("Disabled config should have enabled=false")
    void testDisabledConfig() {
        RetryConfig config = RetryConfig.builder()
                .enabled(false)
                .build();

        assertFalse(config.isEnabled());
    }
}
