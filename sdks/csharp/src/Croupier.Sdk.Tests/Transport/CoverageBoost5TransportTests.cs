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

using System.Net;
using System.Net.Sockets;
using System.Reflection;
using System.Text;
using Croupier.Sdk.Transport;
using FluentAssertions;
using Xunit;

namespace Croupier.Sdk.Tests.Transport;

/// <summary>
/// TCPTransport 低覆盖分支补测：同步 Call、入站响应写失败、读循环一般异常、
/// 入站队列饱和、入站限流器取消/释放后的异常吞并。
/// </summary>
public sealed class CoverageBoost5TransportTests
{
    private static async Task WriteFrameAsync(NetworkStream stream, int msgId, int reqId, byte[] body)
    {
        var payload = Protocol.NewMessage(msgId, reqId, body);
        var header = new byte[4];
        var len = payload.Length;
        header[0] = (byte)(len >> 24);
        header[1] = (byte)(len >> 16);
        header[2] = (byte)(len >> 8);
        header[3] = (byte)len;
        await stream.WriteAsync(header);
        await stream.WriteAsync(payload);
        await stream.FlushAsync();
    }

    private static async Task<(int MsgId, int ReqId, byte[] Body)> ReadFrameAsync(NetworkStream stream)
    {
        var header = new byte[4];
        await ReadExactAsync(stream, header);
        var len = (header[0] << 24) | (header[1] << 16) | (header[2] << 8) | header[3];
        var payload = new byte[len];
        await ReadExactAsync(stream, payload);
        var parsed = Protocol.ParseMessage(payload);
        return (parsed.MsgId, parsed.ReqId, parsed.Body);
    }

    private static async Task ReadExactAsync(NetworkStream stream, byte[] buffer)
    {
        var off = 0;
        while (off < buffer.Length)
        {
            var n = await stream.ReadAsync(buffer.AsMemory(off));
            if (n == 0)
            {
                throw new EndOfStreamException();
            }
            off += n;
        }
    }

    private static object GetField(TCPTransport transport, string name) =>
        typeof(TCPTransport).GetField(name, BindingFlags.NonPublic | BindingFlags.Instance)!.GetValue(transport)!;

    private static void SetField(TCPTransport transport, string name, object? value) =>
        typeof(TCPTransport).GetField(name, BindingFlags.NonPublic | BindingFlags.Instance)!.SetValue(transport, value);

    [Fact]
    public void Call_SynchronousRoundTrip_ReturnsResponseBody()
    {
        using var server = new TcpListener(IPAddress.Loopback, 0);
        server.Start();
        var port = ((IPEndPoint)server.LocalEndpoint).Port;

        var serverTask = Task.Run(async () =>
        {
            using var client = await server.AcceptTcpClientAsync();
            using var stream = client.GetStream();
            var (msgId, reqId, _) = await ReadFrameAsync(stream);
            Protocol.IsRequest(msgId).Should().BeTrue();
            await WriteFrameAsync(stream, Protocol.GetResponseMsgId(msgId), reqId,
                Encoding.UTF8.GetBytes("sync-pong"));
        });

        using var transport = new TCPTransport($"127.0.0.1:{port}", timeoutMs: 5000, connectTimeoutMs: 3000);
        transport.Connect();

        var response = transport.Call(Protocol.MsgInvokeRequest, Encoding.UTF8.GetBytes("{}"));

        Encoding.UTF8.GetString(response).Should().Be("sync-pong");
        serverTask.Wait(TimeSpan.FromSeconds(5)).Should().BeTrue();
    }

    [Fact]
    public async Task InboundResponse_WhenStreamDisposedDuringHandler_IsSwallowed()
    {
        using var server = new TcpListener(IPAddress.Loopback, 0);
        server.Start();
        var port = ((IPEndPoint)server.LocalEndpoint).Port;

        var serverTask = Task.Run(async () =>
        {
            using var client = await server.AcceptTcpClientAsync();
            using var stream = client.GetStream();
            await WriteFrameAsync(stream, Protocol.MsgInvokeRequest, 91, Encoding.UTF8.GetBytes("{}"));
            // 等待对端断开（写失败后 transport 保持存活但不再回帧）。
            var buffer = new byte[16];
            await stream.ReadAsync(buffer.AsMemory());
        });

        using var transport = new TCPTransport($"127.0.0.1:{port}", timeoutMs: 5000, connectTimeoutMs: 3000);
        transport.SetInboundRequestHandler((msgId, reqId, body) =>
        {
            // handler 执行期间 dispose 底层流，令响应写失败（HandleInboundRequest 的
            // WriteFrameAsync catch 分支）。
            var streamField = typeof(TCPTransport).GetField("_stream",
                BindingFlags.NonPublic | BindingFlags.Instance)!;
            ((NetworkStream)streamField.GetValue(transport)!).Dispose();
            return Task.FromResult<byte[]>(Encoding.UTF8.GetBytes("late-response"));
        });
        transport.Connect();

        // 不得抛异常；服务器最终看到连接关闭。
        (await Task.WhenAny(serverTask, Task.Delay(TimeSpan.FromSeconds(5))) == serverTask)
            .Should().BeTrue();
    }

