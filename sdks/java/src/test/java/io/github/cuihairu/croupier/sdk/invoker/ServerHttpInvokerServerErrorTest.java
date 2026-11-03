package io.github.cuihairu.croupier.sdk.invoker;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.lang.reflect.Method;

import static org.junit.jupiter.api.Assertions.*;

/**
 * C-015: serverErrorMessage must surface a structured error message, and must
 * not silently swallow an unparseable error body — it logs a warning and falls
 * back to the raw body (never throws).
 */
@DisplayName("ServerHttpInvoker serverErrorMessage")
class ServerHttpInvokerServerErrorTest {

    private static String serverErrorMessage(String body) throws Exception {
        Method method = ServerHttpInvoker.class
            .getDeclaredMethod("serverErrorMessage", String.class);
        method.setAccessible(true);
        return (String) method.invoke(null, body);
    }

    @Test
    @DisplayName("extracts the message field from a JSON error body")
    void extractsMessage() throws Exception {
        assertEquals("boom", serverErrorMessage("{\"message\":\"boom\"}"));
        assertEquals("boom", serverErrorMessage("{\"error\":\"internal_error\",\"message\":\"boom\"}"));
    }

    @Test
    @DisplayName("falls back to the error field when message is absent or blank")
    void fallsBackToError() throws Exception {
        assertEquals("internal_error", serverErrorMessage("{\"error\":\"internal_error\"}"));
        assertEquals("internal_error", serverErrorMessage("{\"message\":\"  \",\"error\":\"internal_error\"}"));
    }

    @Test
    @DisplayName("returns the raw body when it is valid JSON but carries no message or error")
    void returnsRawBodyForStructuredNonError() throws Exception {
        assertEquals("{\"code\":5}", serverErrorMessage("{\"code\":5}"));
        assertEquals("{}", serverErrorMessage("{}"));
    }

    @Test
    @DisplayName("returns the raw body for unparseable input instead of throwing")
    void returnsRawBodyForUnparseable() throws Exception {
        // C-015: previously this was catch (IllegalArgumentException ignored) {}
        // — the failure was swallowed with no record. It must still return the
        // raw body (never throw), but now logs a warning.
        assertEquals("plain text error", serverErrorMessage("plain text error"));
        assertEquals("{\"message\"", serverErrorMessage("{\"message\""));
    }

    @Test
    @DisplayName("maps empty or null body to a placeholder")
    void emptyBody() throws Exception {
        assertEquals("empty response body", serverErrorMessage(""));
        assertEquals("empty response body", serverErrorMessage("   "));
        assertEquals("empty response body", serverErrorMessage(null));
    }
}
