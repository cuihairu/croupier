// Copyright 2025 Croupier Authors
// Licensed under the Apache License, Version 2.0

using System.Collections.Concurrent;
using System.Net;
using System.Net.Sockets;
using System.Text;
using Croupier.Sdk.Transport;
using Croupier.Sdk.V1;
using Google.Protobuf;

namespace Croupier.Sdk.Tests.MockAgent;

/// <summary>
/// A frame-protocol TCP server that simulates a croupier Agent local SDK gateway
/// for CroupierClient tests. Records every inbound message and responds
/// according to the configured behavior. Agent-initiated (inbound) requests
/// are answered through a waiter table so responses are routed back to the
/// caller instead of the per-connection read loop.
/// </summary>
public sealed class MockAgentServer : IDisposable, IAsyncDisposable
{
    private static byte[] Pb(Google.Protobuf.IMessage message) =>
        Google.Protobuf.MessageExtensions.ToByteArray(message);

    private sealed class ConnectionState
    {
        public required TcpClient Client { get; init; }

        public required NetworkStream Stream { get; init; }

        public object WriteLock { get; } = new();
    }

    private readonly TcpListener _listener;
    private readonly CancellationTokenSource _cts = new();
    private readonly ConcurrentDictionary<int, ConnectionState> _connections = new();
    private readonly ConcurrentDictionary<int, TaskCompletionSource<byte[]>> _inboundWaiters = new();
    private readonly object _recordLock = new();

    private Task? _acceptLoop;
    private int _nextConnectionId;
    private int _nextInboundReqId = 0x700001;

    public MockAgentServer()
    {
        _listener = new TcpListener(IPAddress.Loopback, 0);
    }

    /// <summary>Session id issued in the ProviderConnectResponse.</summary>
    public string SessionIdToIssue { get; set; } = $"session-{Guid.NewGuid():N}";

    /// <summary>When true, the connect response carries an empty session id.</summary>
    public bool IssueEmptySessionId { get; set; }

    /// <summary>When true, the connect response body is invalid protobuf.</summary>
    public bool GarbageConnectResponse { get; set; }

    /// <summary>When true, heartbeat requests are not answered.</summary>
    public bool SilentHeartbeat { get; set; }

    /// <summary>Custom responder for client-initiated invoke requests.</summary>
    public Func<InvokeRequest, byte[]> InvokeResponder { get; set; } =
        request => Pb(new InvokeResponse
        {
            Payload = ByteString.CopyFromUtf8($"echo:{request.Payload.ToStringUtf8()}"),
        });

    public int AcceptedConnections => _acceptedConnections;
    private int _acceptedConnections;

    public List<ProviderConnectRequest> ConnectRequests { get; } = new();

    public List<ProviderHeartbeatRequest> HeartbeatRequests { get; } = new();

    public List<InvokeRequest> InvokeRequests { get; } = new();

    /// <summary>Raw bodies of MsgRegisterCapabilitiesReq frames (control plane).</summary>
    public List<byte[]> CapabilityRequests { get; } = new();

    /// <summary>Responses the client produced for agent-initiated inbound invokes.</summary>
    public List<InvokeResponse> InboundInvokeResponses { get; } = new();

    /// <summary>Bodies the client produced for unsupported inbound message types.</summary>
    public List<byte[]> UnsupportedInboundResponses { get; } = new();

    public string Address => $"127.0.0.1:{Port}";

    public int Port => ((IPEndPoint)_listener.LocalEndpoint).Port;

    public void Start()
    {
        _listener.Start();
        _acceptLoop = Task.Run(AcceptLoopAsync);
    }

    private async Task AcceptLoopAsync()
    {
        while (!_cts.IsCancellationRequested)
        {
            TcpClient client;
            try
            {
                client = await _listener.AcceptTcpClientAsync(_cts.Token).ConfigureAwait(false);
            }
            catch (OperationCanceledException)
            {
                break;
            }
            catch (SocketException)
            {
                break;
            }
            catch (ObjectDisposedException)
            {
                break;
            }

            var id = Interlocked.Increment(ref _nextConnectionId);
            _connections[id] = new ConnectionState { Client = client, Stream = client.GetStream() };
            Interlocked.Increment(ref _acceptedConnections);
            var capturedId = id;
            _ = Task.Run(() => HandleConnectionAsync(capturedId));
        }
    }

