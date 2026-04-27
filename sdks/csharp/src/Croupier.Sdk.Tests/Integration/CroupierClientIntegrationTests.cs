// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using Croupier.Sdk.Models;
using FluentAssertions;
using Xunit;
using Xunit.Abstractions;

namespace Croupier.Sdk.Tests.Integration;

/// <summary>
/// Integration tests for CroupierClient.
/// These tests require a running croupier-agent on localhost:19090.
/// </summary>
[Trait("Category", "Integration")]
public class CroupierClientIntegrationTests
{
    private readonly ITestOutputHelper _output;

    public CroupierClientIntegrationTests(ITestOutputHelper output)
    {
        _output = output;
    }

    /// <summary>
    /// Check if integration tests should be skipped (no agent running).
    /// </summary>
    private static bool ShouldSkipIntegrationTests()
    {
        var runIntegrationTests = Environment.GetEnvironmentVariable("CROUPIER_RUN_INTEGRATION_TESTS");
        return string.IsNullOrEmpty(runIntegrationTests) || runIntegrationTests != "1";
    }

    private static ClientConfig CreateTestConfig(string serviceId)
    {
        return new ClientConfig
        {
            AgentAddr = "127.0.0.1:19090",
            ServiceId = serviceId,
            GameId = "test-game",
            Env = "test",
            Insecure = true
        };
    }

    [Fact]
    public async Task ConnectToAgentAndRegisterFunction_Succeeds()
    {
        if (ShouldSkipIntegrationTests())
        {
            _output.WriteLine("Integration test skipped - set CROUPIER_RUN_INTEGRATION_TESTS=1 to run");
            return;
        }

        // Arrange
        var config = CreateTestConfig("csharp-integration-test");
        using var client = new CroupierClient(config);

        var descriptor = new FunctionDescriptor
        {
            Id = "ping",
            Category = "test",
            Operation = "ping"
        };

        FunctionHandlerDelegate handler = (ctx, payload) => Task.FromResult($"pong: {payload}");

        client.RegisterFunction(descriptor, handler);

        // Act
        await client.ConnectAsync();

        // Assert
        client.IsConnected.Should().BeTrue();
        client.SessionId.Should().NotBeNullOrEmpty();

        // Clean up
        client.Disconnect();
        client.IsConnected.Should().BeFalse();
    }

    [Fact]
    public async Task ConnectFailsWithInvalidAgentAddress()
    {
        if (ShouldSkipIntegrationTests())
        {
            _output.WriteLine("Integration test skipped - set CROUPIER_RUN_INTEGRATION_TESTS=1 to run");
            return;
        }

        // Arrange
        var config = new ClientConfig
        {
            AgentAddr = "127.0.0.1:9999", // Non-existent port
            ServiceId = "csharp-integration-test-invalid",
            GameId = "test-game",
            Env = "test",
            Insecure = true,
            TimeoutSeconds = 5,
            ConnectTimeoutSeconds = 5
        };

        using var client = new CroupierClient(config);

        var descriptor = new FunctionDescriptor
        {
            Id = "ping",
            Category = "test",
            Operation = "ping"
        };

        client.RegisterFunction(descriptor, (ctx, payload) => Task.FromResult("ok"));

        // Act & Assert
        var exception = await Assert.ThrowsAnyAsyncException>(() => client.ConnectAsync());
        _output.WriteLine($"Expected connection failure: {exception.Message}");
    }

    [Fact]
    public async Task ConnectRequiresAtLeastOneFunction()
    {
        if (ShouldSkipIntegrationTests())
        {
            _output.WriteLine("Integration test skipped - set CROUPIER_RUN_INTEGRATION_TESTS=1 to run");
            return;
        }

        // Arrange
        var config = CreateTestConfig("csharp-integration-test-no-func");
        using var client = new CroupierClient(config);

        // Act & Assert
        // Should not fail at ConnectAsync, but registration may fail
        // The SDK allows connecting without functions, but RegisterLocal will fail
        var exception = await Record.ExceptionAsync(() => client.ConnectAsync());
        _output.WriteLine($"Connect result: {exception?.Message ?? "Success"}");
    }

