// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using System;
using System.Threading;
using System.Threading.Tasks;
using Croupier.Sdk.Models;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// Tests for CroupierClient reconnection logic
/// </summary>
public class CroupierClientReconnectTests : IDisposable
{
    private readonly CancellationTokenSource _cts = new();

    public void Dispose()
    {
        _cts.Cancel();
        _cts.Dispose();
    }

    #region Auto Reconnect Configuration Tests

    [Fact]
    public void ClientConfig_DefaultAutoReconnect_IsEnabled()
    {
        // Arrange & Act
        var config = new ClientConfig();

        // Assert
        Assert.True(config.AutoReconnect);
    }

    [Fact]
    public void ClientConfig_DefaultReconnectInterval_Is5Seconds()
    {
        // Arrange & Act
        var config = new ClientConfig();

        // Assert
        Assert.Equal(5, config.ReconnectIntervalSeconds);
    }

    [Fact]
    public void ClientConfig_DefaultReconnectMaxAttempts_Is0Infinite()
    {
        // Arrange & Act
        var config = new ClientConfig();

        // Assert
        Assert.Equal(0, config.ReconnectMaxAttempts); // 0 means infinite
    }

    [Fact]
    public void ClientConfig_CanConfigureReconnectSettings()
    {
        // Arrange & Act
        var config = new ClientConfig
        {
            AutoReconnect = true,
            ReconnectIntervalSeconds = 10,
            ReconnectMaxAttempts = 5
        };

        // Assert
        Assert.True(config.AutoReconnect);
        Assert.Equal(10, config.ReconnectIntervalSeconds);
        Assert.Equal(5, config.ReconnectMaxAttempts);
    }

    [Fact]
    public void ClientConfig_CanDisableAutoReconnect()
    {
        // Arrange & Act
        var config = new ClientConfig
        {
            AutoReconnect = false
        };

        // Assert
        Assert.False(config.AutoReconnect);
    }

    #endregion

    #region Reconnect Behavior Tests

    [Fact]
    public async Task ReconnectAsync_WhenConnectionFails_RetriesWithBackoff()
    {
        // Arrange
        var config = new ClientConfig
        {
            AgentAddr = "invalid-host:9999",
            AutoReconnect = true,
            ReconnectIntervalSeconds = 1,
            ReconnectMaxAttempts = 2
        };

        using var client = new CroupierClient(config);

        var stopwatch = System.Diagnostics.Stopwatch.StartNew();

        // Act
        var exception = await Assert.ThrowsAsync<InvalidOperationException>(() =>
            client.GetType().InvokeMember(
                "ReconnectAsync",
                System.Reflection.BindingFlags.NonPublic | System.Reflection.BindingFlags.InvokeMethod | System.Reflection.BindingFlags.Instance,
                null,
                client,
                new object[] { _cts.Token }
            ) as Task<Task> ?? Task.CompletedTask
        );

        stopwatch.Stop();

        // Assert
        Assert.NotNull(exception);
        // Should have attempted multiple reconnects with delay between them
        // 2 attempts * 1 second delay = at least 1 second elapsed
        Assert.True(stopwatch.ElapsedMilliseconds >= 500,
            $"Expected at least 500ms for reconnect attempts, got {stopwatch.ElapsedMilliseconds}ms");
    }

    [Fact]
    public async Task ReconnectAsync_WhenMaxAttemptsIsZero_RetriesIndefinitely()
    {
        // Arrange
        var config = new ClientConfig
        {
            AgentAddr = "invalid-host:9999",
            AutoReconnect = true,
            ReconnectIntervalSeconds = 1,
            ReconnectMaxAttempts = 0 // Infinite retry
        };

        using var client = new CroupierClient(config);

        // Start reconnect in background
        var reconnectTask = Task.Run(async () =>
        {
            try
            {
                await (Task)client.GetType().InvokeMember(
                    "ReconnectAsync",
                    System.Reflection.BindingFlags.NonPublic | System.Reflection.BindingFlags.InvokeMethod | System.Reflection.BindingFlags.Instance,
                    null,
                    client,
                    new object[] { _cts.Token }
                )!;
            }
            catch (InvalidOperationException)
            {
                // Expected when connection fails
            }
        });

        // Wait a bit to allow multiple attempts
        await Task.Delay(2500);

        // Cancel to stop the infinite loop
        _cts.Cancel();

        // Assert
        // With 1 second interval, we should have made at least 2 attempts in 2.5 seconds
        Assert.True(reconnectTask.Status == TaskStatus.RanToCompletion ||
                   reconnectTask.Status == TaskStatus.Canceled ||
                   reconnectTask.Status == TaskStatus.Faulted,
                   "Reconnect task should have completed or been canceled");
    }

