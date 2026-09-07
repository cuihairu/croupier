package io.github.cuihairu.croupier.sdk.invoker;

import org.junit.jupiter.api.Test;

import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;

/**
 * Equals methods short-circuit at the first differing field, so every field
 * branch needs objects that are equal up to that field and differ in it.
 */
class EqualsShortCircuitBranchTest {

    // ------------------------------------------------------------------
    // InvokerConfig
    // ------------------------------------------------------------------

    private static InvokerConfig.Builder like(InvokerConfig base) {
        return InvokerConfig.builder()
            .address(base.getAddress())
            .authToken(base.getAuthToken())
            .gameId(base.getGameId())
            .env(base.getEnv())
            .taskPollIntervalMs(base.getTaskPollIntervalMs())
            .timeout(base.getTimeout())
            .insecure(base.isInsecure())
            .caFile(base.getCaFile())
            .certFile(base.getCertFile())
            .keyFile(base.getKeyFile())
            .serverName(base.getServerName())
            .reconnect(base.getReconnect())
            .retry(base.getRetry());
    }

    @Test
    void invokerConfigEveryFieldDivergenceIsCompared() {
        InvokerConfig base = InvokerConfig.builder()
            .address("http://127.0.0.1:1/api/v1").authToken("tok").gameId("game").env("prod")
            .taskPollIntervalMs(50).timeout(5000).insecure(true)
            .caFile("/ca").certFile("/cert").keyFile("/key").serverName("agent")
            .reconnect(ReconnectConfig.builder().maxAttempts(2).build())
            .retry(RetryConfig.builder().maxAttempts(3).build())
            .build();

        assertEquals(base, like(base).build());
        assertEquals(base.hashCode(), like(base).build().hashCode());

        assertNotEquals(base, like(base).address("http://other:2/api/v1").build());
        assertNotEquals(base, like(base).authToken("other").build());
        assertNotEquals(base, like(base).gameId("other").build());
        assertNotEquals(base, like(base).env("other").build());
        assertNotEquals(base, like(base).taskPollIntervalMs(51).build());
        assertNotEquals(base, like(base).timeout(5001).build());
        assertNotEquals(base, like(base).insecure(false).build());
        assertNotEquals(base, like(base).caFile("/other").build());
        assertNotEquals(base, like(base).certFile("/other").build());
        assertNotEquals(base, like(base).keyFile("/other").build());
        assertNotEquals(base, like(base).serverName("other").build());
        assertNotEquals(base, like(base)
            .reconnect(ReconnectConfig.builder().maxAttempts(99).build()).build());
        assertNotEquals(base, like(base)
            .retry(RetryConfig.builder().maxAttempts(98).build()).build());
        assertNotNull(base.toString());
    }

    // ------------------------------------------------------------------
    // RetryConfig
    // ------------------------------------------------------------------

    private static RetryConfig.Builder like(RetryConfig base) {
        return RetryConfig.builder()
            .enabled(base.isEnabled())
            .maxAttempts(base.getMaxAttempts())
            .initialDelayMs(base.getInitialDelayMs())
            .maxDelayMs(base.getMaxDelayMs())
            .backoffMultiplier(base.getBackoffMultiplier())
            .jitterFactor(base.getJitterFactor())
            .retryableStatusCodes(base.getRetryableStatusCodes());
    }

    @Test
    void retryConfigEveryFieldDivergenceIsCompared() {
        RetryConfig base = RetryConfig.builder()
            .enabled(true).maxAttempts(4).initialDelayMs(11).maxDelayMs(111)
            .backoffMultiplier(2.5).jitterFactor(0.25)
            .retryableStatusCodes(List.of(502, 503))
            .build();

        assertEquals(base, like(base).build());
        assertEquals(base.hashCode(), like(base).build().hashCode());

        assertNotEquals(base, like(base).enabled(false).build());
        assertNotEquals(base, like(base).maxAttempts(5).build());
        assertNotEquals(base, like(base).initialDelayMs(12).build());
        assertNotEquals(base, like(base).maxDelayMs(112).build());
        assertNotEquals(base, like(base).backoffMultiplier(2.6).build());
        assertNotEquals(base, like(base).jitterFactor(0.26).build());
        assertNotEquals(base, like(base).retryableStatusCodes(List.of(500)).build());
    }

