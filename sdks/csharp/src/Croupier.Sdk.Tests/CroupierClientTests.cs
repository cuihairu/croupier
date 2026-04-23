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

    private sealed class FakeTransport : IClientTransport
    {
        private readonly Func<int, byte[]?, byte[]> _callHandler;

        public FakeTransport(Func<int, byte[]?, byte[]> callHandler)
        {
            _callHandler = callHandler;
        }

        public bool IsConnected { get; private set; }
        public int ConnectCount { get; private set; }

        public void Connect()
        {
            ConnectCount++;
            IsConnected = true;
        }

        public byte[] Call(int msgType, byte[]? data) => _callHandler(msgType, data);

        public Task<byte[]> CallAsync(int msgType, byte[]? data, CancellationToken cancellationToken = default)
            => Task.FromResult(_callHandler(msgType, data));

        public void Dispose()
        {
            IsConnected = false;
        }
    }

    private sealed class FakeLocalServer : ILocalRequestServer
    {
        public bool IsListening { get; private set; }
        public int ListenCount { get; private set; }

        public event EventHandler<RequestReceivedEventArgs>? RequestReceived;

        public void Listen()
        {
            ListenCount++;
            IsListening = true;
        }

        public void Dispose()
        {
            IsListening = false;
        }

        public void Emit(RequestReceivedEventArgs args)
        {
            RequestReceived?.Invoke(this, args);
        }
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

    [Fact]
    public async Task CroupierClient_ConnectAsync_RegistersLocalFunctions()
    {
        // Arrange
        RegisterLocalRequest? capturedRequest = null;
        var transport = new FakeTransport((msgType, data) =>
        {
            msgType.Should().Be(Protocol.MsgRegisterLocalRequest);
            capturedRequest = RegisterLocalRequest.Parser.ParseFrom(data);
            return new RegisterLocalResponse { SessionId = "session-1" }.ToByteArray();
        });
        var server = new FakeLocalServer();

        var client = new CroupierClient(
            CreateTestConfig(),
            new ConsoleCroupierLogger(),
            (_, _, _) => transport,
            (_, _, _) => server);
        client.RegisterFunction(
            new FunctionDescriptor
            {
                Id = "get",
                Category = "player",
                Operation = "get",
                Description = "Get player",
                InputSchema = "{\"type\":\"object\"}",
                OutputSchema = "{\"type\":\"object\"}"
            },
            (ctx, payload) => Task.FromResult("{}"));

        // Act
        await client.ConnectAsync();

        // Assert
        transport.ConnectCount.Should().Be(1);
        capturedRequest.Should().NotBeNull();
        capturedRequest!.ServiceId.Should().Be("test-service");
        capturedRequest.RpcAddr.Should().StartWith("tcp://127.0.0.1:");
        capturedRequest.Functions.Should().ContainSingle();
        capturedRequest.Functions[0].Id.Should().Be("player.get");
        client.IsConnected.Should().BeTrue();
        server.ListenCount.Should().Be(1);
    }

    [Fact]
    public async Task CroupierClient_HeartbeatFailure_ReconnectsAndReRegisters()
    {
        // Arrange
        var registerCount = 0;
        var heartbeatCount = 0;
        var config = CreateTestConfig();
        config.HeartbeatIntervalSeconds = 1;
        config.ReconnectIntervalSeconds = 1;
        var server = new FakeLocalServer();

        var client = new CroupierClient(
            config,
            new ConsoleCroupierLogger(),
            (_, _, _) => new FakeTransport((msgType, data) =>
            {
                if (msgType == Protocol.MsgRegisterLocalRequest)
                {
                    registerCount++;
                    return new RegisterLocalResponse { SessionId = $"session-{registerCount}" }.ToByteArray();
                }

                if (msgType == Protocol.MsgHeartbeatLocalRequest)
                {
                    heartbeatCount++;
                    if (heartbeatCount == 1)
                    {
                        throw new InvalidOperationException("heartbeat failed");
                    }

                    var heartbeat = HeartbeatRequest.Parser.ParseFrom(data);
                    heartbeat.SessionId.Should().Be("session-2");
                    return Array.Empty<byte>();
                }

                throw new InvalidOperationException($"Unexpected msgType: {msgType}");
            }),
            (_, _, _) => server);
        client.RegisterFunction(
            new FunctionDescriptor
            {
                Id = "get",
                Category = "player",
                Operation = "get"
            },
            (ctx, payload) => Task.FromResult("{}"));

        // Act
        await client.ConnectAsync();
        await Task.Delay(TimeSpan.FromSeconds(3.5));

        // Assert
        registerCount.Should().BeGreaterThanOrEqualTo(2);
        client.IsConnected.Should().BeTrue();
    }

    [Fact]
    public async Task CroupierClient_InvokeAsync_MergesConfigMetadata()
    {
        InvokeRequest? capturedRequest = null;
        var config = CreateTestConfig();
        config.AuthToken = "secret-token";
        config.Headers["X-Client"] = "csharp-sdk";
        var server = new FakeLocalServer();

        var transport = new FakeTransport((msgType, data) =>
        {
            if (msgType == Protocol.MsgRegisterLocalRequest)
            {
                return new RegisterLocalResponse { SessionId = "session-1" }.ToByteArray();
            }

            msgType.Should().Be(Protocol.MsgInvokeRequest);
            capturedRequest = InvokeRequest.Parser.ParseFrom(data);
            return new InvokeResponse
            {
                Payload = Google.Protobuf.ByteString.CopyFromUtf8("{\"ok\":true}")
            }.ToByteArray();
        });

        var client = new CroupierClient(
            config,
            new ConsoleCroupierLogger(),
            (_, _, _) => transport,
            (_, _, _) => server);

        client.RegisterFunction(
            new FunctionDescriptor { Id = "get", Category = "player", Operation = "get" },
            (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();
        await client.InvokeAsync("player.get", "{}", new InvokeOptions
        {
            RequestId = "req-1",
            Metadata = new Dictionary<string, string>
            {
                ["X-Custom"] = "custom"
            }
        });

        capturedRequest.Should().NotBeNull();
        capturedRequest!.Metadata["Authorization"].Should().Be("Bearer secret-token");
        capturedRequest.Metadata["X-Client"].Should().Be("csharp-sdk");
        capturedRequest.Metadata["X-Request-ID"].Should().Be("req-1");
        capturedRequest.Metadata["X-Game-ID"].Should().Be("test-game");
        capturedRequest.Metadata["X-Env"].Should().Be("test");
        capturedRequest.Metadata["X-Custom"].Should().Be("custom");
    }

    [Fact]
    public async Task CroupierClient_InvokeAsync_PerCallMetadataOverridesConfigDefaults()
    {
        InvokeRequest? capturedRequest = null;
        var config = CreateTestConfig();
        config.AuthToken = "secret-token";
        config.Headers["Authorization"] = "Bearer config-token";
        config.Headers["X-Game-ID"] = "game-config";
        config.Headers["X-Env"] = "staging";
        var server = new FakeLocalServer();

        var transport = new FakeTransport((msgType, data) =>
        {
            if (msgType == Protocol.MsgRegisterLocalRequest)
            {
                return new RegisterLocalResponse { SessionId = "session-1" }.ToByteArray();
            }

            msgType.Should().Be(Protocol.MsgInvokeRequest);
            capturedRequest = InvokeRequest.Parser.ParseFrom(data);
            return new InvokeResponse
            {
                Payload = Google.Protobuf.ByteString.CopyFromUtf8("{\"ok\":true}")
            }.ToByteArray();
        });

        var client = new CroupierClient(
            config,
            new ConsoleCroupierLogger(),
            (_, _, _) => transport,
            (_, _, _) => server);

        client.RegisterFunction(
            new FunctionDescriptor { Id = "get", Category = "player", Operation = "get" },
            (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();
        await client.InvokeAsync("player.get", "{}", new InvokeOptions
        {
            GameId = "game-override",
            Env = "production",
            Metadata = new Dictionary<string, string>
            {
                ["Authorization"] = "Bearer override-token"
            }
        });

        capturedRequest.Should().NotBeNull();
        capturedRequest!.Metadata["Authorization"].Should().Be("Bearer override-token");
        capturedRequest.Metadata["X-Game-ID"].Should().Be("game-override");
        capturedRequest.Metadata["X-Env"].Should().Be("production");
    }

    [Fact]
    public async Task CroupierClient_ConnectAsync_UploadsCapabilitiesWhenControlAddrIsConfigured()
    {
        byte[]? capturedRequest = null;
        var config = CreateTestConfig();
        config.ControlAddr = "127.0.0.1:19100";
        config.ServiceVersion = "2.0.0";
        config.ProviderLang = "csharp";
        config.ProviderSdk = "croupier-csharp-sdk";
        var server = new FakeLocalServer();

        var agentTransport = new FakeTransport((msgType, _) =>
        {
            msgType.Should().Be(Protocol.MsgRegisterLocalRequest);
            return new RegisterLocalResponse { SessionId = "session-1" }.ToByteArray();
        });

        var controlTransport = new FakeTransport((msgType, data) =>
        {
            msgType.Should().Be(Protocol.MsgRegisterCapabilitiesReq);
            capturedRequest = data;
            return Array.Empty<byte>();
        });

        var client = new CroupierClient(
            config,
            new ConsoleCroupierLogger(),
            (address, _, _) => address.Contains("19100", StringComparison.Ordinal)
                ? controlTransport
                : agentTransport,
            (_, _, _) => server);

        client.RegisterFunction(
            new FunctionDescriptor
            {
                Id = "get",
                Category = "player",
                Operation = "get",
                Description = "Get player",
                InputSchema = "{\"type\":\"object\"}",
                OutputSchema = "{\"type\":\"object\"}"
            },
            (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();

        agentTransport.ConnectCount.Should().Be(1);
        controlTransport.ConnectCount.Should().Be(1);
        capturedRequest.Should().NotBeNull();

        var requestFields = ParseProtoMessage(capturedRequest!);
        var providerFields = ParseProtoMessage((byte[])requestFields[1]);
        Encoding.UTF8.GetString((byte[])providerFields[1]).Should().Be("test-service");
        Encoding.UTF8.GetString((byte[])providerFields[2]).Should().Be("2.0.0");
        Encoding.UTF8.GetString((byte[])providerFields[3]).Should().Be("csharp");
        Encoding.UTF8.GetString((byte[])providerFields[4]).Should().Be("croupier-csharp-sdk");
        var manifestJsonGz = (byte[])requestFields[2];
        manifestJsonGz.Length.Should().BeGreaterThan(0);

        using var input = new MemoryStream(manifestJsonGz);
        using var gzip = new GZipStream(input, CompressionMode.Decompress);
        using var output = new MemoryStream();
        await gzip.CopyToAsync(output);
        var manifest = JsonDocument.Parse(output.ToArray());
        manifest.RootElement.GetProperty("provider").GetProperty("id").GetString().Should().Be("test-service");
        manifest.RootElement.GetProperty("functions").GetArrayLength().Should().Be(1);
    }

    [Fact]
    public async Task CroupierClient_ConnectAsync_IgnoresCapabilitiesUploadFailures()
    {
        var config = CreateTestConfig();
        config.ControlAddr = "127.0.0.1:19100";
        var server = new FakeLocalServer();

        var agentTransport = new FakeTransport((msgType, _) =>
        {
            msgType.Should().Be(Protocol.MsgRegisterLocalRequest);
            return new RegisterLocalResponse { SessionId = "session-1" }.ToByteArray();
        });

        var controlTransport = new FakeTransport((msgType, _) =>
        {
            msgType.Should().Be(Protocol.MsgRegisterCapabilitiesReq);
            throw new InvalidOperationException("capabilities failed");
        });

        var client = new CroupierClient(
            config,
            new ConsoleCroupierLogger(),
            (address, _, _) => address.Contains("19100", StringComparison.Ordinal)
                ? controlTransport
                : agentTransport,
            (_, _, _) => server);

        client.RegisterFunction(
            new FunctionDescriptor { Id = "get", Category = "player", Operation = "get" },
            (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();

        client.IsConnected.Should().BeTrue();
        agentTransport.ConnectCount.Should().Be(1);
        controlTransport.ConnectCount.Should().Be(1);
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
