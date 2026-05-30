// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using Croupier.Sdk.Models;
using FluentAssertions;
using Moq;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// Tests for IFunctionHandler and related classes
/// </summary>
public class FunctionHandlerTests
{
    private static FunctionContext CreateTestContext(string functionId = "test.func")
    {
        return new FunctionContext
        {
            FunctionId = functionId,
            CallId = Guid.NewGuid().ToString(),
            GameId = "test-game",
            Env = "test"
        };
    }

    #region DelegateFunctionHandler Tests

    [Fact]
    public async Task DelegateFunctionHandler_InvokesDelegate()
    {
        // Arrange
        var handlerCalled = false;
        FunctionHandlerDelegate del = (ctx, payload) =>
        {
            handlerCalled = true;
            return Task.FromResult("{\"result\":\"ok\"}");
        };
        var handler = new DelegateFunctionHandler(del);
        var context = CreateTestContext();

        // Act
        var result = await handler.HandleAsync(context, "{}");

        // Assert
        handlerCalled.Should().BeTrue();
        result.Should().Be("{\"result\":\"ok\"}");
    }

    [Fact]
    public async Task DelegateFunctionHandler_PassesContextCorrectly()
    {
        // Arrange
        FunctionContext? capturedContext = null;
        FunctionHandlerDelegate del = (ctx, payload) =>
        {
            capturedContext = ctx;
            return Task.FromResult("ok");
        };
        var handler = new DelegateFunctionHandler(del);
        var context = new FunctionContext
        {
            FunctionId = "player.ban",
            CallId = "call-123",
            GameId = "test-game",
            Env = "production"
        };

        // Act
        await handler.HandleAsync(context, "{}");

        // Assert
        capturedContext.Should().NotBeNull();
        capturedContext!.FunctionId.Should().Be("player.ban");
        capturedContext.CallId.Should().Be("call-123");
        capturedContext.GameId.Should().Be("test-game");
        capturedContext.Env.Should().Be("production");
    }

    [Fact]
    public async Task DelegateFunctionHandler_PassesPayloadCorrectly()
    {
        // Arrange
        string? capturedPayload = null;
        FunctionHandlerDelegate del = (ctx, payload) =>
        {
            capturedPayload = payload;
            return Task.FromResult("ok");
        };
        var handler = new DelegateFunctionHandler(del);
        var expectedPayload = "{\"player_id\":\"123\",\"reason\":\"cheating\"}";

        // Act
        await handler.HandleAsync(CreateTestContext(), expectedPayload);

        // Assert
        capturedPayload.Should().Be(expectedPayload);
    }

    [Fact]
    public void DelegateFunctionHandler_ThrowsOnNullDelegate()
    {
        // Act & Assert
        var action = () => new DelegateFunctionHandler(null!);
        action.Should().Throw<ArgumentNullException>();
    }

    #endregion

    #region SyncDelegateFunctionHandler Tests

    [Fact]
    public async Task SyncDelegateFunctionHandler_InvokesSyncDelegate()
    {
        // Arrange
        var handlerCalled = false;
        SyncFunctionHandlerDelegate del = (ctx, payload) =>
        {
            handlerCalled = true;
            return "{\"sync\":true}";
        };
        var handler = new SyncDelegateFunctionHandler(del);
        var context = CreateTestContext("test.sync");

        // Act
        var result = await handler.HandleAsync(context, "{}");

        // Assert
        handlerCalled.Should().BeTrue();
        result.Should().Be("{\"sync\":true}");
    }

    [Fact]
    public void SyncDelegateFunctionHandler_ThrowsOnNullDelegate()
    {
        // Act & Assert
        var action = () => new SyncDelegateFunctionHandler(null!);
        action.Should().Throw<ArgumentNullException>();
    }

    [Fact]
    public async Task SyncDelegateFunctionHandler_PropagatesException()
    {
        // Arrange
        SyncFunctionHandlerDelegate del = (ctx, payload) => throw new InvalidOperationException("Sync handler error");
        var handler = new SyncDelegateFunctionHandler(del);

        // Act & Assert
        var action = async () => await handler.HandleAsync(CreateTestContext(), "{}");
        await action.Should().ThrowAsync<InvalidOperationException>()
            .WithMessage("Sync handler error");
    }

    #endregion

    #region FunctionHandlerBase Tests

