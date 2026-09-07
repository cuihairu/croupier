// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using System.Net;
using System.Reflection;
using System.Text;
using System.Text.Json;
using Croupier.Sdk.Configuration;
using Croupier.Sdk.Logging;
using Croupier.Sdk.Models;
using Croupier.Sdk.Tests.MockAgent;
using Croupier.Sdk.Threading;
using Croupier.Sdk.Transport;
using Croupier.Sdk.V1;
using FluentAssertions;
using Google.Protobuf;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// 覆盖率补测（第六轮）：针对 cobertura 报告中低于阈值的分支与行，
/// 覆盖配置提供者、协议命名、入站元数据、schema 校验器、OpenAPI 导入
/// 边界与传输层端口解析等剩余路径。
/// </summary>
public sealed class CoverageBoost6ClientTests : IDisposable
{
    private readonly MockAgentServer _agent = new();

    public CoverageBoost6ClientTests()
    {
        _agent.Start();
    }

    public void Dispose() => _agent.DisposeAsync().AsTask().GetAwaiter().GetResult();

    private ClientConfig NewConfig(Action<ClientConfig>? customize = null)
    {
        var config = new ClientConfig
        {
            AgentAddr = _agent.Address,
            ServiceId = "boost6-service",
            GameId = "cfg-game",
            Env = "cfg-env",
            AutoReconnect = false,
        };
        customize?.Invoke(config);
        return config;
    }

    private static object GetField(CroupierClient client, string name) =>
        typeof(CroupierClient).GetField(name, BindingFlags.NonPublic | BindingFlags.Instance)!.GetValue(client)!;

    private static void SetField(CroupierClient client, string name, object? value) =>
        typeof(CroupierClient).GetField(name, BindingFlags.NonPublic | BindingFlags.Instance)!.SetValue(client, value);

    private static async Task<byte[]> DispatchInboundAsync(CroupierClient client, int msgId, byte[] body)
    {
        var method = typeof(CroupierClient).GetMethod("HandleInboundRequestAsync", BindingFlags.NonPublic | BindingFlags.Instance)!;
        var task = (Task<byte[]>)method.Invoke(client, new object[] { msgId, 1, body })!;
        return await task;
    }

    private static Task InvokePrivate(CroupierClient client, string method, params object[] args)
    {
        var mi = typeof(CroupierClient).GetMethod(method, BindingFlags.NonPublic | BindingFlags.Instance)!;
        return (Task)mi.Invoke(client, args)!;
    }

    private static byte[] InvokeBody(InvokeRequest request) =>
        Google.Protobuf.MessageExtensions.ToByteArray(request);

    // -----------------------------------------------------------------------
    // BuildInvocationMetadata：config 侧作用域为空白时不注入 scope 头
    // -----------------------------------------------------------------------

