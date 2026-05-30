package io.github.cuihairu.croupier.sdk.invoker;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.DisplayName;

import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Tests for InvokeOptions.
 */
class InvokeOptionsTest {

    @Test
    @DisplayName("Create should return default options")
    void testCreate() {
        InvokeOptions options = InvokeOptions.create();

        assertNotNull(options);
    }

    @Test
    @DisplayName("Builder should create options with all fields")
    void testBuilderAllFields() {
        RetryConfig retryConfig = RetryConfig.builder()
                .maxAttempts(5)
                .build();

        InvokeOptions options = InvokeOptions.builder()
                .timeout(30000)
                .idempotencyKey("idem-key-123")
                .header("X-Custom", "value")
                .header("X-Game-ID", "game-123")
                .retry(retryConfig)
                .build();

        assertEquals(30000, options.getTimeout());
        assertEquals("idem-key-123", options.getIdempotencyKey());
        assertEquals("value", options.getHeaders().get("X-Custom"));
        assertEquals("game-123", options.getHeaders().get("X-Game-ID"));
        assertNotNull(options.getRetry());
    }

    @Test
    @DisplayName("Builder with partial values should work")
    void testBuilderPartialValues() {
        InvokeOptions options = InvokeOptions.builder()
                .idempotencyKey("test-key")
                .build();

        assertEquals("test-key", options.getIdempotencyKey());
    }

    @Test
    @DisplayName("Options with same values should be equal")
    void testEquals() {
        InvokeOptions options1 = InvokeOptions.builder()
                .idempotencyKey("key-1")
                .build();

        InvokeOptions options2 = InvokeOptions.builder()
                .idempotencyKey("key-1")
                .build();

        assertEquals(options1, options2);
    }

    @Test
    @DisplayName("toString should contain field values")
    void testToString() {
        InvokeOptions options = InvokeOptions.builder()
                .idempotencyKey("test-key")
                .build();

        String str = options.toString();
        assertTrue(str.contains("test-key"));
    }

    @Test
    @DisplayName("headers should be immutable")
    void testHeadersImmutable() {
        InvokeOptions options = InvokeOptions.builder()
                .header("key", "value")
                .build();

        Map<String, String> headers = options.getHeaders();
        assertThrows(UnsupportedOperationException.class, () ->
                headers.put("new-key", "new-value"));
    }

    @Test
    @DisplayName("builder with headers map should work")
    void testBuilderWithHeadersMap() {
        Map<String, String> headers = Map.of("X-Header-1", "value1", "X-Header-2", "value2");

        InvokeOptions options = InvokeOptions.builder()
                .headers(headers)
                .build();

        assertEquals("value1", options.getHeaders().get("X-Header-1"));
        assertEquals("value2", options.getHeaders().get("X-Header-2"));
    }

    @Test
    @DisplayName("builder with null headers map should work")
    void testBuilderWithNullHeadersMap() {
        InvokeOptions options = InvokeOptions.builder()
                .headers(null)
                .build();

        assertTrue(options.getHeaders().isEmpty());
    }

    @Test
    @DisplayName("builder with headers map replaces existing headers")
    void testBuilderWithHeadersMapReplaces() {
        InvokeOptions options = InvokeOptions.builder()
                .header("X-Header-1", "value1")
                .headers(Map.of("X-Header-2", "value2"))  // Replaces previous headers
                .build();

        assertNull(options.getHeaders().get("X-Header-1"));  // Previous header is gone
        assertEquals("value2", options.getHeaders().get("X-Header-2"));
    }

    @Test
    @DisplayName("options with different values should not be equal")
    void testNotEquals() {
        InvokeOptions options1 = InvokeOptions.builder()
                .idempotencyKey("key-1")
                .build();

        InvokeOptions options2 = InvokeOptions.builder()
                .idempotencyKey("key-2")
                .build();

        assertNotEquals(options1, options2);
    }

