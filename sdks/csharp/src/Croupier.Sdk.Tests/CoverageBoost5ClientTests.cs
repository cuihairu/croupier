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

using System.Reflection;
using Croupier.Sdk.Models;
using Croupier.Sdk.Tests.MockAgent;
using Croupier.Sdk.Transport;
using Croupier.Sdk.V1;
using FluentAssertions;
using Google.Protobuf;
using Xunit;

namespace Croupier.Sdk.Tests;

/// <summary>
/// CroupierClient 低覆盖分支补测：BuildInvocationMetadata 回退、Disconnect 对
/// heartbeat 任务异常的吞并、入站校验边界、drain 恢复状态机、调用通道处理循环、
/// 心跳前置检查与 manifest 元数据序列化。
/// </summary>
public sealed class CoverageBoost5ClientTests : IDisposable
{
    private readonly MockAgentServer _agent = new();

    public CoverageBoost5ClientTests()
    {
        _agent.Start();
    }

    public void Dispose() => _agent.DisposeAsync().AsTask().GetAwaiter().GetResult();

    private static readonly MethodInfo InboundMethod = typeof(CroupierClient).GetMethod(
        "HandleInboundRequestAsync", BindingFlags.NonPublic | BindingFlags.Instance)!;

    private static async Task<byte[]> DispatchInboundAsync(CroupierClient client, int msgId, byte[] body)
    {
        var task = (Task<byte[]>)InboundMethod.Invoke(client, new object[] { msgId, 1, body })!;
        return await task;
    }

    private static object GetField(CroupierClient client, string name) =>
        typeof(CroupierClient).GetField(name, BindingFlags.NonPublic | BindingFlags.Instance)!.GetValue(client)!;

    private static void SetField(CroupierClient client, string name, object? value) =>
        typeof(CroupierClient).GetField(name, BindingFlags.NonPublic | BindingFlags.Instance)!.SetValue(client, value);

    private static Task InvokePrivate(CroupierClient client, string method, params object[] args)
    {
        var mi = typeof(CroupierClient).GetMethod(method, BindingFlags.NonPublic | BindingFlags.Instance)!;
        return (Task)mi.Invoke(client, args)!;
    }

    private static byte[] InvokePrivateBytes(CroupierClient client, string method)
    {
        var mi = typeof(CroupierClient).GetMethod(method, BindingFlags.NonPublic | BindingFlags.Instance)!;
        return (byte[])mi.Invoke(client, Array.Empty<object>())!;
    }

    private ClientConfig NewConfig(Action<ClientConfig>? customize = null)
    {
        var config = new ClientConfig
        {
            AgentAddr = _agent.Address,
            ServiceId = "boost5-service",
            GameId = "cfg-game",
            Env = "cfg-env",
            AutoReconnect = false,
            HeartbeatIntervalSeconds = 30,
            TimeoutSeconds = 5,
            ConnectTimeoutSeconds = 5,
        };
        customize?.Invoke(config);
        return config;
    }

    private static byte[] InvokeBody(string functionId, string payload) =>
        Google.Protobuf.MessageExtensions.ToByteArray(new InvokeRequest
        {
            FunctionId = functionId,
            Payload = ByteString.CopyFromUtf8(payload),
        });

    #region BuildInvocationMetadata fallback

