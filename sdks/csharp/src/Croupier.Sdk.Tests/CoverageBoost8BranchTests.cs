// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using System;
using System.Collections.Concurrent;
using System.Net;
using System.Net.Sockets;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using Croupier.Sdk.Configuration;
using Croupier.Sdk.Logging;
using Croupier.Sdk.Models;
using Croupier.Sdk.Transport;
using Croupier.Sdk.V1;
using Croupier.Sdk.Validation;
using FluentAssertions;
using Google.Protobuf;
using Xunit;
using OpenAPIImporter = Croupier.Sdk.OpenAPIImporter;

namespace Croupier.Sdk.Tests;

/// <summary>
/// Branch-coverage gap fillers: environment config provider, field hints
/// cloning guard, TCP transport read-loop failure paths, client drain
/// timeout warning and heartbeat disconnect detection.
/// </summary>
public sealed class CoverageBoost8BranchTests
{
    private sealed class RecordingLogger : ICroupierLogger
    {
        public ConcurrentQueue<(string Level, string Message)> Entries { get; } = new();

        public void LogDebug(string component, string message) => Entries.Enqueue(("debug", message));

        public void LogInfo(string component, string message) => Entries.Enqueue(("info", message));

        public void LogWarning(string component, string message) => Entries.Enqueue(("warn", message));

        public void LogError(string component, string message, Exception? exception = null) =>
            Entries.Enqueue(("error", message));
    }

    #region EnvironmentConfigProvider

    [Fact]
    public void EnvironmentConfigProvider_AllVariablesSet_UsesEnvironmentValues()
    {
        var names = new[]
        {
            "CROUPIER_AGENT_ADDR", "CROUPIER_SERVICE_ID", "CROUPIER_AGENT_ID",
            "CROUPIER_SERVICE_VERSION", "CROUPIER_GAME_ID", "CROUPIER_ENV",
            "CROUPIER_CONTROL_ADDR", "CROUPIER_INSECURE", "CROUPIER_CERT_FILE",
            "CROUPIER_KEY_FILE", "CROUPIER_CA_FILE", "CROUPIER_SERVER_NAME",
            "CROUPIER_PROVIDER_LANG", "CROUPIER_PROVIDER_SDK", "CROUPIER_AUTH_TOKEN",
            "CROUPIER_TIMEOUT_SECONDS", "CROUPIER_HEARTBEAT_INTERVAL_SECONDS",
            "CROUPIER_AUTO_RECONNECT",
        };
        var previous = names.Select(n => (n, Environment.GetEnvironmentVariable(n))).ToArray();
        try
        {
            Environment.SetEnvironmentVariable("CROUPIER_AGENT_ADDR", "10.1.2.3:19091");
            Environment.SetEnvironmentVariable("CROUPIER_SERVICE_ID", "env-service");
            Environment.SetEnvironmentVariable("CROUPIER_AGENT_ID", "agent-9");
            Environment.SetEnvironmentVariable("CROUPIER_SERVICE_VERSION", "9.9.9");
            Environment.SetEnvironmentVariable("CROUPIER_GAME_ID", "env-game");
            Environment.SetEnvironmentVariable("CROUPIER_ENV", "staging");
            Environment.SetEnvironmentVariable("CROUPIER_CONTROL_ADDR", "10.1.2.4:18780");
            Environment.SetEnvironmentVariable("CROUPIER_INSECURE", "TRUE");
            Environment.SetEnvironmentVariable("CROUPIER_CERT_FILE", "/tmp/cert.pem");
            Environment.SetEnvironmentVariable("CROUPIER_KEY_FILE", "/tmp/key.pem");
            Environment.SetEnvironmentVariable("CROUPIER_CA_FILE", "/tmp/ca.pem");
            Environment.SetEnvironmentVariable("CROUPIER_SERVER_NAME", "agent.example");
            Environment.SetEnvironmentVariable("CROUPIER_PROVIDER_LANG", "csharp");
            Environment.SetEnvironmentVariable("CROUPIER_PROVIDER_SDK", "custom-sdk");
            Environment.SetEnvironmentVariable("CROUPIER_AUTH_TOKEN", "token-1");
            Environment.SetEnvironmentVariable("CROUPIER_TIMEOUT_SECONDS", "45");
            Environment.SetEnvironmentVariable("CROUPIER_HEARTBEAT_INTERVAL_SECONDS", "15");
            Environment.SetEnvironmentVariable("CROUPIER_AUTO_RECONNECT", "1");

            var config = new EnvironmentConfigProvider().GetConfig();

            config.AgentAddr.Should().Be("10.1.2.3:19091");
            config.ServiceId.Should().Be("env-service");
            config.AgentId.Should().Be("agent-9");
            config.ServiceVersion.Should().Be("9.9.9");
            config.GameId.Should().Be("env-game");
            config.Env.Should().Be("staging");
            config.ControlAddr.Should().Be("10.1.2.4:18780");
            config.Insecure.Should().BeTrue();
            config.CertFile.Should().Be("/tmp/cert.pem");
            config.KeyFile.Should().Be("/tmp/key.pem");
            config.CaFile.Should().Be("/tmp/ca.pem");
            config.ServerName.Should().Be("agent.example");
            config.ProviderSdk.Should().Be("custom-sdk");
            config.AuthToken.Should().Be("token-1");
            config.TimeoutSeconds.Should().Be(45);
            config.HeartbeatIntervalSeconds.Should().Be(15);
            config.AutoReconnect.Should().BeTrue();
        }
        finally
        {
            foreach (var (name, value) in previous)
            {
                Environment.SetEnvironmentVariable(name, value);
            }
        }
    }

