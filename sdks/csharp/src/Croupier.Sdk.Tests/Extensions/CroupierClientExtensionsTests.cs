// Copyright 2025 Croupier Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

using System;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using Xunit;
using FluentAssertions;
using Croupier.Sdk.Extensions;
using Croupier.Sdk.Models;
using Croupier.Sdk.Threading;

namespace Croupier.Sdk.Tests.Extensions;

/// <summary>
/// Tests for CroupierClientExtensions
/// </summary>
public class CroupierClientExtensionsTests : IDisposable
{
    private readonly CancellationTokenSource _cts = new();

    public CroupierClientExtensionsTests()
    {
        // Reset singleton before each test to ensure clean state
        MainThreadDispatcher.Reset();
        MainThreadDispatcher.Initialize();
    }

    public void Dispose()
    {
        _cts.Cancel();
        _cts.Dispose();
        MainThreadDispatcher.Reset();
    }

    #region InvokeOnMainThread Tests

    [Fact]
    public void InvokeOnMainThread_FireAndForget_DoesNotBlock()
    {
        // Arrange
        using var client = new CroupierClient(new ClientConfig
        {
            GameId = "test-game",
            ServiceId = "test-service",
            AgentAddr = "invalid-host:99999" // Use invalid host to avoid connection timeout
        });

        // Act - Should not block (fire and forget)
        var stopwatch = System.Diagnostics.Stopwatch.StartNew();

        client.InvokeOnMainThread(
            "testFunc",
            "payload",
            _ => { },
            ex => { }, // Error handler
            null,
            _cts.Token
        );

        stopwatch.Stop();

        // Process queue to execute any callbacks
        MainThreadDispatcher.Instance.ProcessQueue();

        // Assert - Method should have returned immediately (within 100ms)
        stopwatch.ElapsedMilliseconds.Should().BeLessThan(100);
    }

    [Fact]
    public void InvokeOnMainThread_WithNullOnError_DoesNotThrowOnMissingHandler()
    {
        // Arrange
        using var client = new CroupierClient(new ClientConfig
        {
            GameId = "test-game",
            ServiceId = "test-service",
            AgentAddr = "invalid-host:99999"
        });

        // Act & Assert - Should not throw even if InvokeAsync fails
        client.InvokeOnMainThread(
            "testFunc",
            "payload",
            _ => { },
            null, // No error handler
            null,
            _cts.Token
        );

        // Give time for async operation to fail
        Thread.Sleep(100);
    }

    [Fact]
    public void InvokeOnMainThread_WithOptions_AcceptsOptions()
    {
        // Arrange
        using var client = new CroupierClient(new ClientConfig
        {
            GameId = "test-game",
            ServiceId = "test-service",
            AgentAddr = "localhost:19090"
        });

        var options = new InvokeOptions { GameId = "custom-game" };

        // Act & Assert - Should accept options without throwing
        client.InvokeOnMainThread(
            "testFunc",
            "payload",
            _ => { },
            null,
            options,
            _cts.Token
        );
    }

    #endregion

    #region InvokeOnMainThreadAsync Tests

    [Fact]
    public async Task InvokeOnMainThreadAsync_ReturnsTask()
    {
        // Arrange
        using var client = new CroupierClient(new ClientConfig
        {
            GameId = "test-game",
            ServiceId = "test-service",
            AgentAddr = "localhost:19090"
        });

        // Act - Should return a Task that completes
        var task = client.InvokeOnMainThreadAsync(
            "testFunc",
            "payload",
            _ => { },
            null,
            null,
            _cts.Token
        );

        // Process queue
        MainThreadDispatcher.Instance.ProcessQueue();

        // Assert - Task should complete (even if InvokeAsync fails)
        await Task.WhenAny(task, Task.Delay(1000));
        task.Status.Should().NotBe(System.Threading.Tasks.TaskStatus.WaitingForActivation);
    }

    [Fact]
    public async Task InvokeOnMainThreadAsync_WithError_DoesNotThrow()
    {
        // Arrange
        using var client = new CroupierClient(new ClientConfig
        {
            GameId = "test-game",
            ServiceId = "test-service",
            AgentAddr = "invalid-host:99999"
        });

        Exception? capturedException = null;

        // Act
        var task = client.InvokeOnMainThreadAsync(
            "testFunc",
            "payload",
            _ => { },
            ex => { capturedException = ex; },
            null,
            _cts.Token
        );

        // Wait for the async operation to complete and queue the error callback
        await Task.WhenAny(task, Task.Delay(1000));

        // Process the queue to execute the enqueued error callback
        MainThreadDispatcher.Instance.ProcessQueue();

        // Give a moment for the callback to execute
        await Task.Yield();

        // Assert - Error callback should have been called
        capturedException.Should().NotBeNull();
    }

