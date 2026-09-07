package io.github.cuihairu.croupier.sdk.invoker;

import org.junit.jupiter.api.Test;

import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;

/**
 * Branch-complete equals/hashCode/toString coverage for invoker value objects.
 */
class InvokerValueObjectsBranchTest {

    // ------------------------------------------------------------------
    // InvokerConfig
    // ------------------------------------------------------------------

    private static InvokerConfig fullConfig() {
        return InvokerConfig.builder()
            .address("http://127.0.0.1:18780/api/v1")
            .authToken("token")
            .gameId("game")
            .env("prod")
            .taskPollIntervalMs(50)
            .timeout(5000)
            .insecure(true)
            .caFile("/tmp/ca.pem")
            .certFile("/tmp/cert.pem")
            .keyFile("/tmp/key.pem")
            .serverName("agent.local")
            .reconnect(ReconnectConfig.builder().maxAttempts(4).build())
            .retry(RetryConfig.builder().maxAttempts(6).build())
            .build();
    }

    @Test
    void invokerConfigEqualsCoversEveryFieldBranch() {
        InvokerConfig base = fullConfig();

        assertEquals(base, base);
        assertNotEquals(base, null);
        assertNotEquals(base, "not-a-config");

        assertEquals(base, fullConfig());
        assertEquals(base.hashCode(), fullConfig().hashCode());
        assertNotNull(base.toString());

        assertNotEquals(base, InvokerConfig.builder().address("http://other:1/api/v1").build());
        assertNotEquals(base, InvokerConfig.builder().authToken("other").build());
        assertNotEquals(base, InvokerConfig.builder().gameId("other").build());
        assertNotEquals(base, InvokerConfig.builder().env("other").build());
        assertNotEquals(base, InvokerConfig.builder().taskPollIntervalMs(999).build());
        assertNotEquals(base, InvokerConfig.builder().timeout(9999).build());
        assertNotEquals(base, InvokerConfig.builder().insecure(false).build());
        assertNotEquals(base, InvokerConfig.builder().caFile("/other").build());
        assertNotEquals(base, InvokerConfig.builder().certFile("/other").build());
        assertNotEquals(base, InvokerConfig.builder().keyFile("/other").build());
        assertNotEquals(base, InvokerConfig.builder().serverName("other").build());
        assertNotEquals(base, InvokerConfig.builder()
            .reconnect(ReconnectConfig.builder().maxAttempts(99).build()).build());
        assertNotEquals(base, InvokerConfig.builder()
            .retry(RetryConfig.builder().maxAttempts(98).build()).build());
    }

    // ------------------------------------------------------------------
    // RetryConfig
    // ------------------------------------------------------------------

    @Test
    void retryConfigEqualsCoversEveryFieldBranch() {
        RetryConfig base = RetryConfig.builder()
            .enabled(true).maxAttempts(3).initialDelayMs(10).maxDelayMs(100)
            .backoffMultiplier(2.0).jitterFactor(0.2)
            .retryableStatusCodes(List.of(502, 503))
            .build();

        assertEquals(base, base);
        assertNotEquals(base, null);
        assertNotEquals(base, new Object());
        assertEquals(base, RetryConfig.builder()
            .enabled(true).maxAttempts(3).initialDelayMs(10).maxDelayMs(100)
            .backoffMultiplier(2.0).jitterFactor(0.2)
            .retryableStatusCodes(List.of(502, 503))
            .build());
        assertEquals(base.hashCode(), base.hashCode());

        assertNotEquals(base, RetryConfig.builder().enabled(false).build());
        assertNotEquals(base, RetryConfig.builder().maxAttempts(7).build());
        assertNotEquals(base, RetryConfig.builder().initialDelayMs(77).build());
        assertNotEquals(base, RetryConfig.builder().maxDelayMs(77).build());
        assertNotEquals(base, RetryConfig.builder().backoffMultiplier(7.7).build());
        assertNotEquals(base, RetryConfig.builder().jitterFactor(0.7).build());
        assertNotEquals(base, RetryConfig.builder()
            .retryableStatusCodes(List.of(500)).build());
    }

