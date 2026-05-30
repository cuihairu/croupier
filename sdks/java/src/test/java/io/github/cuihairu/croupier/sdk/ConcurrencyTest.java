// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

package io.github.cuihairu.croupier.sdk;

import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

import java.util.List;
import java.util.ArrayList;
import java.util.Map;
import java.util.HashMap;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.locks.Lock;
import java.util.concurrent.locks.ReentrantLock;

/**
 * Concurrency tests for Croupier SDK.
 */
public class ConcurrencyTest {

    // ========== Concurrent Client Creation ==========

    @Test
    public void testMultipleThreadsCreateClients() throws InterruptedException {
        int numThreads = 10;
        CountDownLatch startLatch = new CountDownLatch(1);
        CountDownLatch endLatch = new CountDownLatch(numThreads);
        List<Exception> exceptions = new CopyOnWriteArrayList<>();

        for (int i = 0; i < numThreads; i++) {
            final int threadId = i;
            new Thread(() -> {
                try {
                    startLatch.await(); // Wait for all threads to be ready
                    ClientConfig config = new ClientConfig();
                    config.setServiceId("service-" + threadId);
                    CroupierClientImpl client = new CroupierClientImpl(config);
                    assertNotNull(client);
                } catch (Exception e) {
                    exceptions.add(e);
                } finally {
                    endLatch.countDown();
                }
            }).start();
        }

        startLatch.countDown(); // Start all threads
        assertTrue(endLatch.await(10, TimeUnit.SECONDS));
        assertTrue(exceptions.isEmpty(), "Exceptions occurred: " + exceptions);
    }

    @Test
    public void testConcurrentConfigLoading() throws InterruptedException {
        int numThreads = 20;
        CountDownLatch latch = new CountDownLatch(numThreads);
        List<Integer> results = new CopyOnWriteArrayList<>();
        List<Exception> exceptions = new CopyOnWriteArrayList<>();

        for (int i = 0; i < numThreads; i++) {
            final int threadId = i;
            new Thread(() -> {
                try {
                    ClientConfig config = new ClientConfig();
                    config.setServiceId("test-" + threadId);
                    assertEquals("test-" + threadId, config.getServiceId());
                    results.add(threadId);
                } catch (Exception e) {
                    exceptions.add(e);
                } finally {
                    latch.countDown();
                }
            }).start();
        }

        assertTrue(latch.await(5, TimeUnit.SECONDS));
        assertEquals(numThreads, results.size());
        assertTrue(exceptions.isEmpty());
    }

    // ========== Shared Data Access ==========

    @Test
    public void testConcurrentMapAccess() throws InterruptedException {
        int numThreads = 10;
        int numOperations = 100;
        Map<String, Integer> map = new ConcurrentHashMap<>();
        CountDownLatch latch = new CountDownLatch(numThreads);

        for (int i = 0; i < numThreads; i++) {
            final int threadId = i;
            new Thread(() -> {
                try {
                    for (int j = 0; j < numOperations; j++) {
                        map.put("key_" + threadId + "_" + j, j);
                        Integer value = map.get("key_" + threadId + "_" + j);
                        assertEquals(j, value);
                    }
                } finally {
                    latch.countDown();
                }
            }).start();
        }

        assertTrue(latch.await(10, TimeUnit.SECONDS));
        assertEquals(numThreads * numOperations, map.size());
    }

    @Test
    public void testConcurrentListOperations() throws InterruptedException {
        int numThreads = 10;
        int numOperations = 100;
        List<String> list = new CopyOnWriteArrayList<>();
        CountDownLatch latch = new CountDownLatch(numThreads);

        for (int i = 0; i < numThreads; i++) {
            final int threadId = i;
            new Thread(() -> {
                try {
                    for (int j = 0; j < numOperations; j++) {
                        list.add("item_" + threadId + "_" + j);
                    }
                } finally {
                    latch.countDown();
                }
            }).start();
        }

        assertTrue(latch.await(10, TimeUnit.SECONDS));
        assertEquals(numThreads * numOperations, list.size());
    }