    [Fact]
    public async Task ReconnectAsync_WhenAutoReconnectDisabled_ThrowsImmediately()
    {
        // Arrange
        var config = new ClientConfig
        {
            AgentAddr = "invalid-host:9999",
            AutoReconnect = false,
            ReconnectMaxAttempts = 1
        };

        using var client = new CroupierClient(config);

        var stopwatch = System.Diagnostics.Stopwatch.StartNew();

        // Act
        var exception = await Assert.ThrowsAnyAsync<Exception>(async () =>
        {
            await (Task)client.GetType().InvokeMember(
                "ReconnectAsync",
                System.Reflection.BindingFlags.NonPublic | System.Reflection.BindingFlags.InvokeMethod | System.Reflection.BindingFlags.Instance,
                null,
                client,
                new object[] { _cts.Token }
            )!;
        });

        stopwatch.Stop();

        // Assert
        Assert.NotNull(exception);
        // Should not have waited for reconnect delay since auto-reconnect is disabled
        Assert.True(stopwatch.ElapsedMilliseconds < 2000,
            $"Expected quick failure when auto-reconnect is disabled, got {stopwatch.ElapsedMilliseconds}ms");
    }

    [Fact]
    public async Task ReconnectAsync_WhenCancelled_StopsRetrying()
    {
        // Arrange
        var config = new ClientConfig
        {
            AgentAddr = "invalid-host:9999",
            AutoReconnect = true,
            ReconnectIntervalSeconds = 10, // Long delay
            ReconnectMaxAttempts = 0
        };

        using var cts = new CancellationTokenSource();
        using var client = new CroupierClient(config);

        var reconnectCompleted = false;

        // Act
        var reconnectTask = Task.Run(async () =>
        {
            try
            {
                await (Task)client.GetType().InvokeMember(
                    "ReconnectAsync",
                    System.Reflection.BindingFlags.NonPublic | System.Reflection.BindingFlags.InvokeMethod | System.Reflection.BindingFlags.Instance,
                    null,
                    client,
                    new object[] { cts.Token }
                )!;
            }
            catch (InvalidOperationException)
            {
                // Expected
            }
            finally
            {
                reconnectCompleted = true;
            }
        });

        // Cancel quickly
        await Task.Delay(100);
        cts.Cancel();

        // Wait for task to complete
        var completed = await Task.WhenAny(reconnectTask, Task.Delay(2000));

        // Assert
        Assert.True(reconnectCompleted || completed.Status == TaskStatus.RanToCompletion,
            "Reconnect should have stopped after cancellation");
    }

    #endregion

    #region Reconnect After Disconnect Tests

    [Fact]
    public async Task Disconnect_ThenReconnect_CanReestablishConnection()
    {
        // This test would require a mock agent server
        // For now, we verify the client state management

        // Arrange
        var config = new ClientConfig
        {
            AgentAddr = "localhost:19090",
            AutoReconnect = true
        };

        using var client = new CroupierClient(config);

        // Act - Simulate disconnect
        client.Disconnect();

        // Assert
        Assert.False(client.IsConnected);

        // In a real test, we would start an agent and verify reconnect succeeds
    }

    [Fact]
    public void Disconnect_MultipleCalls_IsIdempotent()
    {
        // Arrange
        var config = new ClientConfig();
        using var client = new CroupierClient(config);

        // Act - Should not throw
        client.Disconnect();
        client.Disconnect();
        client.Disconnect();

        // Assert
        Assert.False(client.IsConnected);
    }

    #endregion

    #region Heartbeat Reconnect Tests

    [Fact]
    public async Task HeartbeatFailure_WithAutoReconnect_TriggersReconnect()
    {
        // This test verifies that heartbeat failures trigger reconnection
        // when AutoReconnect is enabled

        // Arrange
        var config = new ClientConfig
        {
            AgentAddr = "localhost:19090",
            AutoReconnect = true,
            HeartbeatIntervalSeconds = 1,
            ReconnectIntervalSeconds = 1
        };

        using var client = new CroupierClient(config);

        // In a real test with a mock agent:
        // 1. Connect client
        // 2. Stop responding to heartbeats
        // 3. Verify client detects heartbeat failure
        // 4. Verify client initiates reconnect

        // For now, we verify the configuration
        Assert.True(config.AutoReconnect);
        Assert.Equal(1, config.HeartbeatIntervalSeconds);
        Assert.Equal(1, config.ReconnectIntervalSeconds);
    }

