package io.github.cuihairu.croupier.sdk.threading;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Immediate-execution and null-callback edge cases for MainThreadDispatcher.
 */
@DisplayName("MainThreadDispatcher immediate execution")
class MainThreadDispatcherExtraTest {

    @Test
    @DisplayName("callbacks throwing on the main thread are swallowed")
    void swallowingImmediateCallbackErrors() {
        MainThreadDispatcher dispatcher = MainThreadDispatcher.getInstance();
        dispatcher.initialize();

        AtomicInteger executed = new AtomicInteger();
        assertDoesNotThrow(() -> dispatcher.enqueue(() -> {
            executed.incrementAndGet();
            throw new IllegalStateException("callback failure");
        }));
        assertEquals(1, executed.get());
    }

    @Test
    @DisplayName("null callbacks and null consumers are ignored")
    void nullCallbacksIgnored() {
        MainThreadDispatcher dispatcher = MainThreadDispatcher.getInstance();
        dispatcher.initialize();
        assertDoesNotThrow(() -> dispatcher.enqueue((Runnable) null));
        assertDoesNotThrow(() -> dispatcher.enqueue(null, "data"));
    }
}
