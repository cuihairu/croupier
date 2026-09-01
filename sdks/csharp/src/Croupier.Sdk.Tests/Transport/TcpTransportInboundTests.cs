// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using System.Net;
using System.Net.Sockets;
using System.Text;
using Croupier.Sdk.Transport;
using FluentAssertions;
using Xunit;
using Xunit.Abstractions;

namespace Croupier.Sdk.Tests.Transport;

/// <summary>
/// 入站派发与读循环边界路径测试（覆盖 HandleInboundRequest / ReadLoop /
/// DispatchInbound 饱和分支 / HandleInboundRequestAsync）。
/// </summary>
public class TcpTransportInboundTests
{
    private readonly ITestOutputHelper _output;

    public TcpTransportInboundTests(ITestOutputHelper output)
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

    private static async Task<byte[]> ReadFrameBodyAsync(NetworkStream stream)
    {
        var header = new byte[4];
        await ReadExactAsync(stream, header);
        var len = (header[0] << 24) | (header[1] << 16) | (header[2] << 8) | header[3];
        var payload = new byte[len];
        await ReadExactAsync(stream, payload);
        var parsed = Protocol.ParseMessage(payload);
        return parsed.Body;
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

    [Fact]
    public async Task InboundRequest_WithoutHandler_RespondsError()
    {
        using var server = new TcpListener(IPAddress.Loopback, 0);
        server.Start();
        var port = ((IPEndPoint)server.LocalEndpoint).Port;

        var serverTask = Task.Run(async () =>
        {
            using var client = await server.AcceptTcpClientAsync();
            using var stream = client.GetStream();
            await WriteFrameAsync(stream, Protocol.MsgInvokeRequest, 77, Encoding.UTF8.GetBytes("{}"));
            var body = await ReadFrameBodyAsync(stream);
            return Encoding.UTF8.GetString(body);
        });

        using var transport = new TCPTransport($"127.0.0.1:{port}", connectTimeoutMs: 3000);
        transport.Connect();
        // 注意：不注册 InboundRequestHandler —— 走 no-handler 分支。

        var resp = await serverTask.WaitAsync(TimeSpan.FromSeconds(5));
        resp.Should().Contain("no handler registered");
    }

    [Fact]
    public async Task InboundRequest_HandlerThrows_RespondsError()
    {
        using var server = new TcpListener(IPAddress.Loopback, 0);
        server.Start();
        var port = ((IPEndPoint)server.LocalEndpoint).Port;

        var serverTask = Task.Run(async () =>
        {
            using var client = await server.AcceptTcpClientAsync();
            using var stream = client.GetStream();
            await WriteFrameAsync(stream, Protocol.MsgInvokeRequest, 78, Encoding.UTF8.GetBytes("{}"));
            var body = await ReadFrameBodyAsync(stream);
            return Encoding.UTF8.GetString(body);
        });

        using var transport = new TCPTransport($"127.0.0.1:{port}", connectTimeoutMs: 3000);
        transport.SetInboundRequestHandler((msgId, reqId, body) =>
            throw new InvalidOperationException("boom"));
        transport.Connect();

        var resp = await serverTask.WaitAsync(TimeSpan.FromSeconds(5));
        resp.Should().Contain("boom");
    }

    [Fact]
    public async Task ReadLoop_FrameTooLarge_Disconnects()
    {
        using var server = new TcpListener(IPAddress.Loopback, 0);
        server.Start();
        var port = ((IPEndPoint)server.LocalEndpoint).Port;

        var serverTask = Task.Run(async () =>
        {
            using var client = await server.AcceptTcpClientAsync();
            using var stream = client.GetStream();
            // 声明超大帧长度（> MaxFrameBytes 32MB）——读循环必须断开。
            var header = new byte[4];
            var len = 33 * 1024 * 1024 + 1;
            header[0] = (byte)(len >> 24);
            header[1] = (byte)(len >> 16);
            header[2] = (byte)(len >> 8);
            header[3] = (byte)len;
            await stream.WriteAsync(header);
            await stream.FlushAsync();
            // 等对端断开。
            var buf = new byte[16];
            await stream.ReadAsync(buf.AsMemory());
            return 0;
        });

        using var transport = new TCPTransport($"127.0.0.1:{port}", connectTimeoutMs: 3000);
        transport.Connect();
        transport.IsConnected.Should().BeTrue();

        // 读循环看到超大帧后 break 并置 _connected=false（不主动关 socket，
        // Dispose 时才关——这里轮询状态翻转即可）。
        var deadline = DateTime.UtcNow + TimeSpan.FromSeconds(5);
        while (transport.IsConnected && DateTime.UtcNow < deadline)
        {
            await Task.Delay(50);
        }
        transport.IsConnected.Should().BeFalse();
    }

    [Fact]
    public async Task ReadLoop_ZeroLengthFrame_Skipped()
    {
        using var server = new TcpListener(IPAddress.Loopback, 0);
        server.Start();
        var port = ((IPEndPoint)server.LocalEndpoint).Port;

        var responded = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var serverTask = Task.Run(async () =>
        {
            using var client = await server.AcceptTcpClientAsync();
            using var stream = client.GetStream();
            // 零长帧：读循环 continue（跳过），连接保持。
            await stream.WriteAsync(new byte[4]);
            await stream.FlushAsync();
            await Task.Delay(100);
            // 随后正常请求仍被处理。
            await WriteFrameAsync(stream, Protocol.MsgInvokeRequest, 79, Encoding.UTF8.GetBytes("{}"));
            await ReadFrameBodyAsync(stream);
            responded.TrySetResult();
        });

        using var transport = new TCPTransport($"127.0.0.1:{port}", connectTimeoutMs: 3000);
        transport.SetInboundRequestHandler((msgId, reqId, body) => Task.FromResult<byte[]>(body));
        transport.Connect();

        await responded.Task.WaitAsync(TimeSpan.FromSeconds(5));
        await serverTask.WaitAsync(TimeSpan.FromSeconds(5));
    }

    [Fact]
    public async Task ReadLoop_ResponseWithoutPendingRequest_IsIgnored()
    {
        using var server = new TcpListener(IPAddress.Loopback, 0);
        server.Start();
        var port = ((IPEndPoint)server.LocalEndpoint).Port;

        var responded = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var serverTask = Task.Run(async () =>
        {
            using var client = await server.AcceptTcpClientAsync();
            using var stream = client.GetStream();
            // 未匹配的响应帧：TryRemove 失败分支（无 pending 等待者）。
            await WriteFrameAsync(stream, Protocol.MsgInvokeRequest + 1, 4242, new byte[0]);
            await Task.Delay(100);
            await WriteFrameAsync(stream, Protocol.MsgInvokeRequest, 80, Encoding.UTF8.GetBytes("{}"));
            await ReadFrameBodyAsync(stream);
            responded.TrySetResult();
        });

        using var transport = new TCPTransport($"127.0.0.1:{port}", connectTimeoutMs: 3000);
        transport.SetInboundRequestHandler((msgId, reqId, body) => Task.FromResult<byte[]>(body));
        transport.Connect();

        await responded.Task.WaitAsync(TimeSpan.FromSeconds(5));
        await serverTask.WaitAsync(TimeSpan.FromSeconds(5));
    }

    [Fact]
    public async Task InboundRequests_Concurrent_WithinLimiter_AllAnswered()
    {
        using var server = new TcpListener(IPAddress.Loopback, 0);
        server.Start();
        var port = ((IPEndPoint)server.LocalEndpoint).Port;

        const int total = 16;
        var answered = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var serverTask = Task.Run(async () =>
        {
            using var client = await server.AcceptTcpClientAsync();
            using var stream = client.GetStream();
            for (var i = 0; i < total; i++)
            {
                await WriteFrameAsync(stream, Protocol.MsgInvokeRequest, 1000 + i, Encoding.UTF8.GetBytes("{}"));
            }
            for (var i = 0; i < total; i++)
            {
                _ = await ReadFrameBodyAsync(stream);
            }
            answered.TrySetResult();
        });

        using var transport = new TCPTransport($"127.0.0.1:{port}", connectTimeoutMs: 3000);
        transport.SetInboundRequestHandler(async (msgId, reqId, body) =>
        {
            await Task.Delay(20);
            return body;
        });
        transport.Connect();

        await answered.Task.WaitAsync(TimeSpan.FromSeconds(10));
        await serverTask.WaitAsync(TimeSpan.FromSeconds(10));
    }
}