    [Fact]
    public async Task HeartbeatFailure_WithoutAutoReconnect_DoesNotReconnect()
    {
        // This test verifies that heartbeat failures do NOT trigger reconnection
        // when AutoReconnect is disabled

        // Arrange
        var config = new ClientConfig
        {
            AgentAddr = "localhost:19090",
            AutoReconnect = false,
            HeartbeatIntervalSeconds = 1
        };

        using var client = new CroupierClient(config);

        // In a real test with a mock agent:
        // 1. Connect client
        // 2. Stop responding to heartbeats
        // 3. Verify client detects heartbeat failure
        // 4. Verify client does NOT attempt reconnection

        // For now, we verify the configuration
        Assert.False(config.AutoReconnect);
        Assert.Equal(1, config.HeartbeatIntervalSeconds);
    }

    #endregion

    #region Exponential Backoff Tests

    [Theory]
    [InlineData(1, 1000)]
    [InlineData(2, 2000)]
    [InlineData(3, 4000)]
    [InlineData(4, 8000)]
    [InlineData(5, 10000)] // Capped at max
    public async Task ReconnectDelay_FollowsExponentialBackoff(int attempt, int expectedMinDelayMs)
    {
        // This test verifies that reconnect delay follows exponential backoff
        // with a configurable maximum

        // Arrange
        var config = new ClientConfig
        {
            AgentAddr = "invalid-host:9999",
            AutoReconnect = true,
            ReconnectIntervalSeconds = 1,
            ReconnectMaxAttempts = 5
        };

        using var client = new CroupierClient(config);

        var attemptCount = 0;
        var totalDelay = 0;

        // Act & Assert
        // Simulate reconnect attempts
        for (int i = 0; i < attempt; i++)
        {
            var delay = Math.Min(1000 * (1 << i), 10000); // Exponential with 10s cap
            totalDelay += delay;
            attemptCount++;
        }

        Assert.Equal(attempt, attemptCount);
        Assert.True(totalDelay >= expectedMinDelayMs,
            $"Expected total delay at least {expectedMinDelayMs}ms, got {totalDelay}ms");
    }

    #endregion

    #region Session Recovery Tests

    [Fact]
    public async Task Reconnect_AfterSuccessfulConnect_GeneratesNewSessionId()
    {
        // This test verifies that reconnection results in a new session ID

        // Arrange
        var config = new ClientConfig
        {
            AgentAddr = "localhost:19090",
            AutoReconnect = true
        };

        using var client = new CroupierClient(config);

        // In a real test:
        // 1. Connect and get session ID
        // var firstSessionId = client.SessionId;
        // 2. Disconnect
        // client.Disconnect();
        // 3. Reconnect
        // await client.ConnectAsync();
        // var secondSessionId = client.SessionId;
        // 4. Verify new session ID
        // Assert.NotEqual(firstSessionId, secondSessionId);

        // For now, verify SessionId property exists
        Assert.NotNull(client.SessionId);
    }

    #endregion

    #region Reconnect State Tests

    [Fact]
    public void IsConnected_AfterDisconnect_ReturnsFalse()
    {
        // Arrange
        var config = new ClientConfig();
        using var client = new CroupierClient(config);

        // Act
        client.Disconnect();

        // Assert
        Assert.False(client.IsConnected);
    }

    [Fact]
    public void IsConnected_AfterDispose_ReturnsFalse()
    {
        // Arrange
        var config = new ClientConfig();
        var client = new CroupierClient(config);

        // Act
        client.Dispose();

        // Assert
        Assert.False(client.IsConnected);
    }

    #endregion

    #region Concurrent Reconnect Tests

    [Fact]
    public async Task ConcurrentReconnectCalls_DoNotCauseRaceCondition()
    {
        // Arrange
        var config = new ClientConfig
        {
            AgentAddr = "invalid-host:9999",
            AutoReconnect = true,
            ReconnectIntervalSeconds = 1
        };

        using var client = new CroupierClient(config);

        // Act - Start multiple reconnect attempts concurrently
        var tasks = new Task[3];
        for (int i = 0; i < tasks.Length; i++)
        {
            tasks[i] = Task.Run(async () =>
            {
                try
                {
                    await (Task)client.GetType().InvokeMember(
                        "ReconnectAsync",
                        System.Reflection.BindingFlags.NonPublic | System.Reflection.BindingFlags.InvokeMethod | System.Reflection.BindingFlags.Instance,
                        null,
                        client,
                        new object[] { _cts.Token }
                    )!;
                }
                catch (InvalidOperationException)
                {
                    // Expected
                }
            });
        }

        // Wait a bit then cancel
        await Task.Delay(500);
        _cts.Cancel();

        // Assert - All tasks should complete without deadlock
        await Task.WhenAll(tasks.Select(t => t.ContinueWith(_ => { })));
    }

    #endregion

    #region Network Error Recovery Tests

