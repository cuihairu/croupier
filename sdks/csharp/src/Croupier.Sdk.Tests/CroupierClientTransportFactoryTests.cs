// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using Croupier.Sdk.Logging;
using Croupier.Sdk.Models;
using Croupier.Sdk.Transport;
using Croupier.Sdk.V1;
using FluentAssertions;
using Google.Protobuf;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// CroupierClient tests driven by a scripted mock IClientTransport injected
/// through the internal transport-factory constructor. These tests pin down
/// address handling, transport lifecycle and invoke request building without
/// touching the network.
/// </summary>
public sealed class CroupierClientTransportFactoryTests
{
    private sealed class MockTransport : IClientTransport
    {
        public Action ConnectAction { get; set; } = () => { };
        public Func<int, byte[]?, Task<byte[]>> CallHandler { get; set; } =
            (_, _) => Task.FromResult(Array.Empty<byte>());

        public bool IsConnectedValue { get; set; }

        public List<(int MsgType, byte[]? Data)> Calls { get; } = new();

        public int ConnectCount { get; private set; }

        public int DisposeCount { get; private set; }

        public Func<int, int, byte[], Task<byte[]>>? CapturedInboundHandler { get; private set; }

        public bool IsConnected => IsConnectedValue;

        public void Connect()
        {
            ConnectCount++;
            ConnectAction();
        }

        public byte[] Call(int msgType, byte[]? data) => CallAsync(msgType, data).GetAwaiter().GetResult();