    private async Task HandleConnectionAsync(int connectionId)
    {
        var state = _connections[connectionId];
        try
        {
            using var _ = state.Client;
            while (!_cts.IsCancellationRequested)
            {
                var payload = await ReadFrameAsync(state.Stream).ConfigureAwait(false);
                if (payload is null)
                {
                    break;
                }

                var parsed = Protocol.ParseMessage(payload);

                if (Protocol.IsResponse(parsed.MsgId) && _inboundWaiters.TryRemove(parsed.ReqId, out var waiter))
                {
                    waiter.TrySetResult(parsed.Body);
                    continue;
                }

                var handled = await DispatchAsync(parsed.MsgId, parsed.ReqId, parsed.Body, state).ConfigureAwait(false);
                if (!handled)
                {
                    break;
                }
            }
        }
        catch (Exception)
        {
            // Connection teardown is an expected outcome in drop/reconnect tests.
        }
        finally
        {
            _connections.TryRemove(connectionId, out _);
        }
    }

    private async Task<bool> DispatchAsync(int msgId, int reqId, byte[] body, ConnectionState state)
    {
        switch (msgId)
        {
            case Protocol.MsgProviderConnectRequest:
                var connectRequest = ProviderConnectRequest.Parser.ParseFrom(body);
                lock (_recordLock)
                {
                    ConnectRequests.Add(connectRequest);
                }

                byte[] connectBody;
                if (GarbageConnectResponse)
                {
                    connectBody = new byte[] { 0xFF, 0xFF, 0xFF };
                }
                else if (IssueEmptySessionId)
                {
                    connectBody = Pb(new ProviderConnectResponse());
                }
                else
                {
                    connectBody = Pb(new ProviderConnectResponse { SessionId = SessionIdToIssue });
                }

                WriteFrameSync(state, Protocol.NewMessage(Protocol.MsgProviderConnectResponse, reqId, connectBody));
                return true;

            case Protocol.MsgProviderHeartbeatRequest:
                var heartbeat = ProviderHeartbeatRequest.Parser.ParseFrom(body);
                lock (_recordLock)
                {
                    HeartbeatRequests.Add(heartbeat);
                }

                if (!SilentHeartbeat)
                {
                    WriteFrameSync(state, Protocol.NewMessage(Protocol.MsgProviderHeartbeatResponse, reqId, Pb(new ProviderHeartbeatResponse())));
                }
                return true;

            case Protocol.MsgInvokeRequest:
                var invokeRequest = InvokeRequest.Parser.ParseFrom(body);
                lock (_recordLock)
                {
                    InvokeRequests.Add(invokeRequest);
                }

                WriteFrameSync(state, Protocol.NewMessage(Protocol.MsgInvokeResponse, reqId, InvokeResponder(invokeRequest)));
                return true;

            case Protocol.MsgRegisterCapabilitiesReq:
                lock (_recordLock)
                {
                    CapabilityRequests.Add(body);
                }

                WriteFrameSync(state, Protocol.NewMessage(Protocol.MsgRegisterCapabilitiesResp, reqId, Array.Empty<byte>()));
                return true;

            default:
                return true;
        }
    }

    /// <summary>
    /// Send an agent-initiated InvokeRequest on the most recently accepted
    /// connection and return the client's InvokeResponse.
    /// </summary>
    public async Task<InvokeResponse> SendInboundInvokeAsync(InvokeRequest request, TimeSpan? timeout = null)
    {
        var body = await SendInboundRequestAsync(Protocol.MsgInvokeRequest, Pb(request), timeout).ConfigureAwait(false);
        var response = InvokeResponse.Parser.ParseFrom(body);
        lock (_recordLock)
        {
            InboundInvokeResponses.Add(response);
        }
        return response;
    }

    /// <summary>
    /// Send an unsupported agent-initiated request and return the raw response body.
    /// </summary>
    public async Task<byte[]> SendUnsupportedInboundAsync(int msgId, TimeSpan? timeout = null) =>
        await SendInboundRequestAsync(msgId, Encoding.UTF8.GetBytes("ping"), timeout).ConfigureAwait(false);

