package io.github.cuihairu.croupier.sdk.invoker;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import static io.github.cuihairu.croupier.sdk.invoker.InvokerException.ErrorCode;
import static org.junit.jupiter.api.Assertions.*;

/**
 * Extra tests for InvokerConfig, InvokerConfig.Builder, and related types.
 */
class InvokerConfigExtraCoverageTest {

    @Test
    @DisplayName("builder() with all fields set")
    void builderAllFields() {
        InvokerConfig config = InvokerConfig.builder()
            .address("10.0.0.1:9090")
            .timeout(60000)
            .insecure(false)
            .caFile("/ca.pem")
            .certFile("/cert.pem")
            .keyFile("/key.pem")
            .serverName("my-server")
            .reconnect(ReconnectConfig.builder().enabled(true).build())
            .retry(RetryConfig.builder().enabled(true).maxAttempts(5).build())
            .build();

        assertEquals("10.0.0.1:9090", config.getAddress());
        assertEquals(60000, config.getTimeout());
        assertFalse(config.isInsecure());
        assertEquals("/ca.pem", config.getCaFile());
        assertEquals("/cert.pem", config.getCertFile());
        assertEquals("/key.pem", config.getKeyFile());
        assertEquals("my-server", config.getServerName());
        assertNotNull(config.getReconnect());
        assertNotNull(config.getRetry());
    }

    @Test
    @DisplayName("builder() with null string fields should use empty string")
    void builderNullStrings() {
        InvokerConfig config = InvokerConfig.builder()
            .caFile(null)
            .certFile(null)
            .keyFile(null)
            .serverName(null)
            .build();

        assertEquals("", config.getCaFile());
        assertEquals("", config.getCertFile());
        assertEquals("", config.getKeyFile());
        assertEquals("", config.getServerName());
    }

    @Test
    @DisplayName("createDefault() should return config with defaults")
    void createDefault() {
        InvokerConfig config = InvokerConfig.createDefault();

        assertEquals("127.0.0.1:19090", config.getAddress());
        assertEquals(30000, config.getTimeout());
        assertTrue(config.isInsecure());
        assertEquals("", config.getCaFile());
        assertEquals("", config.getCertFile());
        assertEquals("", config.getKeyFile());
        assertEquals("", config.getServerName());
        assertNotNull(config.getReconnect());
        assertNotNull(config.getRetry());
    }

    @Test
    @DisplayName("equals and hashCode")
    void equalsHashCode() {
        InvokerConfig config1 = InvokerConfig.builder()
            .address("addr").timeout(1000).insecure(true).build();
        InvokerConfig config2 = InvokerConfig.builder()
            .address("addr").timeout(1000).insecure(true).build();
        InvokerConfig config3 = InvokerConfig.builder()
            .address("other").timeout(1000).insecure(true).build();

        assertEquals(config1, config2);
        assertEquals(config1.hashCode(), config2.hashCode());
        assertNotEquals(config1, config3);
        assertNotEquals(config1, null);
        assertNotEquals(config1, "string");
        assertEquals(config1, config1); // same object
    }

    @Test
    @DisplayName("toString should include all fields")
    void toStringIncludesFields() {
        InvokerConfig config = InvokerConfig.builder()
            .address("addr").timeout(1000).insecure(false)
            .caFile("ca").certFile("cert").keyFile("key").serverName("sn")
            .build();

        String str = config.toString();
        assertTrue(str.contains("address='addr'"));
        assertTrue(str.contains("timeout=1000"));
        assertTrue(str.contains("insecure=false"));
        assertTrue(str.contains("caFile='ca'"));
        assertTrue(str.contains("certFile='cert'"));
        assertTrue(str.contains("keyFile='key'"));
        assertTrue(str.contains("serverName='sn'"));
    }