    private class TestFunctionHandler : FunctionHandlerBase
    {
        public string? LastPayload { get; private set; }
        public FunctionContext? LastContext { get; private set; }
        public string ResponseToReturn { get; set; } = "{}";

        public override Task<string> HandleAsync(FunctionContext context, string payload)
        {
            LastContext = context;
            LastPayload = payload;
            return Task.FromResult(ResponseToReturn);
        }
    }

    [Fact]
    public async Task FunctionHandlerBase_CanBeSubclassed()
    {
        // Arrange
        var handler = new TestFunctionHandler { ResponseToReturn = "{\"custom\":true}" };
        var context = CreateTestContext("custom.handler");

        // Act
        var result = await handler.HandleAsync(context, "{\"input\":1}");

        // Assert
        result.Should().Be("{\"custom\":true}");
        handler.LastPayload.Should().Be("{\"input\":1}");
        handler.LastContext!.FunctionId.Should().Be("custom.handler");
    }

    [Fact]
    public void FunctionHandlerBase_ImplementsIFunctionHandler()
    {
        // Assert
        typeof(FunctionHandlerBase).Should().Implement<IFunctionHandler>();
    }

    #endregion

    #region IFunctionHandler Interface Tests

    [Fact]
    public async Task IFunctionHandler_MockedImplementation()
    {
        // Arrange
        var mockHandler = new Mock<IFunctionHandler>();
        mockHandler
            .Setup(h => h.HandleAsync(It.IsAny<FunctionContext>(), It.IsAny<string>()))
            .ReturnsAsync("{\"mocked\":true}");

        var context = CreateTestContext("mocked.func");

        // Act
        var result = await mockHandler.Object.HandleAsync(context, "{}");

        // Assert
        result.Should().Be("{\"mocked\":true}");
        mockHandler.Verify(h => h.HandleAsync(context, "{}"), Times.Once);
    }

    [Fact]
    public async Task IFunctionHandler_CanThrowException()
    {
        // Arrange
        var mockHandler = new Mock<IFunctionHandler>();
        mockHandler
            .Setup(h => h.HandleAsync(It.IsAny<FunctionContext>(), It.IsAny<string>()))
            .ThrowsAsync(new InvalidOperationException("Handler error"));

        var context = CreateTestContext("error.func");

        // Act & Assert
        var action = async () => await mockHandler.Object.HandleAsync(context, "{}");
        await action.Should().ThrowAsync<InvalidOperationException>()
            .WithMessage("Handler error");
    }

    #endregion

    #region FunctionContext Tests

    [Fact]
    public void FunctionContext_RequiredProperties_MustBeInitialized()
    {
        // Arrange & Act
        var context = new FunctionContext
        {
            FunctionId = "test.func",
            CallId = "call-123",
            GameId = "game-1",
            Env = "prod"
        };

        // Assert
        context.FunctionId.Should().Be("test.func");
        context.CallId.Should().Be("call-123");
        context.GameId.Should().Be("game-1");
        context.Env.Should().Be("prod");
    }

    [Fact]
    public void FunctionContext_OptionalProperties_CanBeSet()
    {
        // Arrange
        var context = new FunctionContext
        {
            FunctionId = "test.func",
            CallId = "call-123",
            GameId = "game-1",
            Env = "prod",
            UserId = "user-456",
            Timestamp = 1234567890,
            IdempotencyKey = "key-789",
            CallerServiceId = "caller-service"
        };

        // Assert
        context.UserId.Should().Be("user-456");
        context.Timestamp.Should().Be(1234567890);
        context.IdempotencyKey.Should().Be("key-789");
        context.CallerServiceId.Should().Be("caller-service");
    }

    #endregion

    #region Concurrent Handler Tests

    [Fact]
    public async Task DelegateFunctionHandler_HandlesConcurrentCalls()
    {
        // Arrange
        var callCount = 0;
        FunctionHandlerDelegate del = async (ctx, payload) =>
        {
            Interlocked.Increment(ref callCount);
            await Task.Delay(10); // Simulate some work
            return $"{{\"call\":{callCount}}}";
        };
        var handler = new DelegateFunctionHandler(del);

        // Act
        var tasks = Enumerable.Range(0, 10)
            .Select(_ => handler.HandleAsync(CreateTestContext(), "{}"))
            .ToArray();

        await Task.WhenAll(tasks);

        // Assert
        callCount.Should().Be(10);
    }

    #endregion
}
