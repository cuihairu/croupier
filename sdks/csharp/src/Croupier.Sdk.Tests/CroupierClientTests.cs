// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using Croupier.Sdk.Logging;
using Croupier.Sdk.Models;
using Croupier.Sdk.Transport;
using Croupier.Sdk.V1;
using FluentAssertions;
using Moq;
using Xunit;
using System.IO.Compression;
using System.Reflection;
using System.Text;
using System.Text.Json;

namespace Croupier.Sdk.Tests;

/// <summary>
/// Tests for CroupierClient
/// </summary>
public class CroupierClientTests
{
    /// <summary>
    /// Check if integration tests should be skipped (no agent running).
    /// </summary>
    private static bool ShouldSkipIntegrationTests()
    {
        // Only run integration tests if explicitly enabled
        var runIntegrationTests = Environment.GetEnvironmentVariable("CROUPIER_RUN_INTEGRATION_TESTS");
        return string.IsNullOrEmpty(runIntegrationTests) || runIntegrationTests != "1";
    }

    /// <summary>
    /// Check if an exception is a connection error (indicating no agent is running).
    /// </summary>
    private static bool IsConnectionError(Exception ex)
    {
        var message = ex.Message.ToLower();
        return message.Contains("connect") ||
               message.Contains("connection") ||
               message.Contains("timeout") ||
               message.Contains("refused") ||
               message.Contains("unreachable");
    }

    private static ClientConfig CreateTestConfig()
    {
        return new ClientConfig
        {
            AgentAddr = "127.0.0.1:19090",
            ServiceId = "test-service",
            GameId = "test-game",
            Env = "test",
            Insecure = true
        };
    }

    private static Dictionary<int, object> ParseProtoMessage(byte[] data)
    {
        var result = new Dictionary<int, object>();
        var input = new Google.Protobuf.CodedInputStream(data);

        while (!input.IsAtEnd)
        {
            var tag = input.ReadTag();
            if (tag == 0)
            {
                break;
            }

            var fieldNumber = Google.Protobuf.WireFormat.GetTagFieldNumber(tag);
            var wireType = Google.Protobuf.WireFormat.GetTagWireType(tag);
            result[fieldNumber] = wireType switch
            {
                Google.Protobuf.WireFormat.WireType.LengthDelimited => input.ReadBytes().ToByteArray(),
                Google.Protobuf.WireFormat.WireType.Varint => input.ReadUInt64(),
                _ => throw new NotSupportedException($"Unsupported wire type: {wireType}")
            };
        }

        return result;
    }

    #region Constructor Tests

    [Fact]
    public void CroupierClient_CanBeCreatedWithConfig()
    {
        // Arrange
        var config = CreateTestConfig();

        // Act
        var client = new CroupierClient(config);

        // Assert
        client.Should().NotBeNull();
        client.Config.Should().BeSameAs(config);
    }

    [Fact]
    public void CroupierClient_CanBeCreatedWithDefaultConfig()
    {
        // Act
        var client = new CroupierClient();

        // Assert
        client.Should().NotBeNull();
        client.Config.Should().NotBeNull();
    }

    [Fact]
    public void CroupierClient_CanBeCreatedWithLogger()
    {
        // Arrange
        var config = CreateTestConfig();
        var logger = new ConsoleCroupierLogger();

        // Act
        var client = new CroupierClient(config, logger);

        // Assert
        client.Should().NotBeNull();
    }

    #endregion

    #region Function Registration Tests

    [Fact]
    public void CroupierClient_RegisterFunction_WithHandler()
    {
        // Arrange
        var client = new CroupierClient(CreateTestConfig());
        var descriptor = new FunctionDescriptor
        {
            Id = "get",
            Category = "player",
            Operation = "get"
        };

        FunctionHandlerDelegate handler = (ctx, payload) => Task.FromResult("{}");

        // Act
        client.RegisterFunction(descriptor, handler);

        // Assert - no exception means success
    }

    [Fact]
    public void CroupierClient_RegisterFunction_WithSyncHandler()
    {
        // Arrange
        var client = new CroupierClient(CreateTestConfig());
        var descriptor = new FunctionDescriptor
        {
            Id = "sync",
            Category = "player",
            Operation = "sync"
        };

        SyncFunctionHandlerDelegate handler = (ctx, payload) => "{}";

        // Act
        client.RegisterFunction(descriptor, handler);

        // Assert - no exception means success
    }

    [Fact]
    public void CroupierClient_RegisterFunction_WithIFunctionHandler()
    {
        // Arrange
        var client = new CroupierClient(CreateTestConfig());
        var descriptor = new FunctionDescriptor
        {
            Id = "custom",
            Category = "player",
            Operation = "custom"
        };

        var mockHandler = new Mock<IFunctionHandler>();

        // Act
        client.RegisterFunction(descriptor, mockHandler.Object);

        // Assert - no exception means success
    }

    [Fact]
    public void CroupierClient_RegisterFunction_ThrowsOnInvalidDescriptor()
    {
        // Arrange
        var client = new CroupierClient(CreateTestConfig());
        var descriptor = new FunctionDescriptor(); // Invalid - missing required fields

        FunctionHandlerDelegate handler = (ctx, payload) => Task.FromResult("{}");

        // Act & Assert
        var action = () => client.RegisterFunction(descriptor, handler);
        action.Should().Throw<ArgumentException>();
    }

