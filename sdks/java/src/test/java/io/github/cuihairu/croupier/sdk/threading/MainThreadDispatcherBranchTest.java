package io.github.cuihairu.croupier.sdk.threading;

import org.junit.jupiter.api.Test;

import java.lang.reflect.Field;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;

import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertSame;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** Branch-complete coverage for MainThreadDispatcher singleton and guard branches. */
class MainThreadDispatcherBranchTest {

    private static void resetSingleton() throws Exception {
        Field instance = MainThreadDispatcher.class.getDeclaredField("instance");
        instance.setAccessible(true);
        MainThreadDispatcher current = (MainThreadDispatcher) instance.get(null);
        if (current != null) {
            Field mainThreadId = MainThreadDispatcher.class.getDeclaredField("mainThreadId");
            mainThreadId.setAccessible(true);
            ((java.util.concurrent.atomic.AtomicLong) mainThreadId.get(current)).set(-1);
            Field initialized = MainThreadDispatcher.class.getDeclaredField("initialized");
            initialized.setAccessible(true);
            initialized.set(current, false);
            current.clear();
        }
        instance.set(null, null);
    }

    @Test
    void getInstanceCreatesAndReusesSingleton() throws Exception {
        resetSingleton();
        MainThreadDispatcher first = MainThreadDispatcher.getInstance();
        MainThreadDispatcher second = MainThreadDispatcher.getInstance();
        assertSame(first, second);
        // restore initialized singleton for other tests
        first.initialize();
        assertTrue(first.isMainThread());
    }

    @Test
    void concurrentGetInstanceExercisesDoubleCheckedLocking() throws Exception {
        for (int round = 0; round < 20; round++) {
            resetSingleton();
            int threads = 8;
            CountDownLatch ready = new CountDownLatch(threads);
            CountDownLatch go = new CountDownLatch(1);
            List<MainThreadDispatcher> results =
                new java.util.concurrent.CopyOnWriteArrayList<>();
            java.util.List<Thread> workers = new ArrayList<>();
            for (int i = 0; i < threads; i++) {
                Thread worker = new Thread(() -> {
                    ready.countDown();
                    try {
                        go.await(2, TimeUnit.SECONDS);
                    } catch (InterruptedException ignored) {
                        Thread.currentThread().interrupt();
                    }
                    results.add(MainThreadDispatcher.getInstance());
                });
                worker.start();
                workers.add(worker);
            }
            ready.await(2, TimeUnit.SECONDS);
            go.countDown();
            for (Thread worker : workers) {
                worker.join(2000);
            }
            assertTrue(results.size() == threads);
            for (MainThreadDispatcher dispatcher : results) {
                assertSame(results.get(0), dispatcher);
            }
        }
        MainThreadDispatcher.getInstance().initialize();
    }

    @Test
    void isMainThreadReturnsFalseBeforeInitialization() throws Exception {
        resetSingleton();
        MainThreadDispatcher dispatcher = MainThreadDispatcher.getInstance();
        assertFalse(dispatcher.isMainThread());
        dispatcher.initialize();
        assertTrue(dispatcher.isMainThread());
    }

    @Test
    void enqueueBeforeInitializationRunsViaProcessQueue() throws Exception {
        resetSingleton();
        MainThreadDispatcher dispatcher = MainThreadDispatcher.getInstance();
        CountDownLatch ran = new CountDownLatch(1);
        AtomicBoolean ranOnCallingThread = new AtomicBoolean(false);
        // not initialized: enqueue must queue instead of running inline
        dispatcher.enqueue(() -> {
            ranOnCallingThread.set(true);
            ran.countDown();
        });
        assertFalse(ran.await(200, TimeUnit.MILLISECONDS), "must not run inline before initialize()");
        dispatcher.initialize();
        dispatcher.processQueue();
        assertTrue(ran.await(2, TimeUnit.SECONDS));
        // after initialization on the main thread it did execute during processQueue
        assertTrue(ranOnCallingThread.get());
    }

    @Test
    void processQueueAfterClearIsSafe() throws Exception {
        MainThreadDispatcher dispatcher = MainThreadDispatcher.getInstance();
        dispatcher.initialize();
        dispatcher.clear();
        dispatcher.processQueue();
        assertEquals(0, dispatcher.getPendingCount());
    }

    private static void assertEquals(long expected, long actual) {
        assertTrue(expected == actual, "expected " + expected + " but was " + actual);
    }
}