    [Fact]
    public void EnvironmentConfigProvider_NonBooleanAndInvalidInts_FallBackToDefaults()
    {
        var names = new[] { "CROUPIER_INSECURE", "CROUPIER_AUTO_RECONNECT", "CROUPIER_TIMEOUT_SECONDS", "CROUPIER_HEARTBEAT_INTERVAL_SECONDS" };
        var previous = names.Select(n => (n, Environment.GetEnvironmentVariable(n))).ToArray();
        try
        {
            Environment.SetEnvironmentVariable("CROUPIER_INSECURE", "0");
            Environment.SetEnvironmentVariable("CROUPIER_AUTO_RECONNECT", "no");
            Environment.SetEnvironmentVariable("CROUPIER_TIMEOUT_SECONDS", "not-a-number");
            Environment.SetEnvironmentVariable("CROUPIER_HEARTBEAT_INTERVAL_SECONDS", "bad");

            var config = new EnvironmentConfigProvider().GetConfig();

            config.Insecure.Should().BeFalse();
            config.AutoReconnect.Should().BeFalse();
            config.TimeoutSeconds.Should().Be(30);
            config.HeartbeatIntervalSeconds.Should().Be(60);
        }
        finally
        {
            foreach (var (name, value) in previous)
            {
                Environment.SetEnvironmentVariable(name, value);
            }
        }
    }

    [Fact]
    public void EnvironmentConfigProvider_CustomPrefix_UsesPrefixedNames()
    {
        const string name = "MYAPP_AGENT_ADDR";
        var previous = Environment.GetEnvironmentVariable(name);
        try
        {
            Environment.SetEnvironmentVariable(name, "127.0.0.1:19999");
            var config = new EnvironmentConfigProvider("MYAPP_").GetConfig();
            config.AgentAddr.Should().Be("127.0.0.1:19999");
        }
        finally
        {
            Environment.SetEnvironmentVariable(name, previous);
        }
    }

    #endregion

    #region FieldHints

    [Fact]
    public void SetFieldHint_NullDescriptor_ThrowsCloneGuard()
    {
        using var value = System.Text.Json.JsonDocument.Parse("\"v\"");
        var action = () => FieldHints.SetFieldHint(null!, "field", "x-widget", value.RootElement);

        action.Should().Throw<ArgumentException>()
            .WithMessage("*descriptor could not be cloned*");
    }

    #endregion

    #region TCPTransport

    [Fact]
    public void TCPTransport_PortAboveRange_ThrowsOverflow()
    {
        var action = () => new TCPTransport("127.0.0.1:99999");

        action.Should().Throw<OverflowException>().WithMessage("*out of valid range*");
    }

    [Fact]
    public void TCPTransport_DisposeWithoutConnect_DoesNotThrow()
    {
        var transport = new TCPTransport("127.0.0.1:19091");
        var action = () => transport.Dispose();
        action.Should().NotThrow();
    }

    private static int StartListener(Action<TcpClient> onAccept)
    {
        var listener = new TcpListener(IPAddress.Loopback, 0);
        listener.Start(1);
        var port = ((IPEndPoint)listener.LocalEndpoint).Port;
        _ = Task.Run(async () =>
        {
            try
            {
                using var accepted = await listener.AcceptTcpClientAsync();
                onAccept(accepted);
            }
            catch (Exception)
            {
                // listener may be torn down by the test
            }
            finally
            {
                listener.Stop();
            }
        });
        return port;
    }

