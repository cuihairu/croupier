package io.github.cuihairu.croupier.sdk.invoker;

import io.github.cuihairu.croupier.sdk.testing.FakeTransportClient;
import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.Test;

import java.lang.reflect.Field;
import java.nio.charset.StandardCharsets;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;

/**
 * Final branch fillers: Json number parser edges and the null-retry
 * short-circuits in InvokerImpl/ServerHttpInvoker (config.retry is normally
 * never null, so the guard is driven via reflection).
 */
class BranchFillFinalTest {

    private static void nullRetryField(InvokerConfig config) throws Exception {
        Field retry = InvokerConfig.class.getDeclaredField("retry");
        retry.setAccessible(true);
        retry.set(config, null);
    }

    // ------------------------------------------------------------------
    // Json number() edges
    // ------------------------------------------------------------------

    @Test
    void minusOnlyNumberIsRejected() {
        // take('-') consumes the sign, then the source is exhausted
        assertThrows(IllegalArgumentException.class, () -> Json.parse("-"));
        assertThrows(IllegalArgumentException.class, () -> Json.parse("-x"));
    }

    @Test
    void integerOverflowFallsIntoNumberFormatCatch() {
        // 30 digits exceeds Long.MAX_VALUE and has no '.', 'e' markers
        assertThrows(IllegalArgumentException.class, () -> Json.parse("999999999999999999999999999999"));
        // exponent without digits after 'e' is rejected too
        assertThrows(IllegalArgumentException.class, () -> Json.parse("1e"));
    }

    // ------------------------------------------------------------------
    // InvokerImpl.withRetry: retryConfig == null -> direct supplier call
    // ------------------------------------------------------------------

    @Test
    void nullRetryConfigInvokesSupplierDirectly() throws Exception {
        InvokerConfig config = InvokerConfig.builder()
            .address("127.0.0.1:19090").insecure(true).timeout(2000).build();
        nullRetryField(config);

        FakeTransportClient transport = new FakeTransportClient((msgType, data) ->
            SdkWireMessages.encodeInvokeResponse(new SdkWireMessages.InvokeResponse(
                "\"ok\"".getBytes(StandardCharsets.UTF_8))));
        InvokerImpl invoker = new InvokerImpl(config, (address, timeout) -> transport);
        assertDoesNotThrow(invoker::connect);
        String result = assertDoesNotThrow(() -> invoker.invoke("fn", "{}"));
        assertEquals("\"ok\"", result);
        // the invoke request actually hit the (fake) transport
        assertEquals(1, transport.getCalls().size());
        assertEquals((Integer) Protocol.MSG_INVOKE_REQUEST, (Integer) transport.getCalls().get(0).msgType());
    }

    // ------------------------------------------------------------------
    // ServerHttpInvoker.request: retry == null -> single attempt
    // ------------------------------------------------------------------

    @Test
    void nullRetryOnHttpConfigPerformsSingleAttempt() throws Exception {
        InvokerConfig config = InvokerConfig.builder()
            .address("http://127.0.0.1:1").taskPollIntervalMs(1)
            .retry(RetryConfig.builder().enabled(true).maxAttempts(5).initialDelayMs(1).build())
            .build();
        nullRetryField(config);

        ServerHttpInvoker invoker = new ServerHttpInvoker(config);
        assertNotNull(invoker.getBaseUrl());
        assertDoesNotThrow(invoker::connect);
        // Connection refused: the retry guard (retry == null -> attempts = 1)
        // must be evaluated before the HTTP call fails fast.
        assertThrows(InvokerException.class, () -> invoker.invoke("fn", "{}"));
    }
}
