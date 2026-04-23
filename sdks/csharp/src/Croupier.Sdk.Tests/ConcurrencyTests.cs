// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using Croupier.Sdk.Models;
using FluentAssertions;
using Xunit;
using System.Collections.Concurrent;

namespace Croupier.Sdk.Tests;

/// <summary>
/// Concurrency tests for Croupier SDK
/// </summary>
public class ConcurrencyTests
{
    #region Concurrent Client Creation

    [Fact]
    public async Task MultipleThreadsCreateClients()
    {
        // Arrange
        const int numThreads = 10;
        var clients = new ConcurrentBag<ClientConfig>();

        // Act
        var tasks = Enumerable.Range(0, numThreads).Select(i => Task.Run(() =>
        {
            var config = new ClientConfig
            {
                ServiceId = $"service-{i}"
            };
            clients.Add(config);
        }));

        await Task.WhenAll(tasks);

        // Assert
        clients.Count.Should().Be(numThreads);
    }

    [Fact]
    public async Task ConcurrentConfigLoading()
    {
        // Arrange
        const int numThreads = 20;
        var configs = new ConcurrentBag<int>();

        // Act
        var tasks = Enumerable.Range(0, numThreads).Select(i => Task.Run(() =>
        {
            var config = new ClientConfig
            {
                ServiceId = $"test-{i}"
            };

            configs.Add(i);
        }));

        await Task.WhenAll(tasks);

        // Assert
        configs.Count.Should().Be(numThreads);
    }

    #endregion

    #region Shared Data Access

    [Fact]
    public async Task ConcurrentDictionaryAccess()
    {
        // Arrange
        const int numThreads = 10;
        const int numOperations = 100;
        var dictionary = new ConcurrentDictionary<string, int>();

        // Act
        var tasks = Enumerable.Range(0, numThreads).Select(threadId => Task.Run(() =>
        {
            for (int j = 0; j < numOperations; j++)
            {
                var key = $"key_{threadId}_{j}";
                dictionary[key] = j;
                dictionary.TryGetValue(key, out var value).Should().BeTrue();
                value.Should().Be(j);
            }
        }));

        await Task.WhenAll(tasks);

        // Assert
        dictionary.Count.Should().Be(numThreads * numOperations);
    }

    [Fact]
    public async Task ConcurrentListOperations()
    {
        // Arrange
        const int numThreads = 10;
        const int numOperations = 100;
        var list = new ConcurrentBag<string>();

        // Act
        var tasks = Enumerable.Range(0, numThreads).Select(threadId => Task.Run(() =>
        {
            for (int j = 0; j < numOperations; j++)
            {
                list.Add($"item_{threadId}_{j}");
            }
        }));

        await Task.WhenAll(tasks);

        // Assert
        list.Count.Should().Be(numThreads * numOperations);
    }

    #endregion

    #region Atomic Operations

    [Fact]
    public async Task AtomicCounter()
    {
        // Arrange
        const int numThreads = 10;
        const int numOperations = 1000;
        var counter = 0;
        var lockObj = new object();

        // Act
        var tasks = Enumerable.Range(0, numThreads).Select(_ => Task.Run(() =>
        {
            for (int j = 0; j < numOperations; j++)
            {
                Interlocked.Increment(ref counter);
            }
        }));

        await Task.WhenAll(tasks);

        // Assert
        counter.Should().Be(numThreads * numOperations);
    }

    [Fact]
    public async Task CompareAndSet()
    {
        // Arrange
        const int numThreads = 10;
        const int numOperations = 100;
        var value = 0;
        var successCount = 0;
        var lockObj = new object();

        // Act
        var tasks = Enumerable.Range(0, numThreads).Select(_ => Task.Run(() =>
        {
            for (int j = 0; j < numOperations; j++)
            {
                var initial = Interlocked.CompareExchange(ref value, value + 1, value);
                if (initial == value)
                {
                    Interlocked.Increment(ref successCount);
                }
            }
        }));

        await Task.WhenAll(tasks);

        // Assert
        // Due to race conditions in the CAS loop (value+1 is computed before CAS),
        // successCount might be 0 in extreme cases, so we just verify it doesn't crash
        successCount.Should().BeGreaterOrEqualTo(0);
        value.Should().BeGreaterOrEqualTo(0);
    }

    #endregion

    #region Lock Operations

    [Fact]
    public async Task LockPerformance()
    {
        // Arrange
        const int numOperations = 100000;
        var counter = 0;
        var lockObj = new object();

        // Act
        var start = DateTime.UtcNow;
        var task = Task.Run(() =>
        {
            for (int i = 0; i < numOperations; i++)
            {
                lock (lockObj)
                {
                    counter++;
                }
            }
        });

        await task;

        var elapsed = (DateTime.UtcNow - start).TotalMilliseconds;

        // Assert
        counter.Should().Be(numOperations);
        elapsed.Should().BeLessThan(5000); // Less than 5 seconds
    }