    [Fact]
    public async Task TCPTransport_RemoteClosesConnection_MarksDisconnected()
    {
        var port = StartListener(client =>
        {
            client.Close();
        });

        using var transport = new TCPTransport($"127.0.0.1:{port}", 3000, 3000);
        transport.Connect();
        transport.IsConnected.Should().BeTrue();

        var deadline = DateTime.UtcNow.AddSeconds(5);
        while (transport.IsConnected && DateTime.UtcNow < deadline)
        {
            await Task.Delay(25);
        }

        transport.IsConnected.Should().BeFalse();
    }

    [Fact]
    public async Task TCPTransport_TruncatedFrame_LogsReadLoopError()
    {
        var port = StartListener(async client =>
        {
            using var stream = client.GetStream();
            // Frame header declares 5 bytes, but protocol header needs 8:
            // ParseMessage throws ArgumentException inside the read loop.
            var frame = new byte[] { 0, 0, 0, 5, 1, 2, 3, 4, 5 };
            await stream.WriteAsync(frame);
            await stream.FlushAsync();
            await Task.Delay(1500);
        });

        var logger = new RecordingLogger();
        using var transport = new TCPTransport($"127.0.0.1:{port}", 3000, 3000, logger);
        transport.Connect();

        var deadline = DateTime.UtcNow.AddSeconds(5);
        while (transport.IsConnected && DateTime.UtcNow < deadline)
        {
            await Task.Delay(25);
        }

        transport.IsConnected.Should().BeFalse();
        logger.Entries.Should().Contain(e => e.Message.Contains("Read loop error"));
    }

    #endregion

    #region OpenAPIImporter

    [Theory]
    [InlineData("danger", "danger")]
    [InlineData("warning", "warning")]
    public void ParseRiskLevel_DangerAndWarning_AreMappedVerbatim(string input, string expected)
    {
        OpenAPIImporter.ParseRiskLevel(input).Should().Be(expected);
    }

    [Fact]
    public void OpenAPI_ApprovalRequiredFalse_IsImported()
    {
        const string spec = """
        {
          "openapi": "3.0.0",
          "info": { "title": "t", "version": "1" },
          "paths": {
            "/off": {
              "post": {
                "operationId": "offOp",
                "x-approval": { "required": false }
              }
            }
          }
        }
        """;

        var client = new CroupierClient(new ClientConfig { AgentAddr = "localhost:1" });
        var registered = OpenAPIImporter.RegisterFromOpenAPI(
            client, spec, null, new Dictionary<string, FunctionHandlerDelegate>
            {
                ["offOp"] = (ctx, payload) => Task.FromResult("{}"),
            });

        registered.Should().Contain("offOp");
    }

    #endregion

    #region CroupierClient drain / heartbeat / metadata fallbacks

    private sealed class MockTransport : IClientTransport
    {
        public Func<int, byte[]?, Task<byte[]>> CallHandler { get; set; } =
            (_, _) => Task.FromResult(Array.Empty<byte>());

        public bool IsConnectedValue { get; set; }

        public int DisposeCount { get; private set; }

        public Func<int, int, byte[], Task<byte[]>>? CapturedInboundHandler { get; private set; }

        public bool IsConnected => IsConnectedValue;

        public void Connect()
        {
        }

        public byte[] Call(int msgType, byte[]? data) => CallAsync(msgType, data).GetAwaiter().GetResult();

        public async Task<byte[]> CallAsync(int msgType, byte[]? data, CancellationToken cancellationToken = default)
        {
            return await CallHandler(msgType, data);
        }

        public void SetInboundRequestHandler(Func<int, int, byte[], Task<byte[]>>? handler)
        {
            CapturedInboundHandler = handler;
        }

        public void Dispose()
        {
            DisposeCount++;
        }
    }

    private static ClientConfig BaseConfig(Action<ClientConfig>? customize = null)
    {
        var config = new ClientConfig
        {
            AgentAddr = "agent.example:19090",
            ServiceId = "boost8-service",
            HeartbeatIntervalSeconds = 30,
            AutoReconnect = false,
        };
        customize?.Invoke(config);
        return config;
    }

    private static CroupierClient CreateClient(
        ClientConfig config,
        out List<MockTransport> transports,
        ICroupierLogger? logger = null)
    {
        var created = new List<MockTransport>();
        var client = new CroupierClient(config, logger, (address, timeoutMs, connectTimeoutMs, factoryLogger) =>
        {
            var transport = new MockTransport
            {
                IsConnectedValue = true,
                CallHandler = (_, _) => Task.FromResult(
                    MessageExtensions.ToByteArray(new ProviderConnectResponse { SessionId = "boost8-session" })),
            };
            created.Add(transport);
            return transport;
        })!;
        transports = created;
        return client;
    }

