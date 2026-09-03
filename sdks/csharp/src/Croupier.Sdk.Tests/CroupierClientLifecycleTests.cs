// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using System.IO.Compression;
using System.Text.Json;
using Croupier.Sdk.Models;
using Croupier.Sdk.Tests.MockAgent;
using Croupier.Sdk.Transport;
using Croupier.Sdk.V1;
using FluentAssertions;
using Google.Protobuf;
using Microsoft.Extensions.Logging.Abstractions;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// End-to-end CroupierClient tests against a local mock Agent TCP server.
/// Covers connect/registration, heartbeat, invoke dispatch, disconnect and
/// auto-reconnect paths.
/// </summary>
public sealed class CroupierClientLifecycleTests : IDisposable
{
    private readonly MockAgentServer _agent = new();

    public CroupierClientLifecycleTests()
    {
        _agent.Start();
    }

    public void Dispose() => _agent.DisposeAsync().AsTask().GetAwaiter().GetResult();

    private ClientConfig CreateConfig(Action<ClientConfig>? customize = null)
    {
        var config = new ClientConfig
        {
            AgentAddr = _agent.Address,
            ServiceId = "lifecycle-service",
            GameId = "game-a",
            Env = "test",
            HeartbeatIntervalSeconds = 30, // keep quiet by default; heartbeat tests lower it
            TimeoutSeconds = 5,
            ConnectTimeoutSeconds = 5,
            AutoReconnect = false,
        };
        customize?.Invoke(config);
        return config;
    }

    private static FunctionDescriptor Descriptor(string id) => new()
    {
        Id = id,
        Version = "1.2.3",
        Resource = "test",
        Operation = "run",
        Summary = "Test function",
    };

    #region Connect / Register

    [Fact]
    public async Task ConnectAsync_SendsProviderConnectWithFunctionsAndStoresSession()
    {
        using var client = new CroupierClient(CreateConfig());
        client.RegisterFunction(Descriptor("player.get"), (ctx, payload) => Task.FromResult("{\"ok\":true}"));

        await client.ConnectAsync();

        client.IsConnected.Should().BeTrue();
        client.SessionId.Should().Be(_agent.SessionIdToIssue);

        var connect = _agent.ConnectRequests.Should().ContainSingle().Subject;
        connect.ServiceId.Should().Be("lifecycle-service");
        connect.SdkLanguage.Should().Be("csharp");
        connect.ProtocolVersion.Should().Be("v1");
        connect.Functions.Should().ContainSingle().Which.Id.Should().Be("player.get");
    }