    [Fact]
    public async Task ReadLoop_ConnectionReset_DisconnectsWithoutCrash()
    {
        using var server = new TcpListener(IPAddress.Loopback, 0);
        server.Start();
        var port = ((IPEndPoint)server.LocalEndpoint).Port;

        var resetTask = Task.Run(async () =>
        {
            using var client = await server.AcceptTcpClientAsync();
            // Linger(true, 0) + Close 触发 RST，让对端读循环进入一般异常分支。
            client.LingerState = new LingerOption(true, 0);
            client.Close();
        });

        using var transport = new TCPTransport($"127.0.0.1:{port}", timeoutMs: 5000, connectTimeoutMs: 3000);
        transport.Connect();

        var deadline = DateTime.UtcNow + TimeSpan.FromSeconds(5);
        while (transport.IsConnected && DateTime.UtcNow < deadline)
        {
            await Task.Delay(50);
        }
        transport.IsConnected.Should().BeFalse();
        await resetTask;
    }

    [Fact]
    public async Task DispatchInbound_QueueSaturated_RespondsWithEmptyBody()
    {
        using var server = new TcpListener(IPAddress.Loopback, 0);
        server.Start();
        var port = ((IPEndPoint)server.LocalEndpoint).Port;

        var serverTask = Task.Run(async () =>
        {
            using var client = await server.AcceptTcpClientAsync();
            using var stream = client.GetStream();
            await WriteFrameAsync(stream, Protocol.MsgInvokeRequest, 92, Encoding.UTF8.GetBytes("{}"));
            var (msgId, _, body) = await ReadFrameAsync(stream);
            Protocol.GetResponseMsgId(Protocol.MsgInvokeRequest).Should().Be(msgId);
            return body.Length;
        });

        using var transport = new TCPTransport($"127.0.0.1:{port}", timeoutMs: 5000, connectTimeoutMs: 3000);
        transport.SetInboundRequestHandler((msgId, reqId, body) =>
            Task.FromResult<byte[]>(Encoding.UTF8.GetBytes("should-not-be-used")));
        transport.Connect();
        SetField(transport, "_inboundQueued", int.MaxValue - 1);

        var bodyLength = await serverTask.WaitAsync(TimeSpan.FromSeconds(5));
        bodyLength.Should().Be(0);
    }

    [Fact]
    public async Task DispatchInbound_LimiterDisposed_SwallowsErrorWithoutResponse()
    {
        using var server = new TcpListener(IPAddress.Loopback, 0);
        server.Start();
        var port = ((IPEndPoint)server.LocalEndpoint).Port;

        var serverTask = Task.Run(async () =>
        {
            using var client = await server.AcceptTcpClientAsync();
            using var stream = client.GetStream();
            await WriteFrameAsync(stream, Protocol.MsgInvokeRequest, 93, Encoding.UTF8.GetBytes("{}"));
            // 不应收到任何响应帧：等待读超时即视为通过。
            using var timeoutCts = new CancellationTokenSource(TimeSpan.FromMilliseconds(800));
            var buffer = new byte[16];
            try
            {
                await stream.ReadAsync(buffer.AsMemory(), timeoutCts.Token);
                return false;
            }
            catch (OperationCanceledException)
            {
                return true;
            }
        });

        using var transport = new TCPTransport($"127.0.0.1:{port}", timeoutMs: 5000, connectTimeoutMs: 3000);
        transport.SetInboundRequestHandler((msgId, reqId, body) => Task.FromResult<byte[]>(body));
        transport.Connect();
        // 释放限流器（不 cancel 令牌）：排队 lambda 的 WaitAsync 抛 ObjectDisposedException，
        // 走 DispatchInbound 的 catch (Exception) 分支。
        ((SemaphoreSlim)GetField(transport, "_inboundLimiter")).Dispose();

        var noResponse = await serverTask.WaitAsync(TimeSpan.FromSeconds(5));
        noResponse.Should().BeTrue();
    }

    [Fact]
    public async Task DispatchInbound_CanceledWhileWaitingForLimiter_SwallowsCancellation()
    {
        using var server = new TcpListener(IPAddress.Loopback, 0);
        server.Start();
        var port = ((IPEndPoint)server.LocalEndpoint).Port;

        var limiterSlots = Math.Max(2, Environment.ProcessorCount);
        var handlerEntered = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var handlerRelease = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);

        var serverTask = Task.Run(async () =>
        {
            using var client = await server.AcceptTcpClientAsync();
            using var stream = client.GetStream();
            // 占满 limiter 的全部槽位，再多发一个令其排队等待 WaitAsync。
            for (var i = 0; i <= limiterSlots; i++)
            {
                await WriteFrameAsync(stream, Protocol.MsgInvokeRequest, 200 + i,
                    Encoding.UTF8.GetBytes($"{{\"slot\":{i}}}"));
            }
            await Task.Delay(TimeSpan.FromSeconds(3));
        });

        using var transport = new TCPTransport($"127.0.0.1:{port}", timeoutMs: 5000, connectTimeoutMs: 3000);
        var entered = 0;
        transport.SetInboundRequestHandler(async (msgId, reqId, body) =>
        {
            if (Interlocked.Increment(ref entered) == 1)
            {
                handlerEntered.TrySetResult();
            }
            await handlerRelease.Task;
            return body;
        });
        transport.Connect();

        await handlerEntered.Task.WaitAsync(TimeSpan.FromSeconds(5));
        await Task.Delay(300); // 让第 limiterSlots+1 个请求进入 WaitAsync 排队。
        ((CancellationTokenSource)GetField(transport, "_readLoopCts")).Cancel();
        handlerRelease.TrySetResult();

        // 不得崩溃：transport 仍可正常释放，读循环退出。
        var action = () => Task.Run(() => transport.Dispose());
        await action.Should().NotThrowAsync();
        await serverTask;
    }
}