    @Test
    @DisplayName("options with same values should have same hashCode")
    void testHashCode() {
        InvokeOptions options1 = InvokeOptions.builder()
                .timeout(5000)
                .idempotencyKey("key-123")
                .build();

        InvokeOptions options2 = InvokeOptions.builder()
                .timeout(5000)
                .idempotencyKey("key-123")
                .build();

        assertEquals(options1.hashCode(), options2.hashCode());
    }

    @Test
    @DisplayName("timeout can be zero")
    void testTimeoutZero() {
        InvokeOptions options = InvokeOptions.builder()
                .timeout(0)
                .build();

        assertEquals(0, options.getTimeout());
    }

    @Test
    @DisplayName("timeout can be large value")
    void testTimeoutLarge() {
        InvokeOptions options = InvokeOptions.builder()
                .timeout(3600000) // 1 hour
                .build();

        assertEquals(3600000, options.getTimeout());
    }

    @Test
    @DisplayName("idempotency key can be empty string")
    void testIdempotencyKeyEmpty() {
        InvokeOptions options = InvokeOptions.builder()
                .idempotencyKey("")
                .build();

        assertEquals("", options.getIdempotencyKey());
    }

    @Test
    @DisplayName("retry config can be set")
    void testRetryConfig() {
        RetryConfig retryConfig = RetryConfig.builder()
                .maxAttempts(3)
                .initialDelayMs(1000)
                .build();

        InvokeOptions options = InvokeOptions.builder()
                .retry(retryConfig)
                .build();

        assertEquals(retryConfig, options.getRetry());
    }

    @Test
    @DisplayName("retry config can be null")
    void testRetryConfigNull() {
        InvokeOptions options = InvokeOptions.builder()
                .retry(null)
                .build();

        assertNull(options.getRetry());
    }

    @Test
    @DisplayName("toString contains all non-null fields")
    void testToStringContainsAllFields() {
        RetryConfig retryConfig = RetryConfig.builder()
                .maxAttempts(5)
                .build();

        InvokeOptions options = InvokeOptions.builder()
                .idempotencyKey("test-key")
                .timeout(10000)
                .header("X-Custom", "value")
                .retry(retryConfig)
                .build();

        String str = options.toString();
        assertTrue(str.contains("test-key"));
        assertTrue(str.contains("10000"));
        assertTrue(str.contains("X-Custom=value"));
        assertTrue(str.contains("retry"));
    }

    @Test
    @DisplayName("multiple headers can be added")
    void testMultipleHeaders() {
        InvokeOptions options = InvokeOptions.builder()
                .header("X-Header-1", "value1")
                .header("X-Header-2", "value2")
                .header("X-Header-3", "value3")
                .header("X-Header-4", "value4")
                .header("X-Header-5", "value5")
                .build();

        assertEquals(5, options.getHeaders().size());
        assertEquals("value1", options.getHeaders().get("X-Header-1"));
        assertEquals("value2", options.getHeaders().get("X-Header-2"));
        assertEquals("value3", options.getHeaders().get("X-Header-3"));
        assertEquals("value4", options.getHeaders().get("X-Header-4"));
        assertEquals("value5", options.getHeaders().get("X-Header-5"));
    }

    @Test
    @DisplayName("header values can be empty string")
    void testHeaderEmptyValue() {
        InvokeOptions options = InvokeOptions.builder()
                .header("X-Empty", "")
                .build();

        assertEquals("", options.getHeaders().get("X-Empty"));
    }

    @Test
    @DisplayName("header keys are case-sensitive")
    void testHeaderCaseSensitive() {
        InvokeOptions options = InvokeOptions.builder()
                .header("X-Header", "value1")
                .header("x-header", "value2")
                .build();

        assertEquals("value1", options.getHeaders().get("X-Header"));
        assertEquals("value2", options.getHeaders().get("x-header"));
        assertEquals(2, options.getHeaders().size());
    }
}
