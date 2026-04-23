package io.github.cuihairu.croupier.sdk.invoker;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.DisplayName;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Tests for InvokerException.
 */
class InvokerExceptionTest {

    @Test
    @DisplayName("Exception with message should be created")
    void testExceptionWithMessage() {
        InvokerException exception = new InvokerException(
                InvokerException.ErrorCode.CONNECTION_FAILED,
                "Connection failed"
        );

        assertEquals(InvokerException.ErrorCode.CONNECTION_FAILED, exception.getErrorCode());
        assertEquals("Connection failed", exception.getMessage());
        assertNull(exception.getCause());
    }

    @Test
    @DisplayName("Exception with message and cause should be created")
    void testExceptionWithMessageAndCause() {
        Throwable cause = new RuntimeException("Underlying error");
        InvokerException exception = new InvokerException(
                InvokerException.ErrorCode.INTERNAL,
                "Internal error",
                cause
        );

        assertEquals(InvokerException.ErrorCode.INTERNAL, exception.getErrorCode());
        assertEquals("Internal error", exception.getMessage());
        assertEquals(cause, exception.getCause());
    }

    @Test
    @DisplayName("Error codes should be defined")
    void testErrorCodes() {
        assertNotNull(InvokerException.ErrorCode.CONNECTION_FAILED);
        assertNotNull(InvokerException.ErrorCode.INTERNAL);
        assertNotNull(InvokerException.ErrorCode.TIMEOUT);
        assertNotNull(InvokerException.ErrorCode.NOT_FOUND);
        assertNotNull(InvokerException.ErrorCode.INVALID_ARGUMENT);
    }

    @Test
    @DisplayName("Exception should be throwable")
    void testExceptionIsThrowable() {
        assertThrows(InvokerException.class, () -> {
            throw new InvokerException(InvokerException.ErrorCode.TIMEOUT, "Timeout");
        });
    }

    @Test
    @DisplayName("Exception should preserve error code")
    void testErrorCodePreservation() {
        for (InvokerException.ErrorCode code : InvokerException.ErrorCode.values()) {
            InvokerException exception = new InvokerException(code, "Test message");
            assertEquals(code, exception.getErrorCode());
        }
    }

    @Test
    @DisplayName("fromStatusCode should map status codes correctly")
    void testFromStatusCode() {
        // Test all known status codes
        assertEquals(InvokerException.ErrorCode.CANCELLED,
                InvokerException.fromStatusCode(1, "Cancelled").getErrorCode());
        assertEquals(InvokerException.ErrorCode.UNKNOWN,
                InvokerException.fromStatusCode(2, "Unknown").getErrorCode());
        assertEquals(InvokerException.ErrorCode.INVALID_ARGUMENT,
                InvokerException.fromStatusCode(3, "Invalid argument").getErrorCode());
        assertEquals(InvokerException.ErrorCode.TIMEOUT,
                InvokerException.fromStatusCode(4, "Timeout").getErrorCode());
        assertEquals(InvokerException.ErrorCode.NOT_FOUND,
                InvokerException.fromStatusCode(5, "Not found").getErrorCode());
        assertEquals(InvokerException.ErrorCode.ALREADY_EXISTS,
                InvokerException.fromStatusCode(6, "Already exists").getErrorCode());
        assertEquals(InvokerException.ErrorCode.PERMISSION_DENIED,
                InvokerException.fromStatusCode(7, "Permission denied").getErrorCode());
        assertEquals(InvokerException.ErrorCode.RESOURCE_EXHAUSTED,
                InvokerException.fromStatusCode(8, "Resource exhausted").getErrorCode());
        assertEquals(InvokerException.ErrorCode.FAILED_PRECONDITION,
                InvokerException.fromStatusCode(9, "Failed precondition").getErrorCode());
        assertEquals(InvokerException.ErrorCode.ABORTED,
                InvokerException.fromStatusCode(10, "Aborted").getErrorCode());
        assertEquals(InvokerException.ErrorCode.OUT_OF_RANGE,
                InvokerException.fromStatusCode(11, "Out of range").getErrorCode());
        assertEquals(InvokerException.ErrorCode.UNIMPLEMENTED,
                InvokerException.fromStatusCode(12, "Unimplemented").getErrorCode());
        assertEquals(InvokerException.ErrorCode.INTERNAL,
                InvokerException.fromStatusCode(13, "Internal").getErrorCode());
        assertEquals(InvokerException.ErrorCode.UNAVAILABLE,
                InvokerException.fromStatusCode(14, "Unavailable").getErrorCode());
        assertEquals(InvokerException.ErrorCode.DATA_LOSS,
                InvokerException.fromStatusCode(15, "Data loss").getErrorCode());
        assertEquals(InvokerException.ErrorCode.UNAUTHENTICATED,
                InvokerException.fromStatusCode(16, "Unauthenticated").getErrorCode());
    }

    @Test
    @DisplayName("fromStatusCode with unknown code should return UNKNOWN")
    void testFromStatusCodeUnknown() {
        InvokerException exception = InvokerException.fromStatusCode(999, "Unknown error");
        assertEquals(InvokerException.ErrorCode.UNKNOWN, exception.getErrorCode());
        assertEquals("Unknown error", exception.getMessage());
    }

    @Test
    @DisplayName("fromStatusCode with message should create exception")
    void testFromStatusCodeWithMessage() {
        InvokerException exception = InvokerException.fromStatusCode(4, "Request timeout");
        assertEquals(InvokerException.ErrorCode.TIMEOUT, exception.getErrorCode());
        assertEquals("Request timeout", exception.getMessage());
    }