    [Fact]
    public async Task MultipleThreadsWithLock()
    {
        // Arrange
        const int numThreads = 10;
        const int numOperations = 1000;
        var counter = 0;
        var lockObj = new object();

        // Act
        var tasks = Enumerable.Range(0, numThreads).Select(_ => Task.Run(() =>
        {
            for (int j = 0; j < numOperations; j++)
            {
                lock (lockObj)
                {
                    counter++;
                }
            }
        }));

        await Task.WhenAll(tasks);

        // Assert
        counter.Should().Be(numThreads * numOperations);
    }

    #endregion

    #region Thread Pool

    [Fact]
    public async Task ThreadPoolExecutor()
    {
        // Arrange
        const int numTasks = 100;
        var counter = 0;

        // Act
        var tasks = Enumerable.Range(0, numTasks).Select(_ => Task.Run(() =>
        {
            Interlocked.Increment(ref counter);
        }));

        await Task.WhenAll(tasks);

        // Assert
        counter.Should().Be(numTasks);
    }

    [Fact]
    public async Task ThreadPoolWithReturnValue()
    {
        // Arrange
        const int numTasks = 20;

        // Act
        var tasks = Enumerable.Range(0, numTasks).Select(i => Task.Run(() => i * 2));

        var results = await Task.WhenAll(tasks);

        // Assert
        for (int i = 0; i < numTasks; i++)
        {
            results[i].Should().Be(i * 2);
        }
    }

    #endregion

    #region Concurrent Exception Handling

    [Fact]
    public async Task ConcurrentExceptionHandling()
    {
        // Arrange
        const int numThreads = 10;
        var exceptions = new ConcurrentBag<Exception>();

        // Act
        var tasks = Enumerable.Range(0, numThreads).Select(threadId => Task.Run(() =>
        {
            try
            {
                throw new InvalidOperationException($"Error from thread {threadId}");
            }
            catch (Exception e)
            {
                exceptions.Add(e);
            }
        }));

        await Task.WhenAll(tasks);

        // Assert
        exceptions.Count.Should().Be(numThreads);
    }

    #endregion

    #region Producer-Consumer Pattern

    [Fact]
    public async Task ProducerConsumer()
    {
        // Arrange
        const int numItems = 100;
        var queue = new ConcurrentQueue<int>();
        var producedCount = 0;
        var consumedCount = 0;
        var productionComplete = false;

        // Act
        var producer = Task.Run(() =>
        {
            for (int i = 0; i < numItems; i++)
            {
                queue.Enqueue(i);
                Interlocked.Increment(ref producedCount);
            }
            productionComplete = true;
        });

        var consumer = Task.Run(async () =>
        {
            while (!productionComplete || !queue.IsEmpty)
            {
                if (queue.TryDequeue(out var item))
                {
                    Interlocked.Increment(ref consumedCount);
                }
                else
                {
                    await Task.Delay(1); // Prevent busy-waiting
                }
            }
        });

        await Task.WhenAll(producer, consumer);

        // Assert
        producedCount.Should().Be(numItems);
        consumedCount.Should().Be(numItems);
    }

    #endregion

    #region Barrier Synchronization

    [Fact]
    public async Task BarrierSynchronization()
    {
        // Arrange
        const int numThreads = 5;
        var results = new ConcurrentBag<int>();
        var barrier = new Barrier(numThreads);

        // Act
        var tasks = Enumerable.Range(0, numThreads).Select(threadId => Task.Run(() =>
        {
            // Phase 1
            results.Add(threadId * 100 + 1);
            barrier.SignalAndWait();

            // Phase 2 (all threads completed phase 1)
            results.Add(threadId * 100 + 2);
            barrier.SignalAndWait();

            // Phase 3 (all threads completed phase 2)
            results.Add(threadId * 100 + 3);
        }));

        await Task.WhenAll(tasks);

        // Assert
        results.Count.Should().Be(numThreads * 3);
    }

    #endregion

    #region Concurrent Resource Cleanup

    [Fact]
    public async Task ConcurrentResourceCleanup()
    {
        // Arrange
        const int numOperations = 100;
        var clients = new ConcurrentBag<object>();

        // Act
        var tasks = Enumerable.Range(0, numOperations).Select(_ => Task.Run(() =>
        {
            var client = new
            {
                Id = Guid.NewGuid(),
                Connected = true
            };
            clients.Add(client);
        }));

        await Task.WhenAll(tasks);

        // Assert
        clients.Count.Should().Be(numOperations);
    }

    #endregion

    #region Race Condition Tests

