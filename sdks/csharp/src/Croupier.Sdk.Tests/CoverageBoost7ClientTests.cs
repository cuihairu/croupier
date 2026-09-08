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
/// 覆盖率补测（第七批）：
/// - DrainAndRecoverAsync 的 30 秒 deadline 超时仍存在在途调用的告警分支；
/// - OpenAPIImporter.RegisterFromOpenAPI 中 RegisterFunction 抛异常时的
///   ContinueOnError 跳过与默认抛出（包装为 InvalidOperationException）分支。
/// </summary>
public sealed class CoverageBoost7ClientTests : IDisposable
{
    private readonly MockAgentServer _agent = new();

    public CoverageBoost7ClientTests()
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

    private ClientConfig NewConfig(Action<ClientConfig>? customize = null)
    {
        var config = new ClientConfig
        {
            AgentAddr = _agent.Address,
            ServiceId = "boost7-service",
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

    private static byte[] DrainBody(string sessionId) =>
        Google.Protobuf.MessageExtensions.ToByteArray(new ProviderDrainRequest
        {
            SessionId = sessionId,
            Reason = "deploy",
            RetryAfterMs = 100,
        });

    [Fact]
    public async Task Drain_InFlightCallExceedsDeadline_CompletesWithTimeoutWarning()
    {
        // 在途调用一直不释放：drain 等待循环在 30 秒 deadline 后退出，
        // 记录 "Drain timeout ... still running" 告警并结束 drain（不重连）。
        using var client = new CroupierClient(NewConfig());
        var handlerEntered = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var handlerRelease = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        client.RegisterFunction(new FunctionDescriptor { Id = "fn.stuck", Version = "1.0.0" },
            async (ctx, payload) =>
            {
                handlerEntered.TrySetResult();
                await handlerRelease.Task;
                return "stuck-done";
            });

        var invokeTask = Task.Run(() => DispatchInboundAsync(client, Protocol.MsgInvokeRequest,
            InvokeBody("fn.stuck", "{}")));
        await handlerEntered.Task.WaitAsync(TimeSpan.FromSeconds(5));
        client.ActiveInboundCalls.Should().Be(1);

        await DispatchInboundAsync(client, Protocol.MsgProviderDrainRequest, DrainBody("session-timeout"));
        client.IsDraining.Should().BeTrue();

        // deadline 为硬编码 30 秒：等待其耗尽后 drain 仍应正常收尾。
        var deadline = DateTime.UtcNow.AddSeconds(40);
        while (client.IsDraining && DateTime.UtcNow < deadline)
        {
            await Task.Delay(200);
        }
        client.IsDraining.Should().BeFalse("deadline exceeded ends the drain loop despite in-flight call");
        client.IsConnected.Should().BeFalse();

        handlerRelease.TrySetResult();
        var invokeResponse = await invokeTask.WaitAsync(TimeSpan.FromSeconds(5));
        InvokeResponse.Parser.ParseFrom(invokeResponse).Payload.ToStringUtf8()
            .Should().Be("stuck-done");
    }

    private const string Spec = """
    {
      "openapi": "3.0.3",
      "paths": {
        "/player/ban": { "post": { "operationId": "player_ban", "x-croupier-function": true } }
      }
    }
    """;

    private static CroupierClient NewDisposedClient(string agentAddr)
    {
        var client = new CroupierClient(new ClientConfig { AgentAddr = agentAddr });
        client.Dispose();
        return client;
    }

    [Fact]
    public void OpenAPIImporter_RegisterFailure_ThrowsWrappedError()
    {
        // disposed client 使 RegisterFunction 抛 ObjectDisposedException，
        // importer 未启用 ContinueOnError 时应包装为 InvalidOperationException 抛出。
        using var client = NewDisposedClient(_agent.Address);
        var action = () => OpenAPIImporter.RegisterFromOpenAPI(
            client, Spec, null, id => (ctx, payload) => Task.FromResult("{}"));

        action.Should().Throw<InvalidOperationException>()
            .WithMessage("register function player_ban failed*")
            .WithInnerException<ObjectDisposedException>();
    }

    [Fact]
    public void OpenAPIImporter_RegisterFailure_ContinueOnErrorSkips()
    {
        // ContinueOnError=true 时注册失败仅跳过，返回已注册列表（此处为空）。
        using var client = NewDisposedClient(_agent.Address);
        var registered = OpenAPIImporter.RegisterFromOpenAPI(
            client, Spec,
            new OpenAPIImportOptions { ContinueOnError = true },
            id => (ctx, payload) => Task.FromResult("{}"));

        registered.Should().BeEmpty();
    }
}
