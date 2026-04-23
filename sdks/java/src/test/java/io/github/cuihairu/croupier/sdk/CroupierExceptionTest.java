package io.github.cuihairu.croupier.sdk;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import org.junit.jupiter.api.Test;

class CroupierExceptionTest {

    @Test
    void constructorWithMessage() {
        String message = "Test error message";
        CroupierException exception = new CroupierException(message);

        assertEquals(message, exception.getMessage());
    }

    @Test
    void constructorWithMessageAndCause() {
        String message = "Error with cause";
        Throwable cause = new RuntimeException("Root cause");
        CroupierException exception = new CroupierException(message, cause);

        assertEquals(message, exception.getMessage());
        assertEquals(cause, exception.getCause());
    }

    @Test
    void constructorWithCause() {
        Throwable cause = new RuntimeException("Root cause");
        CroupierException exception = new CroupierException(cause);

        assertEquals(cause, exception.getCause());
    }

    @Test
    void constructorWithNullMessage() {
        CroupierException exception = new CroupierException((String) null);

        assertNull(exception.getMessage());
    }

    @Test
    void constructorWithNullCause() {
        CroupierException exception = new CroupierException("Message", null);

        assertEquals("Message", exception.getMessage());
        assertNull(exception.getCause());
    }

    @Test
    void canBeThrownAndCaught() {
        CroupierException thrown = assertThrows(
            CroupierException.class,
            () -> {
                throw new CroupierException("Test exception");
            }
        );

        assertEquals("Test exception", thrown.getMessage());
    }

    @Test
    void exceptionExtendsException() {
        // CroupierException extends Exception (checked exception)
        assertTrue(new CroupierException("test") instanceof Exception);
    }

    @Test
    void exceptionHasUsefulToString() {
        CroupierException exception = new CroupierException("Test error");

        String str = exception.toString();
        assertTrue(str.contains("CroupierException"));
        assertTrue(str.contains("Test error"));
    }
}
