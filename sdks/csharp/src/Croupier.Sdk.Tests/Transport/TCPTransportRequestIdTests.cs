// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using System.Collections.Concurrent;
using System.Net;
using System.Net.Sockets;
using System.Reflection;
using System.Text;
using Croupier.Sdk.Logging;
using Croupier.Sdk.Transport;
using FluentAssertions;
using Xunit;
using Xunit.Abstractions;

namespace Croupier.Sdk.Tests.Transport;

/// <summary>
/// AUD-073: request-ID allocation plus waiter registration must be atomic
/// against concurrent callers, a newly allocated ID must never replace an
/// in-flight waiter (including near the 0x7FFFFFFF wraparound), and
/// cancellation must leave the pending registry consistent for reuse.
/// </summary>
public class TCPTransportRequestIdTests
{
    private readonly ITestOutputHelper _output;

    public TCPTransportRequestIdTests(ITestOutputHelper output)
    {
        _output = output;
    }

    private sealed class TestLogger : ICroupierLogger
    {
        private readonly ITestOutputHelper _output;

        public TestLogger(ITestOutputHelper output) => _output = output;

        public void LogDebug(string component, string message) =>
            _output.WriteLine($"[DEBUG] {component}: {message}");

        public void LogInfo(string component, string message) =>
            _output.WriteLine($"[INFO] {component}: {message}");

        public void LogWarning(string component, string message) =>
            _output.WriteLine($"[WARN] {component}: {message}");

        public void LogError(string component, string message, Exception? exception = null) =>
            _output.WriteLine($"[ERROR] {component}: {message} {(exception?.Message ?? string.Empty)}");
    }

    private static ConcurrentDictionary<int, TaskCompletionSource<(int, byte[])>> PendingOf(TCPTransport transport) =>
        (ConcurrentDictionary<int, TaskCompletionSource<(int, byte[])>>)(
            typeof(TCPTransport)
                .GetField("_pending", BindingFlags.NonPublic | BindingFlags.Instance)!
                .GetValue(transport))!;

    private static int RequestIdOf(TCPTransport transport) =>
        (int)typeof(TCPTransport)
            .GetField("_requestId", BindingFlags.NonPublic | BindingFlags.Instance)!
            .GetValue(transport)!;

    private static void SetRequestIdOf(TCPTransport transport, int value) =>
        typeof(TCPTransport)
            .GetField("_requestId", BindingFlags.NonPublic | BindingFlags.Instance)!
            .SetValue(transport, value);

    private static async Task ReadExactAsync(NetworkStream stream, byte[] buffer)
    {
        int offset = 0;
        while (offset < buffer.Length)
        {
            var read = await stream.ReadAsync(buffer.AsMemory(offset, buffer.Length - offset));
            if (read == 0)
            {
                throw new EndOfStreamException();
            }

            offset += read;
        }
    }

    private static async Task<(int MsgId, int ReqId, byte[] Body)> ReadFrameAsync(NetworkStream stream)
    {
        var header = new byte[4];
        await ReadExactAsync(stream, header);
        var length = (header[0] << 24) | (header[1] << 16) | (header[2] << 8) | header[3];
        var payload = new byte[length];
        await ReadExactAsync(stream, payload);
        var parsed = Protocol.ParseMessage(payload);
        return (parsed.MsgId, parsed.ReqId, parsed.Body);
    }

    private static async Task WriteFrameAsync(NetworkStream stream, byte[] payload)
    {
        var header = new byte[4];
        header[0] = (byte)(payload.Length >> 24);
        header[1] = (byte)(payload.Length >> 16);
        header[2] = (byte)(payload.Length >> 8);
        header[3] = (byte)payload.Length;
        await stream.WriteAsync(header);
        await stream.WriteAsync(payload);
        await stream.FlushAsync();
    }

    private static TCPTransport ConnectTransport(int port, ITestOutputHelper output, int timeoutMs = 10000)
    {
        var transport = new TCPTransport(
            $"127.0.0.1:{port}", timeoutMs: timeoutMs, connectTimeoutMs: 5000, logger: new TestLogger(output));
        transport.Connect();
        return transport;
    }