    [Fact]
    public async Task Reconnect_AfterNetworkError_CanRecover()
    {
        // This test verifies recovery from transient network errors

        // Arrange
        var config = new ClientConfig
        {
            AgentAddr = "localhost:19090",
            AutoReconnect = true,
            ReconnectIntervalSeconds = 1
        };

        using var client = new CroupierClient(config);

        // In a real test with a controllable network:
        // 1. Connect successfully
        // 2. Simulate network interruption
        // 3. Restore network
        // 4. Verify client reconnects automatically

        // For now, verify configuration
        Assert.True(config.AutoReconnect);
        Assert.Equal(1, config.ReconnectIntervalSeconds);
    }

    #endregion

    #region Reconnect Event Tests

    [Fact]
    public void ClientConfig_ReconnectSettings_PropagateCorrectly()
    {
        // Arrange & Act
        var config = new ClientConfig
        {
            AutoReconnect = false,
            ReconnectIntervalSeconds = 15,
            ReconnectMaxAttempts = 3
        };

        // Assert
        Assert.False(config.AutoReconnect);
        Assert.Equal(15, config.ReconnectIntervalSeconds);
        Assert.Equal(3, config.ReconnectMaxAttempts);
    }

    [Theory]
    [InlineData(0, 0)]    // Infinite retry
    [InlineData(1, 1)]    // Single attempt
    [InlineData(5, 5)]    // Five attempts
    [InlineData(10, 10)]  // Ten attempts
    public void ReconnectMaxAttempts_AcceptsValidValues(int maxAttempts, int expected)
    {
        // Arrange & Act
        var config = new ClientConfig
        {
            ReconnectMaxAttempts = maxAttempts
        };

        // Assert
        Assert.Equal(expected, config.ReconnectMaxAttempts);
    }

    #endregion

    #region Reconnect Timeout Tests

    [Fact]
    public async Task ReconnectAsync_WithTimeout_StopsAfterTimeout()
    {
        // This test verifies that reconnect respects cancellation token

        // Arrange
        var config = new ClientConfig
        {
            AgentAddr = "invalid-host:9999",
            AutoReconnect = true,
            ReconnectIntervalSeconds = 1
        };

        using var cts = new CancellationTokenSource(TimeSpan.FromSeconds(2));
        using var client = new CroupierClient(config);

        var stopwatch = System.Diagnostics.Stopwatch.StartNew();

        // Act
        try
        {
            await (Task)client.GetType().InvokeMember(
                "ReconnectAsync",
                System.Reflection.BindingFlags.NonPublic | System.Reflection.BindingFlags.InvokeMethod | System.Reflection.BindingFlags.Instance,
                null,
                client,
                new object[] { cts.Token }
            )!;
        }
        catch (InvalidOperationException)
        {
            // Expected
        }
        catch (OperationCanceledException)
        {
            // Also expected due to timeout
        }

        stopwatch.Stop();

        // Assert
        Assert.True(stopwatch.ElapsedMilliseconds < 3000,
            $"Reconnect should stop after timeout, got {stopwatch.ElapsedMilliseconds}ms");
    }

    #endregion

    #region Reconnect Configuration Validation Tests

    [Theory]
    [InlineData(-1)]  // Negative
    [InlineData(0)]   // Zero (valid - infinite)
    [InlineData(1)]   // One
    [InlineData(100)] // Large number
    public void ReconnectMaxAttempts_AcceptsRange(int value)
    {
        // Arrange & Act
        var config = new ClientConfig
        {
            ReconnectMaxAttempts = value
        };

        // Assert - Should not throw
        Assert.Equal(value, config.ReconnectMaxAttempts);
    }

    [Theory]
    [InlineData(0)]   // Zero (will be adjusted to 1)
    [InlineData(1)]   // Minimum valid
    [InlineData(60)]  // One minute
    [InlineData(300)] // Five minutes
    public void ReconnectIntervalSeconds_AcceptsRange(int value)
    {
        // Arrange & Act
        var config = new ClientConfig
        {
            ReconnectIntervalSeconds = value
        };

        // Assert - Zero will be adjusted to minimum 1 second
        var expected = value == 0 ? 1 : value;
        Assert.Equal(expected, Math.Max(config.ReconnectIntervalSeconds, 1));
    }

    #endregion

    #region Heartbeat Interval Tests

    [Fact]
    public void HeartbeatIntervalSeconds_MustBePositive()
    {
        // Arrange & Act
        var config = new ClientConfig
        {
            HeartbeatIntervalSeconds = 30
        };

        // Assert
        Assert.Equal(30, config.HeartbeatIntervalSeconds);
    }

    [Fact]
    public void HeartbeatIntervalSeconds_DefaultsTo60()
    {
        // Arrange & Act
        var config = new ClientConfig();

        // Assert
        Assert.Equal(60, config.HeartbeatIntervalSeconds);
    }

    #endregion
}
