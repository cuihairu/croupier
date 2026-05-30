/*
 * Copyright 2025 Croupier Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package io.github.cuihairu.croupier.sdk.threading;

import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;

import static org.junit.jupiter.api.Assertions.*;

class MainThreadDispatcherTest {

    @BeforeEach
    void setUp() {
        MainThreadDispatcher.resetInstance();
        MainThreadDispatcher.getInstance().initialize();
    }

    @AfterEach
    void tearDown() {
        MainThreadDispatcher.resetInstance();
    }

    @Test
    void initialize_SetsMainThread() {
        assertTrue(MainThreadDispatcher.getInstance().isInitialized());
        assertTrue(MainThreadDispatcher.getInstance().isMainThread());
    }

    @Test
    void enqueue_FromMainThread_ExecutesImmediately() {
        AtomicBoolean executed = new AtomicBoolean(false);

        MainThreadDispatcher.getInstance().enqueue(() -> executed.set(true));

        assertTrue(executed.get());
    }

    @Test
    void enqueue_FromBackgroundThread_QueuesForLater() throws Exception {
        AtomicBoolean executed = new AtomicBoolean(false);

        CompletableFuture.runAsync(() -> {
            MainThreadDispatcher.getInstance().enqueue(() -> executed.set(true));
        }).get(5, TimeUnit.SECONDS);

        // Should not be executed yet
        assertFalse(executed.get());
        assertEquals(1, MainThreadDispatcher.getInstance().getPendingCount());

        // Process the queue
        int processed = MainThreadDispatcher.getInstance().processQueue();

        assertEquals(1, processed);
        assertTrue(executed.get());
        assertEquals(0, MainThreadDispatcher.getInstance().getPendingCount());
    }

    @Test
    void enqueue_WithData_PassesDataCorrectly() throws Exception {
        AtomicReference<String> receivedData = new AtomicReference<>();

        CompletableFuture.runAsync(() -> {
            MainThreadDispatcher.getInstance().enqueue(
                data -> receivedData.set(data),
                "test-data"
            );
        }).get(5, TimeUnit.SECONDS);

        MainThreadDispatcher.getInstance().processQueue();

        assertEquals("test-data", receivedData.get());
    }

    @Test
    void processQueue_RespectsMaxCount() throws Exception {
        AtomicInteger count = new AtomicInteger(0);

        // Enqueue 10 callbacks from background thread
        List<CompletableFuture<Void>> futures = new ArrayList<>();
        for (int i = 0; i < 10; i++) {
            futures.add(CompletableFuture.runAsync(() -> {
                MainThreadDispatcher.getInstance().enqueue(() -> count.incrementAndGet());
            }));
        }
        CompletableFuture.allOf(futures.toArray(new CompletableFuture[0])).get(5, TimeUnit.SECONDS);

        assertEquals(10, MainThreadDispatcher.getInstance().getPendingCount());

        // Process only 5
        int processed = MainThreadDispatcher.getInstance().processQueue(5);

        assertEquals(5, processed);
        assertEquals(5, count.get());
        assertEquals(5, MainThreadDispatcher.getInstance().getPendingCount());

        // Process remaining
        processed = MainThreadDispatcher.getInstance().processQueue();
        assertEquals(5, processed);
        assertEquals(10, count.get());
    }

    @Test
    void processQueue_HandlesExceptions() throws Exception {
        List<Integer> results = Collections.synchronizedList(new ArrayList<>());

        CompletableFuture.runAsync(() -> {
            MainThreadDispatcher.getInstance().enqueue(() -> results.add(1));
            MainThreadDispatcher.getInstance().enqueue(() -> {
                throw new RuntimeException("Test exception");
            });
            MainThreadDispatcher.getInstance().enqueue(() -> results.add(3));
        }).get(5, TimeUnit.SECONDS);

        // Should process all callbacks even with exception
        int processed = MainThreadDispatcher.getInstance().processQueue();

        assertEquals(3, processed);
        assertEquals(2, results.size());
        assertTrue(results.contains(1));
        assertTrue(results.contains(3));
    }

    @Test
    void clear_RemovesAllPendingCallbacks() throws Exception {
        List<CompletableFuture<Void>> futures = new ArrayList<>();
        for (int i = 0; i < 5; i++) {
            futures.add(CompletableFuture.runAsync(() -> {
                MainThreadDispatcher.getInstance().enqueue(() -> {});
            }));
        }
        CompletableFuture.allOf(futures.toArray(new CompletableFuture[0])).get(5, TimeUnit.SECONDS);

        assertEquals(5, MainThreadDispatcher.getInstance().getPendingCount());

        MainThreadDispatcher.getInstance().clear();

        assertEquals(0, MainThreadDispatcher.getInstance().getPendingCount());
    }

    @Test
    void setMaxProcessPerFrame_LimitsProcessing() throws Exception {
        MainThreadDispatcher.getInstance().setMaxProcessPerFrame(3);

        List<CompletableFuture<Void>> futures = new ArrayList<>();
        for (int i = 0; i < 10; i++) {
            futures.add(CompletableFuture.runAsync(() -> {
                MainThreadDispatcher.getInstance().enqueue(() -> {});
            }));
        }
        CompletableFuture.allOf(futures.toArray(new CompletableFuture[0])).get(5, TimeUnit.SECONDS);

        int processed = MainThreadDispatcher.getInstance().processQueue();
        assertEquals(3, processed);

        // Reset to unlimited
        MainThreadDispatcher.getInstance().setMaxProcessPerFrame(0);
    }

    @Test
    void enqueue_NullCallback_IsIgnored() throws Exception {
        int initialCount = MainThreadDispatcher.getInstance().getPendingCount();

        CompletableFuture.runAsync(() -> {
            MainThreadDispatcher.getInstance().enqueue((Runnable) null);
        }).get(5, TimeUnit.SECONDS);

        assertEquals(initialCount, MainThreadDispatcher.getInstance().getPendingCount());
    }

    @Test
    void concurrentEnqueue_IsThreadSafe() throws Exception {
        AtomicInteger counter = new AtomicInteger(0);
        final int threadCount = 10;
        final int enqueuesPerThread = 100;

        ExecutorService executor = Executors.newFixedThreadPool(threadCount);
        CountDownLatch latch = new CountDownLatch(threadCount);

        for (int t = 0; t < threadCount; t++) {
            executor.submit(() -> {
                try {
                    for (int i = 0; i < enqueuesPerThread; i++) {
                        MainThreadDispatcher.getInstance().enqueue(() -> counter.incrementAndGet());
                    }
                } finally {
                    latch.countDown();
                }
            });
        }

        assertTrue(latch.await(10, TimeUnit.SECONDS));
        executor.shutdown();

        // Process all
        int totalProcessed = 0;
        int processed;
        while ((processed = MainThreadDispatcher.getInstance().processQueue(100)) > 0) {
            totalProcessed += processed;
        }

        assertEquals(threadCount * enqueuesPerThread, counter.get());
        assertEquals(threadCount * enqueuesPerThread, totalProcessed);
    }

    @Test
    void isMainThread_ReturnsFalse_OnBackgroundThread() throws Exception {
        AtomicBoolean isMainThread = new AtomicBoolean(true);

        CompletableFuture.runAsync(() -> {
            isMainThread.set(MainThreadDispatcher.getInstance().isMainThread());
        }).get(5, TimeUnit.SECONDS);

        assertFalse(isMainThread.get());
    }
}