    // ------------------------------------------------------------------
    // invoker.ReconnectConfig
    // ------------------------------------------------------------------

    private static ReconnectConfig.Builder like(ReconnectConfig base) {
        return ReconnectConfig.builder()
            .enabled(base.isEnabled())
            .maxAttempts(base.getMaxAttempts())
            .initialDelayMs(base.getInitialDelayMs())
            .maxDelayMs(base.getMaxDelayMs())
            .backoffMultiplier(base.getBackoffMultiplier())
            .jitterFactor(base.getJitterFactor());
    }

    @Test
    void invokerReconnectConfigEveryFieldDivergenceIsCompared() {
        ReconnectConfig base = ReconnectConfig.builder()
            .enabled(true).maxAttempts(4).initialDelayMs(11).maxDelayMs(111)
            .backoffMultiplier(2.5).jitterFactor(0.25)
            .build();

        assertEquals(base, like(base).build());
        assertEquals(base.hashCode(), like(base).build().hashCode());

        assertNotEquals(base, like(base).enabled(false).build());
        assertNotEquals(base, like(base).maxAttempts(5).build());
        assertNotEquals(base, like(base).initialDelayMs(12).build());
        assertNotEquals(base, like(base).maxDelayMs(112).build());
        assertNotEquals(base, like(base).backoffMultiplier(2.6).build());
        assertNotEquals(base, like(base).jitterFactor(0.26).build());
    }

    // ------------------------------------------------------------------
    // InvokeOptions
    // ------------------------------------------------------------------

    private static InvokeOptions.Builder like(InvokeOptions base) {
        return InvokeOptions.builder()
            .idempotencyKey(base.getIdempotencyKey())
            .timeout(base.getTimeout() == null ? 0 : base.getTimeout())
            .headers(base.getHeaders())
            .retry(base.getRetry());
    }

    @Test
    void invokeOptionsEveryFieldDivergenceIsCompared() {
        InvokeOptions base = InvokeOptions.builder()
            .idempotencyKey("key").timeout(1234)
            .header("X-A", "1")
            .retry(RetryConfig.builder().maxAttempts(3).build())
            .build();

        assertEquals(base, like(base).build());
        assertEquals(base.hashCode(), like(base).build().hashCode());

        assertNotEquals(base, like(base).idempotencyKey("other").build());
        assertNotEquals(base, like(base).timeout(999).build());
        assertNotEquals(base, like(base).header("X-A", "2").build());
        assertNotEquals(base, like(base)
            .retry(RetryConfig.builder().maxAttempts(9).build()).build());
    }

    // ------------------------------------------------------------------
    // TaskEventInfo
    // ------------------------------------------------------------------

    private static TaskEventInfo.Builder like(TaskEventInfo base) {
        return TaskEventInfo.builder()
            .type(base.getType())
            .taskId(base.getTaskId())
            .payload(base.getPayload())
            .message(base.getMessage())
            .progress(base.getProgress())
            .error(base.getError())
            .done(base.isDone());
    }

    @Test
    void taskEventInfoEveryFieldDivergenceIsCompared() {
        TaskEventInfo base = TaskEventInfo.builder()
            .type("progress").taskId("task-1").payload("{\"n\":1}")
            .message("msg").progress(42).error(null).done(false)
            .build();

        assertEquals(base, like(base).build());
        assertEquals(base.hashCode(), like(base).build().hashCode());

        assertNotEquals(base, like(base).done(true).build());
        assertNotEquals(base, like(base).type("other").build());
        assertNotEquals(base, like(base).taskId("other").build());
        assertNotEquals(base, like(base).payload("other").build());
        assertNotEquals(base, like(base).message("other").build());
        assertNotEquals(base, like(base).progress(99).build());
        assertNotEquals(base, like(base).error("boom").build());
    }
}