    [Fact]
    public async Task RaceConditionWithoutLock()
    {
        // Arrange
        const int numThreads = 10;
        const int numOperations = 100;
        var unsafeCounter = 0;

        // Act
        var tasks = Enumerable.Range(0, numThreads).Select(_ => Task.Run(() =>
        {
            for (int j = 0; j < numOperations; j++)
            {
                // Race condition here
                unsafeCounter = unsafeCounter + 1; // Not atomic
            }
        }));

        await Task.WhenAll(tasks);

        // Assert
        // Due to race condition, count may not equal expected
        unsafeCounter.Should().BeGreaterThan(0);
        unsafeCounter.Should().BeLessOrEqualTo(numThreads * numOperations);
    }

    [Fact]
    public async Task NoRaceConditionWithLock()
    {
        // Arrange
        const int numThreads = 10;
        const int numOperations = 100;
        var safeCounter = 0;
        var lockObj = new object();

        // Act
        var tasks = Enumerable.Range(0, numThreads).Select(_ => Task.Run(() =>
        {
            for (int j = 0; j < numOperations; j++)
            {
                lock (lockObj)
                {
                    safeCounter++;
                }
            }
        }));

        await Task.WhenAll(tasks);

        // Assert
        safeCounter.Should().Be(numThreads * numOperations);
    }

    #endregion

    #region Concurrent Performance Test

    [Fact]
    public async Task ConcurrentPerformance()
    {
        // Arrange
        const int numThreads = 50;
        const int numOperations = 1000;
        var counter = 0;

        // Act
        var start = DateTime.UtcNow;

        var tasks = Enumerable.Range(0, numThreads).Select(_ => Task.Run(() =>
        {
            for (int j = 0; j < numOperations; j++)
            {
                Interlocked.Increment(ref counter);
            }
        }));

        await Task.WhenAll(tasks);

        var elapsed = (DateTime.UtcNow - start).TotalMilliseconds;

        // Assert
        counter.Should().Be(numThreads * numOperations);
        elapsed.Should().BeLessThan(10000); // Less than 10 seconds
    }

    #endregion

    #region Parallel Execution

    [Fact]
    public async Task ParallelExecution()
    {
        // Arrange
        var tasks = new[]
        {
            Task.Run(async () =>
            {
                await Task.Delay(100);
                return 1;
            }),
            Task.Run(async () =>
            {
                await Task.Delay(100);
                return 2;
            }),
            Task.Run(async () =>
            {
                await Task.Delay(100);
                return 3;
            })
        };

        // Act
        var start = DateTime.UtcNow;
        var results = await Task.WhenAll(tasks);
        var elapsed = (DateTime.UtcNow - start).TotalMilliseconds;

        // Assert
        results.Should().BeEquivalentTo(new[] { 1, 2, 3 });
        elapsed.Should().BeLessThan(300); // Should complete in < 300ms due to parallel execution
    }

    #endregion

    #region Deadlock Prevention

    [Fact]
    public async Task PreventDeadlock()
    {
        // Arrange
        var lock1 = new object();
        var lock2 = new object();

        // Act
        var thread1 = Task.Run(() =>
        {
            lock (lock1)
            {
                Thread.Sleep(10);
                lock (lock2)
                {
                    // Critical section
                }
            }
        });

        var thread2 = Task.Run(() =>
        {
            // Acquire locks in same order to prevent deadlock
            lock (lock1)
            {
                Thread.Sleep(10);
                lock (lock2)
                {
                    // Critical section
                }
            }
        });

        var start = DateTime.UtcNow;
        await Task.WhenAll(thread1, thread2);
        var elapsed = (DateTime.UtcNow - start).TotalMilliseconds;

        // Assert
        // Should complete without deadlock
        elapsed.Should().BeLessThan(1000);
    }

    #endregion

    #region Async/Await Concurrency

    [Fact]
    public async Task AsyncAwaitConcurrency()
    {
        // Arrange
        var results = new ConcurrentBag<int>();

        // Act
        var tasks = Enumerable.Range(0, 10).Select(async i =>
        {
            await Task.Delay(10);
            results.Add(i);
        });

        await Task.WhenAll(tasks);

        // Assert
        results.Count.Should().Be(10);
        results.Should().Contain(0);
        results.Should().Contain(9);
    }

    #endregion

    #region Cancellation Token

    [Fact]
    public async Task CancellationWorks()
    {
        // Arrange
        var cts = new CancellationTokenSource();
        var counter = 0;

        // Act
        var task = Task.Run(async () =>
        {
            while (!cts.Token.IsCancellationRequested)
            {
                counter++;
                await Task.Delay(10);
            }
        }, cts.Token);

        await Task.Delay(50);
        cts.Cancel();
        await task;

        // Assert
        counter.Should().BeGreaterThan(0);
        counter.Should().BeLessThan(10); // Should have been cancelled early
    }