    private async Task<byte[]> SendInboundRequestAsync(int msgId, byte[] body, TimeSpan? timeout)
    {
        var state = _connections.OrderByDescending(pair => pair.Key).Select(pair => pair.Value).FirstOrDefault()
            ?? throw new InvalidOperationException("No active agent connection");

        var reqId = Interlocked.Increment(ref _nextInboundReqId) | 0x40000000;
        var waiter = new TaskCompletionSource<byte[]>(TaskCreationOptions.RunContinuationsAsynchronously);
        _inboundWaiters[reqId] = waiter;

        try
        {
            WriteFrameSync(state, Protocol.NewMessage(msgId, reqId, body));
            using var timeoutCts = new CancellationTokenSource(timeout ?? TimeSpan.FromSeconds(10));
            using var registration = timeoutCts.Token.Register(() => waiter.TrySetException(new TimeoutException("Client did not answer the inbound request")));
            return await waiter.Task.ConfigureAwait(false);
        }
        finally
        {
            _inboundWaiters.TryRemove(reqId, out _);
        }
    }

    /// <summary>Close every active client connection (simulates an agent drop).</summary>
    public void DropConnections()
    {
        foreach (var connection in _connections.Values)
        {
            try
            {
                connection.Client.Close();
            }
            catch (Exception)
            {
                // Already closed.
            }
        }

        _connections.Clear();
    }

    public async Task WaitForConnectionsAsync(int count, TimeSpan? timeout = null)
    {
        var deadline = DateTime.UtcNow + (timeout ?? TimeSpan.FromSeconds(10));
        while (DateTime.UtcNow < deadline)
        {
            if (AcceptedConnections >= count)
            {
                return;
            }

            await Task.Delay(50).ConfigureAwait(false);
        }

        throw new TimeoutException($"Expected {count} connections, saw {AcceptedConnections}");
    }

    public async Task WaitForHeartbeatsAsync(int count, TimeSpan? timeout = null)
    {
        var deadline = DateTime.UtcNow + (timeout ?? TimeSpan.FromSeconds(10));
        while (DateTime.UtcNow < deadline)
        {
            if (HeartbeatRequests.Count >= count)
            {
                return;
            }

            await Task.Delay(50).ConfigureAwait(false);
        }

        throw new TimeoutException($"Expected {count} heartbeats, saw {HeartbeatRequests.Count}");
    }

    private static async Task<byte[]?> ReadFrameAsync(NetworkStream stream)
    {
        var header = new byte[4];
        if (!await ReadExactAsync(stream, header).ConfigureAwait(false))
        {
            return null;
        }

        var length = (header[0] << 24) | (header[1] << 16) | (header[2] << 8) | header[3];
        var payload = new byte[length];
        if (!await ReadExactAsync(stream, payload).ConfigureAwait(false))
        {
            return null;
        }

        return payload;
    }

    private static async Task<bool> ReadExactAsync(NetworkStream stream, byte[] buffer)
    {
        var offset = 0;
        while (offset < buffer.Length)
        {
            int read;
            try
            {
                read = await stream.ReadAsync(buffer.AsMemory(offset, buffer.Length - offset)).ConfigureAwait(false);
            }
            catch (Exception)
            {
                return false;
            }

            if (read == 0)
            {
                return false;
            }

            offset += read;
        }

        return true;
    }

    private static void WriteFrameSync(ConnectionState state, byte[] payload)
    {
        var header = new byte[4];
        header[0] = (byte)(payload.Length >> 24);
        header[1] = (byte)(payload.Length >> 16);
        header[2] = (byte)(payload.Length >> 8);
        header[3] = (byte)(payload.Length & 0xFF);
        lock (state.WriteLock)
        {
            state.Stream.Write(header);
            state.Stream.Write(payload);
            state.Stream.Flush();
        }
    }

    public void Dispose() => DisposeAsync().AsTask().GetAwaiter().GetResult();

    public async ValueTask DisposeAsync()
    {
        _cts.Cancel();
        DropConnections();
        _listener.Stop();
        if (_acceptLoop is not null)
        {
            try
            {
                await _acceptLoop.ConfigureAwait(ConfigureAwaitOptions.SuppressThrowing);
            }
            catch (Exception)
            {
                // Ignore accept loop teardown errors.
            }
        }

        _cts.Dispose();
    }
}