    [Fact]
    public async Task CallAsync_ConcurrentSynchronizedStarts_AllocateDistinctRequestIds()
    {
        const int threads = 12;
        const int callsPerThread = 40;
        const int total = threads * callsPerThread;

        using var server = new TcpListener(IPAddress.Loopback, 0);
        server.Start();
        var port = ((IPEndPoint)server.LocalEndpoint).Port;

        var observedIds = new ConcurrentBag<int>();
        var serverTask = Task.Run(async () =>
        {
            using var peer = await server.AcceptTcpClientAsync();
            using var stream = peer.GetStream();
            for (var i = 0; i < total; i++)
            {
                var frame = await ReadFrameAsync(stream);
                observedIds.Add(frame.ReqId);
                await WriteFrameAsync(
                    stream,
                    Protocol.NewMessage(Protocol.MsgInvokeResponse, frame.ReqId, frame.Body));
            }
        });

        using var transport = ConnectTransport(port, _output);

        var barrier = new Barrier(threads);
        var callerTasks = Enumerable.Range(0, threads).Select(threadIndex => Task.Run(async () =>
        {
            barrier
                .SignalAndWait(TimeSpan.FromSeconds(10))
                .Should()
                .BeTrue("all callers must reach the start line together");
            for (var call = 0; call < callsPerThread; call++)
            {
                var marker = Encoding.UTF8.GetBytes($"t{threadIndex}-c{call}");
                var response = await transport.CallAsync(Protocol.MsgInvokeRequest, marker);
                response
                    .Should()
                    .Equal(marker, "each echo response must route back to its own request");
            }
        })).ToArray();

        await Task.WhenAll(callerTasks);
        await serverTask;

        observedIds.Count.Should().Be(total);
        observedIds
            .Distinct()
            .Count()
            .Should()
            .Be(total, "synchronized concurrent callers must never share a request ID");
        PendingOf(transport)
            .Should()
            .BeEmpty("every completed call must have unregistered its waiter");
    }

    [Fact]
    public async Task CallAsync_OutOfOrderResponses_RouteToCorrectWaiter()
    {
        const int callers = 6;

        using var server = new TcpListener(IPAddress.Loopback, 0);
        server.Start();
        var port = ((IPEndPoint)server.LocalEndpoint).Port;

        var serverTask = Task.Run(async () =>
        {
            using var peer = await server.AcceptTcpClientAsync();
            using var stream = peer.GetStream();

            var frames = new List<(int ReqId, string Marker)>(callers);
            for (var i = 0; i < callers; i++)
            {
                var frame = await ReadFrameAsync(stream);
                frames.Add((frame.ReqId, Encoding.UTF8.GetString(frame.Body)));
            }

            // Answer in strict reverse order: whichever caller registered
            // first is served last.
            for (var i = callers - 1; i >= 0; i--)
            {
                var response = Protocol.NewMessage(
                    Protocol.MsgInvokeResponse,
                    frames[i].ReqId,
                    Encoding.UTF8.GetBytes($"resp:{frames[i].Marker}"));
                await WriteFrameAsync(stream, response);
            }
        });

        using var transport = ConnectTransport(port, _output);

        var barrier = new Barrier(callers);
        var callerTasks = Enumerable.Range(0, callers).Select(i => Task.Run(async () =>
        {
            barrier
                .SignalAndWait(TimeSpan.FromSeconds(10))
                .Should()
                .BeTrue("all callers must reach the start line together");
            var response = await transport.CallAsync(
                Protocol.MsgInvokeRequest, Encoding.UTF8.GetBytes($"t{i}"));
            Encoding.UTF8
                .GetString(response)
                .Should()
                .Be($"resp:t{i}", "responses sent out of order must still route by request ID");
        })).ToArray();

        await Task.WhenAll(callerTasks);
        await serverTask;

        PendingOf(transport).Should().BeEmpty();
    }