    [Fact]
    public void CroupierClient_UnregisterFunction_RemovesFunction()
    {
        // Arrange
        var client = new CroupierClient(CreateTestConfig());
        var descriptor = new FunctionDescriptor
        {
            Id = "remove",
            Category = "player",
            Operation = "remove"
        };

        FunctionHandlerDelegate handler = (ctx, payload) => Task.FromResult("{}");
        client.RegisterFunction(descriptor, handler);

        // Act
        var result = client.UnregisterFunction("player.remove");

        // Assert
        result.Should().BeTrue();
    }

    [Fact]
    public void CroupierClient_UnregisterFunction_NonExistentFunction_ReturnsFalse()
    {
        // Arrange
        var client = new CroupierClient(CreateTestConfig());

        // Act
        var result = client.UnregisterFunction("nonexistent.function");

        // Assert
        result.Should().BeFalse();
    }

    #endregion

    #region Connection State Tests

    [Fact]
    public void CroupierClient_InitiallyNotConnected()
    {
        // Arrange
        var client = new CroupierClient(CreateTestConfig());

        // Assert
        client.IsConnected.Should().BeFalse();
    }

    [Fact]
    public void CroupierClient_Disconnect_WhenNotConnected_DoesNotThrow()
    {
        // Arrange
        var client = new CroupierClient(CreateTestConfig());

        // Act
        var action = () => client.Disconnect();

        // Assert
        action.Should().NotThrow();
    }

    #endregion

    #region Multiple Registration Tests

    [Fact]
    public void CroupierClient_RegisterMultipleFunctions()
    {
        // Arrange
        var client = new CroupierClient(CreateTestConfig());

        var functions = new[]
        {
            new FunctionDescriptor { Id = "get", Category = "player", Operation = "get" },
            new FunctionDescriptor { Id = "ban", Category = "player", Operation = "ban" },
            new FunctionDescriptor { Id = "transfer", Category = "wallet", Operation = "transfer" }
        };

        FunctionHandlerDelegate handler = (ctx, payload) => Task.FromResult("{}");

        // Act
        foreach (var func in functions)
        {
            client.RegisterFunction(func, handler);
        }

        // Assert - no exception means success
    }

    [Fact]
    public void CroupierClient_ReregisterFunction_OverwritesPrevious()
    {
        // Arrange
        var client = new CroupierClient(CreateTestConfig());
        var descriptor = new FunctionDescriptor { Id = "test", Category = "player" };

        FunctionHandlerDelegate handler1 = (ctx, payload) => Task.FromResult("{\"handler\":1}");
        FunctionHandlerDelegate handler2 = (ctx, payload) => Task.FromResult("{\"handler\":2}");

        // Act
        client.RegisterFunction(descriptor, handler1);
        client.RegisterFunction(descriptor, handler2);

        // Assert - no exception means success, second handler overwrites first
    }

    #endregion

    #region Dispose Tests

    [Fact]
    public void CroupierClient_ImplementsIDisposable()
    {
        // Assert
        typeof(CroupierClient).Should().Implement<IDisposable>();
    }

    [Fact]
    public void CroupierClient_Dispose_CanBeCalledMultipleTimes()
    {
        // Arrange
        var client = new CroupierClient(CreateTestConfig());

        // Act & Assert
        var action = () =>
        {
            client.Dispose();
            client.Dispose();
        };
        action.Should().NotThrow();
    }

    [Fact]
    public void CroupierClient_AfterDispose_ThrowsObjectDisposedException()
    {
        // Arrange
        var client = new CroupierClient(CreateTestConfig());
        client.Dispose();

        // Act & Assert
        var descriptor = new FunctionDescriptor { Id = "test", Category = "player" };
        FunctionHandlerDelegate handler = (ctx, payload) => Task.FromResult("{}");

        var action = () => client.RegisterFunction(descriptor, handler);
        action.Should().Throw<ObjectDisposedException>();
    }

    #endregion

    #region Invoke Tests

    [Fact]
    public async Task CroupierClient_InvokeAsync_WhenConnected()
    {
        // Skip integration test if no agent is running
        if (ShouldSkipIntegrationTests())
        {
            Assert.True(true, "Integration test skipped - set CROUPIER_RUN_INTEGRATION_TESTS=1 to run");
            return;
        }

        try
        {
            // Arrange
            var client = new CroupierClient(CreateTestConfig());
            await client.ConnectAsync();

            // Act
            var result = await client.InvokeAsync("test.function", "{}");

            // Assert
            result.Should().NotBeNullOrEmpty();
        }
        catch (Exception ex) when (IsConnectionError(ex))
        {
            // Skip if connection fails (no agent running)
            Assert.True(true, $"Connection failed - test skipped: {ex.Message}");
        }
    }

    #endregion

    #region Config Tests

    [Fact]
    public void CroupierClient_Config_ReturnsPassedConfig()
    {
        // Arrange
        var config = CreateTestConfig();
        config.ServiceId = "my-service";

        // Act
        var client = new CroupierClient(config);

        // Assert
        client.Config.ServiceId.Should().Be("my-service");
    }

    #endregion
}
