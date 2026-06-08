package io.github.cuihairu.croupier.sdk.invoker;

import io.github.cuihairu.croupier.sdk.testing.FakeTransportClient;
import io.github.cuihairu.croupier.sdk.transport.Protocol;
import io.github.cuihairu.croupier.sdk.wire.SdkWireMessages;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.nio.charset.StandardCharsets;
import java.util.List;

import static io.github.cuihairu.croupier.sdk.invoker.InvokerException.ErrorCode;
import static org.junit.jupiter.api.Assertions.*;

/**
 * Tests to cover InvokerImpl.toStatusCode switch branches via retry logic.
 */
class InvokerImplStatusCodeTest {

    private InvokerConfig createRetryConfig(List<Integer> retryableCodes) {
        return InvokerConfig.builder()
            .address("127.0.0.1:19090")
            .insecure(true)
            .timeout(30000)
            .retry(RetryConfig.builder()
                .enabled(true)
                .maxAttempts(2)
                .initialDelayMs(1)
                .retryableStatusCodes(retryableCodes)
                .build())
            .build();
    }

    private void testRetryableWithStatusCode(int transportErrorCode, List<Integer> retryableCodes) {
        int[] callCount = {0};
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            callCount[0]++;
            if (callCount[0] == 1) {
                // Return error response that maps to the desired status code
                return SdkWireMessages.encodeInvokeResponse(
                    new SdkWireMessages.InvokeResponse(("error:" + transportErrorCode).getBytes(StandardCharsets.UTF_8))
                );
            }
            return SdkWireMessages.encodeInvokeResponse(
                new SdkWireMessages.InvokeResponse("ok".getBytes(StandardCharsets.UTF_8))
            );
        });

        InvokerImpl invoker = new InvokerImpl(createRetryConfig(retryableCodes),
            (address, timeout) -> transport);

        // This will invoke, get a response, and the retry logic checks InvokerException
        // Since we return a normal response, the toStatusCode won't be triggered this way.
        // We need to make the transport throw an InvokerException with specific ErrorCode.
    }

    @Test
    @DisplayName("invoke with retry should cover toStatusCode for CANCELLED")
    void retryWithCancelledError() throws InvokerException {
        int[] callCount = {0};
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            callCount[0]++;
            if (callCount[0] == 1) {
                throw new InvokerException(ErrorCode.CANCELLED, "cancelled");
            }
            return SdkWireMessages.encodeInvokeResponse(
                new SdkWireMessages.InvokeResponse("ok".getBytes(StandardCharsets.UTF_8))
            );
        });

        // CANCELLED maps to status 1, so retryable if code 1 is in the list
        InvokerConfig config = createRetryConfig(List.of(1));
        InvokerImpl invoker = new InvokerImpl(config, (address, timeout) -> transport);

        String result = invoker.invoke("func", "payload", InvokeOptions.create());
        assertEquals("ok", result);
        assertEquals(2, callCount[0]);
    }

    @Test
    @DisplayName("invoke with retry should cover toStatusCode for UNKNOWN")
    void retryWithUnknownError() throws InvokerException {
        int[] callCount = {0};
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            callCount[0]++;
            if (callCount[0] == 1) {
                throw new InvokerException(ErrorCode.UNKNOWN, "unknown");
            }
            return SdkWireMessages.encodeInvokeResponse(
                new SdkWireMessages.InvokeResponse("ok".getBytes(StandardCharsets.UTF_8))
            );
        });

        InvokerConfig config = createRetryConfig(List.of(2));
        InvokerImpl invoker = new InvokerImpl(config, (address, timeout) -> transport);

        String result = invoker.invoke("func", "payload", InvokeOptions.create());
        assertEquals("ok", result);
        assertEquals(2, callCount[0]);
    }

    @Test
    @DisplayName("invoke with retry should cover toStatusCode for INVALID_ARGUMENT")
    void retryWithInvalidArgument() throws InvokerException {
        int[] callCount = {0};
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            callCount[0]++;
            if (callCount[0] == 1) {
                throw new InvokerException(ErrorCode.INVALID_ARGUMENT, "invalid");
            }
            return SdkWireMessages.encodeInvokeResponse(
                new SdkWireMessages.InvokeResponse("ok".getBytes(StandardCharsets.UTF_8))
            );
        });

        InvokerConfig config = createRetryConfig(List.of(3));
        InvokerImpl invoker = new InvokerImpl(config, (address, timeout) -> transport);

        String result = invoker.invoke("func", "payload", InvokeOptions.create());
        assertEquals("ok", result);
        assertEquals(2, callCount[0]);
    }

    @Test
    @DisplayName("invoke with retry should cover toStatusCode for TIMEOUT")
    void retryWithTimeoutError() throws InvokerException {
        int[] callCount = {0};
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            callCount[0]++;
            if (callCount[0] == 1) {
                throw new InvokerException(ErrorCode.TIMEOUT, "timeout");
            }
            return SdkWireMessages.encodeInvokeResponse(
                new SdkWireMessages.InvokeResponse("ok".getBytes(StandardCharsets.UTF_8))
            );
        });

        InvokerConfig config = createRetryConfig(List.of(4));
        InvokerImpl invoker = new InvokerImpl(config, (address, timeout) -> transport);

        String result = invoker.invoke("func", "payload", InvokeOptions.create());
        assertEquals("ok", result);
    }

    @Test
    @DisplayName("invoke with retry should cover toStatusCode for NOT_FOUND")
    void retryWithNotFound() throws InvokerException {
        int[] callCount = {0};
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            callCount[0]++;
            if (callCount[0] == 1) {
                throw new InvokerException(ErrorCode.NOT_FOUND, "not found");
            }
            return SdkWireMessages.encodeInvokeResponse(
                new SdkWireMessages.InvokeResponse("ok".getBytes(StandardCharsets.UTF_8))
            );
        });

        InvokerConfig config = createRetryConfig(List.of(5));
        InvokerImpl invoker = new InvokerImpl(config, (address, timeout) -> transport);

        String result = invoker.invoke("func", "payload", InvokeOptions.create());
        assertEquals("ok", result);
    }

    @Test
    @DisplayName("invoke with retry should cover toStatusCode for ALREADY_EXISTS")
    void retryWithAlreadyExists() throws InvokerException {
        int[] callCount = {0};
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            callCount[0]++;
            if (callCount[0] == 1) {
                throw new InvokerException(ErrorCode.ALREADY_EXISTS, "exists");
            }
            return SdkWireMessages.encodeInvokeResponse(
                new SdkWireMessages.InvokeResponse("ok".getBytes(StandardCharsets.UTF_8))
            );
        });

        InvokerConfig config = createRetryConfig(List.of(6));
        InvokerImpl invoker = new InvokerImpl(config, (address, timeout) -> transport);

        String result = invoker.invoke("func", "payload", InvokeOptions.create());
        assertEquals("ok", result);
    }

    @Test
    @DisplayName("invoke with retry should cover toStatusCode for PERMISSION_DENIED")
    void retryWithPermissionDenied() throws InvokerException {
        int[] callCount = {0};
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            callCount[0]++;
            if (callCount[0] == 1) {
                throw new InvokerException(ErrorCode.PERMISSION_DENIED, "denied");
            }
            return SdkWireMessages.encodeInvokeResponse(
                new SdkWireMessages.InvokeResponse("ok".getBytes(StandardCharsets.UTF_8))
            );
        });

        InvokerConfig config = createRetryConfig(List.of(7));
        InvokerImpl invoker = new InvokerImpl(config, (address, timeout) -> transport);

        String result = invoker.invoke("func", "payload", InvokeOptions.create());
        assertEquals("ok", result);
    }

    @Test
    @DisplayName("invoke with retry should cover toStatusCode for RESOURCE_EXHAUSTED")
    void retryWithResourceExhausted() throws InvokerException {
        int[] callCount = {0};
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            callCount[0]++;
            if (callCount[0] == 1) {
                throw new InvokerException(ErrorCode.RESOURCE_EXHAUSTED, "exhausted");
            }
            return SdkWireMessages.encodeInvokeResponse(
                new SdkWireMessages.InvokeResponse("ok".getBytes(StandardCharsets.UTF_8))
            );
        });

        InvokerConfig config = createRetryConfig(List.of(8));
        InvokerImpl invoker = new InvokerImpl(config, (address, timeout) -> transport);

        String result = invoker.invoke("func", "payload", InvokeOptions.create());
        assertEquals("ok", result);
    }

    @Test
    @DisplayName("invoke with retry should cover toStatusCode for FAILED_PRECONDITION")
    void retryWithFailedPrecondition() throws InvokerException {
        int[] callCount = {0};
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            callCount[0]++;
            if (callCount[0] == 1) {
                throw new InvokerException(ErrorCode.FAILED_PRECONDITION, "precondition");
            }
            return SdkWireMessages.encodeInvokeResponse(
                new SdkWireMessages.InvokeResponse("ok".getBytes(StandardCharsets.UTF_8))
            );
        });

        InvokerConfig config = createRetryConfig(List.of(9));
        InvokerImpl invoker = new InvokerImpl(config, (address, timeout) -> transport);

        String result = invoker.invoke("func", "payload", InvokeOptions.create());
        assertEquals("ok", result);
    }

    @Test
    @DisplayName("invoke with retry should cover toStatusCode for ABORTED")
    void retryWithAborted() throws InvokerException {
        int[] callCount = {0};
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            callCount[0]++;
            if (callCount[0] == 1) {
                throw new InvokerException(ErrorCode.ABORTED, "aborted");
            }
            return SdkWireMessages.encodeInvokeResponse(
                new SdkWireMessages.InvokeResponse("ok".getBytes(StandardCharsets.UTF_8))
            );
        });

        InvokerConfig config = createRetryConfig(List.of(10));
        InvokerImpl invoker = new InvokerImpl(config, (address, timeout) -> transport);

        String result = invoker.invoke("func", "payload", InvokeOptions.create());
        assertEquals("ok", result);
    }

    @Test
    @DisplayName("invoke with retry should cover toStatusCode for OUT_OF_RANGE")
    void retryWithOutOfRange() throws InvokerException {
        int[] callCount = {0};
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            callCount[0]++;
            if (callCount[0] == 1) {
                throw new InvokerException(ErrorCode.OUT_OF_RANGE, "out of range");
            }
            return SdkWireMessages.encodeInvokeResponse(
                new SdkWireMessages.InvokeResponse("ok".getBytes(StandardCharsets.UTF_8))
            );
        });

        InvokerConfig config = createRetryConfig(List.of(11));
        InvokerImpl invoker = new InvokerImpl(config, (address, timeout) -> transport);

        String result = invoker.invoke("func", "payload", InvokeOptions.create());
        assertEquals("ok", result);
    }

    @Test
    @DisplayName("invoke with retry should cover toStatusCode for UNIMPLEMENTED")
    void retryWithUnimplemented() throws InvokerException {
        int[] callCount = {0};
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            callCount[0]++;
            if (callCount[0] == 1) {
                throw new InvokerException(ErrorCode.UNIMPLEMENTED, "unimplemented");
            }
            return SdkWireMessages.encodeInvokeResponse(
                new SdkWireMessages.InvokeResponse("ok".getBytes(StandardCharsets.UTF_8))
            );
        });

        InvokerConfig config = createRetryConfig(List.of(12));
        InvokerImpl invoker = new InvokerImpl(config, (address, timeout) -> transport);

        String result = invoker.invoke("func", "payload", InvokeOptions.create());
        assertEquals("ok", result);
    }

    @Test
    @DisplayName("invoke with retry should cover toStatusCode for DATA_LOSS")
    void retryWithDataLoss() throws InvokerException {
        int[] callCount = {0};
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            callCount[0]++;
            if (callCount[0] == 1) {
                throw new InvokerException(ErrorCode.DATA_LOSS, "data loss");
            }
            return SdkWireMessages.encodeInvokeResponse(
                new SdkWireMessages.InvokeResponse("ok".getBytes(StandardCharsets.UTF_8))
            );
        });

        InvokerConfig config = createRetryConfig(List.of(15));
        InvokerImpl invoker = new InvokerImpl(config, (address, timeout) -> transport);

        String result = invoker.invoke("func", "payload", InvokeOptions.create());
        assertEquals("ok", result);
    }

    @Test
    @DisplayName("invoke with retry should cover toStatusCode for UNAUTHENTICATED")
    void retryWithUnauthenticated() throws InvokerException {
        int[] callCount = {0};
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            callCount[0]++;
            if (callCount[0] == 1) {
                throw new InvokerException(ErrorCode.UNAUTHENTICATED, "unauthenticated");
            }
            return SdkWireMessages.encodeInvokeResponse(
                new SdkWireMessages.InvokeResponse("ok".getBytes(StandardCharsets.UTF_8))
            );
        });

        InvokerConfig config = createRetryConfig(List.of(16));
        InvokerImpl invoker = new InvokerImpl(config, (address, timeout) -> transport);

        String result = invoker.invoke("func", "payload", InvokeOptions.create());
        assertEquals("ok", result);
    }

    @Test
    @DisplayName("invoke with retry should cover toStatusCode for CONNECTION_FAILED")
    void retryWithConnectionFailed() throws InvokerException {
        int[] callCount = {0};
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            callCount[0]++;
            if (callCount[0] == 1) {
                throw new InvokerException(ErrorCode.CONNECTION_FAILED, "conn failed");
            }
            return SdkWireMessages.encodeInvokeResponse(
                new SdkWireMessages.InvokeResponse("ok".getBytes(StandardCharsets.UTF_8))
            );
        });

        // CONNECTION_FAILED maps to 2 (same as UNKNOWN)
        InvokerConfig config = createRetryConfig(List.of(2));
        InvokerImpl invoker = new InvokerImpl(config, (address, timeout) -> transport);

        String result = invoker.invoke("func", "payload", InvokeOptions.create());
        assertEquals("ok", result);
    }

    @Test
    @DisplayName("invoke with retry should cover toStatusCode for CONNECTION_TIMEOUT")
    void retryWithConnectionTimeout() throws InvokerException {
        int[] callCount = {0};
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            callCount[0]++;
            if (callCount[0] == 1) {
                throw new InvokerException(ErrorCode.CONNECTION_TIMEOUT, "conn timeout");
            }
            return SdkWireMessages.encodeInvokeResponse(
                new SdkWireMessages.InvokeResponse("ok".getBytes(StandardCharsets.UTF_8))
            );
        });

        // CONNECTION_TIMEOUT maps to 2 (same as UNKNOWN)
        InvokerConfig config = createRetryConfig(List.of(2));
        InvokerImpl invoker = new InvokerImpl(config, (address, timeout) -> transport);

        String result = invoker.invoke("func", "payload", InvokeOptions.create());
        assertEquals("ok", result);
    }

    @Test
    @DisplayName("invoke without retry should call supplier directly")
    void invokeWithoutRetry() throws InvokerException {
        FakeTransportClient transport = new FakeTransportClient((msgType, data) -> {
            return SdkWireMessages.encodeInvokeResponse(
                new SdkWireMessages.InvokeResponse("ok".getBytes(StandardCharsets.UTF_8))
            );
        });

        InvokerConfig config = InvokerConfig.builder()
            .address("127.0.0.1:19090")
            .insecure(true)
            .timeout(30000)
            .retry(RetryConfig.builder().enabled(false).build())
            .build();

        InvokerImpl invoker = new InvokerImpl(config, (address, timeout) -> transport);

        String result = invoker.invoke("func", "payload", InvokeOptions.create());
        assertEquals("ok", result);
    }
}