    [Fact]
    public async Task CallAsync_NearRequestIdWraparound_SkipsIdsWithInFlightWaiters()
    {
        using var server = new TcpListener(IPAddress.Loopback, 0);
        server.Start();
        var port = ((IPEndPoint)server.LocalEndpoint).Port;

        // Hold the peer connection open until the test has finished asserting:
        // when the connection drops, the read loop's teardown clears _pending,
        // which would race with the registry assertions below.
        var keepPeerOpen = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var serverTask = Task.Run(async () =>
        {
            using var peer = await server.AcceptTcpClientAsync();
            using var stream = peer.GetStream();
            var frame = await ReadFrameAsync(stream);
            await WriteFrameAsync(
                stream,
                Protocol.NewMessage(
                    Protocol.MsgInvokeResponse,
                    frame.ReqId,
                    Encoding.UTF8.GetBytes($"resp:{frame.ReqId}")));
            await keepPeerOpen.Task;
        });

        using var transport = ConnectTransport(port, _output);

        try
        {
            // Seed the registry exactly the way an in-flight CallAsync does: two
            // waiters that are still pending after the counter wrapped past the
            // 0x7FFFFFFF boundary.
            var pending = PendingOf(transport);
            var waiter0 = new TaskCompletionSource<(int, byte[])>(TaskCreationOptions.RunContinuationsAsynchronously);
            var waiter1 = new TaskCompletionSource<(int, byte[])>(TaskCreationOptions.RunContinuationsAsynchronously);
            pending.TryAdd(0, waiter0).Should().BeTrue();
            pending.TryAdd(1, waiter1).Should().BeTrue();

            // Park the counter on the last ID before the mask wraps to 0.
            SetRequestIdOf(transport, 0x7FFFFFFF);

            var response = await transport.CallAsync(Protocol.MsgInvokeRequest, Encoding.UTF8.GetBytes("wrap"));

            Encoding.UTF8
                .GetString(response)
                .Should()
                .Be("resp:2", "IDs 0 and 1 still have in-flight waiters, so allocation must skip them");
            RequestIdOf(transport).Should().Be(2, "the counter must rest on the ID that was handed out");

            pending.TryGetValue(0, out var kept0).Should().BeTrue();
            ReferenceEquals(kept0, waiter0)
                .Should()
                .BeTrue("the in-flight waiter for ID 0 must not be replaced or completed");
            kept0!.Task.IsCompleted.Should().BeFalse();
            pending.TryGetValue(1, out var kept1).Should().BeTrue();
            ReferenceEquals(kept1, waiter1)
                .Should()
                .BeTrue("the in-flight waiter for ID 1 must not be replaced or completed");
            kept1!.Task.IsCompleted.Should().BeFalse();
        }
        finally
        {
            keepPeerOpen.TrySetResult();
            await serverTask;
        }
    }

    [Fact]
    public async Task CallAsync_Cancellation_RemovesWaiterAndKeepsRegistryConsistent()
    {
        using var server = new TcpListener(IPAddress.Loopback, 0);
        server.Start();
        var port = ((IPEndPoint)server.LocalEndpoint).Port;

        var firstFrameObserved = new TaskCompletionSource<int>(TaskCreationOptions.RunContinuationsAsynchronously);
        var secondFrameObserved = new TaskCompletionSource<int>(TaskCreationOptions.RunContinuationsAsynchronously);
        var releaseLateResponse = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var serverTask = Task.Run(async () =>
        {
            using var peer = await server.AcceptTcpClientAsync();
            using var stream = peer.GetStream();

            var frame = await ReadFrameAsync(stream);
            firstFrameObserved.TrySetResult(frame.ReqId);

            // Hold the response until the test says so (simulates a slow peer
            // answering a request whose caller already cancelled).
            await releaseLateResponse.Task;
            await WriteFrameAsync(
                stream,
                Protocol.NewMessage(
                    Protocol.MsgInvokeResponse,
                    frame.ReqId,
                    Encoding.UTF8.GetBytes("late")));

            var second = await ReadFrameAsync(stream);
            secondFrameObserved.TrySetResult(second.ReqId);
            await WriteFrameAsync(
                stream,
                Protocol.NewMessage(
                    Protocol.MsgInvokeResponse,
                    second.ReqId,
                    Encoding.UTF8.GetBytes("second-ok")));
        });

        using var transport = ConnectTransport(port, _output);

        using var cts = new CancellationTokenSource();
        var callTask = transport.CallAsync(
            Protocol.MsgInvokeRequest, Encoding.UTF8.GetBytes("cancel-me"), cts.Token);

        var cancelledId = await firstFrameObserved.Task;
        cts.Cancel();
        await Assert.ThrowsAnyAsync<OperationCanceledException>(() => callTask);

        PendingOf(transport)
            .ContainsKey(cancelledId)
            .Should()
            .BeFalse("the cancelled call must unregister its waiter");

        // A late response for the cancelled ID must be ignored silently: the
        // read loop finds no waiter and must not corrupt the registry.
        releaseLateResponse.TrySetResult();

        var secondResponse = await transport.CallAsync(
            Protocol.MsgInvokeRequest, Encoding.UTF8.GetBytes("again"));
        Encoding.UTF8.GetString(secondResponse).Should().Be("second-ok");

        var secondId = await secondFrameObserved.Task;
        secondId
            .Should()
            .Be(cancelledId + 1, "allocation must keep advancing monotonically after a cancellation");

        PendingOf(transport).Should().BeEmpty("the registry must end up clean after both calls settle");
        await serverTask;
    }
}