    @Test
    @DisplayName("fromStatusCode with message and cause should create exception")
    void testFromStatusCodeWithMessageAndCause() {
        Throwable cause = new RuntimeException("Underlying timeout");
        InvokerException exception = InvokerException.fromStatusCode(4, "Request timeout", cause);
        assertEquals(InvokerException.ErrorCode.TIMEOUT, exception.getErrorCode());
        assertEquals("Request timeout", exception.getMessage());
        assertEquals(cause, exception.getCause());
    }

    @Test
    @DisplayName("equals should return true for same error code and message")
    void testEquals() {
        InvokerException ex1 = new InvokerException(
                InvokerException.ErrorCode.CONNECTION_FAILED,
                "Connection failed"
        );
        InvokerException ex2 = new InvokerException(
                InvokerException.ErrorCode.CONNECTION_FAILED,
                "Connection failed"
        );

        assertEquals(ex1, ex2);
        assertEquals(ex1.hashCode(), ex2.hashCode());
    }

    @Test
    @DisplayName("equals should return false for different error codes")
    void testEqualsDifferentErrorCode() {
        InvokerException ex1 = new InvokerException(
                InvokerException.ErrorCode.CONNECTION_FAILED,
                "Error"
        );
        InvokerException ex2 = new InvokerException(
                InvokerException.ErrorCode.TIMEOUT,
                "Error"
        );

        assertNotEquals(ex1, ex2);
    }

    @Test
    @DisplayName("equals should return false for different messages")
    void testEqualsDifferentMessage() {
        InvokerException ex1 = new InvokerException(
                InvokerException.ErrorCode.CONNECTION_FAILED,
                "Error 1"
        );
        InvokerException ex2 = new InvokerException(
                InvokerException.ErrorCode.CONNECTION_FAILED,
                "Error 2"
        );

        assertNotEquals(ex1, ex2);
    }

    @Test
    @DisplayName("toString should contain error code and message")
    void testToString() {
        InvokerException exception = new InvokerException(
                InvokerException.ErrorCode.TIMEOUT,
                "Request timeout after 30s"
        );

        String str = exception.toString();
        assertTrue(str.contains("TIMEOUT"));
        assertTrue(str.contains("Request timeout after 30s"));
    }

    @Test
    @DisplayName("ErrorCode getCode should return code string")
    void testErrorCodeGetCode() {
        assertEquals("CONNECTION_FAILED",
                InvokerException.ErrorCode.CONNECTION_FAILED.getCode());
        assertEquals("TIMEOUT",
                InvokerException.ErrorCode.TIMEOUT.getCode());
        assertEquals("NOT_FOUND",
                InvokerException.ErrorCode.NOT_FOUND.getCode());
    }

    @Test
    @DisplayName("fromGrpcStatus (deprecated) should still work")
    void testFromGrpcStatusDeprecated() {
        @SuppressWarnings("deprecation")
        InvokerException exception = InvokerException.fromGrpcStatus(5, "Not found");
        assertEquals(InvokerException.ErrorCode.NOT_FOUND, exception.getErrorCode());
    }

    @Test
    @DisplayName("fromGrpcStatus with cause (deprecated) should still work")
    void testFromGrpcStatusWithCauseDeprecated() {
        Throwable cause = new RuntimeException("Cause");
        @SuppressWarnings("deprecation")
        InvokerException exception = InvokerException.fromGrpcStatus(5, "Not found", cause);
        assertEquals(InvokerException.ErrorCode.NOT_FOUND, exception.getErrorCode());
        assertEquals(cause, exception.getCause());
    }

    @Test
    @DisplayName("Exception with null cause should work")
    void testExceptionWithNullCause() {
        InvokerException exception = new InvokerException(
                InvokerException.ErrorCode.INTERNAL,
                "Error",
                null
        );

        assertEquals(InvokerException.ErrorCode.INTERNAL, exception.getErrorCode());
        assertEquals("Error", exception.getMessage());
        assertNull(exception.getCause());
    }

    @Test
    @DisplayName("Exception can be caught as Exception")
    void testExceptionCanBeCaughtAsException() {
        Exception caught = null;
        try {
            throw new InvokerException(InvokerException.ErrorCode.TIMEOUT, "Timeout");
        } catch (Exception e) {
            caught = e;
        }

        assertNotNull(caught);
        assertTrue(caught instanceof InvokerException);
        assertEquals(InvokerException.ErrorCode.TIMEOUT,
                ((InvokerException) caught).getErrorCode());
    }

    @Test
    @DisplayName("fromStatusCode with zero should map to UNKNOWN")
    void testFromStatusCodeZero() {
        InvokerException exception = InvokerException.fromStatusCode(0, "Zero status");
        assertEquals(InvokerException.ErrorCode.UNKNOWN, exception.getErrorCode());
    }

    @Test
    @DisplayName("fromStatusCode with negative should map to UNKNOWN")
    void testFromStatusCodeNegative() {
        InvokerException exception = InvokerException.fromStatusCode(-1, "Negative status");
        assertEquals(InvokerException.ErrorCode.UNKNOWN, exception.getErrorCode());
    }

    @Test
    @DisplayName("All error codes have string values")
    void testAllErrorCodesHaveStrings() {
        for (InvokerException.ErrorCode code : InvokerException.ErrorCode.values()) {
            assertNotNull(code.getCode());
            assertFalse(code.getCode().isEmpty());
        }
    }
}