        public async Task<byte[]> CallAsync(int msgType, byte[]? data, CancellationToken cancellationToken = default)
        {
            Calls.Add((msgType, data));
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

    private static (CroupierClient Client, List<MockTransport> Created) CreateClient(
        ClientConfig config,
        Action<MockTransport>? setupTransport = null)
    {
        var created = new List<MockTransport>();
        var client = new CroupierClient(config, new ConsoleCroupierLogger(), (address, timeoutMs, connectTimeoutMs, logger) =>
        {
            var transport = new MockTransport
            {
                IsConnectedValue = true,
                CallHandler = (_, _) => Task.FromResult(Google.Protobuf.MessageExtensions.ToByteArray(new ProviderConnectResponse { SessionId = "mock-session" })),
            };
            setupTransport?.Invoke(transport);
            created.Add(transport);
            return transport;
        });
        return (client, created);
    }

    private static ClientConfig BaseConfig(Action<ClientConfig>? customize = null)
    {
        var config = new ClientConfig
        {
            AgentAddr = "agent.example:19090",
            ServiceId = "factory-service",
            GameId = "game-1",
            Env = "env-1",
            HeartbeatIntervalSeconds = 30,
            TimeoutSeconds = 7,
            ConnectTimeoutSeconds = 3,
            AutoReconnect = false,
        };
        customize?.Invoke(config);
        return config;
    }

    private static object? GetPrivateField(object instance, string name) =>
        instance.GetType().GetField(name, System.Reflection.BindingFlags.NonPublic | System.Reflection.BindingFlags.Instance)?.GetValue(instance);

    [Fact]
    public async Task ConnectAsync_PassesStrippedAddressAndScaledTimeoutsToFactory()
    {
        var addresses = new List<(string Address, int TimeoutMs, int ConnectTimeoutMs)>();
        CroupierClient? client = null;
        client = new CroupierClient(BaseConfig(config =>
        {
            config.AgentAddr = "tcp://10.0.0.1:9999";
        }), null, (address, timeoutMs, connectTimeoutMs, logger) =>
        {
            addresses.Add((address, timeoutMs, connectTimeoutMs));
            var transport = new MockTransport
            {
                IsConnectedValue = false,
                ConnectAction = () => throw new InvalidOperationException("refused"),
            };
            return transport;
        })!;

        client.RegisterFunction(new FunctionDescriptor { Id = "fn.addr", Resource = "r", Operation = "o" }, (ctx, payload) => Task.FromResult("{}"));

        await Assert.ThrowsAsync<InvalidOperationException>(() => client.ConnectAsync());

        addresses.Should().ContainSingle().Which.Should().Match<(string Address, int TimeoutMs, int ConnectTimeoutMs)>(
            args => args.Address == "10.0.0.1:9999" && args.TimeoutMs == 7000 && args.ConnectTimeoutMs == 3000);
    }

    [Fact]
    public async Task ConnectAsync_StripsTcpPrefixFromControlAddr()
    {
        var addresses = new List<string>();
        using var client = new CroupierClient(BaseConfig(config => config.ControlAddr = "tcp://10.0.0.2:7777"), null, (address, timeoutMs, connectTimeoutMs, logger) =>
        {
            addresses.Add(address);
            return new MockTransport
            {
                IsConnectedValue = true,
                CallHandler = (_, _) => Task.FromResult(Google.Protobuf.MessageExtensions.ToByteArray(new ProviderConnectResponse { SessionId = "s" })),
            };
        })!;

        client.RegisterFunction(new FunctionDescriptor { Id = "fn.ctrl", Resource = "r", Operation = "o" }, (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();

        addresses.Should().Contain("10.0.0.2:7777");
    }

    [Fact]
    public async Task ConnectAsync_WhenConnectThrows_TransportIsNotDisposed()
    {
        // Documents current behavior: transport.Connect() runs outside the
        // try/catch in ConnectAndRegisterAsync, so a failing Connect leaks the
        // transport (no Dispose call).
        var (client, created) = CreateClient(BaseConfig(), transport =>
        {
            transport.IsConnectedValue = false;
            transport.ConnectAction = () => throw new InvalidOperationException("connect refused");
        });
        client.RegisterFunction(new FunctionDescriptor { Id = "fn.leak", Resource = "r", Operation = "o" }, (ctx, payload) => Task.FromResult("{}"));

        await Assert.ThrowsAsync<InvalidOperationException>(() => client.ConnectAsync());

        created.Should().ContainSingle().Which.DisposeCount.Should().Be(0);
    }

    [Fact]
    public async Task ConnectAndRegister_WhenRegisterCallFails_DisposesNewTransportButKeepsOld()
    {
        var created = new List<MockTransport>();
        var failNext = false;
        using var client = new CroupierClient(BaseConfig(), null, (address, timeoutMs, connectTimeoutMs, logger) =>
        {
            var transport = new MockTransport
            {
                IsConnectedValue = true,
                CallHandler = failNext
                    ? (_, _) => throw new TimeoutException("register timed out")
                    : (_, _) => Task.FromResult(Google.Protobuf.MessageExtensions.ToByteArray(new ProviderConnectResponse { SessionId = "mock-session" })),
            };
            created.Add(transport);
            return transport;
        })!;

        client.RegisterFunction(new FunctionDescriptor { Id = "fn.keep", Resource = "r", Operation = "o" }, (ctx, payload) => Task.FromResult("{}"));

        var connectAndRegister = typeof(CroupierClient).GetMethod(
            "ConnectAndRegisterAsync",
            System.Reflection.BindingFlags.NonPublic | System.Reflection.BindingFlags.Instance);

        // First registration succeeds.
        await (Task)connectAndRegister!.Invoke(client, new object[] { CancellationToken.None })!;
        var firstTransport = created.Single();

        // Second registration fails at the register call.
        failNext = true;
        await Assert.ThrowsAsync<TimeoutException>(
            () => (Task)connectAndRegister!.Invoke(client, new object[] { CancellationToken.None })!);

        var secondTransport = created.Should().HaveCount(2).And.Subject.ElementAt(1);
        secondTransport.DisposeCount.Should().Be(1);
        firstTransport.DisposeCount.Should().Be(0);
        GetPrivateField(client, "_transport").Should().BeSameAs(firstTransport);
    }

    [Fact]
    public async Task ConnectAsync_Success_InstallsInboundHandlerAndSession()
    {
        var (client, created) = CreateClient(BaseConfig());
        client.RegisterFunction(new FunctionDescriptor { Id = "fn.inbound", Resource = "r", Operation = "o" }, (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();

        var transport = created.Single();
        transport.CapturedInboundHandler.Should().NotBeNull();
        client.SessionId.Should().Be("mock-session");
        client.IsConnected.Should().BeTrue();
    }

    [Fact]
    public async Task InvokeAsync_BuildsInvokeRequestWithIdempotencyKeyAndConfigFallbacks()
    {
        var (client, created) = CreateClient(BaseConfig(config =>
        {
            config.Headers["X-Game-ID"] = "header-game";
        }));
        client.RegisterFunction(new FunctionDescriptor { Id = "fn.meta", Resource = "r", Operation = "o" }, (ctx, payload) => Task.FromResult("{}"));
        await client.ConnectAsync();

        var transport = created.Single();
        transport.CallHandler = (msgType, data) =>
        {
            if (msgType == Protocol.MsgInvokeRequest)
            {
                var request = InvokeRequest.Parser.ParseFrom(data!);
                return Task.FromResult(Google.Protobuf.MessageExtensions.ToByteArray(new InvokeResponse
                {
                    Payload = ByteString.CopyFromUtf8($"id:{request.IdempotencyKey};game:{request.Metadata.GetValueOrDefault("X-Game-ID")};env:{request.Metadata.GetValueOrDefault("X-Env")}"),
                }));
            }

            return Task.FromResult(Google.Protobuf.MessageExtensions.ToByteArray(new ProviderHeartbeatResponse()));
        };

        // options with a null GameId must fall back to config.Headers' X-Game-ID.
        var result = await client.InvokeAsync("fn.meta", "{}", new InvokeOptions { IdempotencyKey = "key-9", GameId = null, Env = "env-opt" });

        result.Should().Be("id:key-9;game:header-game;env:env-opt");
        var invokeCall = transport.Calls.Should().ContainSingle(call => call.MsgType == Protocol.MsgInvokeRequest).Subject;
        var requestParsed = InvokeRequest.Parser.ParseFrom(invokeCall.Data!);
        requestParsed.FunctionId.Should().Be("fn.meta");
        requestParsed.IdempotencyKey.Should().Be("key-9");
    }

    [Fact]
    public async Task InvokeAsync_WithoutIdempotencyKey_LeavesFieldEmpty()
    {
        var (client, created) = CreateClient(BaseConfig());
        client.RegisterFunction(new FunctionDescriptor { Id = "fn.nokey", Resource = "r", Operation = "o" }, (ctx, payload) => Task.FromResult("{}"));
        await client.ConnectAsync();

        var transport = created.Single();
        transport.CallHandler = (_, _) => Task.FromResult(Google.Protobuf.MessageExtensions.ToByteArray(new InvokeResponse { Payload = ByteString.CopyFromUtf8("ok") }));

        var result = await client.InvokeAsync("fn.nokey", "{}");
        result.Should().Be("ok");

        var invokeCall = transport.Calls.Single(call => call.MsgType == Protocol.MsgInvokeRequest);
        InvokeRequest.Parser.ParseFrom(invokeCall.Data!).IdempotencyKey.Should().BeEmpty();
    }

    [Fact]
    public async Task HeartbeatLoop_SendsHeartbeatThroughTransport()
    {
        var (client, created) = CreateClient(BaseConfig(config => config.HeartbeatIntervalSeconds = 1));
        client.RegisterFunction(new FunctionDescriptor { Id = "fn.hb", Resource = "r", Operation = "o" }, (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();
        var transport = created.Single();

        var deadline = DateTime.UtcNow.AddSeconds(8);
        while (DateTime.UtcNow < deadline &&
               transport.Calls.Count(call => call.MsgType == Protocol.MsgProviderHeartbeatRequest) < 2)
        {
            await Task.Delay(50);
        }

        transport.Calls.Should().Contain(call => call.MsgType == Protocol.MsgProviderHeartbeatRequest);
        var heartbeat = transport.Calls.First(call => call.MsgType == Protocol.MsgProviderHeartbeatRequest);
        var parsed = ProviderHeartbeatRequest.Parser.ParseFrom(heartbeat.Data!);
        parsed.SessionId.Should().Be("mock-session");
        parsed.ServiceId.Should().Be("factory-service");

        client.Disconnect();
    }

    [Fact]
    public async Task InvokeAsync_WhenTransportReportsDisconnected_Throws()
    {
        var (client, created) = CreateClient(BaseConfig());
        client.RegisterFunction(new FunctionDescriptor { Id = "fn.off", Resource = "r", Operation = "o" }, (ctx, payload) => Task.FromResult("{}"));
        await client.ConnectAsync();

        created.Single().IsConnectedValue = false;

        var action = () => client.InvokeAsync("fn.off", "{}");

        (await action.Should().ThrowAsync<InvalidOperationException>())
            .WithMessage("*Not connected to Agent*");
    }
}