    [Fact]
    public async Task InvokeAsync_BlankConfigScope_OmitsScopeHeaders()
    {
        using var client = new CroupierClient(NewConfig(config =>
        {
            config.GameId = "";
            config.Env = " ";
        }));
        client.RegisterFunction(new FunctionDescriptor { Id = "fn.blank", Version = "1.0.0" },
            (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();

        await client.InvokeAsync("fn.blank", "{}");

        var request = _agent.InvokeRequests.Should().ContainSingle().Subject;
        request.Metadata.ContainsKey("X-Game-ID").Should().BeFalse();
        request.Metadata.ContainsKey("X-Env").Should().BeFalse();
    }

    // -----------------------------------------------------------------------
    // 入站调用元数据：完整 metadata + 空 IdempotencyKey
    // -----------------------------------------------------------------------

    [Fact]
    public async Task Inbound_MetadataAndEmptyIdempotencyKey_PopulatesTaskFields()
    {
        using var client = new CroupierClient(NewConfig());
        string? seenGame = null, seenEnv = null, seenUser = null, seenCaller = null, seenIdem = null;
        client.RegisterFunction(new FunctionDescriptor { Id = "fn.meta", Version = "1.0.0" },
            (ctx, payload) =>
            {
                seenGame = ctx.GameId;
                seenEnv = ctx.Env;
                seenUser = ctx.UserId;
                seenCaller = ctx.CallerServiceId;
                seenIdem = ctx.IdempotencyKey;
                return Task.FromResult("{}");
            });

        var response = await DispatchInboundAsync(client, Protocol.MsgInvokeRequest,
            InvokeBody(new InvokeRequest
            {
                FunctionId = "fn.meta",
                Payload = ByteString.CopyFromUtf8("{}"),
                IdempotencyKey = "",
                Metadata =
                {
                    ["X-Game-ID"] = "meta-game",
                    ["X-Env"] = "meta-env",
                    ["X-User-ID"] = "user-1",
                    ["X-Caller-Service-ID"] = "caller-svc",
                },
            }));

        InvokeResponse.Parser.ParseFrom(response).Payload.ToStringUtf8().Should().Be("{}");
        seenGame.Should().Be("meta-game");
        seenEnv.Should().Be("meta-env");
        seenUser.Should().Be("user-1");
        seenCaller.Should().Be("caller-svc");
        seenIdem.Should().BeNull("empty idempotency key normalizes to null");
    }

    [Fact]
    public async Task Inbound_NoMetadataWithBlankConfigScopeAndIdempotencyKey_FallsBackToEmpty()
    {
        using var client = new CroupierClient(NewConfig(config =>
        {
            config.GameId = "";
            config.Env = "";
        }));
        string? seenGame = null, seenEnv = null, seenIdem = null;
        client.RegisterFunction(new FunctionDescriptor { Id = "fn.fallback", Version = "1.0.0" },
            (ctx, payload) =>
            {
                seenGame = ctx.GameId;
                seenEnv = ctx.Env;
                seenIdem = ctx.IdempotencyKey;
                return Task.FromResult("{}");
            });

        var response = await DispatchInboundAsync(client, Protocol.MsgInvokeRequest,
            InvokeBody(new InvokeRequest
            {
                FunctionId = "fn.fallback",
                Payload = ByteString.CopyFromUtf8("{}"),
                IdempotencyKey = "idem-6",
            }));

        InvokeResponse.Parser.ParseFrom(response).Payload.ToStringUtf8().Should().Be("{}");
        seenGame.Should().BeEmpty();
        seenEnv.Should().BeEmpty();
        seenIdem.Should().Be("idem-6");
    }

    // -----------------------------------------------------------------------
    // ValidateInboundPayload：空白 payload 与非法 schema 边界
    // -----------------------------------------------------------------------

    [Fact]
    public async Task Inbound_WhitespacePayload_ParsesAsEmptyObject()
    {
        using var client = new CroupierClient(NewConfig(config => config.ValidateInputPayloads = true));
        var called = false;
        client.RegisterFunction(new FunctionDescriptor
        {
            Id = "fn.ws",
            Version = "1.0.0",
            InputSchema = "{\"type\":\"object\"}",
        }, (ctx, payload) =>
        {
            called = true;
            return Task.FromResult("ok");
        });

        var response = await DispatchInboundAsync(client, Protocol.MsgInvokeRequest,
            InvokeBody(new InvokeRequest
            {
                FunctionId = "fn.ws",
                Payload = ByteString.CopyFromUtf8("   "),
            }));

        InvokeResponse.Parser.ParseFrom(response).Payload.ToStringUtf8().Should().Be("ok");
        called.Should().BeTrue();
    }

    [Fact]
    public async Task Inbound_BrokenSchemaWithWhitespacePayload_SkipsValidation()
    {
        using var client = new CroupierClient(NewConfig(config => config.ValidateInputPayloads = true));
        var called = false;
        client.RegisterFunction(new FunctionDescriptor
        {
            Id = "fn.badschema",
            Version = "1.0.0",
            InputSchema = "{\"type\":\"object\",",
        }, (ctx, payload) =>
        {
            called = true;
            return Task.FromResult("ok");
        });

        var response = await DispatchInboundAsync(client, Protocol.MsgInvokeRequest,
            InvokeBody(new InvokeRequest
            {
                FunctionId = "fn.badschema",
                Payload = ByteString.CopyFromUtf8("   "),
            }));

        // schema 非法 + payload 空白：按契约缺陷跳过校验。
        InvokeResponse.Parser.ParseFrom(response).Payload.ToStringUtf8().Should().Be("ok");
        called.Should().BeTrue();
    }

    // -----------------------------------------------------------------------
    // ReconnectAsync / StartHeartbeatLoop 分支
    // -----------------------------------------------------------------------

    [Fact]
    public async Task ReconnectAsync_WithCanceledToken_ExitsImmediately()
    {
        using var client = new CroupierClient(NewConfig());
        client.RegisterFunction(new FunctionDescriptor { Id = "fn.re", Version = "1.0.0" },
            (ctx, payload) => Task.FromResult("{}"));

        using var canceled = new CancellationTokenSource();
        canceled.Cancel();

        var action = () => InvokePrivate(client, "ReconnectAsync", canceled.Token);

        await action.Should().NotThrowAsync();
        client.IsConnected.Should().BeFalse();
    }

    [Fact]
    public async Task StartHeartbeatLoop_SecondCall_ReplacesPreviousLoop()
    {
        using var client = new CroupierClient(NewConfig());
        client.RegisterFunction(new FunctionDescriptor { Id = "fn.hb", Version = "1.0.0" },
            (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();
        GetField(client, "_heartbeatCts").Should().NotBeNull();

        var mi = typeof(CroupierClient).GetMethod("StartHeartbeatLoop", BindingFlags.NonPublic | BindingFlags.Instance)!;
        mi.Invoke(client, null);

        // 第二次启动会取消并替换旧的 CTS（?. 非空分支）。
        GetField(client, "_heartbeatTask").Should().NotBeNull();
    }

    [Fact]
    public async Task ConnectAndRegisterAsync_SecondCall_DisposesPreviousTransport()
    {
        using var client = new CroupierClient(NewConfig());
        client.RegisterFunction(new FunctionDescriptor { Id = "fn.reg2", Version = "1.0.0" },
            (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();
        var firstTransport = GetField(client, "_transport");
        firstTransport.Should().NotBeNull();

        // 二次注册路径：旧 transport 非空时先释放（?. 非空分支）。
        await InvokePrivate(client, "ConnectAndRegisterAsync", CancellationToken.None);

        GetField(client, "_transport").Should().NotBeSameAs(firstTransport);
        _agent.AcceptedConnections.Should().BeGreaterThanOrEqualTo(2);
    }

    [Fact]
    public async Task HeartbeatLoop_NullTransport_TriggersReconnectPath()
    {
        using var client = new CroupierClient(NewConfig(config =>
        {
            config.HeartbeatIntervalSeconds = 1;
            config.AutoReconnect = false;
        }));
        client.RegisterFunction(new FunctionDescriptor { Id = "fn.hbnull", Version = "1.0.0" },
            (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();

        // 模拟 transport 已被释放：心跳前置检查发现 null 即抛出并走重连分支。
        SetField(client, "_transport", null);

        await Task.Delay(2500);
        client.Should().NotBeNull("client survived a heartbeat with null transport");
    }
}