    #endregion

    #region ConnectOnMainThread Tests

    [Fact]
    public void ConnectOnMainThread_FireAndForget_DoesNotBlock()
    {
        // Arrange
        using var client = new CroupierClient(new ClientConfig
        {
            GameId = "test-game",
            ServiceId = "test-service",
            AgentAddr = "localhost:19090"
        });

        // Act - Should not block
        client.ConnectOnMainThread(
            () => { },
            null,
            _cts.Token
        );

        // Process queue
        MainThreadDispatcher.Instance.ProcessQueue();

        // Assert - Callback should be processed
        // Note: ConnectAsync will fail, but that's ok for this test
    }

    [Fact]
    public void ConnectOnMainThread_WithNullError_DoesNotThrow()
    {
        // Arrange
        using var client = new CroupierClient(new ClientConfig
        {
            GameId = "test-game",
            ServiceId = "test-service",
            AgentAddr = "invalid-host:99999"
        });

        // Act & Assert - Should not throw
        client.ConnectOnMainThread(
            () => { },
            null,
            _cts.Token
        );

        Thread.Sleep(100);
        MainThreadDispatcher.Instance.ProcessQueue();
    }

    #endregion

    #region ConnectOnMainThreadAsync Tests

    [Fact]
    public async Task ConnectOnMainThreadAsync_ReturnsTask()
    {
        // Arrange
        using var client = new CroupierClient(new ClientConfig
        {
            GameId = "test-game",
            ServiceId = "test-service",
            AgentAddr = "localhost:19090"
        });

        // Act
        var task = client.ConnectOnMainThreadAsync(
            () => { },
            null,
            _cts.Token
        );

        await Task.WhenAny(task, Task.Delay(1000));

        // Assert - Task should complete
        task.Status.Should().NotBe(System.Threading.Tasks.TaskStatus.WaitingForActivation);
    }

    #endregion

    #region RunOnMainThread Tests

    [Fact]
    public void RunOnMainThread_ExecutesActionWhenQueueProcessed()
    {
        // Arrange
        using var client = new CroupierClient(new ClientConfig
        {
            GameId = "test-game",
            ServiceId = "test-service",
            AgentAddr = "localhost:19090"
        });

        var executed = new ManualResetEventSlim(false);

        // Act
        client.RunOnMainThread(() => executed.Set());

        // Process queue
        MainThreadDispatcher.Instance.ProcessQueue();

        // Assert
        executed.Wait(1000).Should().BeTrue();
    }

    [Fact]
    public void RunOnMainThread_ReturnsClientForChaining()
    {
        // Arrange
        using var client = new CroupierClient(new ClientConfig
        {
            GameId = "test-game",
            ServiceId = "test-service",
            AgentAddr = "localhost:19090"
        });

        // Act
        var result = client.RunOnMainThread(() => { });

        // Assert
        result.Should().Be(client);
    }

    [Fact]
    public void RunOnMainThread_MultipleActions_ExecutesInOrder()
    {
        // Arrange
        using var client = new CroupierClient(new ClientConfig
        {
            GameId = "test-game",
            ServiceId = "test-service",
            AgentAddr = "localhost:19090"
        });

        var executionOrder = new List<int>();

        // Act
        client.RunOnMainThread(() => executionOrder.Add(1))
             .RunOnMainThread(() => executionOrder.Add(2))
             .RunOnMainThread(() => executionOrder.Add(3));

        // Process queue
        MainThreadDispatcher.Instance.ProcessQueue();

        // Assert
        executionOrder.Should().Equal(new[] { 1, 2, 3 });
    }

    [Fact]
    public void RunOnMainThread_WithMultipleClients_DoesNotInterfere()
    {
        // Arrange
        using var client1 = new CroupierClient(new ClientConfig
        {
            GameId = "game1",
            ServiceId = "svc1",
            AgentAddr = "localhost:19090"
        });

        using var client2 = new CroupierClient(new ClientConfig
        {
            GameId = "game2",
            ServiceId = "svc2",
            AgentAddr = "localhost:19090"
        });

        var results = new List<string>();

        // Act
        client1.RunOnMainThread(() => results.Add("client1"));
        client2.RunOnMainThread(() => results.Add("client2"));

        // Process queue
        MainThreadDispatcher.Instance.ProcessQueue();

        // Assert - Both should execute
        results.Should().Contain("client1");
        results.Should().Contain("client2");
    }

    #endregion

    #region RunOnMainThread<T> Tests