    // ========== Atomic Operations ==========

    @Test
    public void testAtomicCounter() throws InterruptedException {
        int numThreads = 10;
        int numOperations = 1000;
        AtomicInteger counter = new AtomicInteger(0);
        CountDownLatch latch = new CountDownLatch(numThreads);

        for (int i = 0; i < numThreads; i++) {
            new Thread(() -> {
                try {
                    for (int j = 0; j < numOperations; j++) {
                        counter.incrementAndGet();
                    }
                } finally {
                    latch.countDown();
                }
            }).start();
        }

        assertTrue(latch.await(10, TimeUnit.SECONDS));
        assertEquals(numThreads * numOperations, counter.get());
    }

    @Test
    public void testCompareAndSet() throws InterruptedException {
        int numThreads = 10;
        int numOperations = 100;
        AtomicInteger value = new AtomicInteger(0);
        AtomicInteger successCount = new AtomicInteger(0);
        CountDownLatch latch = new CountDownLatch(numThreads);

        for (int i = 0; i < numThreads; i++) {
            new Thread(() -> {
                try {
                    for (int j = 0; j < numOperations; j++) {
                        int expected = value.get();
                        int desired = expected + 1;
                        if (value.compareAndSet(expected, desired)) {
                            successCount.incrementAndGet();
                        }
                    }
                } finally {
                    latch.countDown();
                }
            }).start();
        }

        assertTrue(latch.await(10, TimeUnit.SECONDS));
        // At least some operations should succeed
        assertTrue(successCount.get() > 0);
        assertEquals(successCount.get(), value.get());
    }

    // ========== Lock Operations ==========

    @Test
    public void testLockPerformance() throws InterruptedException {
        int numOperations = 100000;
        Lock lock = new ReentrantLock();
        AtomicInteger counter = new AtomicInteger(0);
        CountDownLatch latch = new CountDownLatch(1);

        long startTime = System.currentTimeMillis();
        new Thread(() -> {
            try {
                for (int i = 0; i < numOperations; i++) {
                    lock.lock();
                    try {
                        counter.incrementAndGet();
                    } finally {
                        lock.unlock();
                    }
                }
            } finally {
                latch.countDown();
            }
        }).start();

        assertTrue(latch.await(5, TimeUnit.SECONDS));
        long elapsed = System.currentTimeMillis() - startTime;

        assertEquals(numOperations, counter.get());
        assertTrue(elapsed < 2000, "Should complete in less than 2 seconds, took: " + elapsed + "ms");
    }

    @Test
    public void testMultipleThreadsWithLock() throws InterruptedException {
        int numThreads = 10;
        int numOperations = 1000;
        Lock lock = new ReentrantLock();
        AtomicInteger counter = new AtomicInteger(0);
        CountDownLatch latch = new CountDownLatch(numThreads);

        for (int i = 0; i < numThreads; i++) {
            new Thread(() -> {
                try {
                    for (int j = 0; j < numOperations; j++) {
                        lock.lock();
                        try {
                            counter.incrementAndGet();
                        } finally {
                            lock.unlock();
                        }
                    }
                } finally {
                    latch.countDown();
                }
            }).start();
        }

        assertTrue(latch.await(10, TimeUnit.SECONDS));
        assertEquals(numThreads * numOperations, counter.get());
    }

    // ========== Thread Pool ==========

    @Test
    public void testThreadPoolExecutor() throws InterruptedException {
        int numTasks = 100;
        ExecutorService executor = Executors.newFixedThreadPool(10);
        CountDownLatch latch = new CountDownLatch(numTasks);
        AtomicInteger counter = new AtomicInteger(0);

        for (int i = 0; i < numTasks; i++) {
            executor.submit(() -> {
                try {
                    counter.incrementAndGet();
                } finally {
                    latch.countDown();
                }
            });
        }

        assertTrue(latch.await(10, TimeUnit.SECONDS));
        assertEquals(numTasks, counter.get());
        executor.shutdown();
    }

