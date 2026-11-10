// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using System.Diagnostics;
using System.Net;
using System.Net.Sockets;
using System.Text;
using Croupier.Sdk.Transport;
using FluentAssertions;
using Xunit;
using Xunit.Abstractions;

namespace Croupier.Sdk.Tests.Transport;

/// <summary>
/// 双车道派发（CR-FIX-08，控制优先级）：业务洪峰下心跳依然可达。
///
/// 回归背景：业务/控制共队列时，业务 handler 洪峰打满队列 → 心跳被
/// fail-fast 拒绝 → Agent 判定会话死亡 → 过载升级为连接雪崩。双车道后
/// 控制消息（心跳/注册/drain）走独立单 worker 车道，永不拒绝。
/// 与 Go/JS/Java/Python 同一 golden 场景（业务饱和下心跳延迟上界）。
/// </summary>
public class DualLaneTests
{
    private readonly ITestOutputHelper _output;

    public DualLaneTests(ITestOutputHelper output)
    {
        _output = output;
    }

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

    private static async Task<Protocol.ParsedMessage> ReadParsedFrameAsync(NetworkStream stream)
    {
        var header = new byte[4];
        var off = 0;
        while (off < header.Length)
        {
            var n = await stream.ReadAsync(header.AsMemory(off));
            if (n == 0) throw new EndOfStreamException();
            off += n;
        }
        var len = (header[0] << 24) | (header[1] << 16) | (header[2] << 8) | header[3];
        var payload = new byte[len];
        off = 0;
        while (off < payload.Length)
        {
            var n = await stream.ReadAsync(payload.AsMemory(off));
            if (n == 0) throw new EndOfStreamException();
            off += n;
        }
        return Protocol.ParseMessage(payload);
    }

    /// <summary>在 deadline 内读到目标 req_id 的响应（乱序帧跳过）。</summary>
    private static async Task<Protocol.ParsedMessage?> TryReadResponseForAsync(
        NetworkStream stream, int reqId, TimeSpan deadline)
    {
        var sw = Stopwatch.StartNew();
        while (sw.Elapsed < deadline)
        {
            Protocol.ParsedMessage frame;
            try
            {
                frame = await ReadParsedFrameAsync(stream).WaitAsync(deadline - sw.Elapsed);
            }
            catch (TimeoutException)
            {
                return null;
            }
            if (frame.ReqId == reqId) return frame;
        }
        return null;
    }

    [Fact]
    public void IsControlRequest_Classification_MatchesCrossLanguageSet()
    {
        int[] control =
        {
            Protocol.MsgRegisterRequest,
            Protocol.MsgHeartbeatRequest,
            Protocol.MsgRegisterCapabilitiesReq,
            Protocol.MsgRegisterClientRequest,
            Protocol.MsgClientHeartbeatRequest,
            Protocol.MsgProviderConnectRequest,
            Protocol.MsgProviderHeartbeatRequest,
            Protocol.MsgProviderDrainRequest,
        };
        foreach (var msgId in control)
        {
            Protocol.IsControlRequest(msgId).Should().BeTrue($"0x{msgId:X6} is a control message");
        }
        foreach (var msgId in new[]
                 {
                     Protocol.MsgInvokeRequest,
                     Protocol.MsgInvokeResponse,
                     Protocol.MsgStartTaskRequest,
                     Protocol.MsgCancelTaskRequest,
                 })
        {
            Protocol.IsControlRequest(msgId).Should().BeFalse($"0x{msgId:X6} is a business message");
        }
    }

    [Fact]
    public async Task Heartbeat_SurvivesBusinessSaturation()
    {
        using var server = new TcpListener(IPAddress.Loopback, 0);
        server.Start();
        var port = ((IPEndPoint)server.LocalEndpoint).Port;

        var serverReady = new TaskCompletionSource<NetworkStream>(TaskCreationOptions.RunContinuationsAsynchronously);
        var serverCts = new CancellationTokenSource();
        var serverTask = Task.Run(async () =>
        {
            try
            {
                var client = await server.AcceptTcpClientAsync();
                serverReady.SetResult(client.GetStream());
                // 保持流打开直到测试结束；帧收发由主测试驱动。
                await Task.Delay(TimeSpan.FromSeconds(30), serverCts.Token);
            }
            catch (OperationCanceledException)
            {
                // Test finished.
            }
        });

        using var transport = new TCPTransport($"127.0.0.1:{port}", connectTimeoutMs: 3000);
        transport.Connect();

        // 业务 handler：占住业务车道直到放行；控制帧立即回 pong。
        var businessGate = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var businessEntered = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        transport.SetInboundRequestHandler(async (msgId, reqId, body) =>
        {
            if (Protocol.IsControlRequest(msgId))
            {
                return Encoding.UTF8.GetBytes("pong");
            }
            businessEntered.TrySetResult();
            await businessGate.Task;
            return Array.Empty<byte>();
        });

        var stream = await serverReady.Task.WaitAsync(TimeSpan.FromSeconds(5));

        // 打满业务车道：连发 2 倍容量（in-flight + 队列计数必然越界）。
        int capacity = Math.Max(2, Environment.ProcessorCount) * 4;
        for (int i = 0; i < capacity * 2; i++)
        {
            await WriteFrameAsync(stream, Protocol.MsgInvokeRequest, 1000 + i, Array.Empty<byte>());
        }
        var entered = await Task.WhenAny(businessEntered.Task, Task.Delay(TimeSpan.FromSeconds(5)));
        entered.Should().Be(businessEntered.Task, "business handler should have started");

        // 洪峰中的心跳：控制车道必须即时应答（延迟上界 2s）。
        var start = Stopwatch.StartNew();
        await WriteFrameAsync(stream, Protocol.MsgProviderHeartbeatRequest, 900, Array.Empty<byte>());
        var hb = await TryReadResponseForAsync(stream, 900, TimeSpan.FromSeconds(2));
        hb.Should().NotBeNull("heartbeat response must arrive during business saturation");
        var heartbeat = hb!.Value;
        heartbeat.MsgId.Should().Be(Protocol.MsgProviderHeartbeatResponse);
        heartbeat.ReqId.Should().Be(900);
        Encoding.UTF8.GetString(heartbeat.Body).Should().Be("pong");
        _output.WriteLine($"heartbeat answered in {start.ElapsedMilliseconds}ms under saturation");
        start.ElapsedMilliseconds.Should().BeLessThan(2000, "control lane too slow under load");

        // 对照：业务队列满 → 新业务请求被 fail-fast 回空响应。
        await WriteFrameAsync(stream, Protocol.MsgInvokeRequest, 9001, Array.Empty<byte>());
        var ff = await TryReadResponseForAsync(stream, 9001, TimeSpan.FromSeconds(2));
        ff.Should().NotBeNull("saturated business lane must fast-fail with a frame");
        ff!.Value.Body.Should().BeEmpty("fast-fail must carry an empty body");

        // 放行被门住的业务 handler，收尾（serverTask 由 CTS 收编，不 await）。
        businessGate.TrySetResult();
        transport.Dispose();
        serverCts.Cancel();
        try
        {
            await serverTask.WaitAsync(TimeSpan.FromSeconds(5));
        }
        catch (OperationCanceledException)
        {
            // Expected shutdown path.
        }
    }
}
