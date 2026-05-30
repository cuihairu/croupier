package io.github.cuihairu.croupier.sdk;

import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

/**
 * Tests for ReconnectConfig.
 */
class ReconnectConfigTest {

    @Test
    void testBuilderCreatesCorrectConfig() {
        ReconnectConfig config = ReconnectConfig.builder()
            .enabled(true)
            .maxAttempts(10)
            .initialDelayMs(500)
            .maxDelayMs(60000)
            .backoffMultiplier(3.0)
            .jitterFactor(0.3)
            .build();

        assertTrue(config.isEnabled());
        assertEquals(10, config.getMaxAttempts());
        assertEquals(500, config.getInitialDelayMs());
        assertEquals(60000, config.getMaxDelayMs());
        assertEquals(3.0, config.getBackoffMultiplier(), 0.001);
        assertEquals(0.3, config.getJitterFactor(), 0.001);
    }

    @Test
    void testDefaultValues() {
        ReconnectConfig config = ReconnectConfig.builder().build();

        assertTrue(config.isEnabled());
        assertEquals(0, config.getMaxAttempts());  // Infinite
        assertEquals(1000, config.getInitialDelayMs());
        assertEquals(30000, config.getMaxDelayMs());
        assertEquals(2.0, config.getBackoffMultiplier(), 0.001);
        assertEquals(0.2, config.getJitterFactor(), 0.001);
    }

    @Test
    void testCreateDefault() {
        ReconnectConfig config = ReconnectConfig.createDefault();

        assertTrue(config.isEnabled());
        assertEquals(0, config.getMaxAttempts());
        assertEquals(1000, config.getInitialDelayMs());
        assertEquals(30000, config.getMaxDelayMs());
    }

    @Test
    void testEquality() {
        ReconnectConfig config1 = ReconnectConfig.builder()
            .enabled(true)
            .maxAttempts(5)
            .initialDelayMs(2000)
            .build();

        ReconnectConfig config2 = ReconnectConfig.builder()
            .enabled(true)
            .maxAttempts(5)
            .initialDelayMs(2000)
            .build();

        assertEquals(config1, config2);
        assertEquals(config1.hashCode(), config2.hashCode());
    }

    @Test
    void testInequality() {
        ReconnectConfig config1 = ReconnectConfig.builder()
            .enabled(true)
            .maxAttempts(5)
            .build();

        ReconnectConfig config2 = ReconnectConfig.builder()
            .enabled(false)
            .maxAttempts(5)
            .build();

        assertNotEquals(config1, config2);
    }

    @Test
    void testCanDisableReconnect() {
        ReconnectConfig config = ReconnectConfig.builder()
            .enabled(false)
            .build();

        assertFalse(config.isEnabled());
    }

    @Test
    void testToString() {
        ReconnectConfig config = ReconnectConfig.builder()
            .maxAttempts(10)
            .build();

        String str = config.toString();
        assertTrue(str.contains("enabled"));
        assertTrue(str.contains("maxAttempts"));
        assertTrue(str.contains("10"));
    }

    @Test
    void testMaxAttemptsZeroMeansInfinite() {
        ReconnectConfig config = ReconnectConfig.builder()
            .maxAttempts(0)
            .build();

        assertEquals(0, config.getMaxAttempts());
    }

    @Test
    void testBackoffMultiplierAcceptsDouble() {
        ReconnectConfig config = ReconnectConfig.builder()
            .backoffMultiplier(1.5)
            .build();

        assertEquals(1.5, config.getBackoffMultiplier(), 0.001);
    }

    @Test
    void testJitterFactorRange() {
        ReconnectConfig config = ReconnectConfig.builder()
            .jitterFactor(0.5)
            .build();

        assertEquals(0.5, config.getJitterFactor(), 0.001);
    }
}