    @Test
    @DisplayName("RetryConfig.Builder with all fields")
    void retryConfigBuilderAllFields() {
        RetryConfig config = RetryConfig.builder()
            .enabled(true)
            .maxAttempts(5)
            .initialDelayMs(100)
            .maxDelayMs(5000)
            .backoffMultiplier(2.0)
            .jitterFactor(0.5)
            .retryableStatusCodes(java.util.List.of(14, 8))
            .build();

        assertTrue(config.isEnabled());
        assertEquals(5, config.getMaxAttempts());
        assertEquals(100, config.getInitialDelayMs());
        assertEquals(5000, config.getMaxDelayMs());
        assertEquals(2.0, config.getBackoffMultiplier());
        assertEquals(0.5, config.getJitterFactor());
        assertEquals(2, config.getRetryableStatusCodes().size());
    }

    @Test
    @DisplayName("RetryConfig equals and hashCode")
    void retryConfigEqualsHashCode() {
        RetryConfig config1 = RetryConfig.builder().enabled(true).maxAttempts(3).build();
        RetryConfig config2 = RetryConfig.builder().enabled(true).maxAttempts(3).build();
        RetryConfig config3 = RetryConfig.builder().enabled(false).maxAttempts(3).build();

        assertEquals(config1, config2);
        assertEquals(config1.hashCode(), config2.hashCode());
        assertNotEquals(config1, config3);
    }

    @Test
    @DisplayName("RetryConfig toString")
    void retryConfigToString() {
        RetryConfig config = RetryConfig.builder().enabled(true).maxAttempts(3).build();
        String str = config.toString();
        assertTrue(str.contains("RetryConfig"));
    }

    @Test
    @DisplayName("ReconnectConfig.Builder with all fields")
    void reconnectConfigBuilderAllFields() {
        ReconnectConfig config = ReconnectConfig.builder()
            .enabled(true)
            .maxAttempts(10)
            .initialDelayMs(500)
            .maxDelayMs(30000)
            .build();

        assertTrue(config.isEnabled());
        assertEquals(10, config.getMaxAttempts());
        assertEquals(500, config.getInitialDelayMs());
        assertEquals(30000, config.getMaxDelayMs());
    }

    @Test
    @DisplayName("ReconnectConfig equals and hashCode")
    void reconnectConfigEqualsHashCode() {
        ReconnectConfig config1 = ReconnectConfig.builder().enabled(true).build();
        ReconnectConfig config2 = ReconnectConfig.builder().enabled(true).build();
        ReconnectConfig config3 = ReconnectConfig.builder().enabled(false).build();

        assertEquals(config1, config2);
        assertEquals(config1.hashCode(), config2.hashCode());
        assertNotEquals(config1, config3);
    }

    @Test
    @DisplayName("InvokeOptions.Builder with all fields")
    void invokeOptionsBuilderAllFields() {
        InvokeOptions options = InvokeOptions.builder()
            .idempotencyKey("key-1")
            .header("k1", "v1")
            .header("k2", "v2")
            .timeout(5000)
            .retry(RetryConfig.builder().enabled(true).build())
            .build();

        assertEquals("key-1", options.getIdempotencyKey());
        assertEquals("v1", options.getHeaders().get("k1"));
        assertEquals("v2", options.getHeaders().get("k2"));
        assertEquals(5000, options.getTimeout());
        assertNotNull(options.getRetry());
    }

    @Test
    @DisplayName("InvokeOptions.create() should return defaults")
    void invokeOptionsCreate() {
        InvokeOptions options = InvokeOptions.create();

        assertNotNull(options);
        assertNull(options.getIdempotencyKey());
        assertNull(options.getTimeout());
        assertNotNull(options.getHeaders());
        assertNull(options.getRetry());
    }

    @Test
    @DisplayName("InvokeOptions headers with null map")
    void invokeOptionsHeadersNull() {
        InvokeOptions options = InvokeOptions.builder()
            .headers(null)
            .build();

        assertTrue(options.getHeaders().isEmpty());
    }