    [Fact]
    public async Task InvokeAsync_BlankScopeInOptions_FallsBackToConfigScope()
    {
        using var client = new CroupierClient(NewConfig());
        client.RegisterFunction(new FunctionDescriptor { Id = "fn.scope", Version = "1.0.0" },
            (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();

        await client.InvokeAsync("fn.scope", "{}", new InvokeOptions { GameId = "  ", Env = "" });

        var request = _agent.InvokeRequests.Should().ContainSingle().Subject;
        request.Metadata["X-Game-ID"].Should().Be("cfg-game");
        request.Metadata["X-Env"].Should().Be("cfg-env");
    }

    [Fact]
    public async Task InvokeAsync_ConfigHeaderPreventsScopeFallback()
    {
        using var client = new CroupierClient(NewConfig(config =>
        {
            config.Headers["X-Game-ID"] = "header-game";
            config.Headers["X-Env"] = "header-env";
        }));
        client.RegisterFunction(new FunctionDescriptor { Id = "fn.hdr", Version = "1.0.0" },
            (ctx, payload) => Task.FromResult("{}"));

        await client.ConnectAsync();

        await client.InvokeAsync("fn.hdr", "{}", new InvokeOptions { GameId = "", Env = " " });

        var request = _agent.InvokeRequests.Should().ContainSingle().Subject;
        request.Metadata["X-Game-ID"].Should().Be("header-game");
        request.Metadata["X-Env"].Should().Be("header-env");
    }

    #endregion

    #region Disconnect swallows heartbeat task exceptions

    [Fact]
    public void Disconnect_WhenHeartbeatTaskCanceled_SwallowsException()
    {
        using var client = new CroupierClient(NewConfig());
        using var canceled = new CancellationTokenSource();
        canceled.Cancel();

        SetField(client, "_isConnected", true);
        SetField(client, "_processTask", Task.CompletedTask);
        SetField(client, "_heartbeatTask", Task.FromCanceled(canceled.Token));

        var action = () => client.Disconnect();

        action.Should().NotThrow();
        client.IsConnected.Should().BeFalse();
    }

    [Fact]
    public void Disconnect_WhenHeartbeatTaskFaulted_SwallowsAggregateException()
    {
        using var client = new CroupierClient(NewConfig());

        SetField(client, "_isConnected", true);
        SetField(client, "_processTask", Task.CompletedTask);
        SetField(client, "_heartbeatTask", Task.FromException(new InvalidOperationException("heartbeat dead")));

        var action = () => client.Disconnect();

        action.Should().NotThrow();
        client.IsConnected.Should().BeFalse();
    }

    #endregion

    #region ValidateInboundPayload boundaries

    [Fact]
    public async Task Inbound_UnknownFunction_WithValidation_SkipsSchemaValidation()
    {
        using var client = new CroupierClient(NewConfig(config => config.ValidateInputPayloads = true));
        client.RegisterFunction(new FunctionDescriptor
        {
            Id = "fn.known",
            Version = "1.0.0",
            InputSchema = "{\"type\":\"object\",\"required\":[\"id\"]}",
        }, (ctx, payload) => Task.FromResult("ok"));

        var response = await DispatchInboundAsync(client, Protocol.MsgInvokeRequest,
            InvokeBody("fn.never-registered", "{}"));

        InvokeResponse.Parser.ParseFrom(response).Payload.ToStringUtf8()
            .Should().Contain("Function not found");
    }

    [Fact]
    public async Task Inbound_RegisteredWithoutSchema_InvokesHandler()
    {
        using var client = new CroupierClient(NewConfig(config => config.ValidateInputPayloads = true));
        var called = false;
        client.RegisterFunction(new FunctionDescriptor
        {
            Id = "fn.noschema",
            Version = "1.0.0",
            InputSchema = null,
        }, (ctx, payload) =>
        {
            called = true;
            return Task.FromResult("ok");
        });

        var response = await DispatchInboundAsync(client, Protocol.MsgInvokeRequest,
            InvokeBody("fn.noschema", "{}"));

        InvokeResponse.Parser.ParseFrom(response).Payload.ToStringUtf8().Should().Be("ok");
        called.Should().BeTrue();
    }

    [Fact]
    public async Task Inbound_MalformedPayloadJson_ReturnsValidationError()
    {
        using var client = new CroupierClient(NewConfig(config => config.ValidateInputPayloads = true));
        var called = false;
        client.RegisterFunction(new FunctionDescriptor
        {
            Id = "fn.badjson",
            Version = "1.0.0",
            InputSchema = "{\"type\":\"object\"}",
        }, (ctx, payload) =>
        {
            called = true;
            return Task.FromResult("ok");
        });

        var response = await DispatchInboundAsync(client, Protocol.MsgInvokeRequest,
            InvokeBody("fn.badjson", "{not valid json"));

        InvokeResponse.Parser.ParseFrom(response).Payload.ToStringUtf8()
            .Should().Contain("payload must be valid JSON");
        called.Should().BeFalse();
    }

    #endregion

    #region Drain request / DrainAndRecoverAsync

    private static byte[] DrainBody(string sessionId, string reason, int retryAfterMs) =>
        Google.Protobuf.MessageExtensions.ToByteArray(new ProviderDrainRequest
        {
            SessionId = sessionId,
            Reason = reason,
            RetryAfterMs = (uint)retryAfterMs,
        });

    [Fact]
    public async Task Drain_ValidBody_Acknowledges_Idempotent_AndCompletesWithoutReconnect()
    {
        using var client = new CroupierClient(NewConfig(config => config.AutoReconnect = false));
        client.RegisterFunction(new FunctionDescriptor { Id = "fn.drain", Version = "1.0.0" },
            (ctx, payload) => Task.FromResult("{}"));

        var first = await DispatchInboundAsync(client, Protocol.MsgProviderDrainRequest,
            DrainBody("session-1", "deploy", 500));
        client.IsDraining.Should().BeTrue();

        var second = await DispatchInboundAsync(client, Protocol.MsgProviderDrainRequest,
            DrainBody("session-1", "deploy", 500));
        // 幂等：重复 drain 同样返回空 ProviderDrainResponse 确认。
        second.Should().Equal(first);

        var deadline = DateTime.UtcNow.AddSeconds(5);
        while (client.IsDraining && DateTime.UtcNow < deadline)
        {
            await Task.Delay(50);
        }
        client.IsDraining.Should().BeFalse();
        client.IsConnected.Should().BeFalse();
    }

    [Fact]
    public async Task Drain_WithInFlightCall_WaitsForCompletion()
    {
        using var client = new CroupierClient(NewConfig(config => config.AutoReconnect = false));
        var handlerEntered = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var handlerRelease = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        client.RegisterFunction(new FunctionDescriptor { Id = "fn.slow", Version = "1.0.0" },
            async (ctx, payload) =>
            {
                handlerEntered.TrySetResult();
                await handlerRelease.Task;
                return "slow-done";
            });

        var invokeTask = Task.Run(() => DispatchInboundAsync(client, Protocol.MsgInvokeRequest,
            InvokeBody("fn.slow", "{}")));
        await handlerEntered.Task.WaitAsync(TimeSpan.FromSeconds(5));
        client.ActiveInboundCalls.Should().Be(1);

        await DispatchInboundAsync(client, Protocol.MsgProviderDrainRequest,
            DrainBody("session-2", "deploy", 100));
        client.IsDraining.Should().BeTrue();

        // drain 恢复循环在等待在途调用（100ms 轮询）。
        await Task.Delay(250);
        client.IsDraining.Should().BeTrue("in-flight call keeps the drain loop waiting");

        handlerRelease.TrySetResult();
        var invokeResponse = await invokeTask.WaitAsync(TimeSpan.FromSeconds(5));
        InvokeResponse.Parser.ParseFrom(invokeResponse).Payload.ToStringUtf8()
            .Should().Be("slow-done");

        var deadline = DateTime.UtcNow.AddSeconds(5);
        while (client.IsDraining && DateTime.UtcNow < deadline)
        {
            await Task.Delay(50);
        }
        client.IsDraining.Should().BeFalse();
    }

    [Fact]
    public async Task Drain_WithAutoReconnect_ReconnectsAfterDrain()
    {
        using var client = new CroupierClient(NewConfig(config =>
        {
            config.AutoReconnect = true;
            config.ReconnectIntervalSeconds = 1;
        }));
        client.RegisterFunction(new FunctionDescriptor { Id = "fn.re", Version = "1.0.0" },
            (ctx, payload) => Task.FromResult("{}"));

        await DispatchInboundAsync(client, Protocol.MsgProviderDrainRequest,
            DrainBody("session-3", "deploy", 100));

        var deadline = DateTime.UtcNow.AddSeconds(10);
        while ((!client.IsConnected || client.IsDraining) && DateTime.UtcNow < deadline)
        {
            await Task.Delay(50);
        }
        client.IsConnected.Should().BeTrue();
        client.IsDraining.Should().BeFalse();
        client.SessionId.Should().Be(_agent.SessionIdToIssue);
        _agent.ConnectRequests.Should().ContainSingle();
    }

    [Fact]
    public async Task DrainAndRecover_ReconnectFailure_IsSwallowedAndLogged()
    {
        var probe = new System.Net.Sockets.TcpListener(System.Net.IPAddress.Loopback, 0);
        probe.Start();
        var deadPort = ((System.Net.IPEndPoint)probe.LocalEndpoint).Port;
        probe.Stop();

        using var client = new CroupierClient(NewConfig(config =>
        {
            config.AgentAddr = $"127.0.0.1:{deadPort}";
            config.AutoReconnect = true;
            config.ReconnectMaxAttempts = 1;
            config.ReconnectIntervalSeconds = 1;
            config.ConnectTimeoutSeconds = 1;
        }));
        client.RegisterFunction(new FunctionDescriptor { Id = "fn.dead", Version = "1.0.0" },
            (ctx, payload) => Task.FromResult("{}"));

        var action = () => InvokePrivate(client, "DrainAndRecoverAsync", CancellationToken.None);

        await action.Should().NotThrowAsync();
        client.IsDraining.Should().BeFalse();
    }

    [Fact]
    public async Task DrainAndRecover_CanceledDuringReconnectDelay_SwallowsCancellation()
    {
        var probe = new System.Net.Sockets.TcpListener(System.Net.IPAddress.Loopback, 0);
        probe.Start();
        var deadPort = ((System.Net.IPEndPoint)probe.LocalEndpoint).Port;
        probe.Stop();

        using var client = new CroupierClient(NewConfig(config =>
        {
            config.AgentAddr = $"127.0.0.1:{deadPort}";
            config.AutoReconnect = true;
            config.ReconnectMaxAttempts = 0; // 无限重试
            config.ReconnectIntervalSeconds = 5; // 长延迟，等待期间取消
            config.ConnectTimeoutSeconds = 1;
        }));
        client.RegisterFunction(new FunctionDescriptor { Id = "fn.dead2", Version = "1.0.0" },
            (ctx, payload) => Task.FromResult("{}"));

        using var cts = new CancellationTokenSource(TimeSpan.FromMilliseconds(300));

        var action = () => InvokePrivate(client, "DrainAndRecoverAsync", cts.Token);

        await action.Should().NotThrowAsync();
        client.IsDraining.Should().BeFalse();
    }

    #endregion

    #region ProcessCallsAsync channel loop

    private static object NewCallTask(CroupierClient client, string functionId)
    {
        var taskType = typeof(CroupierClient).GetNestedType("FunctionCallTask", BindingFlags.NonPublic)!;
        var instance = Activator.CreateInstance(taskType)!;
        taskType.GetProperty("FunctionId")!.SetValue(instance, functionId);
        taskType.GetProperty("CallId")!.SetValue(instance, Guid.NewGuid().ToString("N"));
        taskType.GetProperty("GameId")!.SetValue(instance, "cfg-game");
        taskType.GetProperty("Env")!.SetValue(instance, "cfg-env");
        taskType.GetProperty("Payload")!.SetValue(instance, "{}");
        taskType.GetProperty("UserId")!.SetValue(instance, "user-1");
        taskType.GetProperty("IdempotencyKey")!.SetValue(instance, "idem-1");
        taskType.GetProperty("CallerServiceId")!.SetValue(instance, "caller-1");
        return instance;
    }

    private static void WriteCallChannel(CroupierClient client, object callTask)
    {
        var channel = GetField(client, "_callChannel");
        var writer = channel.GetType().GetProperty("Writer")!.GetValue(channel)!;
        writer.GetType().GetMethod("TryWrite")!.Invoke(writer, new[] { callTask });
    }

    private static void CompleteCallChannel(CroupierClient client)
    {
        var channel = GetField(client, "_callChannel");
        var writer = channel.GetType().GetProperty("Writer")!.GetValue(channel)!;
        var complete = writer.GetType().GetMethods()
            .First(method => method.Name == "Complete");
        complete.Invoke(writer, new object?[] { null });
    }

    [Theory]
    [InlineData(2)]
    [InlineData(0)]
    public async Task ProcessCallsAsync_ChannelLoop_DispatchesTasksAndStops(int maxConcurrent)
    {
        using var client = new CroupierClient(NewConfig(config => config.MaxConcurrentCalls = maxConcurrent));
        var handlerCount = 0;
        var handlerDone = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        client.RegisterFunction(new FunctionDescriptor { Id = "fn.chan", Version = "1.0.0" },
            (ctx, payload) =>
            {
                if (Interlocked.Increment(ref handlerCount) == 2)
                {
                    handlerDone.TrySetResult();
                }
                return Task.FromResult($"handled:{ctx.CallId is not null}");
            });

        var processTask = InvokePrivate(client, "ProcessCallsAsync", CancellationToken.None);

        WriteCallChannel(client, NewCallTask(client, "fn.chan"));
        WriteCallChannel(client, NewCallTask(client, "fn.chan"));

        await handlerDone.Task.WaitAsync(TimeSpan.FromSeconds(5));

        CompleteCallChannel(client);
        await processTask.WaitAsync(TimeSpan.FromSeconds(5));
        handlerCount.Should().Be(2);
    }

    #endregion

    #region SendHeartbeatAsync guard

    [Fact]
    public async Task SendHeartbeatAsync_WhenNotConnected_Throws()
    {
        using var client = new CroupierClient(NewConfig());

        var action = () => InvokePrivate(client, "SendHeartbeatAsync", CancellationToken.None);

        await action.Should().ThrowAsync<InvalidOperationException>()
            .WithMessage("*Not connected to Agent*");
    }

    #endregion

    #region Provider meta serialization

    [Fact]
    public void BuildProviderMetaData_EmptyProviderSdk_OmitsField4()
    {
        using var client = new CroupierClient(NewConfig(config => config.ProviderSdk = ""));

        var meta = InvokePrivateBytes(client, "BuildProviderMetaData");

        // 字段4 tag（0x22）不应出现；字段1-3 仍存在。
        meta.Should().NotContain((byte)0x22);
        var input = new Google.Protobuf.CodedInputStream(meta);
        var fields = new List<int>();
        while (!input.IsAtEnd)
        {
            fields.Add(Google.Protobuf.WireFormat.GetTagFieldNumber(input.ReadTag()));
            input.SkipLastField();
        }
        fields.Should().BeEquivalentTo(new[] { 1, 2, 3 });
    }

    [Fact]
    public void BuildProviderMetaData_FullyPopulated_WritesAllFields()
    {
        using var client = new CroupierClient(NewConfig(config => config.ProviderSdk = "croupier-csharp-sdk"));

        var meta = InvokePrivateBytes(client, "BuildProviderMetaData");

        var input = new Google.Protobuf.CodedInputStream(meta);
        var fields = new List<int>();
        while (!input.IsAtEnd)
        {
            fields.Add(Google.Protobuf.WireFormat.GetTagFieldNumber(input.ReadTag()));
            input.SkipLastField();
        }
        fields.Should().BeEquivalentTo(new[] { 1, 2, 3, 4 });
    }

    #endregion
}