    @Test
    public void testThreadPoolWithReturnValue() throws InterruptedException, ExecutionException {
        int numTasks = 20;
        ExecutorService executor = Executors.newFixedThreadPool(5);
        List<Future<Integer>> futures = new ArrayList<>();

        for (int i = 0; i < numTasks; i++) {
            final int taskId = i;
            Future<Integer> future = executor.submit(() -> taskId * 2);
            futures.add(future);
        }

        for (int i = 0; i < numTasks; i++) {
            assertEquals(i * 2, futures.get(i).get());
        }

        executor.shutdown();
    }

    // ========== Concurrent Exception Handling ==========

    @Test
    public void testConcurrentExceptionHandling() throws InterruptedException {
        int numThreads = 10;
        CountDownLatch latch = new CountDownLatch(numThreads);
        List<Exception> caughtExceptions = new CopyOnWriteArrayList<>();

        for (int i = 0; i < numThreads; i++) {
            final int threadId = i;
            new Thread(() -> {
                try {
                    throw new RuntimeException("Error from thread " + threadId);
                } catch (Exception e) {
                    caughtExceptions.add(e);
                } finally {
                    latch.countDown();
                }
            }).start();
        }

        assertTrue(latch.await(5, TimeUnit.SECONDS));
        assertEquals(numThreads, caughtExceptions.size());
    }

    // ========== Producer-Consumer Pattern ==========

    @Test
    public void testProducerConsumer() throws InterruptedException {
        int numItems = 100;
        BlockingQueue<Integer> queue = new ArrayBlockingQueue<>(10);
        CountDownLatch latch = new CountDownLatch(2);
        AtomicInteger producedCount = new AtomicInteger(0);
        AtomicInteger consumedCount = new AtomicInteger(0);

        // Producer
        new Thread(() -> {
            try {
                for (int i = 0; i < numItems; i++) {
                    queue.put(i);
                    producedCount.incrementAndGet();
                }
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            } finally {
                latch.countDown();
            }
        }).start();

        // Consumer
        new Thread(() -> {
            try {
                for (int i = 0; i < numItems; i++) {
                    queue.take();
                    consumedCount.incrementAndGet();
                }
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
            } finally {
                latch.countDown();
            }
        }).start();

        assertTrue(latch.await(10, TimeUnit.SECONDS));
        assertEquals(numItems, producedCount.get());
        assertEquals(numItems, consumedCount.get());
    }

    // ========== Barrier Synchronization ==========

    @Test
    public void testBarrierSynchronization() throws InterruptedException {
        int numThreads = 5;
        CyclicBarrier barrier = new CyclicBarrier(numThreads);
        List<Integer> results = new CopyOnWriteArrayList<>();
        CountDownLatch latch = new CountDownLatch(numThreads);

        for (int i = 0; i < numThreads; i++) {
            final int threadId = i;
            new Thread(() -> {
                try {
                    // Phase 1
                    results.add(threadId * 100 + 1);
                    barrier.await();

                    // Phase 2 (all threads completed phase 1)
                    results.add(threadId * 100 + 2);
                    barrier.await();

                    // Phase 3 (all threads completed phase 2)
                    results.add(threadId * 100 + 3);
                } catch (Exception e) {
                    fail("Barrier synchronization failed: " + e.getMessage());
                } finally {
                    latch.countDown();
                }
            }).start();
        }

        assertTrue(latch.await(10, TimeUnit.SECONDS));
        assertEquals(numThreads * 3, results.size());
    }

    // ========== CountDown Latch ==========