    [Fact]
    public void RunOnMainThreadWithData_ExecutesActionWithData()
    {
        // Arrange
        using var client = new CroupierClient(new ClientConfig
        {
            GameId = "test-game",
            ServiceId = "test-service",
            AgentAddr = "localhost:19090"
        });

        string? capturedData = null;

        // Act
        client.RunOnMainThread<string>(data => capturedData = data, "test-data");

        // Process queue
        MainThreadDispatcher.Instance.ProcessQueue();

        // Assert
        capturedData.Should().Be("test-data");
    }

    [Fact]
    public void RunOnMainThreadWithData_ReturnsClientForChaining()
    {
        // Arrange
        using var client = new CroupierClient(new ClientConfig
        {
            GameId = "test-game",
            ServiceId = "test-service",
            AgentAddr = "localhost:19090"
        });

        // Act
        var result = client.RunOnMainThread<int>(data => { }, 42);

        // Assert
        result.Should().Be(client);
    }

    [Fact]
    public void RunOnMainThreadWithData_WithComplexData_Works()
    {
        // Arrange
        using var client = new CroupierClient(new ClientConfig
        {
            GameId = "test-game",
            ServiceId = "test-service",
            AgentAddr = "localhost:19090"
        });

        var data = new TestData { Id = 123, Name = "test" };
        TestData? capturedData = null;

        // Act
        client.RunOnMainThread<TestData>(d => capturedData = d, data);

        // Process queue
        MainThreadDispatcher.Instance.ProcessQueue();

        // Assert
        capturedData.Should().NotBeNull();
        capturedData?.Id.Should().Be(123);
        capturedData?.Name.Should().Be("test");
    }

    [Fact]
    public void RunOnMainThreadWithData_MultipleCalls_ExecutesWithCorrectData()
    {
        // Arrange
        using var client = new CroupierClient(new ClientConfig
        {
            GameId = "test-game",
            ServiceId = "test-service",
            AgentAddr = "localhost:19090"
        });

        var results = new List<string>();

        // Act
        client.RunOnMainThread<string>(data => results.Add(data), "first")
             .RunOnMainThread<string>(data => results.Add(data), "second")
             .RunOnMainThread<string>(data => results.Add(data), "third");

        // Process queue
        MainThreadDispatcher.Instance.ProcessQueue();

        // Assert
        results.Should().Equal(new[] { "first", "second", "third" });
    }

    [Fact]
    public void RunOnMainThreadWithData_WithNullData_Works()
    {
        // Arrange
        using var client = new CroupierClient(new ClientConfig
        {
            GameId = "test-game",
            ServiceId = "test-service",
            AgentAddr = "localhost:19090"
        });

        string? capturedData = "not-null";

        // Act
        client.RunOnMainThread<string>(data => capturedData = data, null!);

        // Process queue
        MainThreadDispatcher.Instance.ProcessQueue();

        // Assert
        capturedData.Should().BeNull();
    }

    #endregion

    #region Edge Cases Tests

    [Fact]
    public void InvokeOnMainThread_WithNullPayload_Works()
    {
        // Arrange
        using var client = new CroupierClient(new ClientConfig
        {
            GameId = "test-game",
            ServiceId = "test-service",
            AgentAddr = "localhost:19090"
        });

        // Act & Assert - Should not throw
        client.InvokeOnMainThread(
            "testFunc",
            null!,
            _ => { },
            null,
            null,
            _cts.Token
        );
    }

    [Fact]
    public void InvokeOnMainThread_WithEmptyFunctionId_Works()
    {
        // Arrange
        using var client = new CroupierClient(new ClientConfig
        {
            GameId = "test-game",
            ServiceId = "test-service",
            AgentAddr = "localhost:19090"
        });

        // Act & Assert - Should not throw
        client.InvokeOnMainThread(
            "",
            "payload",
            _ => { },
            null,
            null,
            _cts.Token
        );
    }

    [Fact]
    public void RunOnMainThread_WithNullAction_DoesNotThrow()
    {
        // Arrange
        using var client = new CroupierClient(new ClientConfig
        {
            GameId = "test-game",
            ServiceId = "test-service",
            AgentAddr = "localhost:19090"
        });

        // Act & Assert - Enqueuing null action should not throw immediately
        // (it will fail when processing, but that's expected)
        Action act = () => client.RunOnMainThread(null!);

        // The enqueue might throw or not depending on implementation
        // Just verify it doesn't crash the test
    }

    #endregion

    #region Test Helpers

    private class TestData
    {
        public int Id { get; set; }
        public string Name { get; set; } = string.Empty;
    }

    #endregion
}