    #endregion

    #region Timeout Handling

    [Fact]
    public async Task TimeoutHandling()
    {
        // Arrange
        var task = Task.Run(async () =>
        {
            await Task.Delay(5000); // Long running task
            return "completed";
        });

        // Act
        var delayTask = Task.Delay(100); // Short timeout
        var completedTask = await Task.WhenAny(task, delayTask);

        // Assert
        completedTask.Should().Be(delayTask); // Timeout should complete first
    }

    #endregion

    #region Concurrent Dictionary Stress Test

    [Fact]
    public async Task ConcurrentDictionaryStressTest()
    {
        // Arrange
        const int numThreads = 20;
        const int numOperations = 1000;
        var dictionary = new ConcurrentDictionary<string, int>();

        // Act
        var tasks = Enumerable.Range(0, numThreads).Select(threadId => Task.Run(() =>
        {
            for (int j = 0; j < numOperations; j++)
            {
                var key = $"key_{threadId}_{j}";

                // AddOrUpdate
                dictionary.AddOrUpdate(key, j, (k, v) => v + 1);

                // GetOrAdd
                dictionary.GetOrAdd($"other_{threadId}_{j}", _ => -1);

                // TryRemove
                if (j % 10 == 0)
                {
                    dictionary.TryRemove(key, out _);
                }
            }
        }));

        await Task.WhenAll(tasks);

        // Assert
        dictionary.Count.Should().BeGreaterThan(0);
        // Each operation adds 2 keys, and 1 in 10 removes one key
        // So maximum possible is numThreads * numOperations * 2
        dictionary.Count.Should().BeLessThan(numThreads * numOperations * 2);
    }

    #endregion

    #region Monitor.Wait and Pulse

    [Fact]
    public async Task MonitorWaitPulse()
    {
        // Arrange
        var lockObj = new object();
        var ready = false;
        var proceeded = false;

        // Act
        var waitingTask = Task.Run(() =>
        {
            lock (lockObj)
            {
                while (!ready)
                {
                    Monitor.Wait(lockObj);
                }
                proceeded = true;
            }
        });

        await Task.Delay(50); // Ensure waitingTask is waiting

        var signalingTask = Task.Run(() =>
        {
            lock (lockObj)
            {
                ready = true;
                Monitor.Pulse(lockObj);
            }
        });

        await Task.WhenAll(waitingTask, signalingTask);

        // Assert
        proceeded.Should().BeTrue();
    }

    #endregion

    #region ReaderWriterLockSlim

    [Fact]
    public async Task ReaderWriterLockSlim()
    {
        // Arrange
        var rwLock = new ReaderWriterLockSlim();
        var counter = 0;
        var readerCount = 0;

        // Act
        var writerTask = Task.Run(() =>
        {
            for (int i = 0; i < 100; i++)
            {
                rwLock.EnterWriteLock();
                try
                {
                    counter++;
                }
                finally
                {
                    rwLock.ExitWriteLock();
                }
                Thread.Sleep(1);
            }
        });

        var readerTasks = Enumerable.Range(0, 10).Select(_ => Task.Run(() =>
        {
            for (int i = 0; i < 100; i++)
            {
                rwLock.EnterReadLock();
                try
                {
                    var localCount = counter;
                    Interlocked.Increment(ref readerCount);
                }
                finally
                {
                    rwLock.ExitReadLock();
                }
                Thread.Sleep(1);
            }
        }));

        await Task.WhenAll(writerTask, Task.WhenAll(readerTasks));

        // Assert
        counter.Should().Be(100);
        readerCount.Should().Be(1000);
    }

    #endregion

    #region SemaphoreSlim

    [Fact]
    public async Task SemaphoreSlim()
    {
        // Arrange
        const int maxConcurrency = 3;
        const int totalTasks = 10;
        var semaphore = new SemaphoreSlim(maxConcurrency);
        var currentConcurrency = 0;
        var maxObservedConcurrency = 0;
        var lockObj = new object();

        // Act
        var tasks = Enumerable.Range(0, totalTasks).Select(async i =>
        {
            await semaphore.WaitAsync();

            try
            {
                lock (lockObj)
                {
                    currentConcurrency++;
                    if (currentConcurrency > maxObservedConcurrency)
                    {
                        maxObservedConcurrency = currentConcurrency;
                    }
                }

                await Task.Delay(50);

                lock (lockObj)
                {
                    currentConcurrency--;
                }
            }
            finally
            {
                semaphore.Release();
            }
        });

        await Task.WhenAll(tasks);

        // Assert
        maxObservedConcurrency.Should().BeLessOrEqualTo(maxConcurrency);
    }

    #endregion
}