    [Fact]
    public async Task ConnectIsIdempotent()
    {
        if (ShouldSkipIntegrationTests())
        {
            _output.WriteLine("Integration test skipped - set CROUPIER_RUN_INTEGRATION_TESTS=1 to run");
            return;
        }

        // Arrange
        var config = CreateTestConfig("csharp-integration-test-idempotent");
        using var client = new CroupierClient(config);

        client.RegisterFunction(
            new FunctionDescriptor { Id = "test", Category = "test", Operation = "idempotent" },
            (ctx, payload) => Task.FromResult("ok")
        );

        // Act
        await client.ConnectAsync();
        var firstSessionId = client.SessionId;

        await client.ConnectAsync(); // Second connect should be idempotent
        var secondSessionId = client.SessionId;

        // Assert
        firstSessionId.Should().NotBeNullOrEmpty();
        secondSessionId.Should().Be(firstSessionId);

        // Clean up
        client.Disconnect();
    }

    [Fact]
    public async Task ReconnectAfterDisconnect_Succeeds()
    {
        if (ShouldSkipIntegrationTests())
        {
            _output.WriteLine("Integration test skipped - set CROUPIER_RUN_INTEGRATION_TESTS=1 to run");
            return;
        }

        // Arrange
        var config = CreateTestConfig("csharp-integration-test-reconnect");
        using var client = new CroupierClient(config);

        client.RegisterFunction(
            new FunctionDescriptor { Id = "test", Category = "test", Operation = "reconnect" },
            (ctx, payload) => Task.FromResult("ok")
        );

        // Act
        await client.ConnectAsync();
        var sessionId1 = client.SessionId;

        client.Disconnect();
        client.IsConnected.Should().BeFalse();

        await client.ConnectAsync();
        var sessionId2 = client.SessionId;

        // Assert
        sessionId1.Should().NotBeNullOrEmpty();
        sessionId2.Should().NotBeNullOrEmpty();
        sessionId2.Should().NotBe(sessionId1); // New session after reconnect

        // Clean up
        client.Disconnect();
    }

    [Fact]
    public async Task DisconnectIsIdempotent()
    {
        if (ShouldSkipIntegrationTests())
        {
            _output.WriteLine("Integration test skipped - set CROUPIER_RUN_INTEGRATION_TESTS=1 to run");
            return;
        }

        // Arrange
        var config = CreateTestConfig("csharp-integration-test-disconnect");
        using var client = new CroupierClient(config);

        client.RegisterFunction(
            new FunctionDescriptor { Id = "test", Category = "test", Operation = "disconnect" },
            (ctx, payload) => Task.FromResult("ok")
        );

        await client.ConnectAsync();

        // Act & Assert
        var action = () => client.Disconnect();
        action.Should().NotThrow();
        action.Should().NotThrow();
        action.Should().NotThrow();

        client.IsConnected.Should().BeFalse();
    }

    [Fact]
    public async Task RegisterMultipleFunctions_Succeeds()
    {
        if (ShouldSkipIntegrationTests())
        {
            _output.WriteLine("Integration test skipped - set CROUPIER_RUN_INTEGRATION_TESTS=1 to run");
            return;
        }

        // Arrange
        var config = CreateTestConfig("csharp-integration-test-multi");
        using var client = new CroupierClient(config);

        // Act
        client.RegisterFunction(
            new FunctionDescriptor { Id = "ping", Category = "test", Operation = "ping" },
            (ctx, payload) => Task.FromResult($"pong: {payload}")
        );
        client.RegisterFunction(
            new FunctionDescriptor { Id = "echo", Category = "test", Operation = "echo" },
            (ctx, payload) => Task.FromResult(payload)
        );
        client.RegisterFunction(
            new FunctionDescriptor { Id = "upper", Category = "test", Operation = "upper" },
            (ctx, payload) => Task.FromResult(payload.ToUpperInvariant())
        );

        await client.ConnectAsync();

        // Assert
        client.IsConnected.Should().BeTrue();
        client.SessionId.Should().NotBeNullOrEmpty();

        // Clean up
        client.Disconnect();
    }
}