    [Fact]
    public async Task ConnectAsync_WithTcpSchemePrefix_StripsPrefix()
    {
        using var client = new CroupierClient(CreateConfig(config => config.AgentAddr = $"tcp://{_agent.Address}"));
        client.RegisterFunction(Descriptor("fn.tcp"), (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();

        client.IsConnected.Should().BeTrue();
        _agent.ConnectRequests.Should().ContainSingle();
    }

    [Fact]
    public async Task ConnectAsync_WhenAlreadyConnected_SkipsSecondRegistration()
    {
        using var client = new CroupierClient(CreateConfig());
        client.RegisterFunction(Descriptor("fn.idem"), (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();
        await client.ConnectAsync();

        _agent.AcceptedConnections.Should().Be(1);
        _agent.ConnectRequests.Should().ContainSingle();
        client.SessionId.Should().Be(_agent.SessionIdToIssue);
    }

    [Fact]
    public async Task ConnectAsync_WithoutRegisteredFunctions_Throws()
    {
        using var client = new CroupierClient(CreateConfig());

        var action = () => client.ConnectAsync();

        (await action.Should().ThrowAsync<InvalidOperationException>())
            .WithMessage("*Register at least one function*");
    }

    [Fact]
    public async Task ConnectAsync_WithEmptySessionId_ThrowsAndKeepsDisconnected()
    {
        _agent.IssueEmptySessionId = true;
        using var client = new CroupierClient(CreateConfig());
        client.RegisterFunction(Descriptor("fn.empty"), (ctx, payload) => Task.FromResult("{}"));

        var action = () => client.ConnectAsync();

        (await action.Should().ThrowAsync<InvalidOperationException>())
            .WithMessage("*empty session_id*");
        client.IsConnected.Should().BeFalse();
    }

    [Fact]
    public async Task ConnectAsync_WithMalformedConnectResponse_Throws()
    {
        _agent.GarbageConnectResponse = true;
        using var client = new CroupierClient(CreateConfig());
        client.RegisterFunction(Descriptor("fn.bad"), (ctx, payload) => Task.FromResult("{}"));

        var action = () => client.ConnectAsync();

        await action.Should().ThrowAsync<Google.Protobuf.InvalidProtocolBufferException>();
        client.IsConnected.Should().BeFalse();
    }

    [Fact]
    public async Task ConnectAsync_WhenAgentUnreachable_Throws()
    {
        // Grab a free port and close the listener so nothing answers.
        var probe = new System.Net.Sockets.TcpListener(System.Net.IPAddress.Loopback, 0);
        probe.Start();
        var deadPort = ((System.Net.IPEndPoint)probe.LocalEndpoint).Port;
        probe.Stop();

        using var client = new CroupierClient(CreateConfig(config =>
        {
            config.AgentAddr = $"127.0.0.1:{deadPort}";
            config.ConnectTimeoutSeconds = 1;
        }));
        client.RegisterFunction(Descriptor("fn.dead"), (ctx, payload) => Task.FromResult("{}"));

        var action = () => client.ConnectAsync();

        await action.Should().ThrowAsync<Exception>();
        client.IsConnected.Should().BeFalse();
    }

    [Fact]
    public void CroupierClient_WithMicrosoftLogger_CreatesInstance()
    {
        using var client = new CroupierClient(CreateConfig(), NullLogger.Instance);

        client.Config.ServiceId.Should().Be("lifecycle-service");
    }

    [Fact]
    public async Task ConnectAsync_WithControlAddr_RegistersCapabilitiesWithGzippedManifest()
    {
        using var control = new MockAgentServer();
        control.Start();

        using var client = new CroupierClient(CreateConfig(config => config.ControlAddr = control.Address));
        client.RegisterFunction(Descriptor("fn.cap"), (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();

        client.IsConnected.Should().BeTrue();
        control.CapabilityRequests.Should().ContainSingle();

        // Wire format: field1 = provider meta bytes, field2 = gzipped manifest bytes.
        var body = control.CapabilityRequests.Single();
        var input = new Google.Protobuf.CodedInputStream(body);
        byte[]? providerMeta = null;
        byte[]? manifestGzip = null;
        while (!input.IsAtEnd)
        {
            var tag = input.ReadTag();
            var field = Google.Protobuf.WireFormat.GetTagFieldNumber(tag);
            if (field == 1)
            {
                providerMeta = input.ReadBytes().ToByteArray();
            }
            else if (field == 2)
            {
                manifestGzip = input.ReadBytes().ToByteArray();
            }
            else
            {
                input.SkipLastField();
            }
        }

        providerMeta.Should().NotBeNull();
        manifestGzip.Should().NotBeNull();

        using var gzip = new GZipStream(new MemoryStream(manifestGzip!), CompressionMode.Decompress);
        using var decompressed = new MemoryStream();
        gzip.CopyTo(decompressed);
        using var document = JsonDocument.Parse(decompressed.ToArray());

        document.RootElement.GetProperty("provider").GetProperty("id").GetString().Should().Be("lifecycle-service");
        var functions = document.RootElement.GetProperty("functions");
        functions.GetArrayLength().Should().Be(1);
        functions[0].GetProperty("id").GetString().Should().Be("fn.cap");
        functions[0].GetProperty("version").GetString().Should().Be("1.2.3");
    }

    [Fact]
    public async Task ConnectAsync_WithDeadControlAddr_IsFailOpen()
    {
        // 审查发现 #2 修复后：manifest 上传 fire-and-forget 且整体 fail-open
        // （原实现在 try 外 Connect，死控制面会中止整个 provider connect）。
        var probe = new System.Net.Sockets.TcpListener(System.Net.IPAddress.Loopback, 0);
        probe.Start();
        var deadPort = ((System.Net.IPEndPoint)probe.LocalEndpoint).Port;
        probe.Stop();

        using var client = new CroupierClient(CreateConfig(config =>
        {
            config.ControlAddr = $"127.0.0.1:{deadPort}";
            config.ConnectTimeoutSeconds = 1;
        }));
        client.RegisterFunction(Descriptor("fn.cap-dead"), (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();
        client.IsConnected.Should().BeTrue();
    }

    [Fact]
    public async Task ConnectAsync_WithAliveControlAddr_ConnectSucceeds()
    {
        using var control = new MockAgentServer();
        control.Start();

        using var client = new CroupierClient(CreateConfig(config => config.ControlAddr = control.Address));
        client.RegisterFunction(Descriptor("fn.cap-alive"), (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();

        client.IsConnected.Should().BeTrue();
        // fire-and-forget：能力帧异步到达，轮询等待
        var deadline = DateTime.UtcNow.AddSeconds(5);
        while (control.CapabilityRequests.Count == 0 && DateTime.UtcNow < deadline)
        {
            await Task.Delay(50);
        }
        control.CapabilityRequests.Should().ContainSingle();
    }

    #endregion

    #region Heartbeat

    [Fact]
    public async Task HeartbeatLoop_SendsHeartbeatsWithSessionAndServiceId()
    {
        using var client = new CroupierClient(CreateConfig(config =>
        {
            config.HeartbeatIntervalSeconds = 1;
        }));
        client.RegisterFunction(Descriptor("fn.hb"), (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();
        await _agent.WaitForHeartbeatsAsync(2);

        _agent.HeartbeatRequests.Should().OnlyContain(hb =>
            hb.ServiceId == "lifecycle-service" && hb.SessionId == _agent.SessionIdToIssue);
    }

    [Fact]
    public async Task Disconnect_StopsHeartbeatsAndClearsSession()
    {
        using var client = new CroupierClient(CreateConfig(config =>
        {
            config.HeartbeatIntervalSeconds = 1;
        }));
        client.RegisterFunction(Descriptor("fn.dc"), (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();
        await _agent.WaitForHeartbeatsAsync(1);

        client.Disconnect();

        client.IsConnected.Should().BeFalse();
        client.SessionId.Should().BeEmpty();

        var heartbeatsAtDisconnect = _agent.HeartbeatRequests.Count;
        await Task.Delay(2500);
        _agent.HeartbeatRequests.Count.Should().Be(heartbeatsAtDisconnect);
    }

    [Fact]
    public async Task Stop_CancelsHeartbeatLoop()
    {
        using var client = new CroupierClient(CreateConfig(config =>
        {
            config.HeartbeatIntervalSeconds = 1;
        }));
        client.RegisterFunction(Descriptor("fn.stop"), (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();
        await _agent.WaitForHeartbeatsAsync(1);

        client.Stop();

        var heartbeatsAtStop = _agent.HeartbeatRequests.Count;
        await Task.Delay(2500);
        _agent.HeartbeatRequests.Count.Should().Be(heartbeatsAtStop);
    }

    [Fact]
    public async Task Dispose_AfterConnect_StopsEverythingCleanly()
    {
        using var client = new CroupierClient(CreateConfig());
        client.RegisterFunction(Descriptor("fn.dispose"), (ctx, payload) => Task.FromResult("{}"));
        await client.ConnectAsync();

        var action = () => client.Dispose();

        action.Should().NotThrow();
        client.IsConnected.Should().BeFalse();
    }

    #endregion

    #region Invoke (client -> agent)

    [Fact]
    public async Task InvokeAsync_RoundTripsAndSendsAllMetadata()
    {
        using var client = new CroupierClient(CreateConfig(config =>
        {
            config.AuthToken = "secret-token";
            config.Headers["X-Custom"] = "from-config";
        }));
        client.RegisterFunction(Descriptor("fn.invoke"), (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();

        var result = await client.InvokeAsync(
            "fn.invoke",
            "{\"player\":\"42\"}",
            new InvokeOptions
            {
                GameId = "game-override",
                Env = "env-override",
                RequestId = "req-1",
                IdempotencyKey = "idem-1",
                Metadata = new Dictionary<string, string> { ["X-Custom"] = "from-options" },
            });

        result.Should().Be("echo:{\"player\":\"42\"}");
        var request = _agent.InvokeRequests.Should().ContainSingle().Subject;
        request.FunctionId.Should().Be("fn.invoke");
        request.IdempotencyKey.Should().Be("idem-1");
        request.Metadata["X-Game-ID"].Should().Be("game-override");
        request.Metadata["X-Env"].Should().Be("env-override");
        request.Metadata["X-Request-ID"].Should().Be("req-1");
        request.Metadata["X-Custom"].Should().Be("from-options");
        request.Metadata["Authorization"].Should().Be("Bearer secret-token");
    }

    [Fact]
    public async Task InvokeAsync_WithoutOptions_FallsBackToConfigScope()
    {
        using var client = new CroupierClient(CreateConfig());
        client.RegisterFunction(Descriptor("fn.default"), (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();

        await client.InvokeAsync("fn.default", "{}");

        var request = _agent.InvokeRequests.Should().ContainSingle().Subject;
        request.Metadata["X-Game-ID"].Should().Be("game-a");
        request.Metadata["X-Env"].Should().Be("test");
        request.IdempotencyKey.Should().BeEmpty();
    }

    [Fact]
    public async Task InvokeAsync_WhenTransportDisconnected_Throws()
    {
        using var client = new CroupierClient(CreateConfig());
        client.RegisterFunction(Descriptor("fn.offline"), (ctx, payload) => Task.FromResult("{}"));

        var action = () => client.InvokeAsync("fn.offline", "{}");

        (await action.Should().ThrowAsync<InvalidOperationException>())
            .WithMessage("*Not connected to Agent*");
    }

    [Fact]
    public async Task InvokeAsync_AfterAgentDrop_WithReconnectDisabled_Throws()
    {
        using var client = new CroupierClient(CreateConfig(config =>
        {
            config.HeartbeatIntervalSeconds = 1;
        }));
        client.RegisterFunction(Descriptor("fn.drop"), (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();
        _agent.DropConnections();
        await Task.Delay(2500); // let the transport read loop observe the drop

        var action = () => client.InvokeAsync("fn.drop", "{}");

        await action.Should().ThrowAsync<InvalidOperationException>();
    }

    #endregion

    #region Inbound dispatch (agent -> client)

    [Fact]
    public async Task InboundInvoke_DispatchesToHandlerWithFullContext()
    {
        FunctionContext? capturedContext = null;
        string? capturedPayload = null;

        using var client = new CroupierClient(CreateConfig());
        client.RegisterFunction(Descriptor("fn.inbound"), (ctx, payload) =>
        {
            capturedContext = ctx;
            capturedPayload = payload;
            return Task.FromResult($"handled:{payload}");
        });

        await client.ConnectAsync();

        var response = await _agent.SendInboundInvokeAsync(new InvokeRequest
        {
            FunctionId = "fn.inbound",
            Payload = ByteString.CopyFromUtf8("{\"x\":1}"),
            IdempotencyKey = "inbound-idem",
            Metadata =
            {
                ["X-Game-ID"] = "game-meta",
                ["X-Env"] = "env-meta",
                ["X-User-ID"] = "user-7",
                ["X-Caller-Service-ID"] = "caller-svc",
            },
        });

        response.Payload.ToStringUtf8().Should().Be("handled:{\"x\":1}");
        capturedPayload.Should().Be("{\"x\":1}");
        capturedContext.Should().NotBeNull();
        capturedContext!.FunctionId.Should().Be("fn.inbound");
        capturedContext.GameId.Should().Be("game-meta");
        capturedContext.Env.Should().Be("env-meta");
        capturedContext.UserId.Should().Be("user-7");
        capturedContext.CallerServiceId.Should().Be("caller-svc");
        capturedContext.IdempotencyKey.Should().Be("inbound-idem");
        capturedContext.CallId.Should().NotBeNullOrEmpty();
        capturedContext.Timestamp.Should().BeGreaterThan(0);
    }

    [Fact]
    public async Task InboundInvoke_WithoutMetadata_FallsBackToConfigScope()
    {
        FunctionContext? capturedContext = null;
        using var client = new CroupierClient(CreateConfig());
        client.RegisterFunction(Descriptor("fn.nometa"), (ctx, payload) =>
        {
            capturedContext = ctx;
            return Task.FromResult("{}");
        });

        await client.ConnectAsync();

        await _agent.SendInboundInvokeAsync(new InvokeRequest
        {
            FunctionId = "fn.nometa",
            Payload = ByteString.CopyFromUtf8("{}"),
        });

        capturedContext!.GameId.Should().Be("game-a");
        capturedContext.Env.Should().Be("test");
        capturedContext.IdempotencyKey.Should().BeNull();
    }

    [Fact]
    public async Task InboundInvoke_ForUnknownFunction_ReturnsErrorJson()
    {
        using var client = new CroupierClient(CreateConfig());
        client.RegisterFunction(Descriptor("fn.known"), (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();

        var response = await _agent.SendInboundInvokeAsync(new InvokeRequest
        {
            FunctionId = "fn.unknown",
            Payload = ByteString.CopyFromUtf8("{}"),
        });

        response.Payload.ToStringUtf8().Should().Contain("Function not found: fn.unknown");
    }

    [Fact]
    public async Task InboundInvoke_WhenHandlerThrows_ReturnsErrorJson()
    {
        using var client = new CroupierClient(CreateConfig());
        client.RegisterFunction(Descriptor("fn.throw"), (FunctionHandlerDelegate)((ctx, payload) =>
            throw new InvalidOperationException("boom")));

        await client.ConnectAsync();

        var response = await _agent.SendInboundInvokeAsync(new InvokeRequest
        {
            FunctionId = "fn.throw",
            Payload = ByteString.CopyFromUtf8("{}"),
        });

        response.Payload.ToStringUtf8().Should().Be("{\"error\":\"boom\"}");
    }

    [Fact]
    public async Task InboundInvoke_UsesLatestHandlerAfterReregistration()
    {
        using var client = new CroupierClient(CreateConfig());
        client.RegisterFunction(Descriptor("fn.replace"), (ctx, payload) => Task.FromResult("first"));
        client.RegisterFunction(Descriptor("fn.replace"), (ctx, payload) => Task.FromResult("second"));

        await client.ConnectAsync();

        var response = await _agent.SendInboundInvokeAsync(new InvokeRequest
        {
            FunctionId = "fn.replace",
            Payload = ByteString.CopyFromUtf8("{}"),
        });

        response.Payload.ToStringUtf8().Should().Be("second");
    }

    [Fact]
    public async Task InboundInvoke_WithSyncHandler_ExecutesHandler()
    {
        using var client = new CroupierClient(CreateConfig());
        client.RegisterFunction(Descriptor("fn.sync"), new SyncFunctionHandlerDelegate((ctx, payload) => "sync-result"));

        await client.ConnectAsync();

        var response = await _agent.SendInboundInvokeAsync(new InvokeRequest
        {
            FunctionId = "fn.sync",
            Payload = ByteString.CopyFromUtf8("{}"),
        });

        response.Payload.ToStringUtf8().Should().Be("sync-result");
    }

    [Fact]
    public async Task InboundUnsupportedMessageType_ReturnsErrorJson()
    {
        using var client = new CroupierClient(CreateConfig());
        client.RegisterFunction(Descriptor("fn.x"), (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();

        var body = await _agent.SendUnsupportedInboundAsync(Protocol.MsgGetTaskResultRequest);

        var text = System.Text.Encoding.UTF8.GetString(body);
        text.Should().Contain("Unsupported message type");
        text.Should().Contain("GetTaskResultRequest");
    }

    #endregion

    #region Auto reconnect

    [Fact]
    public async Task AgentDrop_WithAutoReconnect_ReestablishesSession()
    {
        _agent.SessionIdToIssue = "session-first";
        using var client = new CroupierClient(CreateConfig(config =>
        {
            config.HeartbeatIntervalSeconds = 1;
            config.ReconnectIntervalSeconds = 1;
            config.AutoReconnect = true;
        }));
        client.RegisterFunction(Descriptor("fn.reconnect"), (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();
        client.SessionId.Should().Be("session-first");

        _agent.SessionIdToIssue = "session-second";
        _agent.DropConnections();

        var deadline = DateTime.UtcNow.AddSeconds(15);
        while (DateTime.UtcNow < deadline && client.SessionId != "session-second")
        {
            await Task.Delay(100);
        }

        client.SessionId.Should().Be("session-second");
        client.IsConnected.Should().BeTrue();
        _agent.AcceptedConnections.Should().BeGreaterThanOrEqualTo(2);
        _agent.ConnectRequests.Should().HaveCountGreaterThan(1);

        // The re-registered provider can still serve inbound invokes.
        var response = await _agent.SendInboundInvokeAsync(new InvokeRequest
        {
            FunctionId = "fn.reconnect",
            Payload = ByteString.CopyFromUtf8("{}"),
        });
        response.Payload.ToStringUtf8().Should().NotBeNullOrEmpty();
    }

    [Fact]
    public async Task AgentDrop_WithoutAutoReconnect_DoesNotReconnect()
    {
        using var client = new CroupierClient(CreateConfig(config =>
        {
            config.HeartbeatIntervalSeconds = 1;
            config.AutoReconnect = false;
        }));
        client.RegisterFunction(Descriptor("fn.noreconnect"), (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();
        await _agent.WaitForConnectionsAsync(1);

        _agent.DropConnections();
        await Task.Delay(3500);

        _agent.AcceptedConnections.Should().Be(1);
        _agent.ConnectRequests.Should().ContainSingle();
    }

    [Fact]
    public async Task Reconnect_AfterManualDisconnect_HeartbeatLoopStaysDead()
    {
        // Documents a known issue: Disconnect() cancels the shared shutdown
        // token and ConnectAsync() never recreates it, so after a manual
        // disconnect+connect cycle the heartbeat loop (and thus crash-level
        // auto reconnect) silently stops running.
        using var client = new CroupierClient(CreateConfig(config =>
        {
            config.HeartbeatIntervalSeconds = 1;
            config.ReconnectIntervalSeconds = 1;
            config.AutoReconnect = true;
        }));
        client.RegisterFunction(Descriptor("fn.cycle"), (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();
        client.Disconnect();

        _agent.SessionIdToIssue = "session-manual";
        await client.ConnectAsync();
        client.SessionId.Should().Be("session-manual");

        var heartbeatsBefore = _agent.HeartbeatRequests.Count;
        await Task.Delay(3000);

        _agent.HeartbeatRequests.Count.Should().Be(heartbeatsBefore,
            "the heartbeat loop is not restarted after Disconnect()+ConnectAsync()");
    }

    #endregion

    #region ServeAsync

    [Fact]
    public async Task ServeAsync_ConnectsWhenNeeded_AndStopsOnCancellation()
    {
        using var client = new CroupierClient(CreateConfig());
        client.RegisterFunction(Descriptor("fn.serve"), (ctx, payload) => Task.FromResult("{}"));

        using var cts = new CancellationTokenSource(500);
        var action = () => client.ServeAsync(cts.Token);

        await action.Should().ThrowAsync<OperationCanceledException>();
        client.IsConnected.Should().BeTrue();
    }

    [Fact]
    public async Task ServeAsync_WhenAlreadyConnected_DoesNotReconnect()
    {
        using var client = new CroupierClient(CreateConfig());
        client.RegisterFunction(Descriptor("fn.serve2"), (ctx, payload) => Task.FromResult("{}"));
        await client.ConnectAsync();

        using var cts = new CancellationTokenSource(300);
        await Assert.ThrowsAnyAsync<OperationCanceledException>(() => client.ServeAsync(cts.Token));

        _agent.ConnectRequests.Should().ContainSingle();
    }

    #endregion
}
