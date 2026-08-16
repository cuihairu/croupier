// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using Croupier.Sdk.Models;
using FluentAssertions;
using Xunit;
using Xunit.Abstractions;

namespace Croupier.Sdk.Tests.Integration;

/// <summary>
/// Integration tests for CroupierClient.
/// These tests require a running croupier-agent local SDK gateway.
/// </summary>
[Trait("Category", "Integration")]
public class CroupierClientIntegrationTests
{
    private readonly ITestOutputHelper _output;

    public CroupierClientIntegrationTests(ITestOutputHelper output)
    {
        _output = output;
    }

    private static ClientConfig CreateTestConfig(string serviceId)
    {
        return new ClientConfig
        {
            AgentAddr = Environment.GetEnvironmentVariable("CROUPIER_AGENT_ADDR") ?? "127.0.0.1:19091",
            ServiceId = serviceId,
            GameId = "test-game",
            Env = "test",
            Insecure = true,
            TimeoutSeconds = 30,
            ConnectTimeoutSeconds = 10  // Increased for CI environment
        };
    }

    [IntegrationFact]
    public async Task ConnectToAgentAndRegisterFunction_Succeeds()
    {
        // Arrange
        var config = CreateTestConfig("csharp-integration-test");
        using var client = new CroupierClient(config);

        var descriptor = new FunctionDescriptor
        {
            Id = "test.ping",
            Resource = "test",
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

    [IntegrationFact]
    public async Task ConnectFailsWithInvalidAgentAddress()
    {
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
            Id = "test.ping",
            Resource = "test",
            Operation = "ping"
        };

        client.RegisterFunction(descriptor, (ctx, payload) => Task.FromResult("ok"));

        // Act & Assert
        var exception = await Assert.ThrowsAnyAsync<Exception>(() => client.ConnectAsync());
        _output.WriteLine($"Expected connection failure: {exception.Message}");
    }

    [IntegrationFact]
    public async Task ConnectRequiresAtLeastOneFunction()
    {
        // Arrange
        var config = CreateTestConfig("csharp-integration-test-no-func");
        using var client = new CroupierClient(config);

        // Act & Assert
        var exception = await Assert.ThrowsAsync<InvalidOperationException>(() => client.ConnectAsync());
        exception.Message.Should().Contain("Register at least one function");
    }

    [IntegrationFact]
    public async Task ConnectIsIdempotent()
    {
        // Arrange
        var config = CreateTestConfig("csharp-integration-test-idempotent");
        using var client = new CroupierClient(config);

        client.RegisterFunction(
            new FunctionDescriptor { Id = "test.idempotent", Resource = "test", Operation = "idempotent" },
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

    [IntegrationFact]
    public async Task ReconnectAfterDisconnect_Succeeds()
    {
        // Arrange
        var config = CreateTestConfig("csharp-integration-test-reconnect");
        using var client = new CroupierClient(config);

        client.RegisterFunction(
            new FunctionDescriptor { Id = "test.reconnect", Resource = "test", Operation = "reconnect" },
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

    [IntegrationFact]
    public async Task DisconnectIsIdempotent()
    {
        // Arrange
        var config = CreateTestConfig("csharp-integration-test-disconnect");
        using var client = new CroupierClient(config);

        client.RegisterFunction(
            new FunctionDescriptor { Id = "test.disconnect", Resource = "test", Operation = "disconnect" },
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

    [IntegrationFact]
    public async Task RegisterMultipleFunctions_Succeeds()
    {
        // Arrange
        var config = CreateTestConfig("csharp-integration-test-multi");
        using var client = new CroupierClient(config);

        // Act
        client.RegisterFunction(
            new FunctionDescriptor { Id = "test.ping", Resource = "test", Operation = "ping" },
            (ctx, payload) => Task.FromResult($"pong: {payload}")
        );
        client.RegisterFunction(
            new FunctionDescriptor { Id = "test.echo", Resource = "test", Operation = "echo" },
            (ctx, payload) => Task.FromResult(payload)
        );
        client.RegisterFunction(
            new FunctionDescriptor { Id = "test.upper", Resource = "test", Operation = "upper" },
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