    @Test
    @DisplayName("InvokeOptions equals and hashCode")
    void invokeOptionsEqualsHashCode() {
        InvokeOptions opt1 = InvokeOptions.builder().idempotencyKey("k1").timeout(1000).build();
        InvokeOptions opt2 = InvokeOptions.builder().idempotencyKey("k1").timeout(1000).build();
        InvokeOptions opt3 = InvokeOptions.builder().idempotencyKey("k2").timeout(1000).build();

        assertEquals(opt1, opt2);
        assertEquals(opt1.hashCode(), opt2.hashCode());
        assertNotEquals(opt1, opt3);
        assertNotEquals(opt1, null);
        assertNotEquals(opt1, "string");
    }

    @Test
    @DisplayName("InvokeOptions toString")
    void invokeOptionsToString() {
        InvokeOptions options = InvokeOptions.builder().idempotencyKey("k1").build();
        String str = options.toString();
        assertTrue(str.contains("InvokeOptions"));
        assertTrue(str.contains("k1"));
    }

    @Test
    @DisplayName("TaskEventInfo.Builder with all fields")
    void taskEventInfoBuilderAllFields() {
        TaskEventInfo info = TaskEventInfo.builder()
            .type("progress")
            .taskId("task-1")
            .message("msg")
            .progress(50)
            .payload("data")
            .error("err")
            .done(false)
            .build();

        assertEquals("progress", info.getType());
        assertEquals("task-1", info.getTaskId());
        assertEquals("msg", info.getMessage());
        assertEquals(50, info.getProgress());
        assertEquals("data", info.getPayload());
        assertEquals("err", info.getError());
        assertFalse(info.isDone());
    }

    @Test
    @DisplayName("TaskEventInfo default values")
    void taskEventInfoDefaults() {
        TaskEventInfo info = TaskEventInfo.builder().build();

        assertNull(info.getType());
        assertNull(info.getTaskId());
        assertNull(info.getMessage());
        assertNull(info.getProgress());
        assertNull(info.getPayload());
        assertNull(info.getError());
        assertFalse(info.isDone());
    }

    @Test
    @DisplayName("InvokerException with error code and cause")
    void invokerExceptionWithCause() {
        RuntimeException cause = new RuntimeException("root cause");
        InvokerException ex = new InvokerException(
            InvokerException.ErrorCode.INTERNAL, "wrapped", cause);

        assertEquals(InvokerException.ErrorCode.INTERNAL, ex.getErrorCode());
        assertEquals("wrapped", ex.getMessage());
        assertEquals(cause, ex.getCause());
    }

    @Test
    @DisplayName("InvokerException.ErrorCode values")
    void invokerExceptionErrorCodes() {
        // Verify all error codes exist
        assertNotNull(InvokerException.ErrorCode.CANCELLED);
        assertNotNull(InvokerException.ErrorCode.UNKNOWN);
        assertNotNull(InvokerException.ErrorCode.CONNECTION_FAILED);
        assertNotNull(InvokerException.ErrorCode.CONNECTION_TIMEOUT);
        assertNotNull(InvokerException.ErrorCode.INVALID_ARGUMENT);
        assertNotNull(InvokerException.ErrorCode.NOT_FOUND);
        assertNotNull(InvokerException.ErrorCode.ALREADY_EXISTS);
        assertNotNull(InvokerException.ErrorCode.PERMISSION_DENIED);
        assertNotNull(InvokerException.ErrorCode.RESOURCE_EXHAUSTED);
        assertNotNull(InvokerException.ErrorCode.FAILED_PRECONDITION);
        assertNotNull(InvokerException.ErrorCode.ABORTED);
        assertNotNull(InvokerException.ErrorCode.OUT_OF_RANGE);
        assertNotNull(InvokerException.ErrorCode.UNIMPLEMENTED);
        assertNotNull(InvokerException.ErrorCode.INTERNAL);
        assertNotNull(InvokerException.ErrorCode.UNAVAILABLE);
        assertNotNull(InvokerException.ErrorCode.DATA_LOSS);
        assertNotNull(InvokerException.ErrorCode.UNAUTHENTICATED);
        assertNotNull(InvokerException.ErrorCode.TIMEOUT);
    }