    @Test
    public void testCountDownLatch() throws InterruptedException {
        int numThreads = 10;
        CountDownLatch startLatch = new CountDownLatch(1);
        CountDownLatch endLatch = new CountDownLatch(numThreads);
        List<Integer> results = new CopyOnWriteArrayList<>();

        for (int i = 0; i < numThreads; i++) {
            final int threadId = i;
            new Thread(() -> {
                try {
                    startLatch.await(); // Wait for signal
                    results.add(threadId);
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                } finally {
                    endLatch.countDown();
                }
            }).start();
        }

        Thread.sleep(100); // Let all threads reach the latch
        startLatch.countDown(); // Signal all threads to proceed

        assertTrue(endLatch.await(5, TimeUnit.SECONDS));
        assertEquals(numThreads, results.size());
    }

    // ========== Concurrent Resource Cleanup ==========

    @Test
    public void testConcurrentResourceCleanup() throws InterruptedException {
        int numOperations = 100;
        CountDownLatch latch = new CountDownLatch(numOperations);
        List<Exception> exceptions = new CopyOnWriteArrayList<>();

        for (int i = 0; i < numOperations; i++) {
            new Thread(() -> {
                try {
                    ClientConfig config = new ClientConfig();
                    config.setServiceId("temp-service");
                    CroupierClientImpl client = new CroupierClientImpl(config);
                    // Client will be garbage collected
                } catch (Exception e) {
                    exceptions.add(e);
                } finally {
                    latch.countDown();
                }
            }).start();
        }

        assertTrue(latch.await(10, TimeUnit.SECONDS));
        assertTrue(exceptions.isEmpty());
    }

    // ========== Race Condition Tests ==========

    @Test
    public void testRaceConditionWithoutLock() throws InterruptedException {
        int numThreads = 10;
        int numOperations = 100;
        int[] unsafeCounter = new int[1];
        CountDownLatch latch = new CountDownLatch(numThreads);

        for (int i = 0; i < numThreads; i++) {
            new Thread(() -> {
                try {
                    for (int j = 0; j < numOperations; j++) {
                        unsafeCounter[0]++;
                    }
                } finally {
                    latch.countDown();
                }
            }).start();
        }

        assertTrue(latch.await(5, TimeUnit.SECONDS));
        // Due to race condition, count may not equal expected
        assertTrue(unsafeCounter[0] > 0);
        assertTrue(unsafeCounter[0] <= numThreads * numOperations);
    }

    @Test
    public void testNoRaceConditionWithLock() throws InterruptedException {
        int numThreads = 10;
        int numOperations = 100;
        int[] safeCounter = new int[1];
        Lock lock = new ReentrantLock();
        CountDownLatch latch = new CountDownLatch(numThreads);

        for (int i = 0; i < numThreads; i++) {
            new Thread(() -> {
                try {
                    for (int j = 0; j < numOperations; j++) {
                        lock.lock();
                        try {
                            safeCounter[0]++;
                        } finally {
                            lock.unlock();
                        }
                    }
                } finally {
                    latch.countDown();
                }
            }).start();
        }

        assertTrue(latch.await(5, TimeUnit.SECONDS));
        assertEquals(numThreads * numOperations, safeCounter[0]);
    }

    // ========== Concurrent Performance Test ==========

    @Test
    public void testConcurrentPerformance() throws InterruptedException {
        int numThreads = 50;
        int numOperations = 1000;
        AtomicInteger counter = new AtomicInteger(0);
        CountDownLatch latch = new CountDownLatch(numThreads);

        long startTime = System.currentTimeMillis();

        for (int i = 0; i < numThreads; i++) {
            new Thread(() -> {
                try {
                    for (int j = 0; j < numOperations; j++) {
                        counter.incrementAndGet();
                    }
                } finally {
                    latch.countDown();
                }
            }).start();
        }

        assertTrue(latch.await(30, TimeUnit.SECONDS));
        long elapsed = System.currentTimeMillis() - startTime;

        assertEquals(numThreads * numOperations, counter.get());
        System.out.println("Completed " + (numThreads * numOperations) + " operations in " + elapsed + "ms");
        assertTrue(elapsed < 10000, "Should complete in less than 10 seconds");
    }
}