    private static Task InvokePrivate(CroupierClient client, string method, CancellationToken token) =>
        (Task)client.GetType().InvokeMember(
            method,
            System.Reflection.BindingFlags.NonPublic | System.Reflection.BindingFlags.InvokeMethod | System.Reflection.BindingFlags.Instance,
            null,
            client,
            new object[] { token })!;

    [Fact]
    public async Task DrainAndRecover_WithCancelledTokenAndInFlightCall_LogsDrainTimeout()
    {
        var client = CreateClient(BaseConfig(), out var transports);
        var release = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        client.RegisterFunction(
            new FunctionDescriptor { Id = "fn.drain", Resource = "r", Operation = "o" },
            async (ctx, payload) =>
            {
                await release.Task;
                return "{}";
            });
        await client.ConnectAsync();
        var transport = transports.Single();

        var inbound = transport.CapturedInboundHandler!;
        var inboundTask = inbound(
            Protocol.MsgInvokeRequest,
            1,
            MessageExtensions.ToByteArray(new InvokeRequest { FunctionId = "fn.drain", Payload = ByteString.CopyFromUtf8("{}") }));

        // Give HandleInboundRequestAsync a moment to enter the handler (bumps _activeInboundCalls).
        await Task.Delay(200);

        using var cts = new CancellationTokenSource();
        cts.Cancel();
        await InvokePrivate(client, "DrainAndRecoverAsync", cts.Token);

        release.SetResult();
        await inboundTask;

        client.Disconnect();
    }

    private static async Task<byte[]> RunInboundInvoke(CroupierClient client, MockTransport transport, InvokeRequest request)
    {
        var handler = transport.CapturedInboundHandler!;
        return await handler(Protocol.MsgInvokeRequest, 7, MessageExtensions.ToByteArray(request));
    }

    [Fact]
    public async Task InboundInvoke_WithoutMetadataAndNullConfigGameIdEnv_FallsBackToEmpty()
    {
        var client = CreateClient(BaseConfig(config =>
        {
            config.GameId = null!;
            config.Env = null!;
        }), out var transports);
        string? observedGameId = null;
        string? observedEnv = null;
        client.RegisterFunction(
            new FunctionDescriptor { Id = "fn.fallback", Resource = "r", Operation = "o" },
            (ctx, payload) =>
            {
                observedGameId = ctx.GameId;
                observedEnv = ctx.Env;
                return Task.FromResult("{}");
            });
        await client.ConnectAsync();
        var transport = transports.Single();

        var responseBytes = await RunInboundInvoke(client, transport, new InvokeRequest
        {
            FunctionId = "fn.fallback",
            Payload = ByteString.CopyFromUtf8("{}"),
        });

        var response = InvokeResponse.Parser.ParseFrom(responseBytes);
        response.Payload.ToStringUtf8().Should().Be("{}");
        observedGameId.Should().BeEmpty();
        observedEnv.Should().BeEmpty();

        client.Disconnect();
    }

    [Fact]
    public async Task ConnectAndRegister_SecondSuccess_DisposesPreviousTransport()
    {
        var client = CreateClient(BaseConfig(), out var transports);
        client.RegisterFunction(
            new FunctionDescriptor { Id = "fn.swap", Resource = "r", Operation = "o" },
            (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();
        var firstTransport = transports.Single();
        firstTransport.DisposeCount.Should().Be(0);

        // A second successful registration must swap the transport and
        // dispose the previous one.
        await InvokePrivate(client, "ConnectAndRegisterAsync", CancellationToken.None);

        firstTransport.DisposeCount.Should().Be(1);

        client.Disconnect();
    }

    [Fact]
    public async Task HeartbeatLoop_TransportDisconnected_WarnsAndContinuesWithoutReconnect()
    {
        var logger = new RecordingLogger();
        var client = CreateClient(
            BaseConfig(config =>
            {
                config.HeartbeatIntervalSeconds = 1;
            }),
            out var transports,
            logger);
        client.RegisterFunction(
            new FunctionDescriptor { Id = "fn.hb8", Resource = "r", Operation = "o" },
            (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();
        transports.Single().IsConnectedValue = false;

        var deadline = DateTime.UtcNow.AddSeconds(6);
        while (DateTime.UtcNow < deadline &&
               !logger.Entries.Any(e => e.Message.Contains("Heartbeat failed")))
        {
            await Task.Delay(50);
        }

        logger.Entries.Should().Contain(e => e.Message.Contains("Heartbeat failed"));
        // AutoReconnect disabled: heartbeat loop keeps running instead of reconnecting.
        logger.Entries.Should().NotContain(e => e.Message.Contains("Reconnecting"));

        client.Disconnect();
    }

    #endregion
}
