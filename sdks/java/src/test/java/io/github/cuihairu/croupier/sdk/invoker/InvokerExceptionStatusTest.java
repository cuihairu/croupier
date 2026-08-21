package io.github.cuihairu.croupier.sdk.invoker;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

/**
 * HTTP status mapping and equality edge cases for InvokerException.
 */
@DisplayName("InvokerException status mapping")
class InvokerExceptionStatusTest {

    @Test
    @DisplayName("fromHttpStatus maps well-known status codes")
    void statusMapping() {
        assertEquals(InvokerException.ErrorCode.INVALID_ARGUMENT, InvokerException.fromHttpStatus(400, "bad").getErrorCode());
        assertEquals(InvokerException.ErrorCode.UNAUTHENTICATED, InvokerException.fromHttpStatus(401, "no").getErrorCode());
        assertEquals(InvokerException.ErrorCode.PERMISSION_DENIED, InvokerException.fromHttpStatus(403, "denied").getErrorCode());
        assertEquals(InvokerException.ErrorCode.NOT_FOUND, InvokerException.fromHttpStatus(404, "missing").getErrorCode());
        assertEquals(InvokerException.ErrorCode.ALREADY_EXISTS, InvokerException.fromHttpStatus(409, "dup").getErrorCode());
        assertEquals(InvokerException.ErrorCode.RESOURCE_EXHAUSTED, InvokerException.fromHttpStatus(429, "busy").getErrorCode());
        assertEquals(InvokerException.ErrorCode.TIMEOUT, InvokerException.fromHttpStatus(408, "slow").getErrorCode());
        assertEquals(InvokerException.ErrorCode.TIMEOUT, InvokerException.fromHttpStatus(504, "gateway").getErrorCode());
        assertEquals(InvokerException.ErrorCode.UNAVAILABLE, InvokerException.fromHttpStatus(503, "down").getErrorCode());
        assertEquals(InvokerException.ErrorCode.UNAVAILABLE, InvokerException.fromHttpStatus(500, "boom").getErrorCode());
        assertEquals(InvokerException.ErrorCode.UNKNOWN, InvokerException.fromHttpStatus(302, "redirect").getErrorCode());
    }

    @Test
    @DisplayName("fromHttpStatus keeps the server-provided message")
    void statusMessage() {
        assertTrue(InvokerException.fromHttpStatus(403, "denied").getMessage().endsWith("denied"));
    }

    @Test
    @DisplayName("equals handles identity, null and foreign types")
    void equalityEdgeCases() {
        InvokerException error = new InvokerException(InvokerException.ErrorCode.NOT_FOUND, "missing");
        assertEquals(error, error);
        assertNotEquals(error, null);
        assertNotEquals(error, "missing");
        assertEquals(error, new InvokerException(InvokerException.ErrorCode.NOT_FOUND, "missing"));
    }
}