    @Test
    @DisplayName("InvokerException.fromStatusCode should map correctly")
    void invokerExceptionFromStatusCode() {
        assertEquals(ErrorCode.CANCELLED, InvokerException.fromStatusCode(1, "msg").getErrorCode());
        assertEquals(ErrorCode.UNKNOWN, InvokerException.fromStatusCode(2, "msg").getErrorCode());
        assertEquals(ErrorCode.INVALID_ARGUMENT, InvokerException.fromStatusCode(3, "msg").getErrorCode());
        assertEquals(ErrorCode.TIMEOUT, InvokerException.fromStatusCode(4, "msg").getErrorCode());
        assertEquals(ErrorCode.NOT_FOUND, InvokerException.fromStatusCode(5, "msg").getErrorCode());
        assertEquals(ErrorCode.ALREADY_EXISTS, InvokerException.fromStatusCode(6, "msg").getErrorCode());
        assertEquals(ErrorCode.PERMISSION_DENIED, InvokerException.fromStatusCode(7, "msg").getErrorCode());
        assertEquals(ErrorCode.RESOURCE_EXHAUSTED, InvokerException.fromStatusCode(8, "msg").getErrorCode());
        assertEquals(ErrorCode.FAILED_PRECONDITION, InvokerException.fromStatusCode(9, "msg").getErrorCode());
        assertEquals(ErrorCode.ABORTED, InvokerException.fromStatusCode(10, "msg").getErrorCode());
        assertEquals(ErrorCode.OUT_OF_RANGE, InvokerException.fromStatusCode(11, "msg").getErrorCode());
        assertEquals(ErrorCode.UNIMPLEMENTED, InvokerException.fromStatusCode(12, "msg").getErrorCode());
        assertEquals(ErrorCode.INTERNAL, InvokerException.fromStatusCode(13, "msg").getErrorCode());
        assertEquals(ErrorCode.UNAVAILABLE, InvokerException.fromStatusCode(14, "msg").getErrorCode());
        assertEquals(ErrorCode.DATA_LOSS, InvokerException.fromStatusCode(15, "msg").getErrorCode());
        assertEquals(ErrorCode.UNAUTHENTICATED, InvokerException.fromStatusCode(16, "msg").getErrorCode());
        // Unknown status code should map to UNKNOWN
        assertEquals(ErrorCode.UNKNOWN, InvokerException.fromStatusCode(99, "msg").getErrorCode());
    }

    @Test
    @DisplayName("InvokerException.fromGrpcStatus should map correctly")
    void invokerExceptionFromGrpcStatus() {
        assertEquals(ErrorCode.CANCELLED, InvokerException.fromGrpcStatus(1, "msg").getErrorCode());
        assertEquals(ErrorCode.UNKNOWN, InvokerException.fromGrpcStatus(2, "msg").getErrorCode());
        assertEquals(ErrorCode.NOT_FOUND, InvokerException.fromGrpcStatus(5, "msg").getErrorCode());
        assertEquals(ErrorCode.INTERNAL, InvokerException.fromGrpcStatus(13, "msg").getErrorCode());
        assertEquals(ErrorCode.UNAVAILABLE, InvokerException.fromGrpcStatus(14, "msg").getErrorCode());
    }

    @Test
    @DisplayName("InvokerException.fromGrpcStatus with cause")
    void invokerExceptionFromGrpcStatusWithCause() {
        RuntimeException cause = new RuntimeException("root");
        InvokerException ex = InvokerException.fromGrpcStatus(14, "unavailable", cause);
        assertEquals(ErrorCode.UNAVAILABLE, ex.getErrorCode());
        assertEquals(cause, ex.getCause());
    }

    @Test
    @DisplayName("InvokerException.ErrorCode.getCode() should return string code")
    void errorCodeGetCode() {
        assertEquals("CANCELLED", ErrorCode.CANCELLED.getCode());
        assertEquals("UNKNOWN", ErrorCode.UNKNOWN.getCode());
        assertEquals("INTERNAL", ErrorCode.INTERNAL.getCode());
        assertEquals("UNAVAILABLE", ErrorCode.UNAVAILABLE.getCode());
    }
}