    // ------------------------------------------------------------------
    // invoker.ReconnectConfig
    // ------------------------------------------------------------------

    @Test
    void invokerReconnectConfigEqualsCoversEveryFieldBranch() {
        ReconnectConfig base = ReconnectConfig.builder()
            .enabled(true).maxAttempts(3).initialDelayMs(10).maxDelayMs(1000)
            .backoffMultiplier(1.5).jitterFactor(0.1)
            .build();

        assertEquals(base, base);
        assertNotEquals(base, null);
        assertNotEquals(base, new Object());
        assertEquals(base, ReconnectConfig.builder()
            .enabled(true).maxAttempts(3).initialDelayMs(10).maxDelayMs(1000)
            .backoffMultiplier(1.5).jitterFactor(0.1)
            .build());

        assertNotEquals(base, ReconnectConfig.builder().enabled(false).build());
        assertNotEquals(base, ReconnectConfig.builder().maxAttempts(9).build());
        assertNotEquals(base, ReconnectConfig.builder().initialDelayMs(9).build());
        assertNotEquals(base, ReconnectConfig.builder().maxDelayMs(9).build());
        assertNotEquals(base, ReconnectConfig.builder().backoffMultiplier(9.9).build());
        assertNotEquals(base, ReconnectConfig.builder().jitterFactor(0.9).build());
    }

    // ------------------------------------------------------------------
    // InvokeOptions
    // ------------------------------------------------------------------

    @Test
    void invokeOptionsEqualsCoversEveryFieldBranch() {
        InvokeOptions base = InvokeOptions.builder()
            .idempotencyKey("key-1")
            .timeout(1234)
            .header("X-Custom", "value")
            .retry(RetryConfig.builder().maxAttempts(2).build())
            .build();

        assertEquals(base, base);
        assertNotEquals(base, null);
        assertNotEquals(base, new Object());
        assertEquals(base, InvokeOptions.builder()
            .idempotencyKey("key-1")
            .timeout(1234)
            .header("X-Custom", "value")
            .retry(RetryConfig.builder().maxAttempts(2).build())
            .build());
        assertEquals(base.hashCode(), base.hashCode());
        assertNotNull(base.toString());

        assertNotEquals(base, InvokeOptions.builder().idempotencyKey("other").build());
        assertNotEquals(base, InvokeOptions.builder().timeout(999).build());
        assertNotEquals(base, InvokeOptions.builder().header("X-Other", "value").build());
        assertNotEquals(base, InvokeOptions.builder()
            .retry(RetryConfig.builder().maxAttempts(9).build()).build());

        // headers(null) and empty-map branches
        assertEquals(InvokeOptions.builder().headers(null).build(), InvokeOptions.create());
        assertEquals(InvokeOptions.builder().headers(Map.of()).build(), InvokeOptions.create());
        assertEquals(Map.of(), InvokeOptions.create().getHeaders());
    }

    // ------------------------------------------------------------------
    // TaskEventInfo
    // ------------------------------------------------------------------

    private static TaskEventInfo fullEvent() {
        return TaskEventInfo.builder()
            .type("progress").taskId("task-9").payload("{\"n\":1}")
            .message("working").progress(42).error(null).done(false)
            .build();
    }

    @Test
    void taskEventInfoEqualsCoversEveryFieldBranch() {
        TaskEventInfo base = fullEvent();

        assertEquals(base, base);
        assertNotEquals(base, null);
        assertNotEquals(base, new Object());
        assertEquals(base, fullEvent());
        assertEquals(base.hashCode(), fullEvent().hashCode());
        assertNotNull(base.toString());

        assertNotEquals(base, TaskEventInfo.builder().type("other").build());
        assertNotEquals(base, TaskEventInfo.builder().taskId("other").build());
        assertNotEquals(base, TaskEventInfo.builder().payload("other").build());
        assertNotEquals(base, TaskEventInfo.builder().message("other").build());
        assertNotEquals(base, TaskEventInfo.builder().progress(99).build());
        assertNotEquals(base, TaskEventInfo.builder().error("boom").build());
        assertNotEquals(base, TaskEventInfo.builder().done(true).build());
    }
}
